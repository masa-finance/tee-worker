package jobserver

import (
	"context"
	"sync"

	"github.com/google/uuid"

	"github.com/masa-finance/tee-worker/api/types"
	"github.com/masa-finance/tee-worker/internal/jobs"
)

type JobServer struct {
	sync.Mutex

	jobChan chan types.Job
	workers int

	results          map[string]types.JobResult
	jobConfiguration types.JobConfiguration

	jobWorkers map[string]*jobWorkerEntry
}

type jobWorkerEntry struct {
	w Worker
	sync.Mutex
}

func NewJobServer(workers int, jc types.JobConfiguration) *JobServer {
	if workers == 0 {
		workers++
	}

	jobworkers := make(map[string]*jobWorkerEntry)

	for _, t := range []string{jobs.WebScraperType, jobs.TwitterScraperType} {
		switch t {
		case jobs.WebScraperType:
			jobworkers[jobs.WebScraperType] = &jobWorkerEntry{
				w: jobs.NewWebScraper(jc),
			}
		case jobs.TwitterScraperType:
			jobworkers[jobs.TwitterScraperType] = &jobWorkerEntry{
				w: jobs.NewTwitterScraper(jc),
			}
		}
	}

	return &JobServer{
		jobChan:          make(chan types.Job),
		results:          make(map[string]types.JobResult),
		workers:          workers,
		jobConfiguration: jc,
		jobWorkers:       jobworkers,
	}
}

func (js *JobServer) Run(ctx context.Context) {
	for i := 0; i < js.workers; i++ {
		go js.worker(ctx)
	}

	<-ctx.Done()
}

func (js *JobServer) AddJob(j types.Job) string {
	j.UUID = uuid.New().String()
	defer func() {
		go func() {
			js.jobChan <- j
		}()
	}()
	return j.UUID
}

func (js *JobServer) GetJobResult(uuid string) (types.JobResult, bool) {
	js.Lock()
	defer js.Unlock()

	result, ok := js.results[uuid]
	return result, ok
}

// GetWorker allows retrieval of a worker by its type from outside the package.
// It is necessary because the jobWorkers map is not exported, but we need to
// access the Status() method of a worker, such as the Twitter worker, externally.
func (js *JobServer) GetWorker(workerType string) (Worker, bool) {
	js.Lock()
	defer js.Unlock()

	entry, exists := js.jobWorkers[workerType]
	if !exists {
		return nil, false
	}
	return entry.w, true
}
