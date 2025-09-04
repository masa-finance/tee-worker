package jobserver

import (
	"context"
	"fmt"

	teetypes "github.com/masa-finance/tee-types/types"
	"github.com/masa-finance/tee-worker/api/types"
	"github.com/sirupsen/logrus"
)

func (js *JobServer) worker(c context.Context) {
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

type worker interface {
	GetStructuredCapabilities() teetypes.WorkerCapabilities
	ExecuteJob(j types.Job) (types.JobResult, error)
}

func (js *JobServer) doWork(j types.Job) error {
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
		logrus.Infof("Error executing job type %s: %s", j.Type, err.Error())
		if len(result.Error) == 0 {
			result.Error = err.Error()
		}
	}

	result.Job = j
	js.results.Set(j.UUID, result)

	return nil
}
