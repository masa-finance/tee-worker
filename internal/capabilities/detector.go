package capabilities

import (
	"slices"
	"strings"

	teetypes "github.com/masa-finance/tee-types/types"
	"github.com/masa-finance/tee-worker/api/types"
	"github.com/masa-finance/tee-worker/internal/jobs/twitter"
)

// JobServerInterface defines the methods we need from JobServer to avoid circular dependencies
type JobServerInterface interface {
	GetWorkerCapabilities() teetypes.WorkerCapabilities
}

// DetectCapabilities automatically detects available capabilities based on configuration
// If jobServer is provided, it will use the actual worker capabilities
func DetectCapabilities(jc types.JobConfiguration, jobServer JobServerInterface) teetypes.WorkerCapabilities {
	// If we have a JobServer, get capabilities directly from the workers
	if jobServer != nil {
		return jobServer.GetWorkerCapabilities()
	}

	// Fallback to basic detection if no JobServer is available
	// This maintains backward compatibility and is used during initialization
	capabilities := make(teetypes.WorkerCapabilities)

	// Start with always available capabilities
	for jobType, caps := range teetypes.AlwaysAvailableCapabilities {
		capabilities[jobType] = caps
	}

	// Check what Twitter authentication methods are available
	accounts := jc.GetStringSlice("twitter_accounts", nil)
	apiKeys := jc.GetStringSlice("twitter_api_keys", nil)
	apifyApiKey := jc.GetString("apify_api_key", "")

	hasAccounts := len(accounts) > 0
	hasApiKeys := len(apiKeys) > 0
	hasApifyKey := apifyApiKey != ""

	// Add Twitter-specific capabilities based on available authentication
	if hasAccounts {
		capabilities[teetypes.TwitterCredentialJob] = teetypes.TwitterCredentialCaps
	}

	if hasApiKeys {
		// Start with basic API capabilities
		apiCaps := make([]teetypes.Capability, len(teetypes.TwitterAPICaps))
		copy(apiCaps, teetypes.TwitterAPICaps)

		// Check for elevated API keys and add searchbyfullarchive capability
		if hasElevatedApiKey(apiKeys) {
			apiCaps = append(apiCaps, teetypes.CapSearchByFullArchive)
		}

		capabilities[teetypes.TwitterApiJob] = apiCaps
	}

	// Add Apify-specific capabilities based on available API key
	if hasApifyKey {
		capabilities[teetypes.TwitterApifyJob] = teetypes.TwitterApifyCaps
	}

	// Add general TwitterJob capability if any Twitter auth is available
	if hasAccounts || hasApiKeys || hasApifyKey {
		var twitterJobCaps []teetypes.Capability
		// Use the most comprehensive capabilities available
		if hasAccounts {
			twitterJobCaps = teetypes.TwitterCredentialCaps
		} else {
			// Use API capabilities if we only have keys
			twitterJobCaps = make([]teetypes.Capability, len(teetypes.TwitterAPICaps))
			copy(twitterJobCaps, teetypes.TwitterAPICaps)

			// Check for elevated API keys and add searchbyfullarchive capability
			if hasElevatedApiKey(apiKeys) {
				twitterJobCaps = append(twitterJobCaps, teetypes.CapSearchByFullArchive)
			}
		}

		// Add Apify capabilities if available
		if hasApifyKey {
			twitterJobCaps = append(twitterJobCaps, teetypes.TwitterApifyCaps...)
		}

		capabilities[teetypes.TwitterJob] = twitterJobCaps
	}

	return capabilities
}

// hasElevatedApiKey checks if any of the provided API keys are elevated
func hasElevatedApiKey(apiKeys []string) bool {
	if len(apiKeys) == 0 {
		return false
	}

	// Parse API keys and create account manager to detect types
	parsedApiKeys := parseApiKeys(apiKeys)
	accountManager := twitter.NewTwitterAccountManager(nil, parsedApiKeys)

	// Detect all API key types
	accountManager.DetectAllApiKeyTypes()

	// Check if any key is elevated
	return slices.ContainsFunc(accountManager.GetApiKeys(), func(apiKey *twitter.TwitterApiKey) bool {
		return apiKey.Type == twitter.TwitterApiKeyTypeElevated
	})
}

// parseApiKeys converts string API keys to TwitterApiKey structs
func parseApiKeys(apiKeys []string) []*twitter.TwitterApiKey {
	result := make([]*twitter.TwitterApiKey, 0, len(apiKeys))
	for _, key := range apiKeys {
		if trimmed := strings.TrimSpace(key); trimmed != "" {
			result = append(result, &twitter.TwitterApiKey{
				Key: trimmed,
			})
		}
	}
	return result
}
