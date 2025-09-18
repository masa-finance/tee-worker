package twitterapify

import (
	"encoding/json"
	"fmt"

	util "github.com/masa-finance/tee-types/pkg/util"
	teetypes "github.com/masa-finance/tee-types/types"
	"github.com/masa-finance/tee-worker/internal/apify"
	"github.com/masa-finance/tee-worker/pkg/client"
	"github.com/sirupsen/logrus"
)

// FollowerActorRunRequest represents the input for running the Twitter follower actor
type FollowerActorRunRequest struct {
	UserNames     []string `json:"user_names"`
	UserIds       []string `json:"user_ids"`
	MaxFollowers  uint     `json:"maxFollowers"`
	MaxFollowings uint     `json:"maxFollowings"`
	GetFollowers  bool     `json:"getFollowers"`
	GetFollowing  bool     `json:"getFollowing"`
}

// TwitterApifyClient wraps the generic Apify client for Twitter-specific operations
type TwitterApifyClient struct {
	apifyClient client.Apify
}

// NewTwitterApifyClient creates a new Twitter Apify client
func NewTwitterApifyClient(apiToken string) (*TwitterApifyClient, error) {
	apifyClient, err := client.NewApifyClient(apiToken)
	if err != nil {
		return nil, fmt.Errorf("failed to create apify client: %w", err)
	}

	return &TwitterApifyClient{
		apifyClient: apifyClient,
	}, nil
}

// ValidateApiKey tests if the Apify API token is valid
func (c *TwitterApifyClient) ValidateApiKey() error {
	return c.apifyClient.ValidateApiKey()
}

// GetFollowers retrieves followers for a username using Apify
func (c *TwitterApifyClient) GetFollowers(username string, maxResults uint, cursor client.Cursor) ([]*teetypes.ProfileResultApify, client.Cursor, error) {
	minimum := uint(200)

	// Ensure minimum of 200 as required by the actor
	maxFollowers := util.Max(maxResults, minimum)

	input := FollowerActorRunRequest{
		UserNames:     []string{username},
		UserIds:       []string{}, // Explicitly set empty array as required by actor
		MaxFollowers:  maxFollowers,
		MaxFollowings: minimum, // Actor requires minimum even when not used
		GetFollowers:  true,
		GetFollowing:  false,
	}

	return c.getProfiles(input, cursor, maxResults)
}

// GetFollowing retrieves following for a username using Apify
func (c *TwitterApifyClient) GetFollowing(username string, cursor client.Cursor, maxResults uint) ([]*teetypes.ProfileResultApify, client.Cursor, error) {
	minimum := uint(200)

	// Ensure minimum of 200 as required by the actor
	maxFollowings := util.Max(maxResults, minimum)

	input := FollowerActorRunRequest{
		UserNames:     []string{username},
		UserIds:       []string{}, // Explicitly set empty array as required by actor
		MaxFollowers:  minimum,    // Actor requires minimum even when not used
		MaxFollowings: maxFollowings,
		GetFollowers:  false,
		GetFollowing:  true,
	}

	return c.getProfiles(input, cursor, maxResults)
}

// getProfiles runs the actor and retrieves profiles from the dataset
func (c *TwitterApifyClient) getProfiles(input FollowerActorRunRequest, cursor client.Cursor, limit uint) ([]*teetypes.ProfileResultApify, client.Cursor, error) {
	dataset, nextCursor, err := c.apifyClient.RunActorAndGetResponse(apify.Actors.TwitterFollowers, input, cursor, limit)
	if err != nil {
		return nil, client.EmptyCursor, err
	}

	profiles := make([]*teetypes.ProfileResultApify, 0, len(dataset.Data.Items))
	for i, item := range dataset.Data.Items {
		var profile teetypes.ProfileResultApify
		if err := json.Unmarshal(item, &profile); err != nil {
			logrus.Warnf("Failed to unmarshal profile at index %d: %v", i, err)
			continue
		}
		profiles = append(profiles, &profile)
	}

	return profiles, nextCursor, nil
}
