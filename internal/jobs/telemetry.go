package jobs

import (
	teetypes "github.com/masa-finance/tee-types/types"
	"github.com/masa-finance/tee-worker/api/types"
	"github.com/masa-finance/tee-worker/internal/jobs/stats"
	"github.com/sirupsen/logrus"
)

type TelemetryJob struct {
	collector *stats.StatsCollector
}

func NewTelemetryJob(jc types.JobConfiguration, c *stats.StatsCollector) TelemetryJob {
	return TelemetryJob{collector: c}
}

// GetStructuredCapabilities returns the structured capabilities supported by the telemetry job
func (t TelemetryJob) GetStructuredCapabilities() teetypes.WorkerCapabilities {
	return teetypes.WorkerCapabilities{
		teetypes.TelemetryJob: teetypes.AlwaysAvailableTelemetryCaps,
	}
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
