package jobserver

import (
	"context"
	"errors"
	"sync"

	"github.com/sirupsen/logrus"

	"github.com/google/uuid"
	"github.com/masa-finance/tee-worker/api/types"
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
	return &JobServer{
		jobChan: make(chan types.Job),
		// TODO The defaults here should come from config.go, but during tests the config is not necessarily read
		results:          NewResultCache(jc.GetInt("result_cache_max_size", 1000), jc.GetDuration("result_cache_max_age_seconds", 600)),
		workers:          workers,
		jobConfiguration: jc,
		jobWorkers:       jobworkers,
		executedJobs:     make(map[string]bool),
	}
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
