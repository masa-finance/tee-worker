package jobs

import (
	"fmt"

	"github.com/masa-finance/tee-worker/api/types"
	"github.com/masa-finance/tee-worker/internal/capabilities/health"
	"github.com/masa-finance/tee-worker/internal/jobs/stats"
	"github.com/sirupsen/logrus"
)

const TelemetryJobType = "telemetry"

type TelemetryJob struct {
	collector     *stats.StatsCollector
	healthTracker health.CapabilityHealthTracker
}

func NewTelemetryJob(jc types.JobConfiguration, c *stats.StatsCollector, h health.CapabilityHealthTracker) TelemetryJob {
	return TelemetryJob{collector: c, healthTracker: h}
}

// GetCapabilities returns the capabilities supported by the telemetry job
func (t TelemetryJob) GetCapabilities() []string {
	return []string{"telemetry"}
}

func (t TelemetryJob) ExecuteJob(j types.Job) (types.JobResult, error) {
	var finalErr error
	defer func() {
		t.healthTracker.UpdateStatus(TelemetryJobType, finalErr == nil, finalErr)
	}()

	logrus.Debug("Executing telemetry job")

	if t.collector == nil {
		finalErr = fmt.Errorf("No StatsCollector configured")
		return types.JobResult{Error: finalErr.Error(), Job: j}, finalErr
	}

	// Get stats from the collector (now includes WorkerID)
	data, err := t.collector.Json()
	if err != nil {
		finalErr = err
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
