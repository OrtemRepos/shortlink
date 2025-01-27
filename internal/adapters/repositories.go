package adapters

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"sync"

	"github.com/OrtemRepos/shortlink/internal/domain"
)

const (
	filePerm = 0662 // owner and group: read/write; others: read
)

type urls struct {
	m  map[string]string
	mu sync.RWMutex
}

type InMemoryURLRepository struct {
	urls
	savePath string
}

func NewInMemoryURLRepository(savePath string) (*InMemoryURLRepository, error) {
	repo := &InMemoryURLRepository{
		urls: urls{
			m: make(map[string]string),
		},
		savePath: savePath,
	}
	err := repo.load()
	if err != nil {
		return nil, err
	}
	return repo, nil
}

func (r *InMemoryURLRepository) Ping(ctx context.Context) error {
	return nil
}

func (r *InMemoryURLRepository) Save(ctx context.Context, url *domain.URL) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if shortURL, ok := r.longURLExists(url.LongURL); ok {
		url.ShortURL = shortURL
		return domain.ErrURLAlreadyExists
	}
	url.GenerateShortURL()
	r.m[url.ShortURL] = url.LongURL
	return r.saveToFile()
}
func (r *InMemoryURLRepository) BatchSave(ctx context.Context, urls []*domain.URL) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, url := range urls {
		if shortURL, ok := r.longURLExists(url.LongURL); ok {
			url.ShortURL = shortURL
		} else {
			url.GenerateShortURL()
			r.m[url.ShortURL] = url.LongURL
		}
	}
	return r.saveToFile()
}

func (r *InMemoryURLRepository) BatchDelete(ctx context.Context, ids map[string][]string) error {
	return nil
}

func (r *InMemoryURLRepository) Find(ctx context.Context, shortURL string) (*domain.URL, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	longURL, ok := r.m[shortURL]
	if !ok {
		return nil, domain.ErrURLNotFound
	}
	return &domain.URL{LongURL: longURL, ShortURL: shortURL}, nil
}

func (r *InMemoryURLRepository) longURLExists(longURL string) (string, bool) {
	for short, l := range r.m {
		if l == longURL {
			return short, true
		}
	}
	return "", false
}

func (r *InMemoryURLRepository) GetAll() map[string]string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.m
}

func (r *InMemoryURLRepository) saveToFile() error {
	file, err := os.OpenFile(r.savePath, os.O_WRONLY|os.O_TRUNC, filePerm)
	if err != nil {
		return err
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(r.m); err != nil {
		return err
	}
	return nil
}

func (r *InMemoryURLRepository) load() error {
	file, err := os.Open(r.savePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	var urls map[string]string
	if err := decoder.Decode(&urls); err != nil && err != io.EOF {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.m = urls
	return nil
}

func (r *InMemoryURLRepository) Close() error {
	return nil
}
