package client

import (
	"fmt"
	"time"
)

type JobResult struct {
	UUID       string
	maxRetries int
	delay      time.Duration
	client     *Client
}

func (jr *JobResult) SetMaxRetries(maxRetries int) {
	jr.maxRetries = maxRetries
}

func (jr *JobResult) SetDelay(delay time.Duration) {
	jr.delay = delay
}

// GetJobResult retrieves the encrypted result of a job.
func (jr *JobResult) getResult() (string, error) {
	return jr.client.GetResult(jr.UUID)
}

// Get polls the server until the job result is ready or a timeout occurs.
func (jr *JobResult) Get() (result string, err error) {
	retries := 0

	for {
		if retries >= jr.maxRetries {
			return "", fmt.Errorf("max retries reached: %w", err)
		}
		retries++

		result, err = jr.getResult()
		if err == nil {
			break
		}
		time.Sleep(jr.delay)
	}

	return
}

// Get polls the server until the job result is ready or a timeout occurs.
func (jr *JobResult) GetDecrypted() (result string, err error) {
	result, err = jr.Get()
	if err == nil {
		result, err = jr.client.Decrypt(result)
	}

	return
}
