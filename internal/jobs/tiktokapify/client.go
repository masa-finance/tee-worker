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

type TikTokApifyClient struct {
	apify *client.ApifyClient
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
func (c *TikTokApifyClient) SearchByQuery(input teeargs.TikTokSearchByQueryArguments, cursor client.Cursor, limit int) ([]*teetypes.TikTokSearchByQueryResult, client.Cursor, error) {
	// Map snake_case fields to Apify actor's expected camelCase input
	startUrls := input.StartUrls
	if startUrls == nil {
		startUrls = []string{}
	}
	searchTerms := input.Search
	if searchTerms == nil {
		searchTerms = []string{}
	}

	apifyInput := map[string]any{
		"search":    searchTerms,
		"startUrls": startUrls,
		"maxItems":  input.MaxItems,
		"endPage":   input.EndPage,
	}
	if input.Proxy != nil {
		apifyInput["proxy"] = map[string]any{"useApifyProxy": input.Proxy.UseApifyProxy}
	}

	dataset, next, err := c.apify.RunActorAndGetResponse(SearchActorID, apifyInput, cursor, limit)
	if err != nil {
		return nil, "", fmt.Errorf("apify run (search): %w", err)
	}

	var results []*teetypes.TikTokSearchByQueryResult
	for _, raw := range dataset.Data.Items {
		var item teetypes.TikTokSearchByQueryResult
		if err := json.Unmarshal(raw, &item); err != nil {
			// If structure differs for some items, skip
			continue
		}
		results = append(results, &item)
	}
	return results, next, nil
}

// SearchByTrending runs the trending actor and returns typed results
func (c *TikTokApifyClient) SearchByTrending(input teeargs.TikTokSearchByTrendingArguments, cursor client.Cursor, limit int) ([]*teetypes.TikTokSearchByTrending, client.Cursor, error) {
	apifyInput := map[string]any{
		"countryCode": input.CountryCode,
		"sortBy":      input.SortBy,
		"maxItems":    input.MaxItems,
		"period":      input.Period,
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
