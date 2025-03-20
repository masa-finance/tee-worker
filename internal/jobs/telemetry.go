package jobs

import (
	"github.com/masa-finance/tee-worker/api/types"
	"github.com/masa-finance/tee-worker/internal/jobs/stats"
	"github.com/sirupsen/logrus"
)

const TelemetryJobType = "telemetry"

type TelemetryJob struct {
	collector *stats.StatsCollector
}

func NewTelemetryJob(jc types.JobConfiguration, c *stats.StatsCollector) TelemetryJob {
	return TelemetryJob{collector: c}
}

func (t TelemetryJob) ExecuteJob(j types.Job) (types.JobResult, error) {
	logrus.Debug("Executing telemetry job")

	if t.collector == nil {
		return types.JobResult{Error: "No StatsCollector configured", Job: j}, nil
	}

	// Get stats from the collector (now includes WorkerID)
	data, err := t.collector.Json()
	if err != nil {
		return types.JobResult{
			Error: err.Error(),
			Job:   j,
		}, err
	}

	return types.JobResult{
		Data: data,
		Job:  j,
	}, nil
}
