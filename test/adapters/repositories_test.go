package adapters_test

import (
	"context"
	"testing"

	"github.com/OrtemRepos/shortlink/internal/adapters"
	"github.com/OrtemRepos/shortlink/internal/domain"
)

func getRepository() *adapters.InMemoryURLRepository {
	repo, err := adapters.NewInMemoryURLRepository("datatest/test.json")
	if err != nil {
		panic(err)
	}
	return repo
}

func TestCreateRepository(t *testing.T) {
	repo := getRepository()
	if repo == nil {
		t.Errorf("Expected %v, got %v", repo, nil)
	}
}

func TestSave(t *testing.T) {
	repo := getRepository()
	url := domain.NewURL("https://github.com")
	err := repo.Save(context.TODO(), url)
	if err != domain.ErrURLAlreadyExists && err != nil {
		t.Errorf("Expected %v, got %v", nil, err)
	}

	repo = getRepository()
	if findURL, err := repo.Find(context.TODO(), url.ShortURL); err != nil {
		t.Errorf("Expected %v, got %v", nil, err)
	} else if findURL.OriginalURL != url.OriginalURL {
		t.Errorf("Expected %s, got %s", url.OriginalURL, findURL.OriginalURL)
	}
}

func TestSaveAlredyExist(t *testing.T) {
	repo := getRepository()
	url := domain.NewURL("https://github.com")
	if err := repo.Save(context.TODO(), url); err != domain.ErrURLAlreadyExists {
		t.Errorf("Expected %v, got %v", domain.ErrURLAlreadyExists, err)
	}
}

func TestFind(t *testing.T) {
	repo := getRepository()
	url := domain.NewURL("https://github.com")
	err := repo.Save(context.TODO(), url)
	if err != nil && err != domain.ErrURLAlreadyExists {
		t.Errorf("Expected %v, got %v", nil, err)
	}
	repo = getRepository()
	if findURL, err := repo.Find(context.TODO(), url.ShortURL); err != nil {
		t.Errorf("Expected %v, got %v", nil, err)
	} else if findURL.OriginalURL != url.OriginalURL {
		t.Errorf("Expected %s, got %s", url.OriginalURL, findURL.OriginalURL)
	}
}

func TestFindNotExist(t *testing.T) {
	repo := getRepository()
	if _, err := repo.Find(context.TODO(), "someNotExistShortURL"); err != domain.ErrURLNotFound {
		t.Errorf("Expected %v, got %v", domain.ErrURLNotFound, err)
	}
}

func TestIndempotencySave(t *testing.T) {
	repo := getRepository()
	url := domain.NewURL("https://github.com")
	_ = repo.Save(context.TODO(), url)
	firstShortURL := &url.ShortURL
	_ = repo.Save(context.TODO(), url)
	secondShortURL := &url.ShortURL
	if firstShortURL != secondShortURL {
		t.Errorf("Expected firstSortURL=%v and secondShortURL=%v not equal", firstShortURL, secondShortURL)
	}
	if len(repo.GetAll()) != 1 {
		t.Errorf("Expected %v, got %v", 1, len(repo.GetAll()))
	}
}
