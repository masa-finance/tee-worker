package jobserver

import (
	"context"
	_ "encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// PriorityManager manages the priority worker-id list
type PriorityManager struct {
	mu                               sync.RWMutex
	priorityWorkers                  map[string]bool
	externalWorkerIdPriorityEndpoint string
	refreshInterval                  time.Duration
	httpClient                       *http.Client
	ctx                              context.Context
	cancel                           context.CancelFunc
}

// PriorityWorkerList represents the response from external priority endpoint
type PriorityWorkerList struct {
	WorkerIDs []string `json:"worker_ids"`
	UpdatedAt string   `json:"updated_at"`
}

// NewPriorityManager creates a new priority manager
func NewPriorityManager(externalWorkerIdPriorityEndpoint string, refreshInterval time.Duration) *PriorityManager {
	ctx, cancel := context.WithCancel(context.Background())

	// Default to 15 minutes if not specified
	if refreshInterval <= 0 {
		refreshInterval = 15 * time.Minute
	}

	pm := &PriorityManager{
		priorityWorkers:                  make(map[string]bool),
		externalWorkerIdPriorityEndpoint: externalWorkerIdPriorityEndpoint,
		refreshInterval:                  refreshInterval,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		ctx:    ctx,
		cancel: cancel,
	}

	// Initialize with dummy data first
	// TODO: Replace this with actual external endpoint call
	pm.initializeDummyData()

	// Fetch initial priority list from external endpoint
	if externalWorkerIdPriorityEndpoint != "" {
		fmt.Printf("Fetching initial priority list from external endpoint: %s\n", externalWorkerIdPriorityEndpoint)
		if err := pm.fetchPriorityList(); err != nil {
			fmt.Printf("Warning: Failed to fetch initial priority list: %v (using dummy data)\n", err)
		}

		// Start background refresh
		go pm.startBackgroundRefresh()
	} else {
		fmt.Println("No external worker ID priority endpoint configured, using dummy priority list")
	}

	return pm
}

// initializeDummyData sets up dummy priority worker IDs for testing
// TODO: Replace this with actual external endpoint call
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

// IsPriorityWorker checks if a worker ID is in the priority list
func (pm *PriorityManager) IsPriorityWorker(workerID string) bool {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.priorityWorkers[workerID]
}

// GetPriorityWorkers returns the current list of priority worker IDs
func (pm *PriorityManager) GetPriorityWorkers() []string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	workers := make([]string, 0, len(pm.priorityWorkers))
	for workerID := range pm.priorityWorkers {
		workers = append(workers, workerID)
	}
	return workers
}

// UpdatePriorityWorkers updates the priority worker list
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

// fetchPriorityList fetches the priority list from external endpoint
func (pm *PriorityManager) fetchPriorityList() error {
	if pm.externalWorkerIdPriorityEndpoint == "" {
		return fmt.Errorf("no external worker ID priority endpoint configured")
	}

	// TODO: Replace this with actual external endpoint call
	// For now, simulate the external API call with a dummy response

	// Simulate network delay
	select {
	case <-time.After(100 * time.Millisecond):
	case <-pm.ctx.Done():
		return fmt.Errorf("context cancelled")
	}

	// Return dummy priority list
	// In production, this would be replaced with:
	// req, err := http.NewRequestWithContext(pm.ctx, http.MethodGet, pm.externalWorkerIdPriorityEndpoint, nil)
	// ... actual HTTP call logic ...

	dummyResponse := PriorityWorkerList{
		WorkerIDs: []string{
			"worker-001",
			"worker-002",
			"worker-005",
			"worker-priority-1",
			"worker-priority-2",
			"worker-vip-1",
			"worker-high-priority-3",
			"worker-fast-lane-1",
		},
		UpdatedAt: time.Now().Format(time.RFC3339),
	}

	pm.UpdatePriorityWorkers(dummyResponse.WorkerIDs)

	// Log the update for debugging
	fmt.Printf("Priority list updated with %d workers from external endpoint (dummy)\n", len(dummyResponse.WorkerIDs))

	return nil
}

// startBackgroundRefresh periodically refreshes the priority list
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
				fmt.Printf("Error refreshing priority list: %v\n", err)
			}
		}
	}
}

// Stop stops the background refresh
func (pm *PriorityManager) Stop() {
	pm.cancel()
}
