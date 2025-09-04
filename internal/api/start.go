package api

import (
	"context"
	"fmt"
	"net/http"
	"runtime"
	"strings"

	"github.com/edgelesssys/ego/enclave"
	"github.com/labstack/echo-contrib/pprof"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/labstack/gommon/log"
	"github.com/masa-finance/tee-worker/internal/config"
	"github.com/masa-finance/tee-worker/internal/jobserver"
	"github.com/masa-finance/tee-worker/pkg/tee"
)

func Start(ctx context.Context, listenAddress, dataDIR string, standalone bool, jc config.JobConfiguration) error {

	// Echo instance
	e := echo.New()

	// Default loglevel
	logLevel := jc.GetString("log_level", "info")
	e.Logger.SetLevel(parseLogLevel(logLevel))

	// Jobserver instance
	maxJobs, _ := jc.GetInt("max_jobs", 10)
	jobServer := jobserver.NewJobServer(maxJobs, jc)

	go jobServer.Run(ctx)

	// Initialize health metrics
	healthMetrics := NewHealthMetrics()

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// API Key Authentication Middleware
	e.Use(APIKeyAuthMiddleware(jc))

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

	debug := e.Group("/debug")
	debug.PUT("/loglevel", func(c echo.Context) error {
		levelStr := c.QueryParam("level")
		if levelStr == "" {
			levelStr = jc.GetString("log_level", "info")
		}

		// Set logrus log level
		logrusLevel := config.ParseLogLevel(levelStr)
		config.SetLogLevel(logrusLevel)

		// Set echo log level
		echoLevel := parseLogLevel(levelStr)
		e.Logger.SetLevel(echoLevel)

		return c.String(http.StatusOK, fmt.Sprintf("log level set to %s", levelStr))
	})

	if standalone {
		// Set up profiling if allowed
		if jc.GetBool("profiling_enabled", false) {
			_ = enableProfiling(e, standalone)
		}

		pprofGroup := debug.Group("/pprof")

		pprofGroup.POST("/enable", func(c echo.Context) error {
			if enableProfiling(e, standalone) {
				return c.String(http.StatusOK, "pprof enabled")
			}
			return c.String(http.StatusBadRequest, "pprof not supported")
		})

		pprofGroup.POST("/disable", func(c echo.Context) error {
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

// parseLogLevel parses a logLevel into a log level appropriate for Echo. This is different from config.ParseLogLevel since that one uses a log level appropriate for logrus.
func parseLogLevel(logLevel string) log.Lvl {
	switch strings.ToLower(logLevel) {
	case "debug":
		return log.DEBUG
	case "info":
		return log.INFO
	case "warn":
		return log.WARN
	case "error":
		return log.ERROR
	default:
		return log.INFO
	}
}

// enableProfiling enables pprof profiling. Only works in standalone mode.
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
