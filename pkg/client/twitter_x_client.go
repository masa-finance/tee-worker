package client

import (
	"encoding/json"
	"fmt"
	"github.com/sirupsen/logrus"
	"io"
	"net/http"
)

const (
	baseURL = "https://api.x.com/2"
)

// TwitterXClient represents a client for the Twitter API v2
type TwitterXClient struct {
	apiKey     string
	baseUrl    string
	httpClient *http.Client
}

// AuthResponse Simple auth response structure
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
	logrus.Info("Creating new TwitterXClient with API key")
	// test if the API key is valid before returning the client
	client := &TwitterXClient{
		apiKey:     apiKey,
		baseUrl:    baseURL,
		httpClient: &http.Client{},
	}

	logrus.Info("TwitterXClient instantiated successfully using base URL: ", client.baseUrl)
	return client
}

// HTTPClient returns the underlying HTTP client.
//
// The returned client is the same instance that is used by the TwitterXClient
// to make requests to the Twitter API. It can be used to customize the client
// or to make requests that are not supported by the TwitterXClient.
func (c *TwitterXClient) HTTPClient() *http.Client {
	return c.httpClient
}

// Do sends an HTTP request and returns an HTTP response.
//
// Do is a wrapper around the underlying HTTP client's Do method. It can be used
// to make custom requests to the Twitter API that are not supported by the
// TwitterXClient.
//
// The returned response is the same as the one returned by the underlying HTTP
// client's Do method.
func (c *TwitterXClient) Do(req *http.Request) (*http.Response, error) {
	return c.httpClient.Do(req)
}

// Get sends a GET request to the Twitter API with the given endpoint URL.
//
// The given endpoint URL is appended to the base URL of the Twitter API and
// the request is made with the provided API key.
//
// The response is the same as the one returned by the underlying HTTP client's
// Do method.
//
// If the request is successful, the HTTP response is returned. If there is an
// error making the request, an error is returned with a descriptive error
// message.
func (c *TwitterXClient) Get(endpointUrl string) (*http.Response, error) {
	url := fmt.Sprintf("%s/%s", c.baseUrl, endpointUrl)
	logrus.Info("GET request to: ", url)

	// Create request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		logrus.Errorf("error creating GET request: %v", err)
		return nil, fmt.Errorf("error creating GET request: %w", err)
	}

	// Add headers
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))
	req.Header.Add("Content-Type", "application/json")

	// Make the request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		logrus.Errorf("error making GET request: %v", err)
		return nil, fmt.Errorf("error making GET request: %w", err)
	}

	return resp, nil
}

// testAuth tests the authentication of the TwitterXClient by making a GET request to the users/me endpoint.
//
// testAuth returns nil if the authentication is successful, or an error if the authentication fails.
//
// The error returned by testAuth contains a descriptive error message and the HTTP status code.
// The error message may contain information returned by the Twitter API.
//
// The possible error messages returned by testAuth are:
//
// - "API error: <message> (code: <code>)": The Twitter API returned an error.
// - "invalid API key": The API key is invalid.
// - "rate limit exceeded": The rate limit for the API has been exceeded.
// - "API auth test failed with status: <status code>": The authentication test failed with an unexpected HTTP status code.
func (c *TwitterXClient) testAuth() error {
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
