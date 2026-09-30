package service

import (
	"context"
	"log"

	"github.com/google/uuid"
	"github.com/movank/sales-api/internal/cache"
	"github.com/movank/sales-api/internal/domain"
	"github.com/movank/sales-api/internal/repository"
)

type DashboardService struct {
	saleRepo *repository.SaleRepository
	cache    *cache.Cache
}

func NewDashboardService(saleRepo *repository.SaleRepository, c *cache.Cache) *DashboardService {
	return &DashboardService{saleRepo: saleRepo, cache: c}
}

// GetToday devuelve el agregado del dashboard.
// Estrategia: cache-aside — lee del cache, si no existe reconstruye desde Postgres.
// El cache tiene TTL de 30s. Si el cache está caído, sigue funcionando desde Postgres.
func (s *DashboardService) GetToday(ctx context.Context, merchantID uuid.UUID) (*domain.DashboardAggregate, error) {
	// Intentar leer del cache
	agg, err := s.cache.GetDashboard(ctx, merchantID)
	if err != nil {
		// Cache no disponible — no es fatal, continuamos sin él
		log.Printf("[dashboard] cache unavailable: %v — rebuilding from postgres", err)
		agg = nil
	}

	if agg != nil {
		return agg, nil
	}

	// Cache miss: reconstruir desde Postgres
	agg, err = s.saleRepo.GetDashboardFromDB(ctx, merchantID)
	if err != nil {
		return nil, err
	}

	// Repoblar el cache en background — no bloqueamos la respuesta
	go func() {
		if err := s.cache.SetDashboard(ctx, agg); err != nil {
			log.Printf("[dashboard] failed to repopulate cache: %v", err)
		}
	}()

	return agg, nil
}
