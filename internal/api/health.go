package api

import (
	"net/http"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/masa-finance/tee-worker/internal/jobserver"
)

// HealthMetrics tracks health-related metrics for the service
type HealthMetrics struct {
	mu              sync.RWMutex
	errorCount      int
	successCount    int
	windowStart     time.Time
	windowDuration  time.Duration
	errorThreshold  float64
}

// NewHealthMetrics creates a new health metrics tracker
func NewHealthMetrics() *HealthMetrics {
	return &HealthMetrics{
		windowStart:     time.Now(),
		windowDuration:  10 * time.Minute,
		errorThreshold:  0.95, // 95% error rate threshold
	}
}

// RecordSuccess records a successful request
func (hm *HealthMetrics) RecordSuccess() {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	
	hm.checkAndResetWindow()
	hm.successCount++
}

// RecordError records an error
func (hm *HealthMetrics) RecordError() {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	
	hm.checkAndResetWindow()
	hm.errorCount++
}

// checkAndResetWindow resets the metrics window if it has expired
func (hm *HealthMetrics) checkAndResetWindow() {
	if time.Since(hm.windowStart) > hm.windowDuration {
		hm.errorCount = 0
		hm.successCount = 0
		hm.windowStart = time.Now()
	}
}

// IsHealthy checks if the service is healthy based on error rate
func (hm *HealthMetrics) IsHealthy() bool {
	hm.mu.RLock()
	defer hm.mu.RUnlock()
	
	total := hm.errorCount + hm.successCount
	if total == 0 {
		return true // No requests yet, consider healthy
	}
	
	errorRate := float64(hm.errorCount) / float64(total)
	return errorRate < hm.errorThreshold
}

// GetStats returns current health statistics
func (hm *HealthMetrics) GetStats() map[string]interface{} {
	hm.mu.RLock()
	defer hm.mu.RUnlock()
	
	total := hm.errorCount + hm.successCount
	errorRate := 0.0
	if total > 0 {
		errorRate = float64(hm.errorCount) / float64(total)
	}
	
	return map[string]interface{}{
		"error_count":   hm.errorCount,
		"success_count": hm.successCount,
		"total_count":   total,
		"error_rate":    errorRate,
		"window_start":  hm.windowStart.Format(time.RFC3339),
		"window_duration": hm.windowDuration.String(),
	}
}

// Healthz is the liveness probe endpoint
func Healthz() func(c echo.Context) error {
	return func(c echo.Context) error {
		// Simple liveness check - service is running
		return c.JSON(http.StatusOK, map[string]string{
			"status": "ok",
			"service": "tee-worker",
		})
	}
}

// Readyz is the readiness probe endpoint
func Readyz(jobServer *jobserver.JobServer, healthMetrics *HealthMetrics) func(c echo.Context) error {
	return func(c echo.Context) error {
		checks := map[string]interface{}{
			"service": "tee-worker",
			"ready": true,
			"checks": map[string]interface{}{},
		}
		
		// Check if job server is running
		if jobServer == nil {
			checks["ready"] = false
			checks["checks"].(map[string]interface{})["job_server"] = "not initialized"
			return c.JSON(http.StatusServiceUnavailable, checks)
		}
		
		// Check error rate
		if !healthMetrics.IsHealthy() {
			checks["ready"] = false
			checks["checks"].(map[string]interface{})["error_rate"] = "unhealthy"
			checks["checks"].(map[string]interface{})["stats"] = healthMetrics.GetStats()
			return c.JSON(http.StatusServiceUnavailable, checks)
		}
		
		// All checks passed
		checks["checks"].(map[string]interface{})["job_server"] = "ok"
		checks["checks"].(map[string]interface{})["error_rate"] = "healthy"
		checks["checks"].(map[string]interface{})["stats"] = healthMetrics.GetStats()
		
		return c.JSON(http.StatusOK, checks)
	}
}