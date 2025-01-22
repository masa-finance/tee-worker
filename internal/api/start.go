package api

import (
	"context"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/masa-finance/tee-worker/api/types"
	"github.com/masa-finance/tee-worker/internal/jobserver"
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
		- POST /job/generate: Generate a job payload
		- POST /job/add: Add a job to the queue
		- GET /job/status/:job_id: Get the status of a job
		- POST /job/result: Get the result of a job, decrypt it and return it
	*/
	job := e.Group("/job")
	job.POST("/generate", generate)
	job.POST("/add", add(jobServer))
	job.GET("/status/:job_id", status(jobServer))
	job.POST("/result", result)

	go func() {
		<-ctx.Done()
		e.Close()
	}()

	// Start server
	e.Logger.Error(e.Start(listenAddress))
}
