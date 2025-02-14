package twitter

import (
	twitterscraper "github.com/imperatrona/twitter-scraper"
)

type Scraper struct {
	*twitterscraper.Scraper
}

func newTwitterScraper() *twitterscraper.Scraper {
	return twitterscraper.New()
}

func newTwitterScraperUsingApiKey(apiKey string) *twitterscraper.Scraper {
	scraper := twitterscraper.New()

	authToken := twitterscraper.AuthToken{
		Token: apiKey,
	}
	scraper.SetAuthToken(authToken)

	return scraper
}
