package health

import (
	"context"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// Verifier defines the interface for a single capability check.
// This avoids a direct import cycle back to the capabilities package.
type Verifier interface {
	Verify(ctx context.Context) (bool, error)
}

// Tracker is the concrete implementation of the CapabilityHealthTracker interface.
type Tracker struct {
	statuses  map[string]CapabilityStatus
	verifiers map[string]Verifier
	mu        sync.RWMutex
}

// NewTracker creates a new instance of a Tracker.
func NewTracker() *Tracker {
	return &Tracker{
		statuses:  make(map[string]CapabilityStatus),
		verifiers: make(map[string]Verifier),
	}
}

// SetVerifiers sets the map of verifiers for the tracker to use.
func (t *Tracker) SetVerifiers(verifiers map[string]Verifier) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.verifiers = verifiers
}

// StartReconciliationLoop starts a background process to periodically re-check unhealthy capabilities.
func (t *Tracker) StartReconciliationLoop(ctx context.Context) {
	logrus.Info("Starting capability health reconciliation loop...")
	ticker := time.NewTicker(1 * time.Minute) // Check every minute

	go func() {
		for {
			select {
			case <-ctx.Done():
				logrus.Info("Stopping capability health reconciliation loop.")
				ticker.Stop()
				return
			case <-ticker.C:
				t.reconcileUnhealthy(ctx)
			}
		}
	}()
}

func (t *Tracker) reconcileUnhealthy(ctx context.Context) {
	t.mu.RLock()
	statusesToCheck := make(map[string]CapabilityStatus)
	for name, status := range t.statuses {
		if !status.IsHealthy {
			statusesToCheck[name] = status
		}
	}
	t.mu.RUnlock()

	if len(statusesToCheck) == 0 {
		return
	}

	logrus.Debugf("Reconciliation: Found %d unhealthy capabilities to re-check.", len(statusesToCheck))

	for name, status := range statusesToCheck {
		// Exponential backoff: wait (5 * error_count) minutes between checks
		// For the first error, this is 5 minutes. Second is 10, etc.
		backoffDuration := time.Duration(5*status.ErrorCount) * time.Minute
		if time.Since(status.LastChecked) < backoffDuration {
			continue // Not time to re-check yet
		}

		verifier, exists := t.verifiers[name]
		if !exists {
			continue // No verifier for this capability
		}

		logrus.Infof("Re-verifying health of unhealthy capability: %s", name)
		isHealthy, err := verifier.Verify(ctx)
		if isHealthy {
			logrus.Infof("Capability '%s' has recovered and is now healthy.", name)
		}
		t.UpdateStatus(name, isHealthy, err)
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
