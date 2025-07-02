package jobserver

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"

	"github.com/google/uuid"
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
	jobConfiguration types.JobConfiguration

	jobWorkers   map[string]*jobWorkerEntry
	executedJobs map[string]bool
}

type jobWorkerEntry struct {
	w worker
	sync.Mutex
}

func NewJobServer(workers int, jc types.JobConfiguration) *JobServer {
	logrus.Info("Initializing JobServer...")

	// Validate and set worker count
	if workers <= 0 {
		logrus.Infof("Invalid worker count (%d), defaulting to 1 worker.", workers)
		workers = 1
	} else {
		logrus.Infof("Setting worker count to %d.", workers)
	}

	// Retrieve and set buffer size for stats collector
	bufSize, ok := jc["stats_buf_size"].(uint)
	if !ok {
		logrus.Info("stats_buf_size not provided or invalid in JobConfiguration. Defaulting to 128.")
		bufSize = 128
	} else {
		logrus.Infof("Using stats_buf_size: %d.", bufSize)
	}

	// Start stats collector
	logrus.Info("Starting stats collector...")
	s := stats.StartCollector(bufSize, jc)
	logrus.Info("Stats collector started successfully.")

	// Set worker ID in stats collector if available
	if workerID, ok := jc["worker_id"].(string); ok && workerID != "" {
		logrus.Infof("Setting worker ID to '%s' in stats collector.", workerID)
		s.SetWorkerID(workerID)
	} else {
		logrus.Info("No worker ID found in JobConfiguration.")
	}

	// Initialize job workers
	logrus.Info("Setting up job workers...")
	jobworkers := map[string]*jobWorkerEntry{
		jobs.WebScraperType: {
			w: jobs.NewWebScraper(jc, s),
		},
		jobs.TwitterScraperType: {
			w: jobs.NewTwitterScraper(jc, s),
		},
		jobs.TwitterCredentialScraperType: {
			w: jobs.NewTwitterScraper(jc, s), // Uses the same implementation as standard Twitter scraper
		},
		jobs.TwitterApiScraperType: {
			w: jobs.NewTwitterScraper(jc, s), // Uses the same implementation as standard Twitter scraper
		},
		jobs.TelemetryJobType: {
			w: jobs.NewTelemetryJob(jc, s),
		},
		jobs.TikTokTranscriptionType: {
			w: jobs.NewTikTokTranscriber(jc, s),
		},
	}
	logrus.Infof("Initialized job worker for: %s", jobs.WebScraperType)
	logrus.Infof("Initialized job worker for: %s", jobs.TwitterScraperType)
	logrus.Infof("Initialized job worker for: %s", jobs.TwitterCredentialScraperType)
	logrus.Infof("Initialized job worker for: %s", jobs.TwitterApiScraperType)
	logrus.Infof("Initialized job worker for: %s", jobs.TelemetryJobType)
	logrus.Infof("Initialized job worker for: %s", jobs.TikTokTranscriptionType)

	logrus.Info("Job workers setup completed.")

	// Return the JobServer instance
	logrus.Info("JobServer initialization complete.")
	js := &JobServer{
		jobChan: make(chan types.Job),
		// TODO The defaults here should come from config.go, but during tests the config is not necessarily read
		results:          NewResultCache(jc.GetInt("result_cache_max_size", 1000), jc.GetDuration("result_cache_max_age_seconds", 600)),
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

// CapabilityProvider is an interface for workers that can report their capabilities
type CapabilityProvider interface {
	GetCapabilities() []string
}

// GetWorkerCapabilities returns the capabilities for all registered workers
func (js *JobServer) GetWorkerCapabilities() map[string][]string {
	capabilities := make(map[string][]string)

	for workerType, workerEntry := range js.jobWorkers {
		if provider, ok := workerEntry.w.(CapabilityProvider); ok {
			capabilities[workerType] = provider.GetCapabilities()
		}
	}

	return capabilities
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

	if j.Type != jobs.TelemetryJobType && config.MinersWhiteList != "" {
		logrus.Debugf("Checking if job from miner %s is whitelisted. Miners white list: %s", j.WorkerID, config.MinersWhiteList)

		var miners []string

		// In standalone mode, we just whitelist ourselves
		if !config.StandaloneMode() {
			miners = []string{tee.WorkerID}
		} else {
			miners = strings.Split(config.MinersWhiteList, ",")
		}

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
