package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/masa-finance/tee-worker/api/types"
)

// Client represents a client to interact with the job server.
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
}

// NewClient creates a new Client instance.
func NewClient(baseURL string) *Client {
	return &Client{
		BaseURL:    baseURL,
		HTTPClient: &http.Client{},
	}
}

// SubmitJob submits a new job to the server and returns the job UID.
func (c *Client) SubmitJob(job types.Job) (string, error) {
	jobJSON, err := json.Marshal(job)
	if err != nil {
		return "", fmt.Errorf("error marshaling job: %w", err)
	}

	resp, err := c.HTTPClient.Post(c.BaseURL+"/job", "application/json", bytes.NewBuffer(jobJSON))
	if err != nil {
		return "", fmt.Errorf("error sending POST request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("error: received status code %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response body: %w", err)
	}

	var jobResp types.JobResponse
	err = json.Unmarshal(body, &jobResp)
	if err != nil {
		return "", fmt.Errorf("error unmarshaling response: %w", err)
	}

	return jobResp.UID, nil
}

// GetJobResult retrieves the encrypted result of a job.
func (c *Client) GetJobResult(jobID string) (string, error) {
	resp, err := c.HTTPClient.Get(c.BaseURL + "/job/" + jobID)
	if err != nil {
		return "", fmt.Errorf("error sending GET request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response body: %w", err)
	}

	if resp.StatusCode == http.StatusNotFound {
		err = fmt.Errorf("job not found or not ready")
	}

	if resp.StatusCode != http.StatusOK {
		respErr := types.JobError{}
		json.Unmarshal(body, &respErr)
		err = fmt.Errorf("error: %s", respErr.Error)
	}

	return string(body), err
}

// DecryptResult sends the encrypted result to the server to decrypt it.
func (c *Client) DecryptResult(encryptedResult string) (string, error) {
	decryptReq := types.EncryptedRequest{
		EncryptedResult: encryptedResult,
	}

	decryptReqJSON, err := json.Marshal(decryptReq)
	if err != nil {
		return "", fmt.Errorf("error marshaling decrypt request: %w", err)
	}

	resp, err := c.HTTPClient.Post(c.BaseURL+"/decrypt", "application/json", bytes.NewBuffer(decryptReqJSON))
	if err != nil {
		return "", fmt.Errorf("error sending POST request to /decrypt: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("error: received status code %d from /decrypt", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response body from /decrypt: %w", err)
	}

	return string(body), nil
}

// WaitForResult polls the server until the job result is ready or a timeout occurs.
func (c *Client) WaitForResult(jobID string, maxRetries int, delay time.Duration) (result string, err error) {
	retries := 0

	for {
		if retries >= maxRetries {
			return "", errors.New("max retries reached")
		}
		retries++

		result, err = c.GetJobResult(jobID)
		if err == nil {
			break
		}
	}

	return
}
