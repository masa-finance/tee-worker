package capabilities

import (
	teetypes "github.com/masa-finance/tee-types/types"
	"github.com/masa-finance/tee-worker/api/types"
)

// AlwaysAvailableCapabilities defines the scrapers that are always available regardless of configuration
var AlwaysAvailableCapabilities = teetypes.WorkerCapabilities{
	teetypes.JobCapability{
		JobType:      string(teetypes.WebJob),
		Capabilities: []teetypes.Capability{"web-scraper"},
	},
	teetypes.JobCapability{
		JobType:      string(teetypes.TelemetryJob),
		Capabilities: []teetypes.Capability{"telemetry"},
	},
	teetypes.JobCapability{
		JobType:      string(teetypes.TiktokJob),
		Capabilities: []teetypes.Capability{"tiktok-transcription"},
	},
}

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
	capabilities = append(capabilities, AlwaysAvailableCapabilities...)

	// Twitter capabilities based on configuration
	if accounts, ok := jc["twitter_accounts"].([]string); ok && len(accounts) > 0 {
		allTwitterCaps := []teetypes.Capability{
			"searchbyquery", "searchbyfullarchive", "searchbyprofile",
			"getbyid", "getreplies", "getretweeters", "gettweets", "getmedia",
			"gethometweets", "getforyoutweets", "getprofilebyid",
			"gettrends", "getfollowing", "getfollowers", "getspace",
		}

		capabilities = append(capabilities,
			teetypes.JobCapability{
				JobType:      string(teetypes.TwitterCredentialJob),
				Capabilities: allTwitterCaps,
			},
			teetypes.JobCapability{
				JobType:      string(teetypes.TwitterJob),
				Capabilities: allTwitterCaps,
			},
		)
	}

	// Twitter API capabilities based on configuration
	if apiKeys, ok := jc["twitter_api_keys"].([]string); ok && len(apiKeys) > 0 {
		apiCaps := []teetypes.Capability{"searchbyquery", "getbyid", "getprofilebyid"}

		capabilities = append(capabilities,
			teetypes.JobCapability{
				JobType:      string(teetypes.TwitterApiJob),
				Capabilities: apiCaps,
			},
			teetypes.JobCapability{
				JobType:      string(teetypes.TwitterJob),
				Capabilities: apiCaps,
			},
		)
	}

	return capabilities
}
