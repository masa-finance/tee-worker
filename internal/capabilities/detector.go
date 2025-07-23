package capabilities

import (
	"github.com/masa-finance/tee-worker/api/types"
)

// JobServerInterface defines the methods we need from JobServer to avoid circular dependencies
type JobServerInterface interface {
	GetWorkerCapabilities() types.WorkerCapabilities
}

// DetectCapabilities automatically detects available capabilities based on configuration
// If jobServer is provided, it will use the actual worker capabilities
func DetectCapabilities(jc types.JobConfiguration, jobServer JobServerInterface) types.WorkerCapabilities {
	// If we have a JobServer, get capabilities directly from the workers
	if jobServer != nil {
		return jobServer.GetWorkerCapabilities()
	}

	// Fallback to basic detection if no JobServer is available
	// This maintains backward compatibility and is used during initialization
	var capabilities types.WorkerCapabilities

	// Always available scrapers
	capabilities = append(capabilities,
		types.JobCapability{
			JobType:      "web",
			Capabilities: []types.Capability{"web-scraper"},
		},
		types.JobCapability{
			JobType:      "telemetry",
			Capabilities: []types.Capability{"telemetry"},
		},
		types.JobCapability{
			JobType:      "tiktok",
			Capabilities: []types.Capability{"tiktok-transcription"},
		},
	)

	// Twitter capabilities based on configuration
	if accounts, ok := jc["twitter_accounts"].([]string); ok && len(accounts) > 0 {
		allTwitterCaps := []types.Capability{
			"searchbyquery", "searchbyfullarchive", "searchbyprofile",
			"getbyid", "getreplies", "getretweeters", "gettweets", "getmedia",
			"gethometweets", "getforyoutweets", "getprofilebyid",
			"gettrends", "getfollowing", "getfollowers", "getspace",
		}

		capabilities = append(capabilities,
			types.JobCapability{
				JobType:      "twitter-credential",
				Capabilities: allTwitterCaps,
			},
			types.JobCapability{
				JobType:      "twitter",
				Capabilities: allTwitterCaps,
			},
		)
	}

	if apiKeys, ok := jc["twitter_api_keys"].([]string); ok && len(apiKeys) > 0 {
		apiCaps := []types.Capability{"searchbyquery", "getbyid", "getprofilebyid"}
		// Note: Can't detect elevated keys during fallback

		capabilities = append(capabilities, types.JobCapability{
			JobType:      "twitter-api",
			Capabilities: apiCaps,
		})

		// If we don't already have general twitter (no accounts), add it
		hasGeneralTwitter := false
		for _, cap := range capabilities {
			if cap.JobType == "twitter" {
				hasGeneralTwitter = true
				break
			}
		}
		if !hasGeneralTwitter {
			capabilities = append(capabilities, types.JobCapability{
				JobType:      "twitter",
				Capabilities: apiCaps,
			})
		}
	}

	return capabilities
}
