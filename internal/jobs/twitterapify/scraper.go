package twitterapify

import (
	teetypes "github.com/masa-finance/tee-types/types"
)

// TwitterApifyScraper provides a high-level interface for Twitter Apify operations
type TwitterApifyScraper struct {
	client *TwitterApifyClient
}

// NewTwitterApifyScraper creates a new Twitter Apify scraper
func NewTwitterApifyScraper(apiToken string) *TwitterApifyScraper {
	return &TwitterApifyScraper{
		client: NewTwitterApifyClient(apiToken),
	}
}

// GetFollowers retrieves followers for a username
func (s *TwitterApifyScraper) GetFollowers(username string, maxResults int, cursor string) ([]*teetypes.ProfileResultApify, string, error) {
	return s.client.GetFollowers(username, maxResults, cursor)
}

// GetFollowing retrieves following for a username
func (s *TwitterApifyScraper) GetFollowing(username string, maxResults int, cursor string) ([]*teetypes.ProfileResultApify, string, error) {
	return s.client.GetFollowing(username, maxResults, cursor)
}

// ValidateApiKey tests if the Apify API token is valid
func (s *TwitterApifyScraper) ValidateApiKey() error {
	return s.client.apifyClient.ValidateApiKey()
}
