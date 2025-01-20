package adapters_test

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"github.com/OrtemRepos/shortlink/configs"
	"github.com/OrtemRepos/shortlink/internal/adapters"
	"github.com/OrtemRepos/shortlink/internal/domain"
)

type MockURLRepository struct {
	urls map[string]domain.URL
}

func NewMockURLRepository() *MockURLRepository {
	return &MockURLRepository{
		urls: make(map[string]domain.URL),
	}
}

func (m *MockURLRepository) Ping() error {
	return nil
}

func (m *MockURLRepository) Save(url *domain.URL) error {
	if shortURL, exist := m.longURLExists(url); exist {
		url.ShortURL = shortURL
		return domain.ErrURLAlreadyExists
	}
	url.GenerateShortURL()
	m.urls[url.ShortURL] = *url
	return nil
}

func (m *MockURLRepository) BatchSave(urls []*domain.URL) error {
	return nil
}

func (m *MockURLRepository) Close() error {
	return nil
}

func (m *MockURLRepository) longURLExists(url *domain.URL) (shortlink string, exist bool) {
	if len(m.urls) == 0 {
		return shortlink, exist
	}
	for k, v := range m.urls {
		if v.LongURL == url.LongURL {
			return k, true
		}
	}
	return "", false
}

func (m *MockURLRepository) Find(shortURL string) (*domain.URL, error) {
	url, ok := m.urls[shortURL]
	if !ok {
		return nil, domain.ErrURLNotFound
	}
	return &url, nil
}

func setupRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	gin.DefaultWriter = io.Discard
	gin.DefaultErrorWriter = io.Discard
	router := gin.Default()
	log.SetOutput(io.Discard)
	return router
}

func TestGetLongURL(t *testing.T) {
	type testCase struct {
		name         string
		shortURL     string
		expectedCode int
		expectedBody string
	}

	tests := []testCase{
		{
			name:         "Successful find",
			shortURL:     "shortURL",
			expectedCode: http.StatusMovedPermanently,
			expectedBody: "",
		},
		{
			name:         "URL not found",
			shortURL:     "nonexistent",
			expectedCode: http.StatusNotFound,
			expectedBody: domain.ErrURLNotFound.Error(),
		},
	}

	repo := NewMockURLRepository()
	repo.urls["shortURL"] = domain.URL{LongURL: "http://example.com", ShortURL: "shortURL"}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := setupRouter()
			cfg, err := configs.GetConfig([]string{"-s", "datatest/test.json"})
			if err != nil {
				t.Fatal(err)
			}
			api := adapters.NewRestAPI(repo, router, cfg)

			router.GET("/:shortURL", api.GetLongURL)

			url := domain.NewURL("http://example.com")
			_ = repo.Save(url)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/"+tt.shortURL, nil)
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedCode, w.Code)
			if tt.expectedBody != "" {
				assert.Contains(t, w.Body.String(), tt.expectedBody)
			}
		})
	}
}

func TestJSONShortURL(t *testing.T) {
	type testCase struct {
		name          string
		requestBody   string
		expectedCode  int
		expectedBody  string
		mockSaveError error
	}

	tests := []testCase{
		{
			name:          "Successful save",
			requestBody:   `{"LongURL": "http://example.com"}`,
			expectedCode:  http.StatusCreated,
			expectedBody:  `result`,
			mockSaveError: nil,
		},
		{
			name:          "Missing LongURL",
			requestBody:   `{}`,
			expectedCode:  http.StatusBadRequest,
			expectedBody:  "url is required",
			mockSaveError: nil,
		},
		{
			name:         "Invalid JSON",
			requestBody:  `invalid json`,
			expectedCode: http.StatusBadRequest,
			expectedBody: "invalid character 'i' looking for beginning of value",
		},
	}

	repo := NewMockURLRepository()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := setupRouter()

			cfg, err := configs.GetConfig([]string{"-s", "datatest/test.json"})
			if err != nil {
				t.Fatal(err)
			}
			api := adapters.NewRestAPI(repo, router, cfg)

			router.POST("/api/shorturl", api.JSONShortURL)

			req, _ := http.NewRequest("POST", "/api/shorturl", bytes.NewBufferString(tt.requestBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedCode, w.Code)
			assert.Contains(t, w.Body.String(), tt.expectedBody)
		})
	}
}
