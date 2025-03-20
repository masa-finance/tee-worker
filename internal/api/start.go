package api

import (
	"context"
	"fmt"
	"net/http"

	"github.com/edgelesssys/ego/enclave"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/masa-finance/tee-worker/api/types"
	"github.com/masa-finance/tee-worker/internal/jobserver"
	"github.com/masa-finance/tee-worker/pkg/tee"
)

func Start(ctx context.Context, listenAddress, dataDIR string, standalone bool, config types.JobConfiguration) error {

	// Echo instance
	e := echo.New()

	// Jobserver instance
	jobServer := jobserver.NewJobServer(2, config)

	go jobServer.Run(ctx)

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// Load already existing key
	tee.LoadKey(dataDIR)

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

	fmt.Println("Starting server server/standalone: "+listenAddress, standalone)
	if standalone {
		e.Logger.Info(fmt.Sprintf("Starting server on %s", listenAddress))
		tee.SealStandaloneMode = true
		e.Logger.Error(e.Start(listenAddress))
	} else {
		e.Logger.Info("Starting server in enclave mode")
		// Set the sealing key
		e.POST("/setkey", setKey(dataDIR))

		// Create a TLS config with a self-signed certificate and an embedded report.
		tlsCfg, err := enclave.CreateAttestationServerTLSConfig()
		if err != nil {
			e.Logger.Error("Failed to create TLS config: ", err)
			return err
		}

		e.Logger.Info(fmt.Sprintf("Starting server on %s", listenAddress))
		s := http.Server{
			Addr:      listenAddress,
			Handler:   e, // set Echo as handler
			TLSConfig: tlsCfg,
			//ReadTimeout: 30 * time.Second, // use custom timeouts
		}
		if err := s.ListenAndServeTLS("", ""); err != http.ErrServerClosed {
			e.Logger.Error(err)
			return err
		}
	}

	return nil
}
