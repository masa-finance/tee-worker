package jobserver

import (
	"context"
	"fmt"

	"github.com/masa-finance/tee-worker/api/types"
	"github.com/sirupsen/logrus"
)

// worker is the main entry point for job processing goroutines.
// It automatically selects the appropriate worker implementation based on
// whether the priority queue system is enabled.
//
// This method is called by Run() to start worker goroutines.
func (js *JobServer) worker(c context.Context) {
	if js.usePriorityQueue && js.priorityQueue != nil {
		// Use priority queue mode
		js.priorityQueueWorker(c)
	} else {
		// Use legacy channel mode
		js.legacyWorker(c)
	}
}

// legacyWorker implements the original channel-based worker logic.
// This is used when the priority queue system is disabled.
//
// The worker:
// - Listens on a single shared channel for all jobs
// - Processes jobs in FIFO order
// - Continues until the context is cancelled
//
// This maintains backward compatibility with the original implementation.
func (js *JobServer) legacyWorker(c context.Context) {
	for {
		select {
		case <-c.Done():
			fmt.Println("Context done")
			return

		case j := <-js.jobChan:
			fmt.Println("Job received: ", j)
			if err := js.doWork(j); err != nil {
				logrus.Errorf("Error while executing job %v: %s", j, err)
			}
		}
	}
}

// priorityQueueWorker implements the priority queue-based worker logic.
// This is used when the priority queue system is enabled.
//
// The worker:
// - Always checks the fast queue first for priority jobs
// - Falls back to the slow queue when fast queue is empty
// - Uses blocking dequeue to wait for jobs efficiently
// - Continues until the context is cancelled or queues are closed
//
// This ensures priority jobs are processed before regular jobs,
// improving response times for important workloads.
func (js *JobServer) priorityQueueWorker(c context.Context) {
	for {
		select {
		case <-c.Done():
			logrus.Info("Worker shutting down: context done")
			return
		default:
			// Try to get a job from the priority queue
			job, err := js.priorityQueue.DequeueBlocking()
			if err != nil {
				if err == ErrQueueClosed {
					logrus.Info("Worker shutting down: queue closed")
					return
				}
				// For other errors, log and continue
				logrus.Debugf("Dequeue error: %v", err)
				continue
			}

			if job != nil {
				logrus.Debugf("Job received from priority queue: %s (type: %s, worker: %s)", 
					job.UUID, job.Type, job.WorkerID)
				if err := js.doWork(*job); err != nil {
					logrus.Errorf("Error while executing job %s: %v", job.UUID, err)
				}
			}
		}
	}
}

type worker interface {
	ExecuteJob(j types.Job) (types.JobResult, error)
}

// doWork executes a single job by delegating to the appropriate job type handler.
//
// This method:
// 1. Looks up the handler for the job type
// 2. Locks the handler to prevent concurrent execution of the same type
// 3. Executes the job
// 4. Stores the result in the cache
//
// Returns an error if the job type is unknown or execution fails.
// The error is also stored in the job result for client retrieval.
//
// Note: The mutex locking per job type is a current limitation that
// prevents parallel execution of jobs of the same type.
func (js *JobServer) doWork(j types.Job) error {
	// TODO: Add the job to the cache with the status set to Running
	w, exists := js.jobWorkers[j.Type]

	if !exists {
		js.results.Set(j.UUID, types.JobResult{
			Job:   j,
			Error: fmt.Sprintf("unknown job type: %s", j.Type),
		})
		return fmt.Errorf("unknown job type: %s", j.Type)
	}

	// TODO: Shall we lock the resource or create a new instance each time? Behavior is not defined yet as the only requirements we have is that some scrapers might have rate limits, so we don't want to create a new clients every time. We might use an object pool with a specific capacity, so we have a max number of workers (of each type?) running concurrently. See e.g. https://github.com/jolestar/go-commons-pool or https://github.com/theodesp/go-object-pool.
	w.Lock()
	defer w.Unlock()

	result, err := w.w.ExecuteJob(j)
	if err != nil {
		result.Error = err.Error()
	}

	result.Job = j
	// TODO: If we send the tweet results to the cache as they are generated, we don't need to set the data here, just set the status to Done
	js.results.Set(j.UUID, result)

	return nil
}
