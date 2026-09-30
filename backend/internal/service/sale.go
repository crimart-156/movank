package service

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/movank/sales-api/internal/domain"
	"github.com/movank/sales-api/internal/repository"
)

type SaleService struct {
	repo        *repository.SaleRepository
	productRepo *repository.ProductRepository
}

func NewSaleService(repo *repository.SaleRepository, productRepo *repository.ProductRepository) *SaleService {
	return &SaleService{repo: repo, productRepo: productRepo}
}

// Create valida y crea una nueva venta calculando el total desde los precios reales.
func (s *SaleService) Create(ctx context.Context, merchantID uuid.UUID, req domain.CreateSaleRequest) (*domain.Sale, error) {
	if len(req.Items) == 0 {
		return nil, domain.ErrInvalidInput("at least one item is required")
	}

	sale := &domain.Sale{
		MerchantID: merchantID,
		Status:     domain.StatusPending,
	}

	var total float64
	for _, item := range req.Items {
		if item.Quantity <= 0 {
			return nil, domain.ErrInvalidInput("quantity must be greater than 0")
		}
		// Obtener el precio real del producto
		product, err := s.productRepo.GetByID(ctx, item.ProductID)
		if err != nil || product == nil {
			return nil, domain.ErrNotFound("product")
		}
		total += product.Price * float64(item.Quantity)
		sale.Items = append(sale.Items, domain.SaleItem{
			ProductID: item.ProductID,
			Quantity:  item.Quantity,
			UnitPrice: product.Price,
		})
	}

	sale.Total = total

	if err := s.repo.Create(ctx, sale); err != nil {
		return nil, err
	}

	return sale, nil
}

// GetByID devuelve una venta verificando que pertenece al merchant.
func (s *SaleService) GetByID(ctx context.Context, saleID uuid.UUID, merchantID uuid.UUID) (*domain.Sale, error) {
	sale, err := s.repo.GetByID(ctx, saleID, merchantID)
	if err != nil {
		return nil, err
	}
	if sale == nil {
		return nil, domain.ErrNotFound("sale")
	}
	return sale, nil
}

// Pay procesa el pago de una venta con idempotencia y patrón outbox.
//
// Decisiones de diseño:
// - TIMEOUT → UNKNOWN (nunca DECLINED). Son semánticamente distintos:
//   DECLINED = el proveedor rechazó el cobro (definitivo).
//   UNKNOWN  = no sabemos qué pasó (requiere reconciliación posterior).
// - El outbox se escribe en la misma transacción que el pago.
//   Si el commit falla, ambos se revierten — consistencia garantizada.
// - La idempotency_key evita cobros duplicados si el cliente reintenta.
func (s *SaleService) Pay(ctx context.Context, saleID uuid.UUID, merchantID uuid.UUID, idempotencyKey string, req domain.PaySaleRequest) (*domain.Sale, error) {

	// Idempotencia: si ya existe un pago con este key, devolvemos la venta sin re-procesar
	existing, err := s.repo.FindPaymentByIdempotencyKey(ctx, idempotencyKey)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		// Ya fue procesado — retornamos el estado actual sin duplicar el cobro
		return s.repo.GetByID(ctx, saleID, merchantID)
	}

	// Iniciar transacción — todo lo que sigue es atómico
	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	// Bloquear la fila para evitar condición de carrera con pagos concurrentes
	currentStatus, err := s.repo.GetSaleStatusForUpdate(ctx, tx, saleID, merchantID)
	if err == pgx.ErrNoRows {
		return nil, domain.ErrNotFound("sale")
	}
	if err != nil {
		return nil, err
	}

	// Validar que la venta está en estado procesable
	if currentStatus != domain.StatusPending {
		return nil, domain.ErrConflict("sale already processed")
	}

	// Resolver el estado final según el escenario
	// Esta lógica vive en el service, no en el repository
	newStatus := s.resolvePaymentStatus(req.Scenario)

	// Actualizar el estado de la venta
	sale, err := s.repo.UpdateSaleStatus(ctx, tx, saleID, merchantID, newStatus)
	if err != nil {
		return nil, err
	}

	// Registrar el pago con idempotency_key
	if err := s.repo.InsertPayment(ctx, tx, saleID, idempotencyKey, req.Method, newStatus, req.Scenario); err != nil {
		return nil, err
	}

	// Escribir en outbox en la MISMA transacción solo si fue aprobado
	// El worker lo consumirá vía LISTEN/NOTIFY para actualizar el dashboard
	if newStatus == domain.StatusApproved {
		if err := s.insertOutboxEvent(ctx, tx, saleID, merchantID, sale.Total); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return sale, nil
}

// resolvePaymentStatus convierte el escenario en un estado de pago.
// TIMEOUT nunca es DECLINED — es UNKNOWN hasta que se reconcilie.
func (s *SaleService) resolvePaymentStatus(scenario domain.PaymentScenario) domain.PaymentStatus {
	switch scenario {
	case domain.ScenarioApproved:
		return domain.StatusApproved
	case domain.ScenarioDeclined:
		return domain.StatusDeclined
	case domain.ScenarioTimeout:
		// Timeout = no sabemos si se cobró o no.
		// Queda como UNKNOWN para reconciliación posterior.
		// Tratarlo como DECLINED sería un error: podría haberse cobrado.
		return domain.StatusUnknown
	default:
		return domain.StatusUnknown
	}
}

// insertOutboxEvent escribe el evento en la tabla outbox dentro de la transacción activa.
func (s *SaleService) insertOutboxEvent(ctx context.Context, tx pgx.Tx, saleID uuid.UUID, merchantID uuid.UUID, total float64) error {
	payload, err := json.Marshal(map[string]interface{}{
		"sale_id":     saleID,
		"merchant_id": merchantID,
		"total":       total,
		"timestamp":   time.Now(),
	})
	if err != nil {
		return err
	}
	return s.repo.InsertOutbox(ctx, tx, merchantID, "sale.paid", payload)
}
