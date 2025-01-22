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
	// TODO: add sync.Mutex for accessing jobWorkers
	w, exists := js.jobWorkers[j.Type]

	if !exists {
		js.Lock()
		js.results[j.UUID] = types.JobResult{
			Error: fmt.Sprintf("unknown job type: %s", j.Type),
		}
		js.Unlock()
		return fmt.Errorf("unknown job type: %s", j.Type)
	}

	// XXX: Shall we lock the resource or create a new instance each time?
	// Behavior is not defined yet as the only requirements we have is that
	// some scrapers might have rate limits, so we don't want to create a new clients
	// every time (?)
	w.Lock()
	defer w.Unlock()
	result, err := w.w.ExecuteJob(j)
	if err != nil {
		result.Error = err.Error()
	}

	js.Lock()
	result.Job = j
	js.results[j.UUID] = result
	js.Unlock()

	return nil
}
