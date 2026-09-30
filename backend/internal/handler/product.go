package handler

import (
	"encoding/json"
	"net/http"

	"github.com/movank/sales-api/internal/domain"
	"github.com/movank/sales-api/internal/service"
)

type ProductHandler struct {
	svc *service.ProductService
}

func NewProductHandler(svc *service.ProductService) *ProductHandler {
	return &ProductHandler{svc: svc}
}

// POST /v1/products
func (h *ProductHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req domain.CreateProductRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, domain.ErrInvalidInput("invalid JSON"))
		return
	}

	product, err := h.svc.Create(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, product)
}

// GET /v1/products
func (h *ProductHandler) GetAll(w http.ResponseWriter, r *http.Request) {
	products, err := h.svc.GetAll(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	if products == nil {
		products = []domain.Product{}
	}
	writeJSON(w, http.StatusOK, products)
}
