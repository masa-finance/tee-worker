package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/masa-finance/tee-worker/api/types"
	"github.com/masa-finance/tee-worker/internal/jobserver"
	"github.com/stretchr/testify/assert"
)

func TestHealthMetrics(t *testing.T) {
	t.Run("NewHealthMetrics", func(t *testing.T) {
		hm := NewHealthMetrics()
		assert.NotNil(t, hm)
		assert.Equal(t, 0, hm.errorCount)
		assert.Equal(t, 0, hm.successCount)
		assert.Equal(t, 10*time.Minute, hm.windowDuration)
		assert.Equal(t, 0.95, hm.errorThreshold)
	})

	t.Run("RecordSuccess", func(t *testing.T) {
		hm := NewHealthMetrics()
		hm.RecordSuccess()
		hm.RecordSuccess()
		assert.Equal(t, 2, hm.successCount)
		assert.Equal(t, 0, hm.errorCount)
	})

	t.Run("RecordError", func(t *testing.T) {
		hm := NewHealthMetrics()
		hm.RecordError()
		hm.RecordError()
		hm.RecordError()
		assert.Equal(t, 0, hm.successCount)
		assert.Equal(t, 3, hm.errorCount)
	})

	t.Run("IsHealthy with no requests", func(t *testing.T) {
		hm := NewHealthMetrics()
		assert.True(t, hm.IsHealthy())
	})

	t.Run("IsHealthy with low error rate", func(t *testing.T) {
		hm := NewHealthMetrics()
		// 5% error rate (healthy)
		for i := 0; i < 95; i++ {
			hm.RecordSuccess()
		}
		for i := 0; i < 5; i++ {
			hm.RecordError()
		}
		assert.True(t, hm.IsHealthy())
	})

	t.Run("IsHealthy with high error rate", func(t *testing.T) {
		hm := NewHealthMetrics()
		// 96% error rate (unhealthy)
		for i := 0; i < 4; i++ {
			hm.RecordSuccess()
		}
		for i := 0; i < 96; i++ {
			hm.RecordError()
		}
		assert.False(t, hm.IsHealthy())
	})

	t.Run("GetStats", func(t *testing.T) {
		hm := NewHealthMetrics()
		hm.RecordSuccess()
		hm.RecordSuccess()
		hm.RecordError()

		stats := hm.GetStats()
		assert.Equal(t, 1, stats["error_count"])
		assert.Equal(t, 2, stats["success_count"])
		assert.Equal(t, 3, stats["total_count"])
		assert.InDelta(t, 0.333, stats["error_rate"], 0.01)
	})

	t.Run("Window reset", func(t *testing.T) {
		hm := NewHealthMetrics()
		hm.windowDuration = 100 * time.Millisecond // Short window for testing
		
		hm.RecordError()
		hm.RecordError()
		assert.Equal(t, 2, hm.errorCount)

		// Wait for window to expire
		time.Sleep(150 * time.Millisecond)
		
		// This should trigger a reset
		hm.RecordSuccess()
		assert.Equal(t, 0, hm.errorCount)
		assert.Equal(t, 1, hm.successCount)
	})
}

func TestHealthzEndpoint(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	handler := healthz()
	err := handler(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"status":"ok"`)
	assert.Contains(t, rec.Body.String(), `"service":"tee-worker"`)
}

func TestReadyzEndpoint(t *testing.T) {
	e := echo.New()

	t.Run("Ready - all checks pass", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		jobServer := jobserver.NewJobServer(10, types.JobConfiguration{})
		hm := NewHealthMetrics()
		// Record mostly successes
		for i := 0; i < 95; i++ {
			hm.RecordSuccess()
		}
		for i := 0; i < 5; i++ {
			hm.RecordError()
		}

		handler := readyz(jobServer, hm)
		err := handler(c)

		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), `"ready":true`)
		assert.Contains(t, rec.Body.String(), `"job_server":"ok"`)
		assert.Contains(t, rec.Body.String(), `"error_rate":"healthy"`)
	})

	t.Run("Not ready - nil job server", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		hm := NewHealthMetrics()
		handler := readyz(nil, hm)
		err := handler(c)

		assert.NoError(t, err)
		assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
		assert.Contains(t, rec.Body.String(), `"ready":false`)
		assert.Contains(t, rec.Body.String(), `"job_server":"not initialized"`)
	})

	t.Run("Not ready - high error rate", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		jobServer := jobserver.NewJobServer(10, types.JobConfiguration{})
		hm := NewHealthMetrics()
		// Record mostly errors
		for i := 0; i < 4; i++ {
			hm.RecordSuccess()
		}
		for i := 0; i < 96; i++ {
			hm.RecordError()
		}

		handler := readyz(jobServer, hm)
		err := handler(c)

		assert.NoError(t, err)
		assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
		assert.Contains(t, rec.Body.String(), `"ready":false`)
		assert.Contains(t, rec.Body.String(), `"error_rate":"unhealthy"`)
		assert.Contains(t, rec.Body.String(), `"error_count":96`)
	})
}

func TestHealthMetricsMiddleware(t *testing.T) {
	e := echo.New()
	hm := NewHealthMetrics()

	// Add the middleware
	e.Use(HealthMetricsMiddleware(hm))

	t.Run("Skip health endpoints", func(t *testing.T) {
		// Add test handlers
		e.GET("/healthz", func(c echo.Context) error {
			return c.String(http.StatusOK, "ok")
		})

		req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		// Should not record any metrics for health endpoints
		assert.Equal(t, 0, hm.successCount)
		assert.Equal(t, 0, hm.errorCount)
	})

	t.Run("Record success for 2xx response", func(t *testing.T) {
		hm = NewHealthMetrics()
		e = echo.New()
		e.Use(HealthMetricsMiddleware(hm))

		e.POST("/job/add", func(c echo.Context) error {
			return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
		})

		req := httptest.NewRequest(http.MethodPost, "/job/add", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, 1, hm.successCount)
		assert.Equal(t, 0, hm.errorCount)
	})

	t.Run("Record error for 5xx response", func(t *testing.T) {
		hm = NewHealthMetrics()
		e = echo.New()
		e.Use(HealthMetricsMiddleware(hm))

		e.POST("/job/add", func(c echo.Context) error {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "server error"})
		})

		req := httptest.NewRequest(http.MethodPost, "/job/add", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, 0, hm.successCount)
		assert.Equal(t, 1, hm.errorCount)
	})

	t.Run("Skip 4xx client errors", func(t *testing.T) {
		hm = NewHealthMetrics()
		e = echo.New()
		e.Use(HealthMetricsMiddleware(hm))

		e.POST("/job/add", func(c echo.Context) error {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "bad request"})
		})

		req := httptest.NewRequest(http.MethodPost, "/job/add", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		// 4xx errors should not be counted
		assert.Equal(t, 0, hm.successCount)
		assert.Equal(t, 0, hm.errorCount)
	})

	t.Run("Skip non-API endpoints", func(t *testing.T) {
		hm = NewHealthMetrics()
		e = echo.New()
		e.Use(HealthMetricsMiddleware(hm))

		e.GET("/static/file.css", func(c echo.Context) error {
			return c.String(http.StatusOK, "css content")
		})

		req := httptest.NewRequest(http.MethodGet, "/static/file.css", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		// Non-API endpoints should not be tracked
		assert.Equal(t, 0, hm.successCount)
		assert.Equal(t, 0, hm.errorCount)
	})
}