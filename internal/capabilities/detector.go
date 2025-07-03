package capabilities

import (
	"strings"

	"golang.org/x/exp/slices"

	"github.com/masa-finance/tee-worker/api/types"
)

// JobServerInterface defines the methods we need from JobServer to avoid circular dependencies
type JobServerInterface interface {
	GetWorkerCapabilities() map[string][]string
}

// DetectCapabilities automatically detects available capabilities based on configuration
// If jobServer is provided, it will use the actual worker capabilities
func DetectCapabilities(jc types.JobConfiguration, jobServer JobServerInterface) []string {
	var detected []string

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
