package jobserver

import (
	"context"
	"fmt"

	"github.com/masa-finance/tee-worker/api/types"
)

func (js *JobServer) worker(c context.Context) {
	for {
		select {
		case <-c.Done():
			fmt.Println("Context done")
			return

		case j := <-js.jobChan:
			fmt.Println("Job received: ", j)
			js.doWork(j)
		}
	}
}

type worker interface {
	ExecuteJob(j types.Job) (types.JobResult, error)
}

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
