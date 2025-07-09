package capabilities

import (
	"golang.org/x/exp/slices"
	"strings"

	"github.com/masa-finance/tee-worker/api/types"
)

// JobServerInterface defines the methods we need from JobServer to avoid circular dependencies
type JobServerInterface interface {
	GetWorkerCapabilities() map[string][]types.Capability
}

// DetectCapabilities automatically detects available capabilities based on configuration
// If jobServer is provided, it will use the actual worker capabilities
func DetectCapabilities(jc types.JobConfiguration, jobServer JobServerInterface) []types.Capability {
	var detected []types.Capability

	// If we have a JobServer, get capabilities directly from the workers
	if jobServer != nil {
		workerCaps := jobServer.GetWorkerCapabilities()
		for _, caps := range workerCaps {
			detected = append(detected, caps...)
		}
		return detected
	}

	// Fallback to basic detection if no JobServer is available
	// This maintains backward compatibility and is used during initialization

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

	return detected
}

// MergeCapabilities combines manual and auto-detected capabilities
func MergeCapabilities(manual string, detected []types.Capability) []types.Capability {
	// Parse manual capabilities
	var manualCaps []types.Capability
	if manual != "" {
		caps := strings.Split(manual, ",")
		// Trim whitespace
		for _, cap := range caps {
			manualCaps = append(manualCaps, types.Capability(strings.TrimSpace(cap)))
		}
	}

	// Use a map to deduplicate
	capMap := make(map[types.Capability]struct{})

	// Add manual capabilities first (they take precedence)
	for _, capability := range manualCaps {
		if capability != "" {
			capMap[capability] = struct{}{}
		}
	}

	// Add auto-detected capabilities
	for _, capability := range detected {
		if capability != "" {
			capMap[capability] = struct{}{}
		}
	}

	// Convert back to slice
	var result []types.Capability
	for capability := range capMap {
		result = append(result, capability)
	}

	return result
}
