package capabilities

import (
	"strings"

	"github.com/masa-finance/tee-worker/api/types"
)

// ScraperCapabilities represents capabilities for a specific scraper
type ScraperCapabilities struct {
	Scraper      string   `json:"scraper"`
	Capabilities []string `json:"capabilities"`
}

// JobServerInterface defines the methods we need from JobServer to avoid circular dependencies
type JobServerInterface interface {
	GetWorkerCapabilities() map[string][]string
}

// DetectCapabilities returns capabilities organized by scraper type
func DetectCapabilities(jc types.JobConfiguration, jobServer JobServerInterface) []ScraperCapabilities {
	var result []ScraperCapabilities

	// Check for manual capabilities from configuration
	manualCaps, _ := jc["capabilities"].(string)
	var manualCapsList []string
	if manualCaps != "" {
		for _, cap := range strings.Split(manualCaps, ",") {
			cap = strings.TrimSpace(cap)
			if cap != "" {
				manualCapsList = append(manualCapsList, cap)
			}
		}
	}

	// If we have a JobServer, get capabilities directly from the workers
	if jobServer != nil {
		workerCaps := jobServer.GetWorkerCapabilities()
		for scraperType, caps := range workerCaps {
			if len(caps) > 0 {
				// Merge with manual capabilities if any
				allCaps := caps
				if len(manualCapsList) > 0 {
					// Add manual capabilities that aren't already in the worker capabilities
					capMap := make(map[string]bool)
					for _, cap := range caps {
						capMap[cap] = true
					}
					for _, manualCap := range manualCapsList {
						if !capMap[manualCap] {
							allCaps = append(allCaps, manualCap)
						}
					}
				}
				result = append(result, ScraperCapabilities{
					Scraper:      scraperType,
					Capabilities: allCaps,
				})
			}
		}

		// Add manual-only capabilities if they don't belong to any worker
		if len(manualCapsList) > 0 {
			// Check which manual capabilities weren't added to any worker
			addedCaps := make(map[string]bool)
			for _, sc := range result {
				for _, cap := range sc.Capabilities {
					addedCaps[cap] = true
				}
			}
			var orphanCaps []string
			for _, manualCap := range manualCapsList {
				if !addedCaps[manualCap] {
					orphanCaps = append(orphanCaps, manualCap)
				}
			}
			if len(orphanCaps) > 0 {
				result = append(result, ScraperCapabilities{
					Scraper:      "manual",
					Capabilities: orphanCaps,
				})
			}
		}

		return result
	}

	// Fallback to basic detection if no JobServer is available
	return detectFallbackCapabilities(jc)
}

// detectFallbackCapabilities returns basic capabilities when no JobServer is available
// This is used during initialization or when workers haven't registered yet
func detectFallbackCapabilities(jc types.JobConfiguration) []ScraperCapabilities {
	var result []ScraperCapabilities

	// Always available capabilities
	result = append(result, ScraperCapabilities{
		Scraper:      "web-scraper",
		Capabilities: []string{"web-scraper"},
	})
	result = append(result, ScraperCapabilities{
		Scraper:      "telemetry",
		Capabilities: []string{"telemetry"},
	})
	result = append(result, ScraperCapabilities{
		Scraper:      "tiktok-transcription",
		Capabilities: []string{"tiktok-transcription"},
	})

	// Check for Twitter capabilities based on credentials
	var twitterCaps []string
	if accounts, ok := jc["twitter_accounts"].([]string); ok && len(accounts) > 0 {
		// If we have accounts, add all Twitter capabilities
		twitterCaps = append(twitterCaps,
			"searchbyquery", "searchbyfullarchive", "searchbyprofile", "searchfollowers",
			"getbyid", "getreplies", "getretweeters", "gettweets", "getmedia",
			"gethometweets", "getforyoutweets", "getbookmarks", "getprofilebyid",
			"gettrends", "getfollowing", "getfollowers", "getspace",
		)
	} else if apiKeys, ok := jc["twitter_api_keys"].([]string); ok && len(apiKeys) > 0 {
		// If we only have API keys, add limited capabilities
		twitterCaps = append(twitterCaps, "searchbyquery", "getbyid", "getprofilebyid")
		// Check if any API key is elevated for full archive search
		// Note: This is a simplified check - in reality, the TwitterScraper checks the API key type
		twitterCaps = append(twitterCaps, "searchbyfullarchive")
	}
	if len(twitterCaps) > 0 {
		result = append(result, ScraperCapabilities{
			Scraper:      "twitter-scraper",
			Capabilities: twitterCaps,
		})
	}

	return result
}
