package twitter

import (
	twitterscraper "github.com/imperatrona/twitter-scraper"
)

type Scraper struct {
	*twitterscraper.Scraper
	skipLoginVerification bool // false by default
}

func newTwitterScraper() *twitterscraper.Scraper {
	return twitterscraper.New()
}

// SetSkipLoginVerification configures whether to skip the Twitter login verification API call
// Setting this to true will avoid rate limiting on Twitter's verify_credentials endpoint
func (s *Scraper) SetSkipLoginVerification(skip bool) *Scraper {
	s.skipLoginVerification = skip
	return s
}

// IsLoggedIn checks if the scraper is logged in
// If skipLoginVerification is true, it will assume the session is valid without making an API call
func (s *Scraper) IsLoggedIn() bool {

	// TODO: we somehow need to set the bearer token regardless. so calling this to set it.
	// if the skip verification is set, we'll just return true.
	loggedIn := s.Scraper.IsLoggedIn()
	if s.skipLoginVerification {
		return true // Skip the verification API call to avoid rate limits
	}

	// whatever the scraper returns, we return
	return loggedIn
}
