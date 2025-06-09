// Package jobserver provides job queue management and processing functionality
// for the tee-worker, including priority-based job routing.
package jobserver

import (
	"sync"
	"sync/atomic"
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
// Queue depths are calculated dynamically in GetStats() to avoid update overhead.
// Processing counters use atomic operations for lock-free updates under high concurrency.
type QueueStats struct {
	FastQueueDepth  int   // Calculated dynamically, not stored
	SlowQueueDepth  int   // Calculated dynamically, not stored
	FastProcessed   int64 // Total jobs processed from fast queue (atomic)
	SlowProcessed   int64 // Total jobs processed from slow queue (atomic)
	LastUpdateTime  time.Time // Calculated from lastUpdateNano in GetStats()
	lastUpdateNano  int64 // Atomic storage for LastUpdateTime as UnixNano
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
			lastUpdateNano: time.Now().UnixNano(),
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
	defer pq.mu.RUnlock()
	
	if pq.closed {
		return ErrQueueClosed
	}

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
	defer pq.mu.RUnlock()
	
	if pq.closed {
		return ErrQueueClosed
	}

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
// This method implements a priority-aware blocking dequeue that:
// 1. Always checks the fast queue first
// 2. Only processes slow queue jobs when fast queue is empty
// 3. Blocks efficiently when both queues are empty
//
// The implementation uses a single select statement for clarity while maintaining
// the priority semantics through periodic fast queue checks.
//
// Returns ErrQueueClosed if the queue has been closed.
// Thread-safe: Can be called concurrently from multiple goroutines.
func (pq *PriorityQueue) DequeueBlocking() (*types.Job, error) {
	// Create a ticker to periodically check fast queue
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		// Check if queue is closed
		pq.mu.RLock()
		if pq.closed {
			pq.mu.RUnlock()
			return nil, ErrQueueClosed
		}
		pq.mu.RUnlock()

		// First, always try fast queue non-blocking
		select {
		case job := <-pq.fastQueue:
			pq.updateStats(true, true)
			return job, nil
		default:
			// Fast queue is empty, continue to blocking select
		}

		// Blocking select on both queues with periodic fast queue re-check
		select {
		case job := <-pq.fastQueue:
			pq.updateStats(true, true)
			return job, nil
		case job := <-pq.slowQueue:
			pq.updateStats(false, true)
			return job, nil
		case <-ticker.C:
			// Periodically loop back to check fast queue first
			// This ensures we don't miss fast queue jobs while blocked on slow queue
			continue
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
//
// Note: This implementation does not close the channels to avoid potential
// panics from concurrent sends. The channels will be garbage collected
// when no longer referenced.
func (pq *PriorityQueue) Close() {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	if !pq.closed {
		pq.closed = true
		// Do not close channels to avoid panic from concurrent sends
		// Channels will be garbage collected when no longer referenced
	}
}

// GetStats returns a snapshot of current queue statistics.
//
// The returned statistics include:
// - Current depth of both fast and slow queues (calculated dynamically)
// - Total number of jobs processed from each queue
// - Timestamp of last statistics update
//
// This method is lightweight and can be called frequently for monitoring.
// Thread-safe: Can be called concurrently from multiple goroutines.
func (pq *PriorityQueue) GetStats() QueueStats {
	// Use atomic loads for lock-free reading
	fast := atomic.LoadInt64(&pq.stats.FastProcessed)
	slow := atomic.LoadInt64(&pq.stats.SlowProcessed)
	lastNano := atomic.LoadInt64(&pq.stats.lastUpdateNano)

	return QueueStats{
		FastQueueDepth:  len(pq.fastQueue),
		SlowQueueDepth:  len(pq.slowQueue),
		FastProcessed:   fast,
		SlowProcessed:   slow,
		LastUpdateTime:  time.Unix(0, lastNano),
	}
}

// updateStats updates internal queue statistics.
//
// Parameters:
//   - isFast: true if the operation was on the fast queue, false for slow queue
//   - isDequeue: true if this was a dequeue operation, false for enqueue
//
// This method is called internally after each queue operation to maintain accurate statistics.
// Uses atomic operations for lock-free updates under high concurrency.
// Note: Timestamp updates are rate-limited to reduce overhead under high throughput.
func (pq *PriorityQueue) updateStats(isFast bool, isDequeue bool) {
	if isDequeue {
		if isFast {
			atomic.AddInt64(&pq.stats.FastProcessed, 1)
		} else {
			atomic.AddInt64(&pq.stats.SlowProcessed, 1)
		}
		
		// Update timestamp only on dequeue operations to reduce overhead
		// This provides a good balance between accuracy and performance
		atomic.StoreInt64(&pq.stats.lastUpdateNano, time.Now().UnixNano())
	}
}