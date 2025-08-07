package twitterapify

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	teetypes "github.com/masa-finance/tee-types/types"
	"github.com/masa-finance/tee-worker/pkg/client"
	"github.com/sirupsen/logrus"
)

const (
	TwitterFollowerActorID = "kaitoeasyapi~premium-x-follower-scraper-following-data"
	MaxActorPolls          = 60              // 5 minutes max wait time
	ActorPollInterval      = 5 * time.Second // polling interval between status checks

	// Actor run status constants
	ActorStatusSucceeded = "SUCCEEDED"
	ActorStatusFailed    = "FAILED"
	ActorStatusAborted   = "ABORTED"
)

// FollowerActorRunRequest represents the input for running the Twitter follower actor
type FollowerActorRunRequest struct {
	UserNames     []string `json:"user_names"`
	UserIds       []string `json:"user_ids"`
	MaxFollowers  int      `json:"maxFollowers"`
	MaxFollowings int      `json:"maxFollowings"`
	GetFollowers  bool     `json:"getFollowers"`
	GetFollowing  bool     `json:"getFollowing"`
}

// TwitterApifyClient wraps the generic Apify client for Twitter-specific operations
type TwitterApifyClient struct {
	apifyClient *client.ApifyClient
}

// CursorData represents the pagination data stored in cursor
type CursorData struct {
	Offset int `json:"offset"`
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

// GetFollowers retrieves followers for a username using Apify
func (c *TwitterApifyClient) GetFollowers(username string, maxResults int, cursor string) ([]*teetypes.ProfileResultApify, string, error) {
	offset := parseCursor(cursor)
	minimum := 200

	// Ensure minimum of 200 as required by the actor
	maxFollowers := maxResults
	if maxFollowers < minimum {
		maxFollowers = minimum
	}

	input := FollowerActorRunRequest{
		UserNames:     []string{username},
		UserIds:       []string{}, // Explicitly set empty array as required by actor
		MaxFollowers:  maxFollowers,
		MaxFollowings: minimum, // Actor requires minimum even when not used
		GetFollowers:  true,
		GetFollowing:  false,
	}

	return c.runActorAndGetProfiles(input, offset, maxResults)
}

// GetFollowing retrieves following for a username using Apify
func (c *TwitterApifyClient) GetFollowing(username string, maxResults int, cursor string) ([]*teetypes.ProfileResultApify, string, error) {
	offset := parseCursor(cursor)
	minimum := 200

	// Ensure minimum of 200 as required by the actor
	maxFollowings := maxResults
	if maxFollowings < minimum {
		maxFollowings = minimum
	}

	input := FollowerActorRunRequest{
		UserNames:     []string{username},
		UserIds:       []string{}, // Explicitly set empty array as required by actor
		MaxFollowers:  minimum,    // Actor requires minimum even when not used
		MaxFollowings: maxFollowings,
		GetFollowers:  false,
		GetFollowing:  true,
	}

	return c.runActorAndGetProfiles(input, offset, maxResults)
}

// runActorAndGetProfiles runs the actor and retrieves profiles from the dataset
func (c *TwitterApifyClient) runActorAndGetProfiles(input FollowerActorRunRequest, offset, limit int) ([]*teetypes.ProfileResultApify, string, error) {
	// 1. Run the actor
	logrus.Infof("Starting Apify actor run for %v", input.UserNames)
	runResp, err := c.apifyClient.RunActor(TwitterFollowerActorID, input)
	if err != nil {
		return nil, "", fmt.Errorf("failed to run actor: %w", err)
	}

	// 2. Poll for completion
	logrus.Infof("Polling for actor run completion: %s", runResp.Data.ID)
	pollCount := 0

	for {
		status, err := c.apifyClient.GetActorRun(runResp.Data.ID)
		if err != nil {
			return nil, "", fmt.Errorf("failed to get actor run status: %w", err)
		}

		logrus.Debugf("Actor run status: %s", status.Data.Status)

		if status.Data.Status == ActorStatusSucceeded {
			logrus.Infof("Actor run completed successfully")
			break
		} else if status.Data.Status == ActorStatusFailed || status.Data.Status == ActorStatusAborted {
			return nil, "", fmt.Errorf("actor run failed with status: %s", status.Data.Status)
		}

		pollCount++
		if pollCount >= MaxActorPolls {
			return nil, "", fmt.Errorf("actor run timed out after %d polls", MaxActorPolls)
		}

		time.Sleep(ActorPollInterval)
	}

	// 3. Get dataset items with pagination
	logrus.Infof("Retrieving dataset items from: %s (offset: %d, limit: %d)", runResp.Data.DefaultDatasetId, offset, limit)
	dataset, err := c.apifyClient.GetDatasetItems(runResp.Data.DefaultDatasetId, offset, limit)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get dataset items: %w", err)
	}

	// 4. Convert to ProfileResultApify
	profiles := make([]*teetypes.ProfileResultApify, 0, len(dataset.Data.Items))
	for i, item := range dataset.Data.Items {
		var profile teetypes.ProfileResultApify
		if err := json.Unmarshal(item, &profile); err != nil {
			logrus.Warnf("Failed to unmarshal profile at index %d: %v", i, err)
			continue
		}
		profiles = append(profiles, &profile)
	}

	// 5. Generate next cursor if more data available
	var nextCursor string
	if offset+limit < dataset.Data.Total {
		nextCursor = generateCursor(offset + limit)
		logrus.Debugf("Generated next cursor for offset %d", offset+limit)
	}

	logrus.Infof("Successfully retrieved %d profiles (total available: %d)", len(profiles), dataset.Data.Total)
	return profiles, nextCursor, nil
}

// parseCursor decodes a base64 cursor to get the offset
func parseCursor(cursor string) int {
	if cursor == "" {
		return 0
	}

	decoded, err := base64.StdEncoding.DecodeString(cursor)
	if err != nil {
		logrus.Warnf("Failed to decode cursor: %v", err)
		return 0
	}

	var cursorData CursorData
	if err := json.Unmarshal(decoded, &cursorData); err != nil {
		logrus.Warnf("Failed to unmarshal cursor data: %v", err)
		return 0
	}

	return cursorData.Offset
}

// generateCursor encodes an offset as a base64 cursor
func generateCursor(offset int) string {
	cursorData := CursorData{Offset: offset}
	data, err := json.Marshal(cursorData)
	if err != nil {
		logrus.Warnf("Failed to marshal cursor data: %v", err)
		return ""
	}

	return base64.StdEncoding.EncodeToString(data)
}
