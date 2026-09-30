package service

import (
	"context"

	"github.com/movank/sales-api/internal/domain"
	"github.com/movank/sales-api/internal/repository"
)

type ProductService struct {
	repo *repository.ProductRepository
}

func NewProductService(repo *repository.ProductRepository) *ProductService {
	return &ProductService{repo: repo}
}

// Create valida y crea un producto.
func (s *ProductService) Create(ctx context.Context, req domain.CreateProductRequest) (*domain.Product, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	product := &domain.Product{
		Name:  req.Name,
		Price: req.Price,
		Stock: req.Stock,
	}

	if err := s.repo.Create(ctx, product); err != nil {
		return nil, err
	}

	return product, nil
}

// GetAll devuelve todos los productos.
func (s *ProductService) GetAll(ctx context.Context) ([]domain.Product, error) {
	return s.repo.GetAll(ctx)
}
