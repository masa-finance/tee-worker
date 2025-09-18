package apify

type actorIds struct {
	RedditScraper         string
	TikTokSearchScraper   string
	TikTokTrendingScraper string
	LLMDatasetProcessor   string
	TwitterFollowers      string
	WebScraper            string
}

var Actors = actorIds{
	RedditScraper:         "trudax~reddit-scraper",
	TikTokSearchScraper:   "epctex~tiktok-search-scraper",
	TikTokTrendingScraper: "lexis-solutions~tiktok-trending-videos-scraper",
	LLMDatasetProcessor:   "dusan.vystrcil~llm-dataset-processor",
	TwitterFollowers:      "kaitoeasyapi~premium-x-follower-scraper-following-data",
	WebScraper:            "apify~website-content-crawler",
}
