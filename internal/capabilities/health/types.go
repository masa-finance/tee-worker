package health

import (
	"context"
	"time"
)

// CapabilityStatus holds the health information for a single capability.
type CapabilityStatus struct {
	Name        string
	IsHealthy   bool
	LastChecked time.Time
	LastError   string
	ErrorCount  int
}

// CapabilityHealthTracker defines the interface for managing the health status
// of all worker capabilities.
type CapabilityHealthTracker interface {
	// UpdateStatus updates the health status of a specific capability.
	UpdateStatus(name string, isHealthy bool, err error)
	// GetStatus retrieves the current health status of a specific capability.
	GetStatus(name string) (CapabilityStatus, bool)
	// GetAllStatuses returns a map of all tracked capability statuses.
	GetAllStatuses() map[string]CapabilityStatus
	// StartReconciliationLoop starts a background process to periodically re-check unhealthy capabilities.
	StartReconciliationLoop(ctx context.Context)
}
