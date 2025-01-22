package api

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/masa-finance/tee-worker/api/types"
	"github.com/masa-finance/tee-worker/internal/jobserver"
)

func generate(c echo.Context) error {
	job := &types.Job{}
	if err := c.Bind(job); err != nil {
		return err
	}

	encryptedSignature, err := job.GenerateJobSignature()
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.JobError{Error: err.Error()})
	}

	return c.String(http.StatusOK, encryptedSignature)
}

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

		return c.JSON(http.StatusOK, types.JobResponse{UID: uuid})
	}
}

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
