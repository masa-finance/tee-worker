package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const (
	baseURL = "https://api.x.com/2"
)

// TwitterXClient represents a client for the Twitter API v2
type TwitterXClient struct {
	apiKey     string
	httpClient *http.Client
}

// Simple auth response structure
type AuthResponse struct {
	Data struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		Username string `json:"username"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
		Code    int    `json:"code"`
	} `json:"errors,omitempty"`
}

func NewTwitterXClient(apiKey string) *TwitterXClient {
	return &TwitterXClient{
		apiKey:     apiKey,
		httpClient: &http.Client{},
	}
}

// TestAuth tests if the API key is valid by making a request to /2/users/me
func (c *TwitterXClient) TestAuth() error {
	// Create request
	req, err := http.NewRequest("GET", baseURL+"/users/me", nil)
	if err != nil {
		return fmt.Errorf("error creating auth test request: %w", err)
	}

	// Add headers
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))
	req.Header.Add("Content-Type", "application/json")

	// Make request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("error making auth test request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading response: %w", err)
	}

	// Parse response
	var authResp AuthResponse
	if err := json.Unmarshal(body, &authResp); err != nil {
		return fmt.Errorf("error parsing response: %w", err)
	}

	// Check for errors
	if len(authResp.Errors) > 0 {
		return fmt.Errorf("API error: %s (code: %d)",
			authResp.Errors[0].Message,
			authResp.Errors[0].Code)
	}

	// Check response status
	switch resp.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusUnauthorized:
		return fmt.Errorf("invalid API key")
	case http.StatusTooManyRequests:
		return fmt.Errorf("rate limit exceeded")
	default:
		return fmt.Errorf("API auth test failed with status: %d", resp.StatusCode)
	}
}
