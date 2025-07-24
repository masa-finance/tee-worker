package capabilities

import (
	teetypes "github.com/masa-finance/tee-types/types"
	"github.com/masa-finance/tee-worker/api/types"
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
	hasAccounts, _ := jc["twitter_accounts"].([]string)
	hasApiKeys, _ := jc["twitter_api_keys"].([]string)

	accountsAvailable := len(hasAccounts) > 0
	apiKeysAvailable := len(hasApiKeys) > 0

	// Add Twitter-specific capabilities based on available authentication
	if accountsAvailable {
		capabilities = append(capabilities,
			teetypes.JobCapability{
				JobType:      string(teetypes.TwitterCredentialJob),
				Capabilities: teetypes.TwitterAllCaps,
			},
		)
	}

	if apiKeysAvailable {
		capabilities = append(capabilities,
			teetypes.JobCapability{
				JobType:      string(teetypes.TwitterApiJob),
				Capabilities: teetypes.TwitterAPICaps,
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
			twitterJobCaps = teetypes.TwitterAPICaps
		}

		capabilities = append(capabilities,
			teetypes.JobCapability{
				JobType:      string(teetypes.TwitterJob),
				Capabilities: twitterJobCaps,
			},
		)
	}

	return capabilities
}
