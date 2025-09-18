package capabilities

import (
	"slices"
	"strings"

	"maps"

	util "github.com/masa-finance/tee-types/pkg/util"
	teetypes "github.com/masa-finance/tee-types/types"
	"github.com/masa-finance/tee-worker/internal/apify"
	"github.com/masa-finance/tee-worker/internal/config"
	"github.com/masa-finance/tee-worker/internal/jobs/twitter"
	"github.com/masa-finance/tee-worker/pkg/client"
	"github.com/sirupsen/logrus"
)

// JobServerInterface defines the methods we need from JobServer to avoid circular dependencies
type JobServerInterface interface {
	GetWorkerCapabilities() teetypes.WorkerCapabilities
}

// DetectCapabilities automatically detects available capabilities based on configuration
// If jobServer is provided, it will use the actual worker capabilities
func DetectCapabilities(jc config.JobConfiguration, jobServer JobServerInterface) teetypes.WorkerCapabilities {
	// If we have a JobServer, get capabilities directly from the workers
	if jobServer != nil {
		return jobServer.GetWorkerCapabilities()
	}

	// Fallback to basic detection if no JobServer is available
	// This maintains backward compatibility and is used during initialization
	capabilities := make(teetypes.WorkerCapabilities)

	// Start with always available capabilities
	maps.Copy(capabilities, teetypes.AlwaysAvailableCapabilities)

	// Check what Twitter authentication methods are available
	accounts := jc.GetStringSlice("twitter_accounts", nil)
	apiKeys := jc.GetStringSlice("twitter_api_keys", nil)
	apifyApiKey := jc.GetString("apify_api_key", "")
	geminiApiKey := config.LlmApiKey(jc.GetString("gemini_api_key", ""))

	hasAccounts := len(accounts) > 0
	hasApiKeys := len(apiKeys) > 0
	hasApifyKey := hasValidApifyKey(apifyApiKey)
	hasLLMKey := geminiApiKey.IsValid()

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
		// Add default Apify capabilities
		capabilities[teetypes.TwitterApifyJob] = teetypes.TwitterApifyCaps

		// Create an Apify client for probing rented actors
		c, err := client.NewApifyClient(apifyApiKey)
		if err != nil {
			logrus.Errorf("Failed to create Apify client for access probe: %v", err)
		} else {
			// Reddit access probe
			if ok, _ := c.ProbeActorAccess(apify.ActorIds.RedditScraper, map[string]any{}); ok {
				capabilities[teetypes.RedditJob] = teetypes.RedditCaps
			} else {
				logrus.Warnf("Apify token does not have access to actor %s", apify.ActorIds.RedditScraper)
			}

			// TikTok probes (search and trending handled independently)
			tiktokCapSet := util.NewSet(capabilities[teetypes.TiktokJob]...)

			if ok, _ := c.ProbeActorAccess(apify.ActorIds.TikTokSearchScraper, map[string]any{"proxy": map[string]any{"useApifyProxy": true}}); ok {
				tiktokCapSet.Add(teetypes.CapSearchByQuery)
			} else {
				logrus.Warnf("Apify token does not have access to actor %s", apify.ActorIds.TikTokSearchScraper)
			}
			if ok, _ := c.ProbeActorAccess(apify.ActorIds.TikTokTrendingScraper, map[string]any{}); ok {
				tiktokCapSet.Add(teetypes.CapSearchByTrending)
			} else {
				logrus.Warnf("Apify token does not have access to actor %s", apify.ActorIds.TikTokTrendingScraper)
			}

			capabilities[teetypes.TiktokJob] = tiktokCapSet.Items()

		}

		if hasLLMKey {
			capabilities[teetypes.WebJob] = teetypes.WebCaps
		}
	}

	// Add general TwitterJob capability if any Twitter auth is available
	// TODO: this will get cleaned up with unique twitter capabilities
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

// hasValidApifyKey checks if the provided Apify API key is valid by attempting to validate it
func hasValidApifyKey(apifyApiKey string) bool {
	if apifyApiKey == "" {
		return false
	}

	// Create temporary Apify client and validate the key
	apifyClient, err := client.NewApifyClient(apifyApiKey)
	if err != nil {
		logrus.Errorf("Failed to create Apify client during capability detection: %v", err)
		return false
	}

	if err := apifyClient.ValidateApiKey(); err != nil {
		logrus.Errorf("Apify API key validation failed during capability detection: %v", err)
		return false
	}

	logrus.Infof("Apify API key validated successfully during capability detection")
	return true
}
