package api

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/masa-finance/tee-worker/api/types"
)

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
