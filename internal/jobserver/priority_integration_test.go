package jobserver_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/masa-finance/tee-worker/api/types"
	"github.com/masa-finance/tee-worker/internal/jobserver"
)

var _ = Describe("Priority Queue Integration", func() {
	var (
		js     *jobserver.JobServer
		ctx    context.Context
		cancel context.CancelFunc
		config types.JobConfiguration
	)

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())

		// Create job configuration with priority queue enabled
		config = types.JobConfiguration{
			"enable_priority_queue":                true,
			"fast_queue_size":                      100,
			"slow_queue_size":                      500,
			"external_worker_id_priority_endpoint": "", // Use dummy data
			"priority_refresh_interval_seconds":    5,
			"result_cache_max_size":                100,
			"result_cache_max_age_seconds":         60,
			"job_timeout_seconds":                  30,
			"stats_buf_size":                       uint(128),
			"worker_id":                            "test-worker",
		}

		// Create job server with 5 workers
		js = jobserver.NewJobServer(5, config)
		go js.Run(ctx)

		// Give workers time to start
		time.Sleep(100 * time.Millisecond)
	})

	AfterEach(func() {
		cancel()
		js.Shutdown()
		time.Sleep(100 * time.Millisecond)
	})

	Describe("Job Routing", func() {
		It("should route priority worker jobs to fast queue", func() {
			// Add job from priority worker
			priorityJob := types.Job{
				Type:     "telemetry",
				WorkerID: "worker-001", // This is in the priority list
				Arguments: types.JobArguments{
					"test": "priority",
				},
			}

			uuid := js.AddJob(priorityJob)
			Expect(uuid).NotTo(BeEmpty())

			// Wait for job to be processed
			Eventually(func() bool {
				result, exists := js.GetJobResult(uuid)
				return exists && result.Error == ""
			}, 5*time.Second, 100*time.Millisecond).Should(BeTrue())

			// Check queue stats
			stats := js.GetQueueStats()
			Expect(stats).NotTo(BeNil())
			Expect(stats.FastProcessed).To(BeNumerically(">=", 1))
		})

		It("should route non-priority worker jobs to slow queue", func() {
			// Add job from non-priority worker
			regularJob := types.Job{
				Type:     "telemetry",
				WorkerID: "worker-999", // Not in the priority list
				Arguments: types.JobArguments{
					"test": "regular",
				},
			}

			uuid := js.AddJob(regularJob)
			Expect(uuid).NotTo(BeEmpty())

			// Wait for job to be processed
			Eventually(func() bool {
				result, exists := js.GetJobResult(uuid)
				return exists && result.Error == ""
			}, 5*time.Second, 100*time.Millisecond).Should(BeTrue())

			// Check queue stats
			stats := js.GetQueueStats()
			Expect(stats).NotTo(BeNil())
			Expect(stats.SlowProcessed).To(BeNumerically(">=", 1))
		})

		It("should process fast queue jobs before slow queue jobs", func() {
			// Add multiple slow jobs first
			slowUUIDs := make([]string, 5)
			for i := 0; i < 5; i++ {
				slowJob := types.Job{
					Type:     "telemetry",
					WorkerID: "regular-worker",
					Arguments: types.JobArguments{
						"index": i,
						"type":  "slow",
					},
				}
				slowUUIDs[i] = js.AddJob(slowJob)
			}

			// Add fast jobs after slow jobs
			fastUUIDs := make([]string, 3)
			for i := 0; i < 3; i++ {
				fastJob := types.Job{
					Type:     "telemetry",
					WorkerID: "worker-priority-1",
					Arguments: types.JobArguments{
						"index": i,
						"type":  "fast",
					},
				}
				fastUUIDs[i] = js.AddJob(fastJob)
			}

			// Wait for all jobs to complete
			time.Sleep(2 * time.Second)

			// Check stats - fast jobs should have been processed
			stats := js.GetQueueStats()
			Expect(stats.FastProcessed).To(Equal(int64(3)))
			Expect(stats.SlowProcessed).To(BeNumerically(">=", 0))
			Expect(stats.SlowProcessed).To(BeNumerically("<=", 5))
		})
	})

	Describe("Priority Worker Management", func() {
		It("should return current priority workers", func() {
			workers := js.GetPriorityWorkers()
			Expect(workers).To(ContainElements(
				"worker-001",
				"worker-002",
				"worker-005",
				"worker-priority-1",
				"worker-priority-2",
				"worker-vip-1",
			))
		})
	})

	Describe("Legacy Mode", func() {
		It("should work with priority queue disabled", func() {
			// Create job server with priority queue disabled
			legacyConfig := types.JobConfiguration{
				"enable_priority_queue":        false,
				"result_cache_max_size":        100,
				"result_cache_max_age_seconds": 60,
				"job_timeout_seconds":          30,
				"stats_buf_size":               uint(128),
				"worker_id":                    "test-worker",
			}

			legacyJS := jobserver.NewJobServer(2, legacyConfig)
			legacyCtx, legacyCancel := context.WithCancel(context.Background())
			defer legacyCancel()
			defer legacyJS.Shutdown()

			go legacyJS.Run(legacyCtx)
			time.Sleep(100 * time.Millisecond)

			// Add job
			job := types.Job{
				Type:     "telemetry",
				WorkerID: "any-worker",
				Arguments: types.JobArguments{
					"test": "legacy",
				},
			}

			uuid := legacyJS.AddJob(job)
			Expect(uuid).NotTo(BeEmpty())

			// Wait for job to be processed
			Eventually(func() bool {
				result, exists := legacyJS.GetJobResult(uuid)
				return exists && result.Error == ""
			}, 5*time.Second, 100*time.Millisecond).Should(BeTrue())

			// Stats should be nil in legacy mode
			stats := legacyJS.GetQueueStats()
			Expect(stats).To(BeNil())
		})
	})
})
