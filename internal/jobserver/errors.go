// Package jobserver provides job queue management and processing functionality.
// This file defines common errors used throughout the jobserver package.
package jobserver

import "errors"

// Common errors returned by jobserver operations.
// These errors help distinguish between different failure scenarios
// and allow callers to handle specific error conditions appropriately.
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