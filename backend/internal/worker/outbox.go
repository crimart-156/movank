package worker

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/movank/sales-api/internal/cache"
	"github.com/movank/sales-api/internal/domain"
	"github.com/movank/sales-api/internal/repository"
)

// Hub distribuye eventos a todos los clientes SSE conectados.
// Un solo worker lee el outbox y publica en el hub — los N clientes
// SSE reciben el mismo evento sin que cada uno haga su propio query.
type Hub struct {
	mu      sync.RWMutex
	clients map[uuid.UUID][]chan *domain.DashboardAggregate
}

func NewHub() *Hub {
	return &Hub{
		clients: make(map[uuid.UUID][]chan *domain.DashboardAggregate),
	}
}

// Subscribe registra un canal SSE para un merchant.
// Devuelve el canal y una función de cleanup.
func (h *Hub) Subscribe(merchantID uuid.UUID) (<-chan *domain.DashboardAggregate, func()) {
	ch := make(chan *domain.DashboardAggregate, 4)
	h.mu.Lock()
	h.clients[merchantID] = append(h.clients[merchantID], ch)
	h.mu.Unlock()

	unsub := func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		chans := h.clients[merchantID]
		for i, c := range chans {
			if c == ch {
				h.clients[merchantID] = append(chans[:i], chans[i+1:]...)
				close(ch)
				break
			}
		}
	}
	return ch, unsub
}

// Publish envía el agregado a todos los clientes SSE del merchant.
func (h *Hub) Publish(merchantID uuid.UUID, agg *domain.DashboardAggregate) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, ch := range h.clients[merchantID] {
		select {
		case ch <- agg:
		default:
			// cliente lento — no bloqueamos el worker
		}
	}
}

// OutboxWorker consume la tabla outbox vía LISTEN/NOTIFY de Postgres.
// Corre como goroutine dentro del mismo proceso Go — no es un segundo servicio.
type OutboxWorker struct {
	db       *pgxpool.Pool
	cache    *cache.Cache
	saleRepo *repository.SaleRepository
	hub      *Hub
}

func NewOutboxWorker(db *pgxpool.Pool, c *cache.Cache, saleRepo *repository.SaleRepository, hub *Hub) *OutboxWorker {
	return &OutboxWorker{db: db, cache: c, saleRepo: saleRepo, hub: hub}
}

// Run inicia el worker. Se llama con go worker.Run(ctx).
func (w *OutboxWorker) Run(ctx context.Context) {
	// Adquirimos una conexión dedicada para LISTEN (no usar el pool)
	conn, err := w.db.Acquire(ctx)
	if err != nil {
		log.Printf("[outbox] failed to acquire connection: %v", err)
		return
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, "LISTEN outbox_event"); err != nil {
		log.Printf("[outbox] failed to LISTEN: %v", err)
		return
	}

	log.Println("[outbox] worker started, listening for outbox_event notifications")

	for {
		// WaitForNotification bloquea hasta que Postgres dispara NOTIFY
		notification, err := conn.Conn().WaitForNotification(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return // contexto cancelado — shutdown limpio
			}
			log.Printf("[outbox] notification error: %v — retrying in 2s", err)
			time.Sleep(2 * time.Second)
			continue
		}

		// El payload del NOTIFY es el UUID de la fila outbox
		outboxID, err := uuid.Parse(notification.Payload)
		if err != nil {
			log.Printf("[outbox] invalid UUID in notification: %s", notification.Payload)
			continue
		}

		w.processOutboxRow(ctx, outboxID)
	}
}

func (w *OutboxWorker) processOutboxRow(ctx context.Context, outboxID uuid.UUID) {
	// UPDATE atómico: solo procesamos si status='pending'
	// Esto hace al worker idempotente — si crashea entre leer y marcar,
	// el segundo intento no vuelve a procesar la misma fila.
	var merchantID uuid.UUID
	var payload []byte
	err := w.db.QueryRow(ctx,
		`UPDATE outbox SET status = 'published'
		 WHERE id = $1 AND status = 'pending'
		 RETURNING merchant_id, payload`,
		outboxID,
	).Scan(&merchantID, &payload)
	if err != nil {
		// Fila ya procesada o no existe — idempotente, no es error
		return
	}

	// Reconstruir el agregado desde Postgres y actualizar cache
	agg, err := w.saleRepo.GetDashboardFromDB(ctx, merchantID)
	if err != nil {
		log.Printf("[outbox] failed to build dashboard for %s: %v", merchantID, err)
		return
	}

	if err := w.cache.SetDashboard(ctx, agg); err != nil {
		log.Printf("[outbox] failed to update cache for %s: %v", merchantID, err)
		// No es fatal — el endpoint reconstruye desde Postgres si el cache falla
	}

	// Publicar a todos los clientes SSE del merchant
	w.hub.Publish(merchantID, agg)

	log.Printf("[outbox] processed event for merchant %s — total_sales=%d revenue=%.2f",
		merchantID, agg.TotalSales, agg.TotalRevenue)

	_ = payload // disponible para logging o auditoría futura
	_ = json.Unmarshal
}
