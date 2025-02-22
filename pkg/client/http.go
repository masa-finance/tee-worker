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
	HTTPClient *http.Client
}

// NewClient creates a new Client instance.
func NewClient(baseURL string) *Client {
	return &Client{
		BaseURL:    baseURL,
		HTTPClient: &http.Client{},
	}
}

// CreateJobSignature generates a signature for a job.
//
// The function marshals the job, sends a POST request to /job/generate, and
// returns the response body as a JobSignature. If the request fails or the
// response is not a 200, an error is returned.
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

	if resp.StatusCode != http.StatusOK {
		return JobSignature(""), fmt.Errorf("error: received status code %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return JobSignature(""), fmt.Errorf("error reading response body: %w", err)
	}

	return JobSignature(string(body)), nil
}

// SubmitJob submits a new job to the server and returns the job result.
//
// The JobSignature is marshaled into a JSON payload and sent to the server via
// a POST request to the "/job/add" endpoint. The response is expected to be a
// JSON payload containing a JobResponse with the UUID of the added job. If the
// request fails, an error is returned with a descriptive error message.
//
// The function will retry up to 60 times with a 1 second delay between retries
// if the job result is not available yet. If the job result is available, the
// function returns a JobResult with the UUID of the job and the client instance.
// If the job result is not available after 60 retries, an error is returned.
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

// Decrypt decrypts a job result given a JobSignature and the encrypted result.
//
// Decrypt sends a POST request to /job/result with a JSON body containing the
// encrypted result and the encrypted job request. If the request is successful,
// the decrypted result is returned. If the request fails, an error is returned.
//
// The function retries the request up to 60 times with a delay of 1 second
// between each retry if the server returns a status code other than 200.
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

// GetResult gets the result of a job. If the job has not finished, an empty
// string is returned with a success status of false. If the job does not exist,
// an error is returned with a status code of 404. If there is an error with the
// job, an error is returned with a status code of 500. If the job has finished,
// the result is returned with a success status of true.
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
