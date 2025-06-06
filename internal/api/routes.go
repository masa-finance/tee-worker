package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/masa-finance/tee-worker/api/types"
	"github.com/masa-finance/tee-worker/internal/jobserver"
	"github.com/masa-finance/tee-worker/pkg/tee"
	"github.com/sirupsen/logrus"
)

func generate(c echo.Context) error {
	job := &types.Job{}

	if err := c.Bind(job); err != nil {
		return err
	}

	job.WorkerID = tee.WorkerID // attach worker ID to job

	encryptedSignature, err := job.GenerateJobSignature()
	if err != nil {
		err2 := c.JSON(http.StatusInternalServerError, types.JobError{Error: err.Error()})
		if err2 != nil {
			logrus.Errorf("Error while sending internal server error: %s", err)
			return c.String(http.StatusInternalServerError, fmt.Sprintf("Error generating job signature: %s\n. Additionally, an error when trying to send the error: %s", err.Error(), err2.Error()))
		}
	}

	return c.String(http.StatusOK, encryptedSignature)
}

// add adds a job to the job server.
//
// The request body should contain a JobRequest, which will be decrypted and
// passed to the job server. The response body will contain a JobResponse with
// the UUID of the added job.
//
// If there is an error, the response body will contain a JobError with an
// appropriate error message.
func add(jobServer *jobserver.JobServer) func(c echo.Context) error {
	return func(c echo.Context) error {
		jobRequest := types.JobRequest{}
		if err := c.Bind(&jobRequest); err != nil {
			return err
		}

		job, err := jobRequest.DecryptJob()
		if err != nil {
			return err
		}

		uuid := jobServer.AddJob(*job)

		// check if uuid is empty
		if uuid == "" {
			return c.JSON(http.StatusInternalServerError, types.JobError{Error: "Failed to add job"})
		}

		return c.JSON(http.StatusOK, types.JobResponse{UID: uuid})
	}
}

// status returns the result of a job. If the job is not found, it returns an
// error with a status code of 404. If there is an error with the job, it
// returns an error with a status code of 500. If the job has not finished, it
// returns an empty string with a status code of 200. Otherwise, it returns the
// sealed result of the job with a status code of 200.
func status(jobServer *jobserver.JobServer) func(c echo.Context) error {
	return func(c echo.Context) error {
		res, exists := jobServer.GetJobResult(c.Param("job_id"))
		if !exists {
			return c.JSON(http.StatusNotFound, types.JobError{Error: "Job not found"})
		}

		if res.Error != "" {
			return c.JSON(http.StatusInternalServerError, types.JobError{Error: res.Error})
		}

		sealedData, err := res.Seal()
		if err != nil {
			return err
		}

		return c.String(http.StatusOK, sealedData)

	}
}

func result(c echo.Context) error {
	payload := types.EncryptedRequest{
		EncryptedResult:  "",
		EncryptedRequest: "",
	}

	if err := c.Bind(&payload); err != nil {
		return err
	}

	result, err := payload.Unseal()
	if err != nil {
		return err
	}

	return c.String(http.StatusOK, result)
}

func setKey(dataDir string) func(c echo.Context) error {
	return func(c echo.Context) error {
		key := &types.Key{}
		if err := c.Bind(key); err != nil {
			return err
		}

		if err := tee.SetKey(dataDir, key.Key, key.Signature); err != nil {
			return c.JSON(http.StatusInternalServerError, types.KeyResponse{Status: err.Error()})
		}

		return c.JSON(http.StatusOK, types.KeyResponse{Status: "Key set"})
	}
}

// queueStats returns the current queue statistics for monitoring the priority queue system.
//
// GET /job/queue/stats
//
// Response format:
//   {
//     "enabled": true,             // Whether priority queue is enabled
//     "fast_queue_depth": 10,      // Current number of jobs waiting in fast queue
//     "slow_queue_depth": 45,      // Current number of jobs waiting in slow queue  
//     "fast_processed": 1234,      // Total jobs processed from fast queue
//     "slow_processed": 5678,      // Total jobs processed from slow queue
//     "last_update": "2024-01-15T10:30:00Z"  // ISO8601 timestamp or null
//   }
//
// The response always includes all fields for consistent client parsing.
//
// This endpoint is useful for:
// - Monitoring queue health and performance
// - Detecting queue backlogs
// - Verifying priority routing is working correctly
// - Calculating processing rates
func queueStats(jobServer *jobserver.JobServer) func(c echo.Context) error {
	return func(c echo.Context) error {
		stats := jobServer.GetQueueStats()
		if stats == nil {
			// Return consistent schema even when disabled
			return c.JSON(http.StatusOK, map[string]interface{}{
				"enabled":          false,
				"fast_queue_depth": 0,
				"slow_queue_depth": 0,
				"fast_processed":   0,
				"slow_processed":   0,
				"last_update":      nil, // Use nil for JSON null
			})
		}
		
		// Format timestamp as ISO8601 string or null if zero
		var lastUpdate interface{} = nil
		if !stats.LastUpdateTime.IsZero() {
			lastUpdate = stats.LastUpdateTime.Format(time.RFC3339)
		}
		
		return c.JSON(http.StatusOK, map[string]interface{}{
			"enabled":          true,
			"fast_queue_depth": stats.FastQueueDepth,
			"slow_queue_depth": stats.SlowQueueDepth,
			"fast_processed":   stats.FastProcessed,
			"slow_processed":   stats.SlowProcessed,
			"last_update":      lastUpdate,
		})
	}
}
