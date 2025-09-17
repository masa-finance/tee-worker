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
	ActorID = "apify~website-content-crawler"
)

type ApifyClient struct {
	client         client.Apify
	statsCollector *stats.StatsCollector
}

// NewInternalClient is a function variable that can be replaced in tests.
// It defaults to the actual implementation.
var NewInternalClient = func(apiKey string) (client.Apify, error) {
	return client.NewApifyClient(apiKey)
}

// NewClient creates a new Reddit Apify client
func NewClient(apiToken string, statsCollector *stats.StatsCollector) (*ApifyClient, error) {
	client, err := NewInternalClient(apiToken)
	if err != nil {
		return nil, fmt.Errorf("failed to create apify client: %w", err)
	}

	return &ApifyClient{
		client:         client,
		statsCollector: statsCollector,
	}, nil
}

// ValidateApiKey tests if the Apify API token is valid
func (c *ApifyClient) ValidateApiKey() error {
	return c.client.ValidateApiKey()
}

func (c *ApifyClient) Scrape(workerID string, args teeargs.WebArguments, cursor client.Cursor) ([]*teetypes.WebScraperResult, string, client.Cursor, error) {
	if c.statsCollector != nil {
		c.statsCollector.Add(workerID, stats.WebQueries, 1)
	}

	input := args.ToWebScraperRequest()

	limit := uint(args.MaxPages)
	dataset, nextCursor, err := c.client.RunActorAndGetResponse(ActorID, input, cursor, limit)
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
