package apify

import teetypes "github.com/masa-finance/tee-types/types"

type ActorId string

type defaultActorInput map[string]any

type actorIds struct {
	RedditScraper         ActorId
	TikTokSearchScraper   ActorId
	TikTokTrendingScraper ActorId
	LLMDatasetProcessor   ActorId
	TwitterFollowers      ActorId
	WebScraper            ActorId
}

var ActorIds = actorIds{
	RedditScraper:         "trudax~reddit-scraper",
	TikTokSearchScraper:   "epctex~tiktok-search-scraper",
	TikTokTrendingScraper: "lexis-solutions~tiktok-trending-videos-scraper",
	LLMDatasetProcessor:   "dusan.vystrcil~llm-dataset-processor",
	TwitterFollowers:      "kaitoeasyapi~premium-x-follower-scraper-following-data",
	WebScraper:            "apify~website-content-crawler",
}

var (
	rentedActorIds = []ActorId{
		ActorIds.RedditScraper,
		ActorIds.TikTokSearchScraper,
		ActorIds.TikTokTrendingScraper,
	}
)

type ActorConfig struct {
	ActorId      ActorId
	Input        defaultActorInput
	Capabilities []teetypes.Capability
	JobType      teetypes.JobType
}

// Actors is a list of actor configurations for Apify.  Omitting LLM for now as it's not a standalone actor / has no dedicated capabilities
var Actors = []ActorConfig{
	{
		ActorId:      ActorIds.RedditScraper,
		Input:        defaultActorInput{},
		Capabilities: []teetypes.Capability{teetypes.CapScrapeUrls},
		JobType:      teetypes.RedditJob,
	},
	{
		ActorId:      ActorIds.TikTokSearchScraper,
		Input:        defaultActorInput{"proxy": map[string]any{"useApifyProxy": true}},
		Capabilities: []teetypes.Capability{teetypes.CapSearchByQuery},
		JobType:      teetypes.TiktokJob,
	},
	{
		ActorId:      ActorIds.TikTokTrendingScraper,
		Input:        defaultActorInput{},
		Capabilities: []teetypes.Capability{teetypes.CapSearchByTrending},
		JobType:      teetypes.TiktokJob,
	},
	{
		ActorId:      ActorIds.TwitterFollowers,
		Input:        defaultActorInput{},
		Capabilities: teetypes.TwitterApifyCaps,
		JobType:      teetypes.TwitterApifyJob,
	},
	{
		ActorId:      ActorIds.WebScraper,
		Input:        defaultActorInput{},
		Capabilities: []teetypes.Capability{teetypes.CapScraper},
		JobType:      teetypes.WebJob,
	},
}
