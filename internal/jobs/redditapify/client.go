package redditapify

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/masa-finance/tee-worker/api/types/reddit"
	"github.com/masa-finance/tee-worker/internal/jobs/stats"
	"github.com/masa-finance/tee-worker/pkg/client"

	teeargs "github.com/masa-finance/tee-types/args"
	teetypes "github.com/masa-finance/tee-types/types"
)

const (
	RedditActorID = "trudax~reddit-scraper" // must rent this actor from apify explicitly
)

// CommonArgs holds the parameters that all Reddit searches support, in a single struct
type CommonArgs struct {
	Sort           teetypes.RedditSortType
	IncludeNSFW    bool
	MaxItems       uint
	MaxPosts       uint
	MaxComments    uint
	MaxCommunities uint
	MaxUsers       uint
}

func (ca *CommonArgs) CopyFromArgs(a *teeargs.RedditArguments) {
	ca.Sort = a.Sort
	ca.IncludeNSFW = a.IncludeNSFW
	ca.MaxItems = a.MaxItems
	ca.MaxPosts = a.MaxPosts
	ca.MaxComments = a.MaxComments
	ca.MaxCommunities = a.MaxCommunities
	ca.MaxUsers = a.MaxUsers
}

func (args *CommonArgs) ToActorRequest() RedditActorRequest {
	return RedditActorRequest{
		Sort:                args.Sort,
		IncludeNSFW:         args.IncludeNSFW,
		MaxItems:            args.MaxItems,
		MaxPostCount:        args.MaxPosts,
		MaxComments:         args.MaxComments,
		MaxCommunitiesCount: args.MaxCommunities,
		MaxUserCount:        args.MaxUsers,
	}
}

// ApifyRedditQuery represents the query parameters for the Apify Reddit Scraper actor.
// Based on the input schema of https://apify.com/trudax/reddit-scraper
type RedditActorRequest struct {
	Type                teetypes.RedditQueryType  `json:"type,omitempty"`
	Searches            []string                  `json:"searches,omitempty"`
	StartUrls           []teetypes.RedditStartURL `json:"startUrls,omitempty"`
	Sort                teetypes.RedditSortType   `json:"sort,omitempty"`
	PostDateLimit       *time.Time                `json:"postDateLimit,omitempty"`
	IncludeNSFW         bool                      `json:"includeNSFW"`
	MaxItems            uint                      `json:"maxItems,omitempty"`            // Total number of items to scrape
	MaxPostCount        uint                      `json:"maxPostCount,omitempty"`        // Max number of posts per page
	MaxComments         uint                      `json:"maxComments,omitempty"`         // Max number of comments per page
	MaxCommunitiesCount uint                      `json:"maxCommunitiesCount,omitempty"` // Max number of communities per page
	MaxUserCount        uint                      `json:"maxUserCount,omitempty"`        // Max number of users per page
	SearchComments      bool                      `json:"searchComments"`
	SearchCommunities   bool                      `json:"searchCommunities"`
	SearchPosts         bool                      `json:"searchPosts"`
	SearchUsers         bool                      `json:"searchUsers"`
	SkipUserPosts       bool                      `json:"skipUserPosts"`
	SkipComments        bool                      `json:"skipComments"`
}

// RedditApifyClient wraps the generic Apify client for Reddit-specific operations
type RedditApifyClient struct {
	apifyClient    client.Apify
	statsCollector *stats.StatsCollector
}

// NewInternalClient is a function variable that can be replaced in tests.
// It defaults to the actual implementation.
var NewInternalClient = func(apiKey string) (client.Apify, error) {
	return client.NewApifyClient(apiKey)
}

// NewClient creates a new Reddit Apify client
func NewClient(apiToken string, statsCollector *stats.StatsCollector) (*RedditApifyClient, error) {
	apifyClient, err := NewInternalClient(apiToken)
	if err != nil {
		return nil, fmt.Errorf("failed to create apify client: %w", err)
	}

	return &RedditApifyClient{
		apifyClient:    apifyClient,
		statsCollector: statsCollector,
	}, nil
}

// ValidateApiKey tests if the Apify API token is valid
func (c *RedditApifyClient) ValidateApiKey() error {
	return c.apifyClient.ValidateApiKey()
}

// ScrapeUrls scrapes Reddit URLs
func (c *RedditApifyClient) ScrapeUrls(workerID string, urls []teetypes.RedditStartURL, after time.Time, args CommonArgs, cursor client.Cursor, maxResults uint) ([]*reddit.Response, client.Cursor, error) {
	input := args.ToActorRequest()
	input.StartUrls = urls
	input.Searches = nil
	if after.IsZero() {
		input.PostDateLimit = &after
	}
	input.SearchUsers = true
	input.SearchComments = true
	input.SearchPosts = true
	input.SearchCommunities = true
	input.SkipUserPosts = input.MaxPostCount == 0

	return c.queryReddit(workerID, input, cursor, maxResults)
}

// SearchPosts searches Reddit posts
func (c *RedditApifyClient) SearchPosts(workerID string, queries []string, after time.Time, args CommonArgs, cursor client.Cursor, maxResults uint) ([]*reddit.Response, client.Cursor, error) {
	input := args.ToActorRequest()
	input.Searches = queries
	input.StartUrls = nil
	if after.IsZero() {
		input.PostDateLimit = &after
	}
	input.PostDateLimit = &after
	input.Type = "posts"

	input.SearchPosts = true
	input.SkipComments = input.MaxComments == 0

	return c.queryReddit(workerID, input, cursor, maxResults)
}

// SearchCommunities searches Reddit communities
func (c *RedditApifyClient) SearchCommunities(workerID string, queries []string, args CommonArgs, cursor client.Cursor, maxResults uint) ([]*reddit.Response, client.Cursor, error) {
	input := args.ToActorRequest()
	input.Searches = queries
	input.StartUrls = nil
	input.Type = "communities"
	input.SearchCommunities = true

	return c.queryReddit(workerID, input, cursor, maxResults)
}

// SearchUsers searches Reddit users
func (c *RedditApifyClient) SearchUsers(workerID string, queries []string, skipPosts bool, args CommonArgs, cursor client.Cursor, maxResults uint) ([]*reddit.Response, client.Cursor, error) {
	input := args.ToActorRequest()
	input.Searches = queries
	input.StartUrls = nil
	input.SkipUserPosts = skipPosts
	input.Type = "users"
	input.SearchUsers = true

	return c.queryReddit(workerID, input, cursor, maxResults)
}

// getProfiles runs the actor and retrieves profiles from the dataset
func (c *RedditApifyClient) queryReddit(workerID string, input RedditActorRequest, cursor client.Cursor, limit uint) ([]*reddit.Response, client.Cursor, error) {
	if c.statsCollector != nil {
		c.statsCollector.Add(workerID, stats.RedditQueries, 1)
	}

	dataset, nextCursor, err := c.apifyClient.RunActorAndGetResponse(RedditActorID, input, cursor, limit)
	if err != nil {
		if c.statsCollector != nil {
			c.statsCollector.Add(workerID, stats.RedditErrors, 1)
		}
		return nil, client.EmptyCursor, err
	}

	response := make([]*reddit.Response, 0, len(dataset.Data.Items))
	for i, item := range dataset.Data.Items {
		var resp reddit.Response
		if err := json.Unmarshal(item, &resp); err != nil {
			logrus.Warnf("Failed to unmarshal profile at index %d: %v", i, err)
			continue
		}
		response = append(response, &resp)
	}

	if c.statsCollector != nil {
		c.statsCollector.Add(workerID, stats.RedditReturnedItems, uint(len(response)))
	}

	return response, nextCursor, nil
}
