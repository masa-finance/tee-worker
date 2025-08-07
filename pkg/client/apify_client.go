package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/sirupsen/logrus"
)

const (
	apifyBaseURL = "https://api.apify.com/v2"
)

// ApifyClient represents a client for the Apify API
type ApifyClient struct {
	apiToken string
	baseUrl  string
	options  *Options
}

// ActorRunResponse represents the response from running an actor
type ActorRunResponse struct {
	Data struct {
		ID               string `json:"id"`
		Status           string `json:"status"`
		DefaultDatasetId string `json:"defaultDatasetId"`
	} `json:"data"`
}

// DatasetResponse represents the response from getting dataset items
type DatasetResponse struct {
	Data struct {
		Items  []json.RawMessage `json:"items"`
		Count  int               `json:"count"`
		Offset int               `json:"offset"`
		Limit  int               `json:"limit"`
		Total  int               `json:"total"`
	} `json:"data"`
}

// NewApifyClient creates a new Apify client with functional options
func NewApifyClient(apiToken string, opts ...Option) (*ApifyClient, error) {
	logrus.Info("Creating new ApifyClient with API token")

	options, err := NewOptions(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create options: %w", err)
	}

	return &ApifyClient{
		apiToken: apiToken,
		baseUrl:  apifyBaseURL,
		options:  options,
	}, nil
}

// HTTPClient exposes the configured http client
func (c *ApifyClient) HTTPClient() *http.Client {
	return c.options.HttpClient
}

// RunActor runs an actor with the given input
func (c *ApifyClient) RunActor(actorId string, input interface{}) (*ActorRunResponse, error) {
	url := fmt.Sprintf("%s/acts/%s/runs?token=%s", c.baseUrl, actorId, c.apiToken)
	logrus.Infof("Running actor %s", actorId)

	// Marshal input to JSON
	inputJSON, err := json.Marshal(input)
	if err != nil {
		logrus.Errorf("error marshaling actor input: %v", err)
		return nil, fmt.Errorf("error marshaling actor input: %w", err)
	}

	// Create request
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(inputJSON))
	if err != nil {
		logrus.Errorf("error creating POST request: %v", err)
		return nil, fmt.Errorf("error creating POST request: %w", err)
	}

	// Add headers
	req.Header.Add("Content-Type", "application/json")

	// Make the request
	resp, err := c.options.HttpClient.Do(req)
	if err != nil {
		logrus.Errorf("error making POST request: %v", err)
		return nil, fmt.Errorf("error making POST request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logrus.Errorf("error reading response body: %v", err)
		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	// Check response status
	if resp.StatusCode != http.StatusCreated {
		logrus.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var runResp ActorRunResponse
	if err := json.Unmarshal(body, &runResp); err != nil {
		logrus.Errorf("error parsing response: %v", err)
		return nil, fmt.Errorf("error parsing response: %w", err)
	}

	logrus.Infof("Actor run started with ID: %s", runResp.Data.ID)
	return &runResp, nil
}

// GetActorRun gets the status of an actor run
func (c *ApifyClient) GetActorRun(runId string) (*ActorRunResponse, error) {
	url := fmt.Sprintf("%s/actor-runs/%s?token=%s", c.baseUrl, runId, c.apiToken)
	logrus.Debugf("Getting actor run status: %s", runId)

	// Create request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		logrus.Errorf("error creating GET request: %v", err)
		return nil, fmt.Errorf("error creating GET request: %w", err)
	}

	// Make the request
	resp, err := c.options.HttpClient.Do(req)
	if err != nil {
		logrus.Errorf("error making GET request: %v", err)
		return nil, fmt.Errorf("error making GET request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logrus.Errorf("error reading response body: %v", err)
		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	// Check response status
	if resp.StatusCode != http.StatusOK {
		logrus.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var runResp ActorRunResponse
	if err := json.Unmarshal(body, &runResp); err != nil {
		logrus.Errorf("error parsing response: %v", err)
		return nil, fmt.Errorf("error parsing response: %w", err)
	}

	return &runResp, nil
}

// GetDatasetItems gets items from a dataset with pagination
func (c *ApifyClient) GetDatasetItems(datasetId string, offset, limit int) (*DatasetResponse, error) {
	url := fmt.Sprintf("%s/datasets/%s/items?token=%s&offset=%d&limit=%d",
		c.baseUrl, datasetId, c.apiToken, offset, limit)
	logrus.Debugf("Getting dataset items: %s (offset: %d, limit: %d)", datasetId, offset, limit)

	// Create request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		logrus.Errorf("error creating GET request: %v", err)
		return nil, fmt.Errorf("error creating GET request: %w", err)
	}

	// Make the request
	resp, err := c.options.HttpClient.Do(req)
	if err != nil {
		logrus.Errorf("error making GET request: %v", err)
		return nil, fmt.Errorf("error making GET request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logrus.Errorf("error reading response body: %v", err)
		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	// Check response status
	if resp.StatusCode != http.StatusOK {
		logrus.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	// Parse response - Apify returns a direct array of items, not wrapped in a data object
	var items []json.RawMessage
	if err := json.Unmarshal(body, &items); err != nil {
		logrus.Errorf("error parsing response: %v", err)
		return nil, fmt.Errorf("error parsing response: %w", err)
	}

	// Create a DatasetResponse object with the items and estimated pagination info
	datasetResp := &DatasetResponse{
		Data: struct {
			Items  []json.RawMessage `json:"items"`
			Count  int               `json:"count"`
			Offset int               `json:"offset"`
			Limit  int               `json:"limit"`
			Total  int               `json:"total"`
		}{
			Items:  items,
			Count:  len(items),
			Offset: offset,
			Limit:  limit,
			Total:  offset + len(items), // Estimate total, could be more if limit is reached
		},
	}

	logrus.Debugf("Retrieved %d items from dataset", len(items))
	return datasetResp, nil
}

// ValidateApiKey tests if the API token is valid by making a request to /users/me
// This endpoint doesn't consume any actor runs or quotas - it's perfect for validation
func (c *ApifyClient) ValidateApiKey() error {
	url := fmt.Sprintf("%s/users/me?token=%s", c.baseUrl, c.apiToken)
	logrus.Debug("Testing Apify API token")

	// Create request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		logrus.Errorf("error creating auth test request: %v", err)
		return fmt.Errorf("error creating auth test request: %w", err)
	}

	// Make the request
	resp, err := c.options.HttpClient.Do(req)
	if err != nil {
		logrus.Errorf("error making auth test request: %v", err)
		return fmt.Errorf("error making auth test request: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	switch resp.StatusCode {
	case http.StatusOK:
		logrus.Debug("Apify API token validation successful")
		return nil
	case http.StatusUnauthorized:
		return fmt.Errorf("invalid Apify API token")
	case http.StatusForbidden:
		return fmt.Errorf("insufficient permissions for Apify API token")
	case http.StatusTooManyRequests:
		return fmt.Errorf("rate limit exceeded")
	default:
		return fmt.Errorf("Apify API auth test failed with status: %d", resp.StatusCode)
	}
}
