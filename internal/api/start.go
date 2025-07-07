package api

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/edgelesssys/ego/enclave"
	"github.com/labstack/echo-contrib/pprof"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/labstack/gommon/log"
	"github.com/masa-finance/tee-worker/api/types"
	"github.com/masa-finance/tee-worker/internal/capabilities/health"
	"github.com/masa-finance/tee-worker/internal/jobserver"
	"github.com/masa-finance/tee-worker/pkg/tee"
)

func Start(ctx context.Context, listenAddress, dataDIR string, standalone bool, config types.JobConfiguration, healthTracker health.CapabilityHealthTracker) error {

	// Echo instance
	e := echo.New()

	logLevel := os.Getenv("LOG_LEVEL")

	maxJobs := os.Getenv("MAX_JOBS")

	switch strings.ToLower(logLevel) {
	case "debug":
		e.Logger.SetLevel(log.DEBUG)
	case "info":
		e.Logger.SetLevel(log.INFO)
	case "warn":
		e.Logger.SetLevel(log.WARN)
	case "error":
		e.Logger.SetLevel(log.ERROR)
	default:
		e.Logger.SetLevel(log.INFO)
	}

	var maxJobsInt int
	if maxJobs != "" {
		var err error
		maxJobsInt, err = strconv.Atoi(maxJobs)
		if err != nil {
			e.Logger.Error("Failed to parse MAX_JOBS: ", err)
			return err
		}
	}

	if maxJobsInt == 0 {
		maxJobsInt = 10
		e.Logger.Warn("MAX_JOBS is not set, using default of 10")
	}

	// Jobserver instance
	jobServer := jobserver.NewJobServer(maxJobsInt, config)

	go jobServer.Run(ctx)
	go healthTracker.StartReconciliationLoop(ctx)

	// Initialize health metrics
	healthMetrics := NewHealthMetrics()

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// API Key Authentication Middleware
	e.Use(APIKeyAuthMiddleware(config))

	// Health metrics tracking middleware
	e.Use(HealthMetricsMiddleware(healthMetrics))

	// Initialize empty key ring
	tee.CurrentKeyRing = tee.NewKeyRing()

	// Validate keyring to ensure it doesn't exceed the maximum allowed keys
	if tee.CurrentKeyRing != nil {
		tee.CurrentKeyRing.ValidateAndPrune()
	}

	// Routes

	// Health check endpoints (no auth required)
	e.GET("/healthz", Healthz())
	e.GET("/readyz", Readyz(jobServer, healthMetrics))

	// Set up profiling if allowed
	if ok, p := config["profiling_enabled"].(bool); ok && p {
		_ = enableProfiling(e, standalone)
	}

	if standalone {
		e.Logger.Info("Enabling profiling control endpoints")
		debug := e.Group("/debug/pprof")

		debug.POST("/enable", func(c echo.Context) error {
			if enableProfiling(e, standalone) {
				return c.String(http.StatusOK, "pprof enabled")
			}
			return c.String(http.StatusBadRequest, "pprof not supported")
		})

		debug.POST("/disable", func(c echo.Context) error {
			if disableProfiling(e, standalone) {
				return c.String(http.StatusOK, "pprof disabled")
			}
			return c.String(http.StatusBadRequest, "pprof not supported")
		})
	}

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
		if err := e.Close(); err != nil {
			e.Logger.Error("Failed to close Echo server: ", err)
		}
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

// enableProfiling enables pprof profiling
// In TEE/enclave mode, a warning is displayed but profiling is still enabled
func enableProfiling(e *echo.Echo, standaloneMode bool) bool {
	// Warning if using profiling in TEE mode
	if !standaloneMode {
		e.Logger.Warn("Profiling is not supported in TEE/enclave mode. Not enabling.")
		return false
	}

	e.Logger.Info("Enabling profiling - this may impact performance")

	// TODO: These values should probably come from configuration, and/or be settable at runtime when enabling profiling
	//
	// Sample time in nanoseconds, see https://github.com/DataDog/go-profiler-notes/blob/main/block.md#usage
	runtime.SetBlockProfileRate(500)
	// Fraction of contention events that are reported https://gist.github.com/andrewhodel/ed7625a14eb87404cafd37493849d1ba
	runtime.SetMutexProfileFraction(1)
	// CPU profiling rate samples per second https://gist.github.com/andrewhodel/ed7625a14eb87404cafd37493849d1ba
	runtime.SetCPUProfileRate(30)

	pprof.Register(e)

	return true
}

func disableProfiling(e *echo.Echo, standaloneMode bool) bool {
	if !standaloneMode {
		e.Logger.Warn("Profiling is not supported in TEE/enclave mode.")
		return false
	}

	e.Logger.Info("Disabling performance-intensive profiling probes")

	// Sample time in nanoseconds, see https://github.com/DataDog/go-profiler-notes/blob/main/block.md#usage
	runtime.SetBlockProfileRate(0)
	// Fraction of contention events that are reported https://gist.github.com/andrewhodel/ed7625a14eb87404cafd37493849d1ba
	runtime.SetMutexProfileFraction(0)
	// CPU profiling rate samples per second https://gist.github.com/andrewhodel/ed7625a14eb87404cafd37493849d1ba
	runtime.SetCPUProfileRate(0)

	// TODO: The endpoints remain registered, but the most resource-intensive profiling data collection is disabled. Figure out how to completely unregister (and ideally disable stats gathering)

	return true
}
