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
			// Create a larger queue for this test
			concurrentPQ := jobserver.NewPriorityQueue(500, 500)
			defer concurrentPQ.Close()

			var wg sync.WaitGroup
			numJobs := 100
			processedJobs := make([]string, 0, numJobs)
			var mutex sync.Mutex

			// Enqueue jobs concurrently
			for i := 0; i < numJobs; i++ {
				wg.Add(1)
				go func(id int) {
					defer wg.Done()
					job := &types.Job{
						UUID:     fmt.Sprintf("job-%d", id),
						WorkerID: fmt.Sprintf("worker-%d", id),
					}
					if id%2 == 0 {
						err := concurrentPQ.EnqueueFast(job)
						Expect(err).NotTo(HaveOccurred())
					} else {
						err := concurrentPQ.EnqueueSlow(job)
						Expect(err).NotTo(HaveOccurred())
					}
				}(i)
			}

			// Wait for all enqueues to complete
			wg.Wait()

			// Dequeue all jobs
			for i := 0; i < numJobs; i++ {
				job, err := concurrentPQ.Dequeue()
				Expect(err).NotTo(HaveOccurred())
				mutex.Lock()
				processedJobs = append(processedJobs, job.UUID)
				mutex.Unlock()
			}

			// Verify all jobs were processed
			Expect(len(processedJobs)).To(Equal(numJobs))
			
			// Verify no more jobs in queue
			_, err := concurrentPQ.Dequeue()
			Expect(err).To(Equal(jobserver.ErrQueueEmpty))
		})
	})
})