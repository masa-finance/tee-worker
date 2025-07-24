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

	// Twitter capabilities based on configuration
	if accounts, ok := jc["twitter_accounts"].([]string); ok && len(accounts) > 0 {
		capabilities = append(capabilities,
			teetypes.JobCapability{
				JobType:      string(teetypes.TwitterCredentialJob),
				Capabilities: teetypes.TwitterAllCaps,
			},
			teetypes.JobCapability{
				JobType:      string(teetypes.TwitterJob),
				Capabilities: teetypes.TwitterAllCaps,
			},
		)
	}

	// Twitter API capabilities based on configuration
	if apiKeys, ok := jc["twitter_api_keys"].([]string); ok && len(apiKeys) > 0 {
		capabilities = append(capabilities,
			teetypes.JobCapability{
				JobType:      string(teetypes.TwitterApiJob),
				Capabilities: teetypes.TwitterAPICaps,
			},
			teetypes.JobCapability{
				JobType:      string(teetypes.TwitterJob),
				Capabilities: teetypes.TwitterAPICaps,
			},
		)
	}

	return capabilities
}
