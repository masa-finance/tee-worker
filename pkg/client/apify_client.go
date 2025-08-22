package client

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	apifyBaseURL      = "https://api.apify.com/v2"
	MaxActorPolls     = 60              // 5 minutes max wait time
	ActorPollInterval = 5 * time.Second // polling interval between status checks

	// Actor run status constants
	ActorStatusSucceeded = "SUCCEEDED"
	ActorStatusFailed    = "FAILED"
	ActorStatusAborted   = "ABORTED"
)

// Apify provides an interface for interacting with the Apify API.
type Apify interface {
	RunActorAndGetResponse(actorId string, input any, cursor Cursor, limit uint) (*DatasetResponse, Cursor, error)
	ValidateApiKey() error
}

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

// ApifyDatasetData holds the items from an Apify dataset
type ApifyDatasetData struct {
	Items  []json.RawMessage `json:"items"`
	Count  uint              `json:"count"`
	Offset uint              `json:"offset"`
	Limit  uint              `json:"limit"`
}

// DatasetResponse represents the response from getting dataset items
type DatasetResponse struct {
	Data ApifyDatasetData `json:"data"`
}

// CursorData represents the pagination data stored in cursor
type CursorData struct {
	Offset uint `json:"offset"`
}

// Cursor represents an encoded CursorData
type Cursor string

// EmptyCursor represents the state when there is no cursor (i.e. at the start of a fetch loop)
const EmptyCursor Cursor = ""

func (c Cursor) String() string {
	return string(c)
}

// NewApifyClient creates a new Apify client with functional options
func NewApifyClient(apiToken string, opts ...Option) (Apify, error) {
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
func (c *ApifyClient) RunActor(actorId string, input any) (*ActorRunResponse, error) {
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
func (c *ApifyClient) GetDatasetItems(datasetId string, offset, limit uint) (*DatasetResponse, error) {
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
		Data: ApifyDatasetData{
			Items:  items,
			Count:  uint(len(items)),
			Offset: offset,
			Limit:  limit,
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

var (
	ErrActorFailed  = errors.New("Actor run failed")
	ErrActorAborted = errors.New("Actor run aborted")
)

// runActorAndGetProfiles runs the actor and retrieves profiles from the dataset
func (c *ApifyClient) RunActorAndGetResponse(actorId string, input any, cursor Cursor, limit uint) (*DatasetResponse, Cursor, error) {
	var offset uint
	if cursor != EmptyCursor {
		offset = parseCursor(cursor)
	}

	// 1. Run the actor
	runResp, err := c.RunActor(actorId, input)
	if err != nil {
		return nil, "", fmt.Errorf("failed to run actor: %w", err)
	}

	// 2. Poll for completion
	logrus.Infof("Polling for actor run completion: %s", runResp.Data.ID)
	pollCount := 0

PollLoop:
	for {
		status, err := c.GetActorRun(runResp.Data.ID)
		if err != nil {
			return nil, "", fmt.Errorf("failed to get actor run status: %w", err)
		}

		logrus.Debugf("Actor run status: %s", status.Data.Status)

		switch status.Data.Status {
		case ActorStatusSucceeded:
			logrus.Debug("Actor run completed successfully")
			break PollLoop
		case ActorStatusFailed:
			return nil, "", ErrActorFailed
		case ActorStatusAborted:
			return nil, "", ErrActorAborted
		}

		// TODO: Parametrize these two
		pollCount++
		if pollCount >= MaxActorPolls {
			return nil, "", fmt.Errorf("actor run timed out after %d polls", MaxActorPolls)
		}

		time.Sleep(ActorPollInterval)
	}

	// 3. Get dataset items with pagination
	logrus.Infof("Retrieving dataset items from: %s (offset: %d, limit: %d)", runResp.Data.DefaultDatasetId, offset, limit)
	dataset, err := c.GetDatasetItems(runResp.Data.DefaultDatasetId, offset, limit)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get dataset items: %w", err)
	}

	// 4. Generate next cursor if more data may be available
	var nextCursor Cursor
	if uint(len(dataset.Data.Items)) == limit {
		nextOffset := offset + uint(len(dataset.Data.Items))
		nextCursor = generateCursor(nextOffset)
		logrus.Debugf("Generated next cursor for offset %d", nextOffset)
	}

	if uint(len(dataset.Data.Items)) == limit {
		logrus.Infof("Successfully retrieved %d profiles; more may be available", len(dataset.Data.Items))
	} else {
		logrus.Infof("Successfully retrieved %d profiles", len(dataset.Data.Items))
	}
	return dataset, nextCursor, nil
}

// parseCursor decodes a base64 cursor to get the offset
func parseCursor(cursor Cursor) uint {
	if cursor == "" {
		return 0
	}

	decoded, err := base64.StdEncoding.DecodeString(cursor.String())
	if err != nil {
		logrus.Warnf("Failed to decode cursor: %v", err)
		return 0
	}

	var cursorData CursorData
	if err := json.Unmarshal(decoded, &cursorData); err != nil {
		logrus.Warnf("Failed to unmarshal cursor data: %v", err)
		return 0
	}

	return cursorData.Offset
}

// generateCursor encodes an offset as a base64 cursor
func generateCursor(offset uint) Cursor {
	cursorData := CursorData{Offset: offset}
	data, err := json.Marshal(cursorData)
	if err != nil {
		logrus.Warnf("Failed to marshal cursor data: %v", err)
		return ""
	}

	return Cursor(base64.StdEncoding.EncodeToString(data))
}
