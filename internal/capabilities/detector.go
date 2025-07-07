package capabilities

import (
	"context"
	"strings"
	"sync"
	"time"

	"golang.org/x/exp/slices"

	"github.com/masa-finance/tee-worker/api/types"
	"github.com/masa-finance/tee-worker/internal/capabilities/health"
)

// inMemoryHealthTracker is a temporary, in-memory implementation of CapabilityHealthTracker.
// This will be replaced by the full implementation in Step 4.
type inMemoryHealthTracker struct {
	statuses map[string]health.CapabilityStatus
	mu       sync.RWMutex
}

func newInMemoryHealthTracker() *inMemoryHealthTracker {
	return &inMemoryHealthTracker{
		statuses: make(map[string]health.CapabilityStatus),
	}
}

func (t *inMemoryHealthTracker) UpdateStatus(name string, isHealthy bool, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	status := t.statuses[name]
	status.Name = name
	status.IsHealthy = isHealthy
	status.LastChecked = time.Now()

	if err != nil {
		status.LastError = err.Error()
		if !isHealthy {
			status.ErrorCount++
		}
	} else {
		status.LastError = ""
	}
	t.statuses[name] = status
}

func (t *inMemoryHealthTracker) GetStatus(name string) (health.CapabilityStatus, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	status, exists := t.statuses[name]
	return status, exists
}

func (t *inMemoryHealthTracker) GetAllStatuses() map[string]health.CapabilityStatus {
	t.mu.RLock()
	defer t.mu.RUnlock()
	// Return a copy to prevent race conditions
	statusesCopy := make(map[string]health.CapabilityStatus)
	for k, v := range t.statuses {
		statusesCopy[k] = v
	}
	return statusesCopy
}

// JobServerInterface defines the methods we need from JobServer to avoid circular dependencies
type JobServerInterface interface {
	GetWorkerCapabilities() map[string][]string
}

// DetectCapabilities automatically detects and verifies available capabilities.
// It returns a list of only the healthy, verified capabilities.
func DetectCapabilities(ctx context.Context, jc types.JobConfiguration, jobServer JobServerInterface) []string {
	// If we have a JobServer, get capabilities directly from the workers
	// The verification logic currently runs at the individual worker level on startup.
	// This top-level path may need revisiting if centralized verification is required.
	if jobServer != nil {
		var detected []string
		workerCaps := jobServer.GetWorkerCapabilities()
		for _, caps := range workerCaps {
			detected = append(detected, caps...)
		}
		// Note: We are currently trusting the capabilities reported by the job server workers,
		// as they should have run their own verification.
		return detected
	}

	// For a standalone worker, detect from config and then verify.
	detectedCaps := detectCapabilitiesFromConfig(jc)

	// Step 1: Initialize the health tracker and verifier
	tracker := newInMemoryHealthTracker()
	verifier := NewCapabilityVerifier(tracker)

	// Step 2: Register specific verifiers (we'll add real ones later)
	// For now, no verifiers are registered, so all capabilities will be marked as healthy by default.

	// Step 3: Run verification
	verifier.VerifyCapabilities(ctx, detectedCaps)

	// Step 4: Filter out unhealthy capabilities
	var healthyCapabilities []string
	allStatuses := tracker.GetAllStatuses()
	for _, capName := range detectedCaps {
		// A capability is considered healthy if it passes verification, or if there was no
		// specific verifier for it (in which case it's assumed to be healthy).
		if status, exists := allStatuses[capName]; exists && status.IsHealthy {
			healthyCapabilities = append(healthyCapabilities, capName)
		}
	}
	return healthyCapabilities
}

// detectCapabilitiesFromConfig detects potential capabilities based on configuration.
// It doesn't verify if they are functional.
func detectCapabilitiesFromConfig(jc types.JobConfiguration) []string {
	var detected []string
	// Always available capabilities
	detected = append(detected, "web-scraper", "telemetry", "tiktok-transcription")

	// Check for Twitter capabilities based on credentials
	if accounts, ok := jc["twitter_accounts"].([]string); ok && len(accounts) > 0 {
		// Basic Twitter capabilities when accounts are available
		detected = append(detected, "searchbyquery", "getbyid", "getprofilebyid")
	}

	if apiKeys, ok := jc["twitter_api_keys"].([]string); ok && len(apiKeys) > 0 {
		// Basic API capabilities
		if !slices.Contains(detected, "searchbyquery") {
			detected = append(detected, "searchbyquery", "getbyid", "getprofilebyid")
		}
	}

	// Check for LinkedIn capabilities based on credentials
	hasLinkedInCreds := false

	// Check for linkedin_credentials array
	if linkedinCreds, ok := jc["linkedin_credentials"].([]interface{}); ok && len(linkedinCreds) > 0 {
		hasLinkedInCreds = true
	}

	// Check for individual LinkedIn credential fields
	if !hasLinkedInCreds {
		liAtCookie, _ := jc["linkedin_li_at_cookie"].(string)
		csrfToken, _ := jc["linkedin_csrf_token"].(string)
		jsessionID, _ := jc["linkedin_jsessionid"].(string)
		if liAtCookie != "" && csrfToken != "" && jsessionID != "" {
			hasLinkedInCreds = true
		}
	}

	if hasLinkedInCreds {
		// Add LinkedIn capabilities when credentials are available
		if !slices.Contains(detected, "searchbyquery") {
			detected = append(detected, "searchbyquery")
		}
		detected = append(detected, "getprofile")
	}

	return detected
}

// MergeCapabilities combines manual and auto-detected capabilities
func MergeCapabilities(manual string, detected []string) []string {
	// Parse manual capabilities
	var manualCaps []string
	if manual != "" {
		manualCaps = strings.Split(manual, ",")
		// Trim whitespace
		for i := range manualCaps {
			manualCaps[i] = strings.TrimSpace(manualCaps[i])
		}
	}

	// Use a map to deduplicate
	capMap := make(map[string]bool)

	// Add manual capabilities first (they take precedence)
	for _, capability := range manualCaps {
		if capability != "" {
			capMap[capability] = true
		}
	}

	// Add auto-detected capabilities
	for _, capability := range detected {
		if capability != "" {
			capMap[capability] = true
		}
	}

	// Convert back to slice
	var result []string
	for capability := range capMap {
		result = append(result, capability)
	}

	return result
}
