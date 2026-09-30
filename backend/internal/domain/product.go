package domain

import (
	"time"

	"github.com/google/uuid"
)

type Product struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Price     float64   `json:"price"`
	Stock     int       `json:"stock"`
	CreatedAt time.Time `json:"created_at"`
}

type CreateProductRequest struct {
	Name  string  `json:"name"`
	Price float64 `json:"price"`
	Stock int     `json:"stock"`
}

func (r *CreateProductRequest) Validate() error {
	if r.Name == "" {
		return ErrInvalidInput("name is required")
	}
	if r.Price <= 0 {
		return ErrInvalidInput("price must be greater than 0")
	}
	if r.Stock < 0 {
		return ErrInvalidInput("stock cannot be negative")
	}
	return nil
}
