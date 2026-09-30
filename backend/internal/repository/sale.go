package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/movank/sales-api/internal/domain"
)

type SaleRepository struct {
	db *pgxpool.Pool
}

func NewSaleRepository(db *pgxpool.Pool) *SaleRepository {
	return &SaleRepository{db: db}
}

// Create inserta la venta y sus items en una transacción.
// Solo persistencia — sin lógica de negocio.
func (r *SaleRepository) Create(ctx context.Context, sale *domain.Sale) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	err = tx.QueryRow(ctx,
		`INSERT INTO sales (merchant_id, status, total)
		 VALUES ($1, $2, $3)
		 RETURNING id, created_at, updated_at`,
		sale.MerchantID, sale.Status, sale.Total,
	).Scan(&sale.ID, &sale.CreatedAt, &sale.UpdatedAt)
	if err != nil {
		return err
	}

	for i := range sale.Items {
		sale.Items[i].SaleID = sale.ID
		err = tx.QueryRow(ctx,
			`INSERT INTO sale_items (sale_id, product_id, quantity, unit_price)
			 VALUES ($1, $2, $3, $4)
			 RETURNING id`,
			sale.ID, sale.Items[i].ProductID, sale.Items[i].Quantity, sale.Items[i].UnitPrice,
		).Scan(&sale.Items[i].ID)
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

// GetByID devuelve una venta con sus items.
// Siempre filtra por merchant_id — aislamiento de tenant a nivel de dato.
func (r *SaleRepository) GetByID(ctx context.Context, id uuid.UUID, merchantID uuid.UUID) (*domain.Sale, error) {
	sale := &domain.Sale{}

	err := r.db.QueryRow(ctx,
		`SELECT id, merchant_id, status, total, created_at, updated_at
		 FROM sales WHERE id = $1 AND merchant_id = $2`,
		id, merchantID,
	).Scan(&sale.ID, &sale.MerchantID, &sale.Status, &sale.Total, &sale.CreatedAt, &sale.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	rows, err := r.db.Query(ctx,
		`SELECT id, sale_id, product_id, quantity, unit_price
		 FROM sale_items WHERE sale_id = $1`,
		sale.ID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var item domain.SaleItem
		if err := rows.Scan(&item.ID, &item.SaleID, &item.ProductID, &item.Quantity, &item.UnitPrice); err != nil {
			return nil, err
		}
		sale.Items = append(sale.Items, item)
	}

	return sale, nil
}

// FindPaymentByIdempotencyKey busca un pago existente por su idempotency key.
// Devuelve nil si no existe.
func (r *SaleRepository) FindPaymentByIdempotencyKey(ctx context.Context, key string) (*domain.PaymentStatus, error) {
	var status domain.PaymentStatus
	err := r.db.QueryRow(ctx,
		`SELECT status FROM payments WHERE idempotency_key = $1`, key,
	).Scan(&status)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &status, nil
}

// GetSaleStatusForUpdate bloquea la fila para actualización y devuelve el estado actual.
// Siempre filtra por merchant_id.
func (r *SaleRepository) GetSaleStatusForUpdate(ctx context.Context, tx pgx.Tx, saleID uuid.UUID, merchantID uuid.UUID) (domain.PaymentStatus, error) {
	var status domain.PaymentStatus
	err := tx.QueryRow(ctx,
		`SELECT status FROM sales WHERE id = $1 AND merchant_id = $2 FOR UPDATE`,
		saleID, merchantID,
	).Scan(&status)
	return status, err
}

// UpdateSaleStatus actualiza el estado de la venta dentro de una transacción.
func (r *SaleRepository) UpdateSaleStatus(ctx context.Context, tx pgx.Tx, saleID uuid.UUID, merchantID uuid.UUID, status domain.PaymentStatus) (*domain.Sale, error) {
	sale := &domain.Sale{}
	err := tx.QueryRow(ctx,
		`UPDATE sales SET status = $1, updated_at = NOW()
		 WHERE id = $2 AND merchant_id = $3
		 RETURNING id, merchant_id, status, total, created_at, updated_at`,
		status, saleID, merchantID,
	).Scan(&sale.ID, &sale.MerchantID, &sale.Status, &sale.Total, &sale.CreatedAt, &sale.UpdatedAt)
	return sale, err
}

// InsertPayment registra el intento de pago con su idempotency key.
func (r *SaleRepository) InsertPayment(ctx context.Context, tx pgx.Tx, saleID uuid.UUID, key string, method domain.PaymentMethod, status domain.PaymentStatus, scenario domain.PaymentScenario) error {
	_, err := tx.Exec(ctx,
		`INSERT INTO payments (sale_id, idempotency_key, method, status, scenario)
		 VALUES ($1, $2, $3, $4, $5)`,
		saleID, key, method, status, scenario,
	)
	return err
}

// InsertOutbox escribe el evento en el outbox dentro de la misma transacción.
func (r *SaleRepository) InsertOutbox(ctx context.Context, tx pgx.Tx, merchantID uuid.UUID, eventType string, payload []byte) error {
	_, err := tx.Exec(ctx,
		`INSERT INTO outbox (merchant_id, event_type, payload) VALUES ($1, $2, $3)`,
		merchantID, eventType, payload,
	)
	return err
}

// BeginTx inicia una transacción y la expone al service para orquestar múltiples operaciones.
func (r *SaleRepository) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return r.db.Begin(ctx)
}

// GetDashboardFromDB reconstruye el agregado del día desde Postgres.
// Se usa cuando el cache no está disponible o expiró.
func (r *SaleRepository) GetDashboardFromDB(ctx context.Context, merchantID uuid.UUID) (*domain.DashboardAggregate, error) {
	agg := &domain.DashboardAggregate{MerchantID: merchantID, UpdatedAt: time.Now()}

	rows, err := r.db.Query(ctx,
		`SELECT status, COUNT(*), COALESCE(SUM(total), 0)
		 FROM sales
		 WHERE merchant_id = $1
		   AND created_at >= CURRENT_DATE
		 GROUP BY status`,
		merchantID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var status domain.PaymentStatus
		var count int
		var revenue float64
		if err := rows.Scan(&status, &count, &revenue); err != nil {
			return nil, err
		}
		agg.TotalSales += count
		agg.TotalRevenue += revenue
		switch status {
		case domain.StatusApproved:
			agg.Approved = count
		case domain.StatusDeclined:
			agg.Declined = count
		case domain.StatusUnknown:
			agg.Unknown = count
		}
	}

	return agg, nil
}
