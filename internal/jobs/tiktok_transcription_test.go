package jobs_test

import (
	"encoding/json"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	teetypes "github.com/masa-finance/tee-types/types"
	"github.com/masa-finance/tee-worker/api/types"
	. "github.com/masa-finance/tee-worker/internal/jobs"
	"github.com/masa-finance/tee-worker/internal/jobs/stats"
	"github.com/sirupsen/logrus"
)

var _ = Describe("TikTokTranscriber", func() {
	var statsCollector *stats.StatsCollector
	var tikTokTranscriber *TikTokTranscriber
	var jobConfig types.JobConfiguration

	BeforeEach(func() {
		// Initialize a real stats collector, similar to webscraper_test.go
		// Assuming stats.StartCollector is the correct way to get an instance
		// The buffer size and jobConfig for stats can be minimal for tests.
		jobConfigForStats := types.JobConfiguration{"stats_buf_size": uint(32)}
		statsCollector = stats.StartCollector(32, jobConfigForStats) // Use the actual StartCollector

		// Ensure debug logging is enabled for the test run
		logrus.SetLevel(logrus.DebugLevel)

		// Initialize JobConfiguration for the transcriber
		// It will use hardcoded endpoint, but we can set other defaults if needed for tests
		jobConfig = types.JobConfiguration{
			"tiktok_default_language": "eng-US", // Example default
		}
		tikTokTranscriber = NewTikTokTranscriber(jobConfig, statsCollector)
		Expect(tikTokTranscriber).NotTo(BeNil())
	})

	Context("when a valid TikTok URL is provided", func() {
		It("should successfully transcribe the video and record success stats", func(ctx SpecContext) {
			videoURL := "https://www.tiktok.com/@.jake.ai/video/7516694182245813509"
			jobArguments := map[string]interface{}{
				"video_url": videoURL,
				// default language is eng-US from tee types
			}

			job := types.Job{
				Type:      teetypes.TiktokJob,
				Arguments: jobArguments,
				WorkerID:  "tiktok-test-worker-happy",
				UUID:      "test-uuid-happy",
			}

			// Potentially long running due to live API call
			By("Executing the TikTok transcription job")
			res, err := tikTokTranscriber.ExecuteJob(job)

			By("Checking for job execution errors")
			Expect(err).NotTo(HaveOccurred())
			Expect(res.Error).To(BeEmpty(), "JobResult.Error should be empty on success")

			By("Verifying the result data")
			Expect(res.Data).NotTo(BeNil())
			Expect(res.Data).NotTo(BeEmpty())

			var transcriptionResult teetypes.TikTokTranscriptionResult
			err = json.Unmarshal(res.Data, &transcriptionResult)
			Expect(err).NotTo(HaveOccurred(), "Failed to unmarshal result data")

			Expect(transcriptionResult.OriginalURL).To(Equal(videoURL))
			Expect(transcriptionResult.TranscriptionText).NotTo(BeEmpty(), "TranscriptionText should not be empty")
			Expect(transcriptionResult.VideoTitle).NotTo(BeEmpty(), "VideoTitle should not be empty")
			// Language might be reported differently by API, checking for non-empty is safer for live test
			Expect(transcriptionResult.DetectedLanguage).NotTo(BeEmpty(), "DetectedLanguage should not be empty")
			// Thumbnail URL might or might not be present or could be empty.
			// For a robust live test, we might just check it's a string or if present, a valid URL prefix.
			if transcriptionResult.ThumbnailURL != "" {
				Expect(strings.HasPrefix(transcriptionResult.ThumbnailURL, "http")).To(BeTrue(), "ThumbnailURL if present should be a valid URL")
			}

			By("Verifying success statistics")
			Eventually(func() uint {
				// Attempting to access stats similarly to webscraper_test.go
				// This assumes statsCollector has an exported structure like: Stats.Stats[workerID][statType]
				// The actual type of statsCollector.Stats.Stats might be map[string]map[stats.StatType]uint
				// Ensure the types are correct for your actual stats package structure.
				if statsCollector == nil || statsCollector.Stats == nil || statsCollector.Stats.Stats == nil {
					return 0 // Guard against nil pointers if initialization is complex
				}
				workerStatsMap := statsCollector.Stats.Stats[job.WorkerID]
				if workerStatsMap == nil {
					return 0
				}
				return workerStatsMap[stats.TikTokTranscriptionSuccess]
			}, 15*time.Second, 250*time.Millisecond).Should(BeNumerically("==", 1), "TikTokTranscriptionSuccess count should be 1")

			Eventually(func() uint {
				if statsCollector == nil || statsCollector.Stats == nil || statsCollector.Stats.Stats == nil {
					return 0
				}
				workerStatsMap := statsCollector.Stats.Stats[job.WorkerID]
				if workerStatsMap == nil {
					return 0
				}
				return workerStatsMap[stats.TikTokTranscriptionErrors]
			}, 5*time.Second, 100*time.Millisecond).Should(BeNumerically("==", 0), "TikTokTranscriptionErrors count should be 0")
		}, NodeTimeout(30*time.Second)) // Increased timeout for this specific test case
	})

	Context("when arguments are invalid", func() {
		It("should return an error if VideoURL is empty and not record error stats", func() {
			jobArguments := map[string]interface{}{
				"video_url": "", // Empty URL
			}

			job := types.Job{
				Type:      teetypes.TiktokJob,
				Arguments: jobArguments,
				WorkerID:  "tiktok-test-worker-invalid",
				UUID:      "test-uuid-invalid",
			}

			By("Executing the job with an empty VideoURL")
			res, err := tikTokTranscriber.ExecuteJob(job)

			By("Checking for job execution errors")
			Expect(err).To(HaveOccurred(), "An error should occur for empty VideoURL")
			Expect(res.Error).NotTo(BeEmpty(), "JobResult.Error should detail the validation failure")
			Expect(res.Error).To(ContainSubstring("Failed to unmarshal job arguments"))
			Expect(res.Data).To(BeNil())

			By("Verifying error statistics")
			Eventually(func() uint {
				if statsCollector == nil || statsCollector.Stats == nil || statsCollector.Stats.Stats == nil {
					return 0
				}
				workerStatsMap := statsCollector.Stats.Stats[job.WorkerID]
				if workerStatsMap == nil {
					return 0
				}
				return workerStatsMap[stats.TikTokTranscriptionErrors]
			}, 5*time.Second, 100*time.Millisecond).Should(BeNumerically("==", 0), "TikTokTranscriptionErrors count should be 0")

			Eventually(func() uint {
				if statsCollector == nil || statsCollector.Stats == nil || statsCollector.Stats.Stats == nil {
					return 0
				}
				workerStatsMap := statsCollector.Stats.Stats[job.WorkerID]
				if workerStatsMap == nil {
					return 0
				}
				return workerStatsMap[stats.TikTokTranscriptionSuccess]
			}, 5*time.Second, 100*time.Millisecond).Should(BeNumerically("==", 0), "TikTokTranscriptionSuccess count should be 0")
		})
	})
})
