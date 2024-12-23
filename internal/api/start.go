package api

import (
	"context"
	"encoding/base64"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/masa-finance/tee-worker/api/types"
	"github.com/masa-finance/tee-worker/internal/jobs"
	"github.com/masa-finance/tee-worker/internal/jobserver"
	"github.com/masa-finance/tee-worker/pkg/tee"
)

func Start(ctx context.Context, listenAddress string, config types.JobConfiguration) {
	// Echo instance
	e := echo.New()

	// Jobserver instance
	jobServer := jobserver.NewJobServer(2, config)

	go jobServer.Run(ctx)

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// Routes
	/*
		- /job - POST - to send a job request
		  A job request has a type (string) and a map[string]interface{} as parameter.
		- /job/{job_id} - GET - to get the status of a job
		- /decrypt - POST - to decrypt a message
		  Decripts a message. Takes two parameters: the encrypted result and the encrypted request (both strings)

	*/

	/*
			curl localhost:8080/job -H "Content-Type: application/json" -d '{
		   "type": "webscraper", "arguments": { "url": "google" }
		   }'

	*/

	e.POST("/job", func(c echo.Context) error {
		job := types.Job{}
		if err := c.Bind(&job); err != nil {
			return err
		}

		uuid := jobServer.AddJob(job)

		return c.JSON(http.StatusOK, types.JobResponse{UID: uuid})
	})

	/*
		curl localhost:8080/job/b678ff77-118d-4a7a-a6ea-190eb850c28a
	*/

	e.GET("/job/:job_id", func(c echo.Context) error {
		res, exists := jobServer.GetJobResult(c.Param("job_id"))
		if !exists {
			return c.JSON(http.StatusNotFound, types.JobError{Error: "Job not found"})
		}

		if res.Error != "" {
			return c.JSON(http.StatusInternalServerError, types.JobError{Error: res.Error})
		}

		sealedData, err := tee.Seal(res.Data)
		if err != nil {
			return err
		}

		b64 := base64.StdEncoding.EncodeToString(sealedData)

		return c.String(http.StatusOK, b64)
	})

	/*
		curl localhost:8080/decrypt -H "Content-Type: application/json" -d '{ "encrypted_result": "'$result'" }'

	*/

	e.POST("/decrypt", func(c echo.Context) error {
		payload := types.EncryptedRequest{
			EncryptedResult: "",
		}

		if err := c.Bind(&payload); err != nil {
			return err
		}

		b64, err := base64.StdEncoding.DecodeString(payload.EncryptedResult)
		if err != nil {
			return err
		}

		dat, err := tee.Unseal(b64)
		if err != nil {
			return err
		}

		return c.String(http.StatusOK, string(dat))
	})

	go func() {
		<-ctx.Done()
		err := e.Close()
		if err != nil {
			e.Logger.Errorf("error closing server: %v", err)
		}
	}()

	e.GET("/status/twitter", func(c echo.Context) error {
		worker, exists := jobServer.GetWorker(jobs.TwitterScraperType)
		if !exists {
			return c.JSON(http.StatusNotFound, types.JobError{Error: "Twitter worker not found"})
		}

		status := worker.Status()
		return c.String(http.StatusOK, status)
	})

	// Start server
	e.Logger.Error(e.Start(listenAddress))
}
