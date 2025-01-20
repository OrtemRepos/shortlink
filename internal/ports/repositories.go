package ports

import "github.com/OrtemRepos/shortlink/internal/domain"

type URLRepositoryPort interface {
	Save(url *domain.URL) error
	BatchSave(url []*domain.URL) error
	Find(shortURL string) (*domain.URL, error)
	Close() error
	Ping() error
}
