package capabilities

import (
	"context"
	"strings"

	"github.com/masa-finance/tee-worker/internal/capabilities/verifiers"
	"github.com/sirupsen/logrus"
	"golang.org/x/exp/slices"

	"github.com/masa-finance/tee-worker/api/types"
	"github.com/masa-finance/tee-worker/internal/capabilities/health"
)

// JobServerInterface defines the methods we need from JobServer to avoid circular dependencies
type JobServerInterface interface {
	GetWorkerCapabilities() map[string][]string
}

// VerifierConfig defines how to create and register a verifier
type VerifierConfig struct {
	Name         string
	Capabilities []string
	ConfigCheck  func(types.JobConfiguration) bool
	Factory      func(types.JobConfiguration) (health.Verifier, error)
}

// verifierConfigs defines all available verifiers and their configuration
var verifierConfigs = []VerifierConfig{
	{
		Name:         "web-scraper",
		Capabilities: []string{"web-scraper"},
		ConfigCheck:  func(jc types.JobConfiguration) bool { return true }, // Always available
		Factory: func(jc types.JobConfiguration) (health.Verifier, error) {
			return verifiers.NewWebScraperVerifier(), nil
		},
	},
	{
		Name:         "tiktok-transcription",
		Capabilities: []string{"tiktok-transcription"},
		ConfigCheck:  func(jc types.JobConfiguration) bool { return true }, // Always available
		Factory: func(jc types.JobConfiguration) (health.Verifier, error) {
			return verifiers.NewTikTokVerifier(), nil
		},
	},
	{
		Name:         "twitter-credentials",
		Capabilities: []string{"searchbyquery", "getbyid", "getprofilebyid"},
		ConfigCheck: func(jc types.JobConfiguration) bool {
			accounts, ok := jc["twitter_accounts"].([]string)
			return ok && len(accounts) > 0
		},
		Factory: func(jc types.JobConfiguration) (health.Verifier, error) {
			accounts := jc["twitter_accounts"].([]string)
			dataDir, _ := jc["data_dir"].(string)
			return verifiers.NewTwitterVerifier(accounts, dataDir)
		},
	},
	{
		Name:         "twitter-api-keys",
		Capabilities: []string{"searchbyquery", "getbyid", "getprofilebyid"},
		ConfigCheck: func(jc types.JobConfiguration) bool {
			apiKeys, ok := jc["twitter_api_keys"].([]string)
			return ok && len(apiKeys) > 0
		},
		Factory: func(jc types.JobConfiguration) (health.Verifier, error) {
			apiKeys := jc["twitter_api_keys"].([]string)
			return verifiers.NewTwitterApiKeyVerifier(apiKeys)
		},
	},
	{
		Name:         "linkedin",
		Capabilities: []string{"getprofile"},
		ConfigCheck: func(jc types.JobConfiguration) bool {
			return hasLinkedInCredentials(jc)
		},
		Factory: func(jc types.JobConfiguration) (health.Verifier, error) {
			liCreds := parseLinkedInCredentials(jc)
			return verifiers.NewLinkedInVerifier(liCreds)
		},
	},
}

// registerVerifiers creates verifiers based on configuration using table-driven approach
func registerVerifiers(jc types.JobConfiguration) map[string]health.Verifier {
	verifiersMap := make(map[string]health.Verifier)
	registeredCapabilities := make(map[string]bool)

	for _, config := range verifierConfigs {
		if !config.ConfigCheck(jc) {
			continue
		}

		// Skip if capabilities are already registered (Twitter credentials take precedence over API keys)
		hasConflict := false
		for _, cap := range config.Capabilities {
			if registeredCapabilities[cap] {
				hasConflict = true
				break
			}
		}
		if hasConflict {
			logrus.WithField("verifier", config.Name).Debug("Skipping verifier due to capability conflict")
			continue
		}

		verifier, err := config.Factory(jc)
		if err != nil {
			logrus.WithError(err).WithField("verifier", config.Name).Error("Failed to initialize verifier")
			continue
		}

		// Register verifier for all its capabilities
		for _, cap := range config.Capabilities {
			verifiersMap[cap] = verifier
			registeredCapabilities[cap] = true
		}

		logrus.WithField("verifier", config.Name).WithField("capabilities", config.Capabilities).Debug("Registered verifier")
	}

	return verifiersMap
}

// hasLinkedInCredentials checks if LinkedIn credentials are available in any format
func hasLinkedInCredentials(jc types.JobConfiguration) bool {
	// Check for linkedin_credentials array
	if linkedinCreds, ok := jc["linkedin_credentials"].([]interface{}); ok && len(linkedinCreds) > 0 {
		return true
	}

	// Check for individual LinkedIn credential fields
	liAtCookie, _ := jc["linkedin_li_at_cookie"].(string)
	csrfToken, _ := jc["linkedin_csrf_token"].(string)
	jsessionID, _ := jc["linkedin_jsessionid"].(string)
	return liAtCookie != "" && csrfToken != "" && jsessionID != ""
}

// parseLinkedInCredentials extracts LinkedIn credentials from job configuration
func parseLinkedInCredentials(jc types.JobConfiguration) []types.LinkedInCredential {
	var liCreds []types.LinkedInCredential

	// Try linkedin_credentials array first
	if creds, ok := jc["linkedin_credentials"].([]interface{}); ok {
		for _, credInterface := range creds {
			if credMap, ok := credInterface.(map[string]interface{}); ok {
				cred := types.LinkedInCredential{
					LiAtCookie: credMap["li_at_cookie"].(string),
					CSRFToken:  credMap["csrf_token"].(string),
					JSESSIONID: credMap["jsessionid"].(string),
				}
				liCreds = append(liCreds, cred)
			}
		}
	} else {
		// Fall back to individual credential fields
		liAt, _ := jc["linkedin_li_at_cookie"].(string)
		csrf, _ := jc["linkedin_csrf_token"].(string)
		jsess, _ := jc["linkedin_jsessionid"].(string)
		if liAt != "" && csrf != "" {
			liCreds = append(liCreds, types.LinkedInCredential{LiAtCookie: liAt, CSRFToken: csrf, JSESSIONID: jsess})
		}
	}

	return liCreds
}

// DetectCapabilities automatically detects and verifies available capabilities.
// It returns a list of only the healthy, verified capabilities and the configured health tracker.
func DetectCapabilities(ctx context.Context, jc types.JobConfiguration, jobServer JobServerInterface) ([]string, health.CapabilityHealthTracker) {
	if jobServer != nil {
		// When running with a job server, we rely on the capabilities reported by workers.
		// The health tracking is managed by the individual workers, so we don't need a central tracker here.
		var detected []string
		workerCaps := jobServer.GetWorkerCapabilities()
		for _, caps := range workerCaps {
			detected = append(detected, caps...)
		}
		return detected, nil
	}

	detectedCaps := detectCapabilitiesFromConfig(jc)
	tracker := health.NewTracker()
	verifier := NewCapabilityVerifier(tracker)

	// Use table-driven verifier registration
	verifiersMap := registerVerifiers(jc)

	// Register all verifiers with the main verifier and the tracker
	for name, v := range verifiersMap {
		verifier.RegisterVerifier(name, v)
	}
	tracker.SetVerifiers(verifiersMap)

	verifier.VerifyCapabilities(ctx, detectedCaps)

	var healthyCapabilities []string
	allStatuses := tracker.GetAllStatuses()
	for _, capName := range detectedCaps {
		if status, exists := allStatuses[capName]; exists && status.IsHealthy {
			healthyCapabilities = append(healthyCapabilities, capName)
		}
	}
	logrus.Infof("Verified and healthy capabilities: %v", healthyCapabilities)
	return healthyCapabilities, tracker
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
