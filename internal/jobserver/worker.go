package jobserver

import (
	"context"
	"fmt"

	"github.com/masa-finance/tee-worker/api/types"
	"github.com/masa-finance/tee-worker/internal/jobs"
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
	var w worker
	switch j.Type {
	case jobs.WebScraperType:
		w = jobs.NewWebScraper(js.jobConfiguration)
	default:
		js.Lock()
		js.results[j.UUID] = types.JobResult{
			Error: fmt.Sprintf("unknown job type: %s", j.Type),
		}
		js.Unlock()
		return fmt.Errorf("unknown job type: %s", j.Type)
	}

	result, err := w.ExecuteJob(j)
	if err != nil {
		result.Error = err.Error()
	}

	js.Lock()
	js.results[j.UUID] = result
	js.Unlock()

	return nil
}
