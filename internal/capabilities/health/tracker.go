package health

import (
	"sync"
	"time"
)

// Tracker is the concrete implementation of the CapabilityHealthTracker interface.
type Tracker struct {
	statuses map[string]CapabilityStatus
	mu       sync.RWMutex
}

// NewTracker creates a new instance of a Tracker.
func NewTracker() *Tracker {
	return &Tracker{
		statuses: make(map[string]CapabilityStatus),
	}
}

// UpdateStatus updates the health status of a specific capability.
func (t *Tracker) UpdateStatus(name string, isHealthy bool, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	status, exists := t.statuses[name]
	if !exists {
		status = CapabilityStatus{Name: name}
	}

	status.IsHealthy = isHealthy
	status.LastChecked = time.Now()

	if err != nil {
		status.LastError = err.Error()
		if !isHealthy {
			status.ErrorCount++
		}
	} else {
		// Reset error state on success
		status.LastError = ""
		status.ErrorCount = 0
	}
	t.statuses[name] = status
}

// GetStatus retrieves the current health status of a specific capability.
func (t *Tracker) GetStatus(name string) (CapabilityStatus, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	status, exists := t.statuses[name]
	return status, exists
}

// GetAllStatuses returns a map of all tracked capability statuses.
func (t *Tracker) GetAllStatuses() map[string]CapabilityStatus {
	t.mu.RLock()
	defer t.mu.RUnlock()
	// Return a copy to prevent race conditions and ensure thread safety.
	statusesCopy := make(map[string]CapabilityStatus)
	for k, v := range t.statuses {
		statusesCopy[k] = v
	}
	return statusesCopy
}
