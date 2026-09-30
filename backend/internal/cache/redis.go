package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/movank/sales-api/internal/domain"
	"github.com/redis/go-redis/v9"
)

const dashboardTTL = 30 * time.Second

type Cache struct {
	client *redis.Client
}

func New(addr string) *Cache {
	rdb := redis.NewClient(&redis.Options{Addr: addr})
	return &Cache{client: rdb}
}

func (c *Cache) Ping(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}

func dashboardKey(merchantID uuid.UUID) string {
	return fmt.Sprintf("dashboard:%s:today", merchantID)
}

// GetDashboard lee el agregado desde cache.
// Retorna nil, nil si no existe (cache miss).
func (c *Cache) GetDashboard(ctx context.Context, merchantID uuid.UUID) (*domain.DashboardAggregate, error) {
	val, err := c.client.Get(ctx, dashboardKey(merchantID)).Bytes()
	if err == redis.Nil {
		return nil, nil // cache miss — el caller reconstruye desde Postgres
	}
	if err != nil {
		return nil, err
	}

	var agg domain.DashboardAggregate
	if err := json.Unmarshal(val, &agg); err != nil {
		return nil, err
	}
	return &agg, nil
}

// SetDashboard guarda el agregado con TTL de 30s.
// Usamos SET con NX=false para que siempre sobreescriba — el worker
// es el único escritor del agregado, así que no hay race aquí.
func (c *Cache) SetDashboard(ctx context.Context, agg *domain.DashboardAggregate) error {
	data, err := json.Marshal(agg)
	if err != nil {
		return err
	}
	return c.client.Set(ctx, dashboardKey(agg.MerchantID), data, dashboardTTL).Err()
}

// DeleteDashboard invalida el cache (útil para tests y reconstrucción manual)
func (c *Cache) DeleteDashboard(ctx context.Context, merchantID uuid.UUID) error {
	return c.client.Del(ctx, dashboardKey(merchantID)).Err()
}
