package tiktokapify

import (
	"encoding/json"
	"fmt"

	teeargs "github.com/masa-finance/tee-types/args"
	teetypes "github.com/masa-finance/tee-types/types"
	"github.com/masa-finance/tee-worker/pkg/client"
)

const (
	// Actors
	SearchActorID   = "epctex~tiktok-search-scraper"                   // must rent this actor from apify explicitly
	TrendingActorID = "lexis-solutions~tiktok-trending-videos-scraper" // must rent this actor from apify explicitly
)

type TikTokSearchByQueryRequest struct {
	SearchTerms []string       `json:"search"`
	StartUrls   []string       `json:"startUrls"`
	MaxItems    uint           `json:"maxItems"`
	EndPage     uint           `json:"endPage"`
	Proxy       map[string]any `json:"proxy"`
}

type TikTokSearchByTrendingRequest struct {
	CountryCode string `json:"countryCode"`
	SortBy      string `json:"sortBy"`
	MaxItems    uint   `json:"maxItems"`
	Period      string `json:"period"`
}

type TikTokApifyClient struct {
	apify client.Apify
}

func NewTikTokApifyClient(apiToken string) (*TikTokApifyClient, error) {
	apifyClient, err := client.NewApifyClient(apiToken)
	if err != nil {
		return nil, fmt.Errorf("failed to create Apify client: %w", err)
	}
	return &TikTokApifyClient{apify: apifyClient}, nil
}

// ValidateApiKey validates the underlying Apify API token
func (c *TikTokApifyClient) ValidateApiKey() error {
	return c.apify.ValidateApiKey()
}

// SearchByQuery runs the search actor and returns typed results
func (c *TikTokApifyClient) SearchByQuery(input teeargs.TikTokSearchByQueryArguments, cursor client.Cursor, limit uint) ([]*teetypes.TikTokSearchByQueryResult, client.Cursor, error) {
	// Map snake_case fields to Apify actor's expected camelCase input
	startUrls := input.StartUrls
	if startUrls == nil {
		startUrls = []string{}
	}
	searchTerms := input.Search
	if searchTerms == nil {
		searchTerms = []string{}
	}

	// Create structured request using the TikTokSearchByQueryRequest struct
	request := TikTokSearchByQueryRequest{
		SearchTerms: searchTerms,
		StartUrls:   startUrls,
		MaxItems:    input.MaxItems,
		EndPage:     input.EndPage,
		Proxy:       map[string]any{"useApifyProxy": true},
	}

	// Convert struct to map[string]any for Apify client
	requestBytes, err := json.Marshal(request)
	if err != nil {
		return nil, "", fmt.Errorf("failed to marshal request: %w", err)
	}

	var apifyInput map[string]any
	if err := json.Unmarshal(requestBytes, &apifyInput); err != nil {
		return nil, "", fmt.Errorf("failed to unmarshal to map: %w", err)
	}

	dataset, next, err := c.apify.RunActorAndGetResponse(SearchActorID, apifyInput, cursor, limit)
	if err != nil {
		return nil, "", fmt.Errorf("apify run (search): %w", err)
	}

	var results []*teetypes.TikTokSearchByQueryResult
	for _, raw := range dataset.Data.Items {
		var item teetypes.TikTokSearchByQueryResult
		if err := json.Unmarshal(raw, &item); err != nil {
			// Skip any items whose structure doesn't match
			continue
		}
		results = append(results, &item)
	}
	return results, next, nil
}

// SearchByTrending runs the trending actor and returns typed results
func (c *TikTokApifyClient) SearchByTrending(input teeargs.TikTokSearchByTrendingArguments, cursor client.Cursor, limit uint) ([]*teetypes.TikTokSearchByTrending, client.Cursor, error) {
	request := TikTokSearchByTrendingRequest{
		CountryCode: input.CountryCode,
		SortBy:      input.SortBy,
		MaxItems:    uint(input.MaxItems),
		Period:      input.Period,
	}

	requestBytes, err := json.Marshal(request)
	if err != nil {
		return nil, "", fmt.Errorf("failed to marshal request: %w", err)
	}

	var apifyInput map[string]any
	if err := json.Unmarshal(requestBytes, &apifyInput); err != nil {
		return nil, "", fmt.Errorf("failed to unmarshal to map: %w", err)
	}

	dataset, next, err := c.apify.RunActorAndGetResponse(TrendingActorID, apifyInput, cursor, limit)
	if err != nil {
		return nil, "", fmt.Errorf("apify run (trending): %w", err)
	}

	var results []*teetypes.TikTokSearchByTrending
	for _, raw := range dataset.Data.Items {
		var item teetypes.TikTokSearchByTrending
		if err := json.Unmarshal(raw, &item); err != nil {
			continue
		}
		results = append(results, &item)
	}
	return results, next, nil
}
