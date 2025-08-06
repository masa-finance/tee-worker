package client

import (
	"fmt"
	"net/http"

	"github.com/sirupsen/logrus"
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

// HTTPClient expose the http client
func (c *TwitterXClient) HTTPClient() *http.Client {
	return c.httpClient
}

// Do execute the GET or POST request
func (c *TwitterXClient) Do(req *http.Request) (*http.Response, error) {
	return c.httpClient.Do(req)
}

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

// TestAuth tests if the API key is valid by making a minimal search request
// Mimics the detectTwitterKeyType function but with max_results=10 to minimize quota usage
func (c *TwitterXClient) TestAuth() error {
	// Use minimal search similar to detectTwitterKeyType but with max_results=10 (minimum allowed)
	endpoint := "tweets/search/recent?query=from:twitterdev&max_results=10"
	resp, err := c.Get(endpoint)
	if err != nil {
		return fmt.Errorf("error making auth test request: %w", err)
	}
	defer resp.Body.Close()

	// Check response status - same logic as detectTwitterKeyType
	switch resp.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusUnauthorized:
		return fmt.Errorf("invalid API key")
	case http.StatusTooManyRequests:
		return fmt.Errorf("rate limit exceeded")
	case http.StatusForbidden:
		return fmt.Errorf("insufficient permissions for API key")
	default:
		return fmt.Errorf("API auth test failed with status: %d", resp.StatusCode)
	}
}
