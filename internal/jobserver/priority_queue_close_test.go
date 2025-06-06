package jobserver_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/masa-finance/tee-worker/api/types"
	"github.com/masa-finance/tee-worker/internal/jobserver"
)

var _ = Describe("PriorityQueue Close Handling", func() {
	var pq *jobserver.PriorityQueue

	BeforeEach(func() {
		pq = jobserver.NewPriorityQueue(10, 10)
	})

	Describe("DequeueBlocking with closed queues", func() {
		It("should return ErrQueueClosed when queue is closed while blocking", func() {
			// Start a goroutine that will block on dequeue
			done := make(chan struct{})
			var err error
			
			go func() {
				defer close(done)
				_, err = pq.DequeueBlocking()
			}()

			// Give the goroutine time to start blocking
			time.Sleep(100 * time.Millisecond)

			// Close the queue
			pq.Close()

			// Wait for the goroutine to finish
			Eventually(done, 1*time.Second).Should(BeClosed())
			
			// Should have received ErrQueueClosed
			Expect(err).To(Equal(jobserver.ErrQueueClosed))
		})

		It("should handle enqueue errors after close", func() {
			// Add a job before closing
			job1 := &types.Job{UUID: "job-1", WorkerID: "worker-1"}
			err := pq.EnqueueFast(job1)
			Expect(err).NotTo(HaveOccurred())

			// Close the queue
			pq.Close()

			// Try to enqueue after close - should fail
			job2 := &types.Job{UUID: "job-2", WorkerID: "worker-2"}
			err = pq.EnqueueFast(job2)
			Expect(err).To(Equal(jobserver.ErrQueueClosed))
			
			err = pq.EnqueueSlow(job2)
			Expect(err).To(Equal(jobserver.ErrQueueClosed))

			// Non-blocking dequeue should return ErrQueueClosed after close
			_, err = pq.Dequeue()
			Expect(err).To(Equal(jobserver.ErrQueueClosed))
			
			// Blocking dequeue should also return ErrQueueClosed
			_, err = pq.DequeueBlocking()
			Expect(err).To(Equal(jobserver.ErrQueueClosed))
		})

		It("should not panic when multiple goroutines dequeue from closed queue", func() {
			numWorkers := 5
			errors := make(chan error, numWorkers)

			// Start multiple dequeuers
			for i := 0; i < numWorkers; i++ {
				go func() {
					_, err := pq.DequeueBlocking()
					errors <- err
				}()
			}

			// Give them time to start blocking
			time.Sleep(100 * time.Millisecond)

			// Close the queue
			pq.Close()

			// All workers should receive ErrQueueClosed
			for i := 0; i < numWorkers; i++ {
				select {
				case err := <-errors:
					Expect(err).To(Equal(jobserver.ErrQueueClosed))
				case <-time.After(1 * time.Second):
					Fail("Worker did not return after queue close")
				}
			}
		})
	})
})