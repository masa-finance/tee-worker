package jobserver

import (
	"context"
	"sync"

	"github.com/google/uuid"
	"github.com/masa-finance/tee-worker/api/types"
)

type JobServer struct {
	sync.Mutex

	jobChan chan types.Job
	workers int

	results          map[string]types.JobResult
	jobConfiguration types.JobConfiguration
}

func NewJobServer(workers int, jc types.JobConfiguration) *JobServer {
	if workers == 0 {
		workers++
	}

	return &JobServer{
		jobChan:          make(chan types.Job),
		results:          make(map[string]types.JobResult),
		workers:          workers,
		jobConfiguration: jc,
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
