package ports

import (
	"context"

	"github.com/OrtemRepos/shortlink/internal/domain"
)

type URLRepositoryPort interface {
	Save(ctx context.Context, url *domain.URL) error
	BatchSave(ctx context.Context, url []*domain.URL) error
	BatchDelete(ctx context.Context, ids map[string][]string) error
	Find(ctx context.Context, shortURL string) (*domain.URL, error)
	Close() error
	Ping(ctx context.Context) error
}
