package domain

import (
	"time"

	"github.com/google/uuid"
)

// PaymentStatus representa el estado de un pago.
// UNKNOWN es estado de primera clase — un TIMEOUT nunca es DECLINED.
type PaymentStatus string

const (
	StatusPending     PaymentStatus = "PENDING"
	StatusApproved    PaymentStatus = "APPROVED"
	StatusDeclined    PaymentStatus = "DECLINED"
	StatusUnknown     PaymentStatus = "UNKNOWN"      // timeout del proveedor
	StatusPendingSync PaymentStatus = "PENDING_SYNC" // offline, esperando sync
)

type PaymentScenario string

const (
	ScenarioApproved PaymentScenario = "APPROVED"
	ScenarioDeclined PaymentScenario = "DECLINED"
	ScenarioTimeout  PaymentScenario = "TIMEOUT"
)

type PaymentMethod string

const (
	MethodCard PaymentMethod = "CARD"
	MethodCash PaymentMethod = "CASH"
)

type Sale struct {
	ID         uuid.UUID  `json:"id"`
	MerchantID uuid.UUID  `json:"merchant_id"`
	Status     PaymentStatus `json:"status"`
	Total      float64    `json:"total"`
	Items      []SaleItem `json:"items"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

type SaleItem struct {
	ID        uuid.UUID `json:"id"`
	SaleID    uuid.UUID `json:"sale_id"`
	ProductID uuid.UUID `json:"product_id"`
	Quantity  int       `json:"quantity"`
	UnitPrice float64   `json:"unit_price"`
}

type CreateSaleRequest struct {
	Items []CreateSaleItemRequest `json:"items"`
}

type CreateSaleItemRequest struct {
	ProductID uuid.UUID `json:"product_id"`
	Quantity  int       `json:"quantity"`
}

type PaySaleRequest struct {
	Method   PaymentMethod   `json:"method"`
	Scenario PaymentScenario `json:"scenario"`
	// IdempotencyKey viene del header X-Idempotency-Key, no del body
}

// DashboardAggregate es lo que se guarda en cache y se empuja por SSE
type DashboardAggregate struct {
	MerchantID   uuid.UUID `json:"merchant_id"`
	TotalSales   int       `json:"total_sales"`
	TotalRevenue float64   `json:"total_revenue"`
	Approved     int       `json:"approved"`
	Declined     int       `json:"declined"`
	Unknown      int       `json:"unknown"`
	UpdatedAt    time.Time `json:"updated_at"`
}
