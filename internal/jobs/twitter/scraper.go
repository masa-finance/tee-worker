package twitter

import (
	twitterscraper "github.com/imperatrona/twitter-scraper"
)

type Scraper struct {
	*twitterscraper.Scraper
}

// newTwitterScraper returns a new instance of twitterscraper.Scraper.
// It does not authenticate to Twitter, so it can only be used to scrape
// publicly available information.
// IsLoggedIn returns whether the scraper is currently logged in to Twitter.
func newTwitterScraper() *twitterscraper.Scraper {
	return twitterscraper.New()
}

// IsLoggedIn returns true if the scraper is currently logged in to Twitter,
// false otherwise.
func (s *Scraper) IsLoggedIn() bool {
	return s.Scraper.IsLoggedIn()
}
