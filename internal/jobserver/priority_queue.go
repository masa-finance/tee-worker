package jobserver

import (
	"sync"
	"time"

	"github.com/masa-finance/tee-worker/api/types"
)

// PriorityQueue represents a dual-queue system with fast and slow queues
type PriorityQueue struct {
	fastQueue  chan *types.Job
	slowQueue  chan *types.Job
	mu         sync.RWMutex
	closed     bool
	stats      *QueueStats
}

// QueueStats tracks queue metrics
type QueueStats struct {
	mu              sync.RWMutex
	FastQueueDepth  int
	SlowQueueDepth  int
	FastProcessed   int64
	SlowProcessed   int64
	LastUpdateTime  time.Time
}

// NewPriorityQueue creates a new priority queue with specified buffer sizes
func NewPriorityQueue(fastQueueSize, slowQueueSize int) *PriorityQueue {
	return &PriorityQueue{
		fastQueue: make(chan *types.Job, fastQueueSize),
		slowQueue: make(chan *types.Job, slowQueueSize),
		stats: &QueueStats{
			LastUpdateTime: time.Now(),
		},
	}
}

// EnqueueFast adds a job to the fast queue
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

// EnqueueSlow adds a job to the slow queue
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

// Dequeue retrieves a job, prioritizing the fast queue
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

// DequeueBlocking retrieves a job with blocking, prioritizing the fast queue
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

// Close closes the priority queue
func (pq *PriorityQueue) Close() {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	if !pq.closed {
		pq.closed = true
		close(pq.fastQueue)
		close(pq.slowQueue)
	}
}

// GetStats returns current queue statistics
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

// updateStats updates queue statistics
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