package api

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/masa-finance/tee-worker/api/types"
)

const HealthCheckPath = "/healthz"
const ReadinessCheckPath = "/readyz"

// APIKeyAuthMiddleware returns an Echo middleware that checks for the API key in the request headers.
func APIKeyAuthMiddleware(config types.JobConfiguration) echo.MiddlewareFunc {
	apiKey, ok := config["api_key"].(string)
	if !ok || apiKey == "" {
		// No API key set; allow all requests (no-op)
		return func(next echo.HandlerFunc) echo.HandlerFunc {
			return func(c echo.Context) error {
				return next(c)
			}
		}
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Skip auth for health check endpoints
			path := c.Request().URL.Path
			if path == HealthCheckPath || path == ReadinessCheckPath {
				return next(c)
			}

			// Check Authorization: Bearer <API_KEY> or X-API-Key header
			header := c.Request().Header.Get("Authorization")
			if header == "Bearer "+apiKey {
				return next(c)
			}
			if c.Request().Header.Get("X-API-Key") == apiKey {
				return next(c)
			}
			return echo.NewHTTPError(http.StatusUnauthorized, "missing or invalid API key")
		}
	}
}

// HealthMetricsMiddleware tracks success and error rates for readiness probe
func HealthMetricsMiddleware(healthMetrics *HealthMetrics) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Skip metrics for health check endpoints to avoid self-influence
			path := c.Request().URL.Path
			if path == HealthCheckPath || path == ReadinessCheckPath {
				return next(c)
			}

			// Process the request
			err := next(c)

			// Track metrics based on response status
			// Only track API endpoints (skip static files, etc)
			if strings.HasPrefix(path, "/job/") || path == "/setkey" {
				statusCode := c.Response().Status
				if statusCode >= 500 {
					healthMetrics.RecordError()
				} else if statusCode >= 200 && statusCode < 400 {
					healthMetrics.RecordSuccess()
				}
				// 4xx errors are not counted as they indicate client errors
			}

			return err
		}
	}
}
