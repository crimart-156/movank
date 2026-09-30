package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/movank/sales-api/internal/domain"
)

type ProductRepository struct {
	db *pgxpool.Pool
}

func NewProductRepository(db *pgxpool.Pool) *ProductRepository {
	return &ProductRepository{db: db}
}

func (r *ProductRepository) Create(ctx context.Context, p *domain.Product) error {
	return r.db.QueryRow(ctx,
		`INSERT INTO products (name, price, stock)
		 VALUES ($1, $2, $3)
		 RETURNING id, created_at`,
		p.Name, p.Price, p.Stock,
	).Scan(&p.ID, &p.CreatedAt)
}

func (r *ProductRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Product, error) {
	p := &domain.Product{}
	err := r.db.QueryRow(ctx,
		`SELECT id, name, price, stock, created_at FROM products WHERE id = $1`,
		id,
	).Scan(&p.ID, &p.Name, &p.Price, &p.Stock, &p.CreatedAt)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (r *ProductRepository) GetAll(ctx context.Context) ([]domain.Product, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, name, price, stock, created_at FROM products ORDER BY created_at ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var products []domain.Product
	for rows.Next() {
		var p domain.Product
		if err := rows.Scan(&p.ID, &p.Name, &p.Price, &p.Stock, &p.CreatedAt); err != nil {
			return nil, err
		}
		products = append(products, p)
	}
	return products, nil
}
