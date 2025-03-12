package client

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/masa-finance/tee-worker/api/types"
)

// Client represents a client to interact with the job server.
type Client struct {
	BaseURL    string
	options    *Options
	HTTPClient *http.Client
}

// NewClient creates a new Client instance.
func NewClient(baseURL string, opts ...Option) *Client {
	options := NewOptions(opts...)
	c := &Client{
		BaseURL:    baseURL,
		options:    options,
		HTTPClient: &http.Client{
			//Timeout: options.timeout,
		},
	}
	if options.ignoreTLSCert {
		c.HTTPClient.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	}

	return c
}

// SubmitJob submits a new job to the server and returns the job result.
func (c *Client) CreateJobSignature(job types.Job) (JobSignature, error) {
	jobJSON, err := json.Marshal(job)
	if err != nil {
		return JobSignature(""), fmt.Errorf("error marshaling job: %w", err)
	}

	resp, err := c.HTTPClient.Post(c.BaseURL+"/job/generate", "application/json", bytes.NewBuffer(jobJSON))
	if err != nil {
		return JobSignature(""), fmt.Errorf("error sending POST request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return JobSignature(""), fmt.Errorf("error reading response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return JobSignature(""), fmt.Errorf("error: received status code %d, body: %s", resp.StatusCode, string(body))
	}

	return JobSignature(string(body)), nil
}

// SubmitJob submits a new job to the server and returns the job result.
func (c *Client) SubmitJob(JobSignature JobSignature) (*JobResult, error) {
	jr := types.JobRequest{EncryptedJob: string(JobSignature)}

	jobJSON, err := json.Marshal(jr)
	if err != nil {
		return nil, fmt.Errorf("error marshaling job: %w", err)
	}

	resp, err := c.HTTPClient.Post(c.BaseURL+"/job/add", "application/json", bytes.NewBuffer(jobJSON))
	if err != nil {
		return nil, fmt.Errorf("error sending POST request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error: received status code %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	var jobResp types.JobResponse
	err = json.Unmarshal(body, &jobResp)
	if err != nil {
		return nil, fmt.Errorf("error unmarshaling response: %w", err)
	}

	return &JobResult{UUID: jobResp.UID, client: c, maxRetries: 60, delay: 1 * time.Second}, nil
}

// Decrypt sends the encrypted result to the server to decrypt it.
func (c *Client) Decrypt(JobSignature JobSignature, encryptedResult string) (string, error) {
	decryptReq := types.EncryptedRequest{
		EncryptedResult:  encryptedResult,
		EncryptedRequest: string(JobSignature),
	}

	decryptReqJSON, err := json.Marshal(decryptReq)
	if err != nil {
		return "", fmt.Errorf("error marshaling decrypt request: %w", err)
	}

	resp, err := c.HTTPClient.Post(c.BaseURL+"/job/result", "application/json", bytes.NewBuffer(decryptReqJSON))
	if err != nil {
		return "", fmt.Errorf("error sending POST request to /job/result: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("error: received status code %d from /job/result", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response body from /job/result: %w", err)
	}

	return string(body), nil
}

// GetJobResult retrieves the encrypted result of a job.
func (c *Client) GetResult(jobUUID string) (string, bool, error) {
	resp, err := c.HTTPClient.Get(c.BaseURL + "/job/status/" + jobUUID)
	if err != nil {
		return "", false, fmt.Errorf("error sending GET request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", false, fmt.Errorf("error reading response body: %w", err)
	}

	if resp.StatusCode == http.StatusNotFound {
		return "", false, fmt.Errorf("job not found")
	}

	respErr := types.JobError{}
	json.Unmarshal(body, &respErr)
	if respErr.Error != "" {
		err = fmt.Errorf("error: %s", respErr.Error)
		return "", false, err
	}

	return string(body), true, err
}
