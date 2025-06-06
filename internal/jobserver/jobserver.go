package jobserver

import (
	"context"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/google/uuid"
	"github.com/masa-finance/tee-worker/api/types"
	"github.com/masa-finance/tee-worker/internal/jobs"
	"github.com/masa-finance/tee-worker/internal/jobs/stats"
)

type JobServer struct {
	sync.Mutex

	jobChan chan types.Job // Legacy channel for backward compatibility
	workers int

	results          *ResultCache
	jobConfiguration types.JobConfiguration

	jobWorkers map[string]*jobWorkerEntry

	// Priority queue system
	priorityQueue   *PriorityQueue
	priorityManager *PriorityManager
	usePriorityQueue bool
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
	}
	logrus.Infof("Initialized job worker for: %s", jobs.WebScraperType)
	logrus.Infof("Initialized job worker for: %s", jobs.TwitterScraperType)
	logrus.Infof("Initialized job worker for: %s", jobs.TwitterCredentialScraperType)
	logrus.Infof("Initialized job worker for: %s", jobs.TwitterApiScraperType)
	logrus.Infof("Initialized job worker for: %s", jobs.TelemetryJobType)

	logrus.Info("Job workers setup completed.")

	// Initialize priority queue system if enabled
	var priorityQueue *PriorityQueue
	var priorityManager *PriorityManager
	usePriorityQueue := jc.GetBool("enable_priority_queue", true)
	
	if usePriorityQueue {
		logrus.Info("Initializing priority queue system...")
		
		// Create priority queue with configurable sizes
		fastQueueSize := jc.GetInt("fast_queue_size", 1000)
		slowQueueSize := jc.GetInt("slow_queue_size", 5000)
		priorityQueue = NewPriorityQueue(fastQueueSize, slowQueueSize)
		
		// Create priority manager
		externalWorkerIDPriorityEndpoint := jc.GetString("external_worker_id_priority_endpoint", "")
		// Default to 15 minutes (900 seconds) if not specified
		refreshIntervalSecs := jc.GetInt("priority_refresh_interval_seconds", 900)
		refreshInterval := time.Duration(refreshIntervalSecs) * time.Second
		priorityManager = NewPriorityManager(externalWorkerIDPriorityEndpoint, refreshInterval)
		
		logrus.Infof("Priority queue initialized (fast: %d, slow: %d)", fastQueueSize, slowQueueSize)
		if externalWorkerIDPriorityEndpoint != "" {
			logrus.Infof("External worker ID priority endpoint: %s (refresh every %v)", externalWorkerIDPriorityEndpoint, refreshInterval)
		} else {
			logrus.Info("Using dummy priority list (no external worker ID priority endpoint configured)")
		}
	}

	// Return the JobServer instance
	logrus.Info("JobServer initialization complete.")
	return &JobServer{
		jobChan: make(chan types.Job),
		// TODO The defaults here should come from config.go, but during tests the config is not necessarily read
		results: NewResultCache(jc.GetInt("result_cache_max_size", 1000), jc.GetDuration("result_cache_max_age_seconds", 600)),
		workers:          workers,
		jobConfiguration: jc,
		jobWorkers:       jobworkers,
		priorityQueue:    priorityQueue,
		priorityManager:  priorityManager,
		usePriorityQueue: usePriorityQueue,
	}
}

func (js *JobServer) Run(ctx context.Context) {
	for i := 0; i < js.workers; i++ {
		go js.worker(ctx)
	}

	<-ctx.Done()
}

// AddJob submits a new job to the job server for processing.
//
// This method:
// 1. Assigns a unique UUID to the job
// 2. Sets the job timeout from configuration
// 3. Routes the job to the appropriate queue based on the job's WorkerID:
//    - If priority queue is enabled AND the job's WorkerID is in the priority list → fast queue
//    - Otherwise → slow queue (or legacy channel if priority queue is disabled)
//
// The job's WorkerID represents the ID of the worker that submitted this job,
// NOT the ID of this tee-worker instance.
//
// Returns the assigned UUID for tracking the job status.
// The actual job processing happens asynchronously.
func (js *JobServer) AddJob(j types.Job) string {
	// TODO The default should come from config.go, but during tests the config is not necessarily read
	j.Timeout = js.jobConfiguration.GetDuration("job_timeout_seconds", 300)
	j.UUID = uuid.New().String()
	
	if js.usePriorityQueue && js.priorityQueue != nil {
		// Route job based on worker ID priority
		go func() {
			jobCopy := j // Create a copy to avoid data races
			if js.priorityManager.IsPriorityWorker(jobCopy.WorkerID) {
				if err := js.priorityQueue.EnqueueFast(&jobCopy); err != nil {
					logrus.Warnf("Failed to enqueue to fast queue: %v, trying slow queue", err)
					if err := js.priorityQueue.EnqueueSlow(&jobCopy); err != nil {
						logrus.Errorf("Failed to enqueue job %s: %v", jobCopy.UUID, err)
					}
				}
			} else {
				if err := js.priorityQueue.EnqueueSlow(&jobCopy); err != nil {
					logrus.Errorf("Failed to enqueue job %s to slow queue: %v", jobCopy.UUID, err)
				}
			}
		}()
	} else {
		// Use legacy channel-based approach
		go func() {
			js.jobChan <- j
		}()
	}
	
	return j.UUID
}

func (js *JobServer) GetJobResult(uuid string) (types.JobResult, bool) {
	return js.results.Get(uuid)
}

// GetQueueStats returns real-time statistics about the priority queue system.
//
// Returns:
// - Queue depths (number of pending jobs in each queue)
// - Processing counts (total jobs processed from each queue)
// - Last update timestamp
//
// Returns nil if the priority queue system is disabled.
//
// This method is useful for monitoring system performance and queue health.
// It can be called frequently without significant performance impact.
func (js *JobServer) GetQueueStats() *QueueStats {
	if !js.usePriorityQueue || js.priorityQueue == nil {
		return nil
	}
	stats := js.priorityQueue.GetStats()
	return &stats
}

// GetPriorityWorkers returns the current list of worker IDs that have priority status.
//
// These are the worker IDs whose jobs will be routed to the fast queue
// for expedited processing.
//
// Returns an empty slice if:
// - Priority queue system is disabled
// - No priority workers are configured
//
// The returned list reflects the most recent update from the external endpoint
// or the default dummy data if no endpoint is configured.
func (js *JobServer) GetPriorityWorkers() []string {
	if !js.usePriorityQueue || js.priorityManager == nil {
		return []string{}
	}
	return js.priorityManager.GetPriorityWorkers()
}

// Shutdown performs a graceful shutdown of the job server.
//
// This method:
// 1. Stops the priority manager's background refresh goroutine
// 2. Closes the priority queues (preventing new job submissions)
// 3. Allows existing jobs to complete processing
//
// This method should be called when the application is shutting down
// to ensure proper cleanup of resources.
//
// Note: This does not cancel running jobs or wait for them to complete.
// Use context cancellation in Run() for immediate shutdown.
func (js *JobServer) Shutdown() {
	if js.priorityManager != nil {
		js.priorityManager.Stop()
	}
	if js.priorityQueue != nil {
		js.priorityQueue.Close()
	}
}
