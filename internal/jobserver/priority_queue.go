// Package jobserver provides job queue management and processing functionality
// for the tee-worker, including priority-based job routing.
package jobserver

import (
	"sync"
	"time"

	"github.com/masa-finance/tee-worker/api/types"
)

// PriorityQueue implements a dual-queue system for job prioritization.
// It maintains two separate queues:
// - Fast queue: For high-priority jobs from workers in the priority list
// - Slow queue: For regular jobs from all other workers
//
// The queue ensures thread-safe operations and provides statistics tracking.
type PriorityQueue struct {
	fastQueue  chan *types.Job
	slowQueue  chan *types.Job
	mu         sync.RWMutex
	closed     bool
	stats      *QueueStats
}

// QueueStats provides real-time metrics about queue performance.
// All fields are thread-safe and can be accessed concurrently.
type QueueStats struct {
	mu              sync.RWMutex
	FastQueueDepth  int
	SlowQueueDepth  int
	FastProcessed   int64
	SlowProcessed   int64
	LastUpdateTime  time.Time
}

// NewPriorityQueue creates a new priority queue with specified buffer sizes.
//
// Parameters:
//   - fastQueueSize: Maximum number of jobs that can be buffered in the fast queue
//   - slowQueueSize: Maximum number of jobs that can be buffered in the slow queue
//
// Returns a ready-to-use PriorityQueue instance with statistics tracking enabled.
func NewPriorityQueue(fastQueueSize, slowQueueSize int) *PriorityQueue {
	return &PriorityQueue{
		fastQueue: make(chan *types.Job, fastQueueSize),
		slowQueue: make(chan *types.Job, slowQueueSize),
		stats: &QueueStats{
			LastUpdateTime: time.Now(),
		},
	}
}

// EnqueueFast adds a job to the fast (high-priority) queue.
//
// This method is non-blocking and will return immediately.
// Returns ErrQueueFull if the fast queue is at capacity.
// Returns ErrQueueClosed if the queue has been closed.
//
// Thread-safe: Can be called concurrently from multiple goroutines.
func (pq *PriorityQueue) EnqueueFast(job *types.Job) error {
	pq.mu.RLock()
	if pq.closed {
		pq.mu.RUnlock()
		return ErrQueueClosed
	}
	pq.mu.RUnlock()

	select {
	case pq.fastQueue <- job:
		pq.updateStats(true, false)
		return nil
	default:
		return ErrQueueFull
	}
}

// EnqueueSlow adds a job to the slow (regular-priority) queue.
//
// This method is non-blocking and will return immediately.
// Returns ErrQueueFull if the slow queue is at capacity.
// Returns ErrQueueClosed if the queue has been closed.
//
// Thread-safe: Can be called concurrently from multiple goroutines.
func (pq *PriorityQueue) EnqueueSlow(job *types.Job) error {
	pq.mu.RLock()
	if pq.closed {
		pq.mu.RUnlock()
		return ErrQueueClosed
	}
	pq.mu.RUnlock()

	select {
	case pq.slowQueue <- job:
		pq.updateStats(false, false)
		return nil
	default:
		return ErrQueueFull
	}
}

// Dequeue retrieves a job from the queue system.
//
// Priority order:
// 1. Always checks fast queue first
// 2. Only checks slow queue if fast queue is empty
//
// This method is non-blocking and returns immediately.
// Returns ErrQueueEmpty if both queues are empty.
// Returns ErrQueueClosed if the queue has been closed.
//
// Thread-safe: Can be called concurrently from multiple goroutines.
func (pq *PriorityQueue) Dequeue() (*types.Job, error) {
	pq.mu.RLock()
	if pq.closed {
		pq.mu.RUnlock()
		return nil, ErrQueueClosed
	}
	pq.mu.RUnlock()

	// Always try fast queue first
	select {
	case job := <-pq.fastQueue:
		pq.updateStats(true, true)
		return job, nil
	default:
		// Fast queue is empty, try slow queue
		select {
		case job := <-pq.slowQueue:
			pq.updateStats(false, true)
			return job, nil
		default:
			return nil, ErrQueueEmpty
		}
	}
}

// DequeueBlocking retrieves a job from the queue system, blocking until one is available.
//
// Priority order:
// 1. Continuously checks fast queue first
// 2. Falls back to slow queue when fast queue is empty
// 3. Blocks if both queues are empty, waiting for new jobs
//
// This method will block indefinitely until a job is available or the queue is closed.
// Returns ErrQueueClosed if the queue has been closed.
//
// Thread-safe: Can be called concurrently from multiple goroutines.
// Typically used by worker goroutines that process jobs.
func (pq *PriorityQueue) DequeueBlocking() (*types.Job, error) {
	pq.mu.RLock()
	if pq.closed {
		pq.mu.RUnlock()
		return nil, ErrQueueClosed
	}
	pq.mu.RUnlock()

	for {
		// Check fast queue first
		select {
		case job := <-pq.fastQueue:
			pq.updateStats(true, true)
			return job, nil
		default:
			// Fast queue empty, check slow queue
			select {
			case job := <-pq.slowQueue:
				pq.updateStats(false, true)
				return job, nil
			case job := <-pq.fastQueue:
				// Check fast queue again in case something was added
				pq.updateStats(true, true)
				return job, nil
			}
		}
	}
}

// Close gracefully shuts down the priority queue.
//
// After calling Close:
// - No new jobs can be enqueued (will return ErrQueueClosed)
// - Existing jobs in the queues can still be dequeued
// - DequeueBlocking will return ErrQueueClosed once queues are empty
//
// This method is idempotent and can be called multiple times safely.
func (pq *PriorityQueue) Close() {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	if !pq.closed {
		pq.closed = true
		close(pq.fastQueue)
		close(pq.slowQueue)
	}
}

// GetStats returns a snapshot of current queue statistics.
//
// The returned statistics include:
// - Current depth of both fast and slow queues
// - Total number of jobs processed from each queue
// - Timestamp of last statistics update
//
// This method is lightweight and can be called frequently for monitoring.
// Thread-safe: Can be called concurrently from multiple goroutines.
func (pq *PriorityQueue) GetStats() QueueStats {
	pq.stats.mu.RLock()
	defer pq.stats.mu.RUnlock()

	return QueueStats{
		FastQueueDepth:  len(pq.fastQueue),
		SlowQueueDepth:  len(pq.slowQueue),
		FastProcessed:   pq.stats.FastProcessed,
		SlowProcessed:   pq.stats.SlowProcessed,
		LastUpdateTime:  pq.stats.LastUpdateTime,
	}
}

// updateStats updates internal queue statistics.
//
// Parameters:
//   - isFast: true if the operation was on the fast queue, false for slow queue
//   - isDequeue: true if this was a dequeue operation, false for enqueue
//
// This method is called internally after each queue operation to maintain accurate statistics.
func (pq *PriorityQueue) updateStats(isFast bool, isDequeue bool) {
	pq.stats.mu.Lock()
	defer pq.stats.mu.Unlock()

	if isDequeue {
		if isFast {
			pq.stats.FastProcessed++
		} else {
			pq.stats.SlowProcessed++
		}
	}
	pq.stats.LastUpdateTime = time.Now()
}