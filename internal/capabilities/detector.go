package capabilities

import (
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
	var capabilities teetypes.WorkerCapabilities

	// Start with always available scrapers
	capabilities = append(capabilities, teetypes.AlwaysAvailableCapabilities...)

	// Check what Twitter authentication methods are available
	hasAccounts := jc.GetStringSlice("twitter_accounts", nil)
	hasApiKeys := jc.GetStringSlice("twitter_api_keys", nil)

	accountsAvailable := len(hasAccounts) > 0
	apiKeysAvailable := len(hasApiKeys) > 0

	// Add Twitter-specific capabilities based on available authentication
	if accountsAvailable {
		capabilities = append(capabilities,
			teetypes.JobCapability{
				JobType:      teetypes.TwitterCredentialJob,
				Capabilities: teetypes.TwitterAllCaps,
			},
		)
	}

	if apiKeysAvailable {
		// Start with basic API capabilities
		apiCaps := make([]teetypes.Capability, len(teetypes.TwitterAPICaps))
		copy(apiCaps, teetypes.TwitterAPICaps)

		// Check for elevated API keys and add searchbyfullarchive capability
		if hasElevatedApiKey(hasApiKeys) {
			apiCaps = append(apiCaps, teetypes.CapSearchByFullArchive)
		}

		capabilities = append(capabilities,
			teetypes.JobCapability{
				JobType:      teetypes.TwitterApiJob,
				Capabilities: apiCaps,
			},
		)
	}

	// Add general TwitterJob capability if any Twitter auth is available
	if accountsAvailable || apiKeysAvailable {
		var twitterJobCaps []teetypes.Capability
		// Use the most comprehensive capabilities available
		if accountsAvailable {
			twitterJobCaps = teetypes.TwitterAllCaps
		} else {
			// Use API capabilities if we only have keys
			twitterJobCaps = make([]teetypes.Capability, len(teetypes.TwitterAPICaps))
			copy(twitterJobCaps, teetypes.TwitterAPICaps)

			// Check for elevated API keys and add searchbyfullarchive capability
			if hasElevatedApiKey(hasApiKeys) {
				twitterJobCaps = append(twitterJobCaps, teetypes.CapSearchByFullArchive)
			}
		}

		capabilities = append(capabilities,
			teetypes.JobCapability{
				JobType:      teetypes.TwitterJob,
				Capabilities: twitterJobCaps,
			},
		)
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
	for _, apiKey := range accountManager.GetApiKeys() {
		if apiKey.Type == twitter.TwitterApiKeyTypeElevated {
			return true
		}
	}

	return false
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
