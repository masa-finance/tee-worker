package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func TestAPIKeyAuthMiddleware(t *testing.T) {

	tests := []struct {
		name           string
		config         map[string]interface{}
		headerKey      string
		headerValue    string
		expectedStatus int
	}{
		{"no api key set (open)", map[string]interface{}{}, "", "", http.StatusOK},
		{"correct api key (Authorization)", map[string]interface{}{"api_key": "test123"}, "Authorization", "Bearer test123", http.StatusOK},
		{"correct api key (X-API-Key)", map[string]interface{}{"api_key": "test123"}, "X-API-Key", "test123", http.StatusOK},
		{"missing api key", map[string]interface{}{"api_key": "test123"}, "", "", http.StatusUnauthorized},
		{"wrong api key", map[string]interface{}{"api_key": "test123"}, "Authorization", "Bearer wrong", http.StatusUnauthorized},
	}

	handler := func(c echo.Context) error {
		return c.String(http.StatusOK, "passed")
	}

	for _, tt := range tests {
		e := echo.New()
		e.Use(APIKeyAuthMiddleware(tt.config))
		e.GET("/test", handler)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		if tt.headerKey != "" {
			req.Header.Set(tt.headerKey, tt.headerValue)
		}
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		assert.Equal(t, tt.expectedStatus, rec.Code, tt.name)
	}
}
