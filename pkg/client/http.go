package client

import (
	"bytes"
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

// setAPIKeyHeader sets the API key on the request if configured.
func (c *Client) setAPIKeyHeader(req *http.Request) {
	if c.options != nil && c.options.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.options.APIKey)
	}
}

// NewClient creates a new Client instance. It will use the given http.Client, or create a new one with the given options if you pass in nil.
func NewClient(baseURL string, httpClient *http.Client, opts ...Option) (*Client, error) {
	options, err := NewOptions(opts...)
	if err != nil {
		return nil, err
	}

	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: options.Timeout,
		}

		t := http.DefaultTransport.(*http.Transport).Clone()
		t.IdleConnTimeout = options.IdleConnTimeout
		t.MaxIdleConns = options.MaxIdleConns
		t.MaxIdleConnsPerHost = options.MaxIdleConnsPerHost
		t.MaxConnsPerHost = options.MaxConnsPerHost
		t.TLSClientConfig.InsecureSkipVerify = options.ignoreTLSCert
		httpClient.Transport = t
	}

	c := &Client{
		BaseURL:    baseURL,
		options:    options,
		HTTPClient: httpClient,
	}

	return c, nil
}

// CreateJobSignature sends a job to the server to generate a job signature.
// The server will attach its worker ID to the job before generating the signature.
func (c *Client) CreateJobSignature(job types.Job) (JobSignature, error) {
	jobJSON, err := json.Marshal(job)
	if err != nil {
		return JobSignature(""), fmt.Errorf("error marshaling job: %w", err)
	}

	req, err := http.NewRequest("POST", c.BaseURL+"/job/generate", bytes.NewBuffer(jobJSON))
	if err != nil {
		return JobSignature(""), fmt.Errorf("error creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	c.setAPIKeyHeader(req)
	resp, err := c.HTTPClient.Do(req)
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

	req, err := http.NewRequest("POST", c.BaseURL+"/job/add", bytes.NewBuffer(jobJSON))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	c.setAPIKeyHeader(req)
	resp, err := c.HTTPClient.Do(req)
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

	req, err := http.NewRequest("POST", c.BaseURL+"/job/result", bytes.NewBuffer(decryptReqJSON))
	if err != nil {
		return "", fmt.Errorf("error creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	c.setAPIKeyHeader(req)
	resp, err := c.HTTPClient.Do(req)
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
	req, err := http.NewRequest("GET", c.BaseURL+"/job/status/"+jobUUID, nil)
	if err != nil {
		return "", false, fmt.Errorf("error creating request: %w", err)
	}
	c.setAPIKeyHeader(req)
	resp, err := c.HTTPClient.Do(req)
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
	// We ignore the error here. We're just interested in unmarshalling if it's an error, otherwise we just return the raw body
	json.Unmarshal(body, &respErr)
	if respErr.Error != "" {
		err = fmt.Errorf("error while getting results of job %s: %s", jobUUID, respErr.Error)
		return "", false, err
	}

	return string(body), true, err
}
