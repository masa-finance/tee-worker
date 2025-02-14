package twitter

import (
	twitterscraper "github.com/imperatrona/twitter-scraper"
)

type Scraper struct {
	*twitterscraper.Scraper
	apiClient *Client // Add API client
}

func newTwitterScraper() *twitterscraper.Scraper {
	return twitterscraper.New()
}
