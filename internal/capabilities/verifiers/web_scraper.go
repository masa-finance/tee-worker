package verifiers

import (
	"context"
	"net/http"
)

// WebScraperVerifier verifies the web scraper capability.
type WebScraperVerifier struct {
	Client *http.Client
}

// NewWebScraperVerifier creates a new WebScraperVerifier.
func NewWebScraperVerifier() *WebScraperVerifier {
	return &WebScraperVerifier{
		Client: &http.Client{},
	}
}

// Verify performs a lightweight check to ensure network connectivity.
func (v *WebScraperVerifier) Verify(ctx context.Context) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, "HEAD", "https://example.com", nil)
	if err != nil {
		return false, err
	}

	resp, err := v.Client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, nil // Not an error, but connectivity is not confirmed.
	}
	return true, nil
}
