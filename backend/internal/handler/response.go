package handler

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/movank/sales-api/internal/domain"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, err error) {
	if appErr, ok := err.(*domain.AppError); ok {
		writeJSON(w, appErr.Code, map[string]string{"error": appErr.Message})
		return
	}
	// Log del error real para debugging
	log.Printf("[error] %v", err)
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
}

// merchantFromCtx extrae el merchant_id del contexto.
// El middleware de auth lo inyecta desde el token — nunca del body/query.
func merchantFromCtx(r *http.Request) string {
	if v := r.Context().Value(ctxMerchantKey); v != nil {
		return v.(string)
	}
	// Para desarrollo/testing sin auth real, usamos un merchant fijo
	return "00000000-0000-0000-0000-000000000001"
}

type ctxKey string

const ctxMerchantKey ctxKey = "merchant_id"
