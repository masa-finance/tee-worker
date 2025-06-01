package jobserver_test

import (
	"fmt"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/masa-finance/tee-worker/api/types"
	"github.com/masa-finance/tee-worker/internal/jobserver"
)

var _ = Describe("PriorityQueue", func() {
	var pq *jobserver.PriorityQueue

	BeforeEach(func() {
		pq = jobserver.NewPriorityQueue(10, 10)
	})

	AfterEach(func() {
		pq.Close()
	})

	Describe("Enqueue and Dequeue", func() {
		It("should enqueue and dequeue from fast queue", func() {
			job := &types.Job{
				Type:     "test",
				UUID:     "test-123",
				WorkerID: "worker-001",
			}

			err := pq.EnqueueFast(job)
			Expect(err).NotTo(HaveOccurred())

			dequeuedJob, err := pq.Dequeue()
			Expect(err).NotTo(HaveOccurred())
			Expect(dequeuedJob.UUID).To(Equal("test-123"))
		})

		It("should enqueue and dequeue from slow queue", func() {
			job := &types.Job{
				Type:     "test",
				UUID:     "test-456",
				WorkerID: "worker-002",
			}

			err := pq.EnqueueSlow(job)
			Expect(err).NotTo(HaveOccurred())

			dequeuedJob, err := pq.Dequeue()
			Expect(err).NotTo(HaveOccurred())
			Expect(dequeuedJob.UUID).To(Equal("test-456"))
		})

		It("should prioritize fast queue over slow queue", func() {
			slowJob := &types.Job{
				Type:     "test",
				UUID:     "slow-1",
				WorkerID: "worker-slow",
			}
			fastJob := &types.Job{
				Type:     "test",
				UUID:     "fast-1",
				WorkerID: "worker-fast",
			}

			// Add slow job first
			err := pq.EnqueueSlow(slowJob)
			Expect(err).NotTo(HaveOccurred())

			// Add fast job second
			err = pq.EnqueueFast(fastJob)
			Expect(err).NotTo(HaveOccurred())

			// Fast job should be dequeued first
			dequeuedJob, err := pq.Dequeue()
			Expect(err).NotTo(HaveOccurred())
			Expect(dequeuedJob.UUID).To(Equal("fast-1"))

			// Slow job should be dequeued second
			dequeuedJob, err = pq.Dequeue()
			Expect(err).NotTo(HaveOccurred())
			Expect(dequeuedJob.UUID).To(Equal("slow-1"))
		})

		It("should return error when queues are empty", func() {
			_, err := pq.Dequeue()
			Expect(err).To(Equal(jobserver.ErrQueueEmpty))
		})

		It("should handle queue full scenario", func() {
			// Fill the fast queue
			smallQueue := jobserver.NewPriorityQueue(2, 2)
			defer smallQueue.Close()

			for i := 0; i < 2; i++ {
				err := smallQueue.EnqueueFast(&types.Job{UUID: fmt.Sprintf("job-%d", i)})
				Expect(err).NotTo(HaveOccurred())
			}

			// Try to add one more
			err := smallQueue.EnqueueFast(&types.Job{UUID: "overflow"})
			Expect(err).To(Equal(jobserver.ErrQueueFull))
		})
	})

	Describe("Blocking Dequeue", func() {
		It("should block until job is available", func() {
			var wg sync.WaitGroup
			var dequeuedJob *types.Job
			var dequeueErr error

			wg.Add(1)
			go func() {
				defer wg.Done()
				dequeuedJob, dequeueErr = pq.DequeueBlocking()
			}()

			// Give the goroutine time to start blocking
			time.Sleep(100 * time.Millisecond)

			// Add a job
			job := &types.Job{UUID: "blocking-test"}
			err := pq.EnqueueFast(job)
			Expect(err).NotTo(HaveOccurred())

			// Wait for dequeue to complete
			wg.Wait()

			Expect(dequeueErr).NotTo(HaveOccurred())
			Expect(dequeuedJob.UUID).To(Equal("blocking-test"))
		})
	})

	Describe("Statistics", func() {
		It("should track queue statistics", func() {
			// Add jobs to both queues
			for i := 0; i < 3; i++ {
				err := pq.EnqueueFast(&types.Job{UUID: fmt.Sprintf("fast-%d", i)})
				Expect(err).NotTo(HaveOccurred())
			}
			for i := 0; i < 5; i++ {
				err := pq.EnqueueSlow(&types.Job{UUID: fmt.Sprintf("slow-%d", i)})
				Expect(err).NotTo(HaveOccurred())
			}

			stats := pq.GetStats()
			Expect(stats.FastQueueDepth).To(Equal(3))
			Expect(stats.SlowQueueDepth).To(Equal(5))

			// Dequeue some jobs
			for i := 0; i < 4; i++ {
				_, err := pq.Dequeue()
				Expect(err).NotTo(HaveOccurred())
			}

			stats = pq.GetStats()
			Expect(stats.FastProcessed).To(Equal(int64(3)))
			Expect(stats.SlowProcessed).To(Equal(int64(1)))
		})
	})

	Describe("Concurrent Operations", func() {
		It("should handle concurrent enqueue and dequeue operations", func() {
			var wg sync.WaitGroup
			numWorkers := 10
			jobsPerWorker := 100

			// Start enqueuers
			for i := 0; i < numWorkers; i++ {
				wg.Add(1)
				go func(workerID int) {
					defer wg.Done()
					for j := 0; j < jobsPerWorker; j++ {
						job := &types.Job{
							UUID:     fmt.Sprintf("worker-%d-job-%d", workerID, j),
							WorkerID: fmt.Sprintf("worker-%d", workerID),
						}
						if workerID%2 == 0 {
							pq.EnqueueFast(job)
						} else {
							pq.EnqueueSlow(job)
						}
					}
				}(i)
			}

			// Start dequeuers
			processedJobs := make(chan string, numWorkers*jobsPerWorker)
			for i := 0; i < numWorkers/2; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for {
						job, err := pq.Dequeue()
						if err == jobserver.ErrQueueEmpty {
							time.Sleep(10 * time.Millisecond)
							continue
						}
						if job != nil {
							processedJobs <- job.UUID
						}
						if len(processedJobs) >= numWorkers*jobsPerWorker {
							return
						}
					}
				}()
			}

			// Wait for all operations to complete
			wg.Wait()
			close(processedJobs)

			// Verify all jobs were processed
			processedCount := len(processedJobs)
			Expect(processedCount).To(Equal(numWorkers * jobsPerWorker))
		})
	})
})