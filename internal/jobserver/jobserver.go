package jobserver

import (
	"context"
	"sync"

	"github.com/google/uuid"
	"github.com/masa-finance/tee-worker/api/types"
	"github.com/masa-finance/tee-worker/internal/jobs"
	"github.com/masa-finance/tee-worker/internal/jobs/stats"
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
	w worker
	sync.Mutex
}

func NewJobServer(workers int, jc types.JobConfiguration) *JobServer {
	if workers <= 0 {
		workers = 1
	}

	bufSize, ok := jc["stats_buf_size"].(uint)
	if !ok {
		bufSize = 128
	}
	s := stats.StartCollector(bufSize)

	jobworkers := map[string]*jobWorkerEntry{
		jobs.WebScraperType: {
			w: jobs.NewWebScraper(jc, s),
		},
		jobs.TwitterScraperType: {
			w: jobs.NewTwitterScraper(jc, s),
		},
		jobs.TelemetryJobType: {
			w: jobs.NewTelemetryJob(jc, s),
		},
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
