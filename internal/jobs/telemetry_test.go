package jobs_test

import (
	"encoding/json"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"

	teetypes "github.com/masa-finance/tee-types/types"
	"github.com/masa-finance/tee-worker/api/types"
	. "github.com/masa-finance/tee-worker/internal/jobs"
	"github.com/masa-finance/tee-worker/internal/jobs/stats"
)

var _ = Describe("Telemetry Job", func() {
	var telemetryJob TelemetryJob
	var statsCollector *stats.StatsCollector

	BeforeEach(func() {
		logrus.SetLevel(logrus.DebugLevel)
		os.Setenv("LOG_LEVEL", "debug")

		// Create a stats collector for the telemetry job
		statsCollector = stats.StartCollector(128, types.JobConfiguration{})

		// Create the telemetry job
		telemetryJob = NewTelemetryJob(types.JobConfiguration{}, statsCollector)
	})

	Context("Telemetry Data Fetching", func() {
		It("should fetch telemetry data and log it", func() {
			// Add some test stats to the collector
			statsCollector.Add("test-worker-1", stats.WebSuccess, 5)
			statsCollector.Add("test-worker-1", stats.WebErrors, 2)
			statsCollector.Add("test-worker-2", stats.TwitterScrapes, 10)
			statsCollector.Add("test-worker-2", stats.TwitterTweets, 50)

			// Execute the telemetry job
			job := types.Job{
				Type:     string(teetypes.TelemetryJob),
				WorkerID: "telemetry-test",
			}

			result, err := telemetryJob.ExecuteJob(job)

			// Verify the job executed successfully
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Error).To(BeEmpty())
			Expect(result.Data).NotTo(BeNil())

			// Parse and log the telemetry data
			var telemetryData map[string]interface{}
			err = json.Unmarshal(result.Data, &telemetryData)
			Expect(err).NotTo(HaveOccurred())

			logrus.WithFields(logrus.Fields{
				"telemetry_data": telemetryData,
			}).Info("Fetched telemetry data successfully")

			// Verify key telemetry fields are present
			Expect(telemetryData).To(HaveKey("boot_time"))
			Expect(telemetryData).To(HaveKey("current_time"))
			Expect(telemetryData).To(HaveKey("stats"))
			Expect(telemetryData).To(HaveKey("reported_capabilities"))
			Expect(telemetryData).To(HaveKey("worker_version"))
			Expect(telemetryData).To(HaveKey("application_version"))

			// Verify stats data contains our test data
			statsData, ok := telemetryData["stats"].(map[string]interface{})
			Expect(ok).To(BeTrue())

			// Log specific stats for each worker
			for workerID, workerStats := range statsData {
				logrus.WithFields(logrus.Fields{
					"worker_id": workerID,
					"stats":     workerStats,
				}).Info("Worker telemetry stats")
			}
		})

		It("should handle telemetry job without stats collector", func() {
			// Create a telemetry job without a stats collector
			telemetryJobNoStats := NewTelemetryJob(types.JobConfiguration{}, nil)

			job := types.Job{
				Type:     string(teetypes.TelemetryJob),
				WorkerID: "telemetry-test-no-stats",
			}

			result, err := telemetryJobNoStats.ExecuteJob(job)

			// Should not return an error but should have an error message in result
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Error).To(ContainSubstring("No StatsCollector configured"))

			logrus.WithField("error", result.Error).Info("Telemetry job handled missing stats collector correctly")
		})

		It("should return structured capabilities", func() {
			capabilities := telemetryJob.GetStructuredCapabilities()

			Expect(capabilities).NotTo(BeEmpty())
			Expect(capabilities).To(HaveLen(1))
			Expect(capabilities[0].JobType).To(Equal("telemetry"))
			Expect(capabilities[0].Capabilities).To(ContainElement(types.Capability("telemetry")))

			logrus.WithField("capabilities", capabilities).Info("Telemetry job capabilities verified")
		})
	})
})
