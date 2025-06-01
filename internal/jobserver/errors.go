package jobserver

import "errors"

var (
	// ErrQueueClosed is returned when attempting to use a closed queue
	ErrQueueClosed = errors.New("queue is closed")
	
	// ErrQueueFull is returned when attempting to enqueue to a full queue
	ErrQueueFull = errors.New("queue is full")
	
	// ErrQueueEmpty is returned when attempting to dequeue from empty queues
	ErrQueueEmpty = errors.New("all queues are empty")
	
	// ErrJobNotFound is returned when a job is not found
	ErrJobNotFound = errors.New("job not found")
	
	// ErrInvalidJobType is returned when job type is invalid
	ErrInvalidJobType = errors.New("invalid job type")
)