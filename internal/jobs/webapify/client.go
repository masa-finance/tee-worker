package webapify

import (
	"encoding/json"
	"fmt"

	teeargs "github.com/masa-finance/tee-types/args"
	teetypes "github.com/masa-finance/tee-types/types"
	"github.com/masa-finance/tee-worker/internal/jobs/stats"
	"github.com/masa-finance/tee-worker/pkg/client"
	"github.com/sirupsen/logrus"
)

const (
	WebActorID = "apify~website-content-crawler"
)

type WebApifyClient struct {
	apifyClient    client.Apify
	statsCollector *stats.StatsCollector
}

// NewInternalClient is a function variable that can be replaced in tests.
// It defaults to the actual implementation.
var NewInternalClient = func(apiKey string) (client.Apify, error) {
	return client.NewApifyClient(apiKey)
}

// NewClient creates a new Reddit Apify client
func NewClient(apiToken string, statsCollector *stats.StatsCollector) (*WebApifyClient, error) {
	apifyClient, err := NewInternalClient(apiToken)
	if err != nil {
		return nil, fmt.Errorf("failed to create apify client: %w", err)
	}

	return &WebApifyClient{
		apifyClient:    apifyClient,
		statsCollector: statsCollector,
	}, nil
}

// ValidateApiKey tests if the Apify API token is valid
func (c *WebApifyClient) ValidateApiKey() error {
	return c.apifyClient.ValidateApiKey()
}

func (c *WebApifyClient) Scrape(workerID string, args teeargs.WebArguments, cursor client.Cursor) ([]*teetypes.WebScraperResult, string, client.Cursor, error) {
	if c.statsCollector != nil {
		c.statsCollector.Add(workerID, stats.WebQueries, 1)
	}

	input := args.ToWebScraperRequest()

	// TODO: limit could be greater than max pages if max depth is greater than 0?
	// TODO: need to test this more thoroughly with various request types
	limit := uint(args.MaxPages)
	dataset, nextCursor, err := c.apifyClient.RunActorAndGetResponse(WebActorID, input, cursor, limit)
	if err != nil {
		if c.statsCollector != nil {
			c.statsCollector.Add(workerID, stats.WebErrors, 1)
		}
		return nil, "", client.EmptyCursor, err
	}

	response := make([]*teetypes.WebScraperResult, 0, len(dataset.Data.Items))

	for i, item := range dataset.Data.Items {
		var resp teetypes.WebScraperResult
		if err := json.Unmarshal(item, &resp); err != nil {
			logrus.Warnf("Failed to unmarshal scrape result at index %d: %v", i, err)
			continue
		}
		response = append(response, &resp)
	}

	if c.statsCollector != nil {
		c.statsCollector.Add(workerID, stats.WebScrapedPages, uint(len(response)))
	}

	return response, dataset.DatasetId, nextCursor, nil
}
