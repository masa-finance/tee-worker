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

	verifiersMap := make(map[string]health.Verifier)

	// Register Web Scraper Verifier
	verifiersMap["web-scraper"] = verifiers.NewWebScraperVerifier()

	// Register TikTok Verifier
	verifiersMap["tiktok-transcription"] = verifiers.NewTikTokVerifier()

	// Register Twitter Verifier if accounts are present
	if twitterAccounts, ok := jc["twitter_accounts"].([]string); ok && len(twitterAccounts) > 0 {
		dataDir, _ := jc["data_dir"].(string)
		twitterVerifier, err := verifiers.NewTwitterVerifier(twitterAccounts, dataDir)
		if err != nil {
			logrus.WithError(err).Error("Failed to initialize Twitter verifier")
		} else {
			// These capabilities are tied to twitter credentials
			verifiersMap["searchbyquery"] = twitterVerifier
			verifiersMap["getbyid"] = twitterVerifier
			verifiersMap["getprofilebyid"] = twitterVerifier
		}
	}

	// Register LinkedIn Verifier if credentials are present
	var liCreds []types.LinkedInCredential
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
		liAt, _ := jc["linkedin_li_at_cookie"].(string)
		csrf, _ := jc["linkedin_csrf_token"].(string)
		jsess, _ := jc["linkedin_jsessionid"].(string)
		if liAt != "" && csrf != "" {
			liCreds = append(liCreds, types.LinkedInCredential{LiAtCookie: liAt, CSRFToken: csrf, JSESSIONID: jsess})
		}
	}

	if len(liCreds) > 0 {
		linkedInVerifier, err := verifiers.NewLinkedInVerifier(liCreds)
		if err != nil {
			logrus.WithError(err).Error("Failed to initialize LinkedIn verifier")
		} else {
			// These capabilities are tied to linkedin credentials
			verifiersMap["getprofile"] = linkedInVerifier
		}
	}

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
		if !slices.Contains(detected, "searchbyquery") {
			// searchbyquery is a twitter capability, but we are adding it here for linkedin as well
			// This is because the capabilities are not yet granular enough
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
