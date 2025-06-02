package jobserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// PriorityManager manages the list of worker IDs that receive priority processing.
// It supports both static configuration and dynamic updates from an external endpoint.
//
// Key features:
// - Maintains an in-memory set of priority worker IDs
// - Optionally fetches updates from an external API endpoint
// - Refreshes the priority list periodically (configurable interval)
// - Thread-safe for concurrent access
type PriorityManager struct {
	mu                               sync.RWMutex
	priorityWorkers                  map[string]bool
	externalWorkerIDPriorityEndpoint string
	refreshInterval                  time.Duration
	httpClient                       *http.Client
	ctx                              context.Context
	cancel                           context.CancelFunc
}

// PriorityWorkerList represents the expected JSON response format from the external priority endpoint.
// This structure should match the API response from the external service that provides
// the list of worker IDs that should receive priority processing.
type PriorityWorkerList struct {
	Workers []string `json:"workers"`
}

// NewPriorityManager creates and initializes a new priority manager.
//
// Parameters:
//   - externalWorkerIdPriorityEndpoint: URL of the external API to fetch priority worker IDs.
//     If empty, only uses the built-in dummy data.
//   - refreshInterval: How often to refresh the priority list from the external endpoint.
//     If <= 0, defaults to 15 minutes.
//
// The manager will:
// 1. Initialize with dummy data for testing
// 2. Immediately fetch from the external endpoint (if configured)
// 3. Start a background goroutine to refresh the list periodically
//
// Returns a fully initialized PriorityManager ready for use.
func NewPriorityManager(externalWorkerIDPriorityEndpoint string, refreshInterval time.Duration) *PriorityManager {
	ctx, cancel := context.WithCancel(context.Background())

	// Default to 15 minutes if not specified
	if refreshInterval <= 0 {
		refreshInterval = 15 * time.Minute
	}

	pm := &PriorityManager{
		priorityWorkers:                  make(map[string]bool),
		externalWorkerIDPriorityEndpoint: externalWorkerIDPriorityEndpoint,
		refreshInterval:                  refreshInterval,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		ctx:    ctx,
		cancel: cancel,
	}

	// Fetch initial priority list from external endpoint
	if externalWorkerIDPriorityEndpoint != "" {
		logrus.Infof("Fetching initial priority list from external endpoint: %s", externalWorkerIDPriorityEndpoint)
		if err := pm.fetchPriorityList(); err != nil {
			logrus.Warnf("Failed to fetch initial priority list: %v", err)
			// Initialize with dummy data as fallback
			pm.initializeDummyData()
		}

		// Start background refresh
		go pm.startBackgroundRefresh()
	} else {
		logrus.Info("No external worker ID priority endpoint configured, using dummy priority list")
		// Initialize with dummy data for testing
		pm.initializeDummyData()
	}

	return pm
}

// initializeDummyData populates the priority manager with test data.
// This is useful for local development and testing without an external endpoint.
//
// The dummy data includes various worker ID patterns that can be used
// to test the priority queue behavior.
//
// TODO: This method will be removed once real external endpoint integration is complete.
func (pm *PriorityManager) initializeDummyData() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// Dummy priority worker IDs
	dummyWorkers := []string{
		"worker-001",
		"worker-002",
		"worker-005",
		"worker-priority-1",
		"worker-priority-2",
		"worker-vip-1",
	}

	for _, workerID := range dummyWorkers {
		pm.priorityWorkers[workerID] = true
	}
}

// IsPriorityWorker checks if a given worker ID should receive priority processing.
//
// Parameters:
//   - workerID: The worker ID to check
//
// Returns true if the worker ID is in the priority list, false otherwise.
//
// This method is designed to be called frequently (on every job submission)
// and is optimized for performance with O(1) lookup time.
// Thread-safe: Can be called concurrently from multiple goroutines.
func (pm *PriorityManager) IsPriorityWorker(workerID string) bool {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.priorityWorkers[workerID]
}

// GetPriorityWorkers returns a snapshot of all worker IDs currently in the priority list.
//
// Returns a slice containing all priority worker IDs. The order is not guaranteed.
// The returned slice is a copy, so modifications won't affect the internal state.
//
// This method is useful for monitoring and debugging purposes.
// Thread-safe: Can be called concurrently from multiple goroutines.
func (pm *PriorityManager) GetPriorityWorkers() []string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	workers := make([]string, 0, len(pm.priorityWorkers))
	for workerID := range pm.priorityWorkers {
		workers = append(workers, workerID)
	}
	return workers
}

// UpdatePriorityWorkers replaces the entire priority worker list with a new set.
//
// Parameters:
//   - workerIDs: The new complete list of worker IDs that should have priority
//
// This method completely replaces the existing priority list. Any worker IDs
// not in the new list will lose their priority status.
//
// Thread-safe: Can be called concurrently with other methods.
func (pm *PriorityManager) UpdatePriorityWorkers(workerIDs []string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// Clear existing map
	pm.priorityWorkers = make(map[string]bool)

	// Add new worker IDs
	for _, workerID := range workerIDs {
		pm.priorityWorkers[workerID] = true
	}
}

// fetchPriorityList retrieves the latest priority worker list from the external endpoint.
//
// This method:
// 1. Makes an HTTP GET request to the configured endpoint
// 2. Parses the JSON response into PriorityWorkerList format
// 3. Updates the internal priority list with the new data
//
// Returns an error if:
// - No external endpoint is configured
// - The HTTP request fails
// - The response cannot be parsed
//
// Note: Currently returns dummy data for testing. The TODO comment indicates
// where real HTTP implementation should be added.
func (pm *PriorityManager) fetchPriorityList() error {
	if pm.externalWorkerIDPriorityEndpoint == "" {
		return fmt.Errorf("no external worker ID priority endpoint configured")
	}

	// Create HTTP request with context
	req, err := http.NewRequestWithContext(pm.ctx, http.MethodGet, pm.externalWorkerIDPriorityEndpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Execute request
	resp, err := pm.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch priority list: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Parse JSON response
	var workerList PriorityWorkerList
	if err := json.NewDecoder(resp.Body).Decode(&workerList); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	// Extract worker IDs from URLs
	// The response contains full URLs like "https://20.245.90.64:50001"
	// We need to extract just the worker IDs if they're embedded in the URLs
	// For now, we'll use the full URLs as the worker IDs
	workerIDs := make([]string, 0, len(workerList.Workers))
	for _, workerURL := range workerList.Workers {
		// You can add logic here to extract worker ID from URL if needed
		// For example, if the worker ID is the host:port combination
		workerIDs = append(workerIDs, workerURL)
	}

	pm.UpdatePriorityWorkers(workerIDs)

	// Log the update for debugging
	logrus.Infof("Priority list updated with %d workers from external endpoint", len(workerIDs))

	return nil
}

// startBackgroundRefresh runs a background goroutine that periodically fetches
// updates from the external endpoint.
//
// This method:
// - Runs indefinitely until Stop() is called
// - Refreshes at the interval specified during initialization
// - Logs errors but continues running if a refresh fails
//
// This method should be called as a goroutine and is started automatically
// by NewPriorityManager when an external endpoint is configured.
func (pm *PriorityManager) startBackgroundRefresh() {
	ticker := time.NewTicker(pm.refreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-pm.ctx.Done():
			return
		case <-ticker.C:
			if err := pm.fetchPriorityList(); err != nil {
				// Log error but continue running
				logrus.Errorf("Error refreshing priority list: %v", err)
			}
		}
	}
}

// Stop gracefully shuts down the priority manager.
//
// This method:
// - Cancels the background refresh goroutine
// - Ensures all resources are properly cleaned up
//
// After calling Stop, the manager can still be queried but will no longer
// update from the external endpoint.
//
// This method is idempotent and can be called multiple times safely.
func (pm *PriorityManager) Stop() {
	pm.cancel()
}
