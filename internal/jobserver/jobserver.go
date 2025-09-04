package jobserver

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"
	"golang.org/x/exp/maps"

	"github.com/google/uuid"
	teetypes "github.com/masa-finance/tee-types/types"
	"github.com/masa-finance/tee-worker/api/types"
	"github.com/masa-finance/tee-worker/internal/config"
	"github.com/masa-finance/tee-worker/internal/jobs"
	"github.com/masa-finance/tee-worker/internal/jobs/stats"
	"github.com/masa-finance/tee-worker/pkg/tee"
)

type JobServer struct {
	sync.Mutex

	jobChan chan types.Job
	workers int

	results          *ResultCache
	jobConfiguration config.JobConfiguration

	jobWorkers   map[teetypes.JobType]*jobWorkerEntry
	executedJobs map[string]bool
}

type jobWorkerEntry struct {
	w worker
	sync.Mutex
}

func NewJobServer(workers int, jc config.JobConfiguration) *JobServer {
	logrus.Info("Initializing JobServer...")

	// Validate and set worker count
	if workers <= 0 {
		logrus.Infof("Invalid worker count (%d), defaulting to 1 worker.", workers)
		workers = 1
	} else {
		logrus.Infof("Setting worker count to %d.", workers)
	}

	// Retrieve and set buffer size for stats collector
	bufSizeInt, err := jc.GetInt("stats_buf_size", 128)
	var bufSize uint
	if err != nil || bufSizeInt <= 0 {
		if err != nil {
			logrus.Errorf("Invalid stats_buf_size config: %v", err)
		} else {
			logrus.Errorf("stats_buf_size must be positive: %d", bufSizeInt)
		}
		bufSize = 128
		logrus.Infof("Using default stats_buf_size: %d.", bufSize)
	} else {
		bufSize = uint(bufSizeInt)
		logrus.Infof("Using stats_buf_size: %d.", bufSize)
	}

	// Start stats collector
	logrus.Info("Starting stats collector...")
	s := stats.StartCollector(bufSize, jc)
	logrus.Info("Stats collector started successfully.")

	// Set worker ID in stats collector if available
	workerID := jc.GetString("worker_id", "")
	if workerID != "" {
		logrus.Infof("Setting worker ID to '%s' in stats collector.", workerID)
		s.SetWorkerID(workerID)
	} else {
		logrus.Info("No worker ID found in JobConfiguration.")
	}

	// Initialize job workers
	logrus.Info("Setting up job workers...")
	jobworkers := map[teetypes.JobType]*jobWorkerEntry{
		teetypes.WebJob: {
			w: jobs.NewWebScraper(jc, s),
		},
		teetypes.TwitterJob: {
			w: jobs.NewTwitterScraper(jc, s),
		},
		teetypes.TwitterCredentialJob: {
			w: jobs.NewTwitterScraper(jc, s), // Uses the same implementation as standard Twitter scraper
		},
		teetypes.TwitterApiJob: {
			w: jobs.NewTwitterScraper(jc, s), // Uses the same implementation as standard Twitter scraper
		},
		teetypes.TwitterApifyJob: {
			w: jobs.NewTwitterScraper(jc, s), // Register Apify job type with Twitter scraper
		},
		teetypes.TiktokJob: {
			w: jobs.NewTikTokScraper(jc, s),
		},
		teetypes.RedditJob: {
			w: jobs.NewRedditScraper(jc, s),
		},
		teetypes.TelemetryJob: {
			w: jobs.NewTelemetryJob(jc, s),
		},
	}
	// Validate that all workers were initialized successfully
	for jobType, workerEntry := range jobworkers {
		if workerEntry.w == nil {
			logrus.Errorf("Failed to initialize worker for job type: %s. This worker will not be available.", jobType)
			// Remove the nil worker from the map to prevent runtime issues
			delete(jobworkers, jobType)
		} else {
			logrus.Infof("Successfully initialized job worker for: %s", jobType)
		}
	}

	if len(jobworkers) == 0 {
		logrus.Error("No job workers were successfully initialized!")
	}

	logrus.Info("Job workers setup completed.")

	// Return the JobServer instance
	logrus.Info("JobServer initialization complete.")

	// Get result cache max size with error handling
	resultCacheMaxSize, err := jc.GetInt("result_cache_max_size", 1000)
	if err != nil {
		logrus.Errorf("Invalid result_cache_max_size config: %v", err)
		resultCacheMaxSize = 1000
	}

	js := &JobServer{
		jobChan: make(chan types.Job),
		// TODO The defaults here should come from config.go, but during tests the config is not necessarily read
		results:          NewResultCache(resultCacheMaxSize, jc.GetDuration("result_cache_max_age_seconds", 600)),
		workers:          workers,
		jobConfiguration: jc,
		jobWorkers:       jobworkers,
		executedJobs:     make(map[string]bool),
	}

	// Set the JobServer reference in the stats collector for capability reporting
	if s != nil {
		s.SetJobServer(js)
	}

	return js
}

// GetWorkerCapabilities returns the structured capabilities for all registered workers
func (js *JobServer) GetWorkerCapabilities() teetypes.WorkerCapabilities {
	// Use a map to deduplicate capabilities by job type
	jobTypeCapMap := make(map[teetypes.JobType]map[teetypes.Capability]struct{})

	for _, workerEntry := range js.jobWorkers {
		workerCapabilities := workerEntry.w.GetStructuredCapabilities()
		for jobType, capabilities := range workerCapabilities {
			if _, exists := jobTypeCapMap[jobType]; !exists {
				jobTypeCapMap[jobType] = make(map[teetypes.Capability]struct{})
			}
			for _, capability := range capabilities {
				jobTypeCapMap[jobType][capability] = struct{}{}
			}
		}
	}

	// Convert to final map format
	allCapabilities := make(teetypes.WorkerCapabilities)
	for jobType, capabilitySet := range jobTypeCapMap {
		capabilities := maps.Keys(capabilitySet)
		allCapabilities[jobType] = capabilities
	}

	return allCapabilities
}

func (js *JobServer) Run(ctx context.Context) {
	for i := 0; i < js.workers; i++ {
		go js.worker(ctx)
	}

	<-ctx.Done()
}

func (js *JobServer) AddJob(j types.Job) (string, error) {
	js.Lock()
	defer js.Unlock()

	if _, ok := js.executedJobs[j.Nonce]; ok {
		return "", errors.New("job already executed")
	}

	js.executedJobs[j.Nonce] = true

	if j.TargetWorker != "" && j.TargetWorker != tee.WorkerID {
		return "", errors.New("this job is not for this worker")
	}

	if j.Type != teetypes.TelemetryJob && config.MinersWhiteList != "" {
		var miners []string

		// In standalone mode, we just whitelist ourselves
		if js.jobConfiguration.IsStandaloneMode() {
			miners = []string{tee.WorkerID}
		} else {
			miners = strings.Split(config.MinersWhiteList, ",")
		}

		logrus.Debugf("Checking if job from miner %s is whitelisted. Miners white list: %+v", j.WorkerID, miners)

		if !slices.Contains(miners, j.WorkerID) {
			logrus.Debugf("Job from non-whitelisted miner %s", j.WorkerID)
			return "", errors.New("this job is not from a whitelisted miner")
		}
		logrus.Debugf("Job from whitelisted miner %s", j.WorkerID)
	}

	// TODO The default should come from config.go, but during tests the config is not necessarily read
	j.Timeout = js.jobConfiguration.GetDuration("job_timeout_seconds", 300)

	jobUUID := uuid.New().String()
	j.UUID = jobUUID

	go func() {
		js.jobChan <- j
	}()

	return jobUUID, nil
}

func (js *JobServer) GetJobResult(uuid string) (types.JobResult, bool) {
	return js.results.Get(uuid)
}
