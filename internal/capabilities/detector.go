package capabilities

import (
	"strings"

	"github.com/masa-finance/tee-worker/api/types"
	"github.com/sirupsen/logrus"
)

// DetectCapabilities automatically detects available capabilities based on configuration
func DetectCapabilities(jc types.JobConfiguration) []string {
	var detected []string

	// Check for Twitter capabilities
	if twitterCapabilities := detectTwitterCapabilities(jc); len(twitterCapabilities) > 0 {
		detected = append(detected, twitterCapabilities...)
	}

	// Check for TikTok capabilities
	if tiktokCapabilities := detectTikTokCapabilities(jc); len(tiktokCapabilities) > 0 {
		detected = append(detected, tiktokCapabilities...)
	}

	// Check for web scraper capabilities (always available)
	detected = append(detected, "web-scraper")

	// Check for telemetry capabilities (always available)
	detected = append(detected, "telemetry")

	return detected
}

// detectTwitterCapabilities detects Twitter capabilities based on available credentials
func detectTwitterCapabilities(jc types.JobConfiguration) []string {
	var capabilities []string

	// Check for Twitter accounts (username:password pairs)
	if accounts, ok := jc["twitter_accounts"].([]string); ok && len(accounts) > 0 {
		// For now, we'll check if accounts are in the correct format (username:password)
		validAccounts := 0
		for _, acc := range accounts {
			if strings.Contains(acc, ":") {
				validAccounts++
			}
		}
		if validAccounts > 0 {
			// Credential-based capabilities
			capabilities = append(capabilities,
				"searchbyquery",
				"searchbyprofile",
				"searchfollowers",
				"getbyid",
				"getreplies",
				"getretweeters",
				"gettweets",
				"getmedia",
				"gethometweets",
				"getforyoutweets",
				"getbookmarks",
				"getprofilebyid",
				"gettrends",
				"getfollowing",
				"getfollowers",
				"getspace",
			)
			logrus.Debug("Detected Twitter credential-based capabilities")
		}
	}

	// Check for Twitter API keys
	if apiKeys, ok := jc["twitter_api_keys"].([]string); ok && len(apiKeys) > 0 {
		// API-based capabilities (subset of credential-based)
		if !hasCapability(capabilities, "searchbyquery") {
			capabilities = append(capabilities,
				"searchbyquery",
				"getbyid",
				"getprofilebyid",
			)
		}

		// For elevated API detection, we'll need to create API keys and check their type
		// This requires making API calls, so we'll add a note that full archive search
		// capability detection happens at runtime
		logrus.Debug("Detected Twitter API-based capabilities (full archive detection happens at runtime)")

		// Note: In a production implementation, you might want to cache the API key types
		// to avoid making API calls during capability detection. For now, we'll assume
		// that if API keys are present, basic search is available, and elevated access
		// will be determined when the Twitter scraper is initialized.
	}

	return capabilities
}

// detectTikTokCapabilities detects TikTok capabilities based on configuration
func detectTikTokCapabilities(_ types.JobConfiguration) []string {
	// TikTok transcription is always available as it doesn't require authentication
	// The capability is based on the worker's ability to process TikTok URLs
	return []string{"tiktok-transcription"}
}

// hasCapability checks if a capability already exists in the list
func hasCapability(capabilities []string, capability string) bool {
	for _, c := range capabilities {
		if c == capability {
			return true
		}
	}
	return false
}

// MergeCapabilities combines manual and auto-detected capabilities
func MergeCapabilities(manual string, detected []string) []string {
	// Parse manual capabilities
	var manualCaps []string
	if manual != "" {
		if strings.Contains(manual, ",") {
			manualCaps = strings.Split(manual, ",")
		} else {
			manualCaps = []string{manual}
		}
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