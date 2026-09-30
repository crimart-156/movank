package handler

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/movank/sales-api/internal/domain"
	"github.com/movank/sales-api/internal/service"
)

type SaleHandler struct {
	svc *service.SaleService
}

func NewSaleHandler(svc *service.SaleService) *SaleHandler {
	return &SaleHandler{svc: svc}
}

// POST /v1/sales
func (h *SaleHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req domain.CreateSaleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, domain.ErrInvalidInput("invalid JSON"))
		return
	}

	merchantID, err := uuid.Parse(merchantFromCtx(r))
	if err != nil {
		writeError(w, domain.ErrInvalidInput("invalid merchant_id"))
		return
	}

	sale, err := h.svc.Create(r.Context(), merchantID, req)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, sale)
}

// GET /v1/sales/:id
func (h *SaleHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	saleID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, domain.ErrInvalidInput("invalid sale id"))
		return
	}

	merchantID, err := uuid.Parse(merchantFromCtx(r))
	if err != nil {
		writeError(w, domain.ErrInvalidInput("invalid merchant_id"))
		return
	}

	sale, err := h.svc.GetByID(r.Context(), saleID, merchantID)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, sale)
}

// POST /v1/sales/:id/pay
func (h *SaleHandler) Pay(w http.ResponseWriter, r *http.Request) {
	saleID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, domain.ErrInvalidInput("invalid sale id"))
		return
	}

	merchantID, err := uuid.Parse(merchantFromCtx(r))
	if err != nil {
		writeError(w, domain.ErrInvalidInput("invalid merchant_id"))
		return
	}

	// Idempotency key del header — no del body
	idempotencyKey := r.Header.Get("X-Idempotency-Key")
	if idempotencyKey == "" {
		idempotencyKey = fmt.Sprintf("auto:%s", saleID)
	}

	var req domain.PaySaleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, domain.ErrInvalidInput("invalid JSON"))
		return
	}

	sale, err := h.svc.Pay(r.Context(), saleID, merchantID, idempotencyKey, req)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, sale)
}
