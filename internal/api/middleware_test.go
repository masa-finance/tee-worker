package api_test

import (
	"net/http"
	"net/http/httptest"

	"github.com/labstack/echo/v4"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/masa-finance/tee-worker/internal/api"
)

var _ = Describe("APIKeyAuthMiddleware", func() {
	var (
		e       *echo.Echo
		handler echo.HandlerFunc
	)

	BeforeEach(func() {
		e = echo.New()
		handler = func(c echo.Context) error {
			return c.String(http.StatusOK, "passed")
		}
	})

	Context("when no API key is configured", func() {
		It("should allow all requests", func() {
			config := map[string]interface{}{}
			e.Use(APIKeyAuthMiddleware(config))
			e.GET("/test", handler)

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			Expect(rec.Code).To(Equal(http.StatusOK))
			Expect(rec.Body.String()).To(Equal("passed"))
		})
	})

	Context("when API key is configured", func() {
		var config map[string]interface{}

		BeforeEach(func() {
			config = map[string]interface{}{"api_key": "test123"}
			e.Use(APIKeyAuthMiddleware(config))
			e.GET("/test", handler)
		})

		It("should accept correct API key in Authorization header", func() {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.Header.Set("Authorization", "Bearer test123")
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			Expect(rec.Code).To(Equal(http.StatusOK))
			Expect(rec.Body.String()).To(Equal("passed"))
		})

		It("should accept correct API key in X-API-Key header", func() {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.Header.Set("X-API-Key", "test123")
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			Expect(rec.Code).To(Equal(http.StatusOK))
			Expect(rec.Body.String()).To(Equal("passed"))
		})

		It("should reject requests with missing API key", func() {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			Expect(rec.Code).To(Equal(http.StatusUnauthorized))
		})

		It("should reject requests with wrong API key", func() {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.Header.Set("Authorization", "Bearer wrong")
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			Expect(rec.Code).To(Equal(http.StatusUnauthorized))
		})

		It("should allow health check endpoints without API key", func() {
			e.GET("/healthz", handler)
			e.GET("/readyz", handler)

			// Test /healthz
			req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			Expect(rec.Code).To(Equal(http.StatusOK))

			// Test /readyz
			req = httptest.NewRequest(http.MethodGet, "/readyz", nil)
			rec = httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			Expect(rec.Code).To(Equal(http.StatusOK))
		})
	})
})