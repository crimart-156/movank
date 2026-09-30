package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/movank/sales-api/internal/service"
	"github.com/movank/sales-api/internal/worker"
)

type DashboardHandler struct {
	svc *service.DashboardService
	hub *worker.Hub
}

func NewDashboardHandler(svc *service.DashboardService, hub *worker.Hub) *DashboardHandler {
	return &DashboardHandler{svc: svc, hub: hub}
}

// GET /v1/dashboard/today
func (h *DashboardHandler) Today(w http.ResponseWriter, r *http.Request) {
	merchantID, err := uuid.Parse(merchantFromCtx(r))
	if err != nil {
		writeError(w, err)
		return
	}

	agg, err := h.svc.GetToday(r.Context(), merchantID)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, agg)
}

// GET /v1/dashboard/stream — SSE
// El hub distribuye el mismo evento a N clientes sin que cada uno haga su propio query.
func (h *DashboardHandler) Stream(w http.ResponseWriter, r *http.Request) {
	merchantID, err := uuid.Parse(merchantFromCtx(r))
	if err != nil {
		writeError(w, err)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	// Enviar estado actual al conectar
	agg, _ := h.svc.GetToday(r.Context(), merchantID)
	if agg != nil {
		data, _ := json.Marshal(agg)
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}

	ch, unsub := h.hub.Subscribe(merchantID)
	defer unsub()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case agg, ok := <-ch:
			if !ok {
				return
			}
			data, _ := json.Marshal(agg)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		case <-ticker.C:
			fmt.Fprintf(w, ": keepalive\n\n")
			flusher.Flush()
		}
	}
}
