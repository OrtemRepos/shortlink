package domain_test

import (
	"testing"

	"github.com/OrtemRepos/shortlink/internal/domain"
)

func getURL() *domain.URL {
	url := domain.NewURL("https://github.com")
	return url
}

func TestNewURL(t *testing.T) {
	url := getURL()

	if url.LongURL != "https://github.com" {
		t.Errorf("Expected %s, got %s", "https://github.com", url.LongURL)
	}

	if url.ShortURL != "" {
		t.Errorf("Expected %s, got %s", "", url.ShortURL)
	}
}

func TestGenerateShortURL(t *testing.T) {
	url := getURL()

	url.GenerateShortURL()

	if url.ShortURL == "" {
		t.Errorf("Expected %s, got %s", "", url.ShortURL)
	}

	if len(url.ShortURL) != 8 {
		t.Errorf("Expected %d, got %d", 8, len(url.ShortURL))
	}
}
