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
