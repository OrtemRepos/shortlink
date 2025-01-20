package adapters_test

import (
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
	err := repo.Save(url)
	if err != domain.ErrURLAlreadyExists && err != nil {
		t.Errorf("Expected %v, got %v", nil, err)
	}

	repo = getRepository()
	if findURL, err := repo.Find(url.ShortURL); err != nil {
		t.Errorf("Expected %v, got %v", nil, err)
	} else if findURL.LongURL != url.LongURL {
		t.Errorf("Expected %s, got %s", url.LongURL, findURL.LongURL)
	}
}

func TestSaveAlredyExist(t *testing.T) {
	repo := getRepository()
	url := domain.NewURL("https://github.com")
	if err := repo.Save(url); err != domain.ErrURLAlreadyExists {
		t.Errorf("Expected %v, got %v", domain.ErrURLAlreadyExists, err)
	}
}

func TestFind(t *testing.T) {
	repo := getRepository()
	url := domain.NewURL("https://github.com")
	err := repo.Save(url)
	if err != nil && err != domain.ErrURLAlreadyExists {
		t.Errorf("Expected %v, got %v", nil, err)
	}
	repo = getRepository()
	if findURL, err := repo.Find(url.ShortURL); err != nil {
		t.Errorf("Expected %v, got %v", nil, err)
	} else if findURL.LongURL != url.LongURL {
		t.Errorf("Expected %s, got %s", url.LongURL, findURL.LongURL)
	}
}

func TestFindNotExist(t *testing.T) {
	repo := getRepository()
	if _, err := repo.Find("someNotExistShortURL"); err != domain.ErrURLNotFound {
		t.Errorf("Expected %v, got %v", domain.ErrURLNotFound, err)
	}
}

func TestIndempotencySave(t *testing.T) {
	repo := getRepository()
	url := domain.NewURL("https://github.com")
	_ = repo.Save(url)
	firstShortURL := &url.ShortURL
	_ = repo.Save(url)
	secondShortURL := &url.ShortURL
	if firstShortURL != secondShortURL {
		t.Errorf("Expected firstSortURL=%v and secondShortURL=%v not equal", firstShortURL, secondShortURL)
	}
	if len(repo.GetAll()) != 1 {
		t.Errorf("Expected %v, got %v", 1, len(repo.GetAll()))
	}
}
