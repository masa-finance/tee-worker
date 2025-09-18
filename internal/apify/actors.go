package apify

import teetypes "github.com/masa-finance/tee-types/types"

type ActorId string

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

type defaultActorInput map[string]any

type ActorConfig struct {
	ActorId      ActorId
	DefaultInput defaultActorInput
	Capabilities []teetypes.Capability
	JobType      teetypes.JobType
}

// Actors is a list of actor configurations for Apify.  Omitting LLM for now as it's not a standalone actor / has no dedicated capabilities
var Actors = []ActorConfig{
	{
		ActorId:      ActorIds.RedditScraper,
		DefaultInput: defaultActorInput{},
		Capabilities: teetypes.RedditCaps,
		JobType:      teetypes.RedditJob,
	},
	{
		ActorId:      ActorIds.TikTokSearchScraper,
		DefaultInput: defaultActorInput{"proxy": map[string]any{"useApifyProxy": true}},
		Capabilities: []teetypes.Capability{teetypes.CapSearchByQuery},
		JobType:      teetypes.TiktokJob,
	},
	{
		ActorId:      ActorIds.TikTokTrendingScraper,
		DefaultInput: defaultActorInput{},
		Capabilities: []teetypes.Capability{teetypes.CapSearchByTrending},
		JobType:      teetypes.TiktokJob,
	},
	{
		ActorId:      ActorIds.TwitterFollowers,
		DefaultInput: defaultActorInput{"maxFollowers": 200, "maxFollowings": 200},
		Capabilities: teetypes.TwitterApifyCaps,
		JobType:      teetypes.TwitterApifyJob,
	},
	{
		ActorId:      ActorIds.WebScraper,
		DefaultInput: defaultActorInput{},
		Capabilities: teetypes.WebCaps,
		JobType:      teetypes.WebJob,
	},
}
