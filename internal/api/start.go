package api

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/edgelesssys/ego/enclave"
	"github.com/labstack/echo-contrib/pprof"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/masa-finance/tee-worker/api/types"
	"github.com/masa-finance/tee-worker/internal/jobserver"
	"github.com/masa-finance/tee-worker/pkg/tee"
)

func Start(ctx context.Context, listenAddress, dataDIR string, standalone bool, config types.JobConfiguration) error {

	// Echo instance
	e := echo.New()

	// Set up profiling
	if ok, p := config["profiling_enabled"].(bool); ok && p {
		enableProfiling(e)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGUSR1)
	go func(e *echo.Echo) {
		s := <-sig
		switch s {
		case syscall.SIGUSR1:
			enableProfiling(e)
		case syscall.SIGUSR2:
			disableProfiling(e)
		}
	}(e)

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

var profilingRegistered bool = false

func enableProfiling(e *echo.Echo) {
	e.Logger.Info("Enabling profiling")
	// TODO These values should probably come from configuration, and/or be settable at runtime when enabling profiling
	// Sample time in nanoseconds, see https://github.com/DataDog/go-profiler-notes/blob/main/block.md#usage
	runtime.SetBlockProfileRate(500)
	// Fraction of contention events that are reported https://gist.github.com/andrewhodel/ed7625a14eb87404cafd37493849d1ba
	runtime.SetMutexProfileFraction(1)
	// CPU profiling rate samples per second https://gist.github.com/andrewhodel/ed7625a14eb87404cafd37493849d1ba
	runtime.SetCPUProfileRate(30)

	if !profilingRegistered {
		pprof.Register(e)
	}
	profilingRegistered = true
}

func disableProfiling(e *echo.Echo) {
	e.Logger.Info("Disabling profiling")
	// Sample time in nanoseconds, see https://github.com/DataDog/go-profiler-notes/blob/main/block.md#usage
	runtime.SetBlockProfileRate(0)
	// Fraction of contention events that are reported https://gist.github.com/andrewhodel/ed7625a14eb87404cafd37493849d1ba
	runtime.SetMutexProfileFraction(0)
	// CPU profiling rate samples per second https://gist.github.com/andrewhodel/ed7625a14eb87404cafd37493849d1ba
	runtime.SetCPUProfileRate(0)

	// TODO: Figure out how to completely unregister (and ideally disable stats gathering)
}
