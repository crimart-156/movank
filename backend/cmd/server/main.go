package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/joho/godotenv"
	"github.com/movank/sales-api/internal/cache"
	"github.com/movank/sales-api/internal/db"
	"github.com/movank/sales-api/internal/handler"
	"github.com/movank/sales-api/internal/repository"
	"github.com/movank/sales-api/internal/service"
	"github.com/movank/sales-api/internal/worker"
)

func main() {
	_ = godotenv.Load()

	databaseURL := mustEnv("DATABASE_URL")
	redisAddr := mustEnv("REDIS_URL")
	port := envOrDefault("PORT", "8080")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Infraestructura
	pool, err := db.NewPool(ctx, databaseURL)
	if err != nil {
		log.Fatalf("connecting to postgres: %v", err)
	}
	defer pool.Close()
	log.Println("postgres connected")

	c := cache.New(redisAddr)
	if err := c.Ping(ctx); err != nil {
		log.Printf("WARNING: cache not available: %v — dashboard will rebuild from postgres", err)
	} else {
		log.Println("cache connected")
	}

	// Repositorios — solo persistencia
	productRepo := repository.NewProductRepository(pool)
	saleRepo := repository.NewSaleRepository(pool)

	// Services — lógica de negocio
	productSvc := service.NewProductService(productRepo)
	saleSvc := service.NewSaleService(saleRepo, productRepo)
	dashboardSvc := service.NewDashboardService(saleRepo, c)

	// Hub SSE — distribuye eventos a N clientes sin duplicar queries
	hub := worker.NewHub()

	// Outbox worker — goroutine dentro del mismo proceso Go
	outboxWorker := worker.NewOutboxWorker(pool, c, saleRepo, hub)
	go outboxWorker.Run(ctx)

	// Handlers — solo parseo HTTP
	productHandler := handler.NewProductHandler(productSvc)
	saleHandler := handler.NewSaleHandler(saleSvc)
	dashboardHandler := handler.NewDashboardHandler(dashboardSvc, hub)

	// Router
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(corsMiddleware)

	r.Route("/v1", func(r chi.Router) {
		r.Post("/products", productHandler.Create)
		r.Get("/products", productHandler.GetAll)
		r.Post("/sales", saleHandler.Create)
		r.Get("/sales/{id}", saleHandler.GetByID)
		r.Post("/sales/{id}/pay", saleHandler.Pay)
		r.Get("/dashboard/today", dashboardHandler.Today)
		r.Get("/dashboard/stream", dashboardHandler.Stream)
	})

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}

	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
		log.Println("shutting down...")
		cancel()
		shutCtx, shutCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutCancel()
		srv.Shutdown(shutCtx)
	}()

	log.Printf("server listening on :%s", port)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Idempotency-Key")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("env var %s is required", key)
	}
	return v
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
