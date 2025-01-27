package adapters_test

import (
	"bytes"
	"context"
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

	repo, err := adapters.NewInMemoryURLRepository("./testdata/data.json")
	if err != nil {
		t.Fatal(err)
	}

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
			_ = repo.Save(context.TODO(), url)

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

	repo, err := adapters.NewInMemoryURLRepository("./testdata/data.json")
	if err != nil {
		t.Fatal(err)
	}

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
