package llmapify

import (
	"encoding/json"
	"errors"
	"fmt"

	teeargs "github.com/masa-finance/tee-types/args"
	teetypes "github.com/masa-finance/tee-types/types"
	"github.com/masa-finance/tee-worker/internal/jobs/stats"
	"github.com/masa-finance/tee-worker/pkg/client"
	"github.com/sirupsen/logrus"
)

const (
	LLMActorID = "dusan.vystrcil~llm-dataset-processor"
)

var (
	ErrLlmProviderKeyRequired    = errors.New("llm provider key is required")
	ErrFailedToCreateApifyClient = errors.New("failed to create apify client")
)

type LLMApifyClient struct {
	apifyClient    client.Apify
	statsCollector *stats.StatsCollector
	llmProviderKey string
}

// NewInternalClient is a function variable that can be replaced in tests.
// It defaults to the actual implementation.
var NewInternalClient = func(apiKey string) (client.Apify, error) {
	return client.NewApifyClient(apiKey)
}

// NewClient creates a new LLM Apify client
func NewClient(apiToken string, llmProviderKey string, statsCollector *stats.StatsCollector) (*LLMApifyClient, error) {
	apifyClient, err := NewInternalClient(apiToken)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrFailedToCreateApifyClient, err)
	}

	if llmProviderKey == "" {
		return nil, ErrLlmProviderKeyRequired
	}

	return &LLMApifyClient{
		apifyClient:    apifyClient,
		statsCollector: statsCollector,
		llmProviderKey: llmProviderKey,
	}, nil
}

// ValidateApiKey tests if the Apify API token is valid
func (c *LLMApifyClient) ValidateApiKey() error {
	return c.apifyClient.ValidateApiKey()
}

func (c *LLMApifyClient) Process(workerID string, args teeargs.LLMProcessorArguments, cursor client.Cursor) ([]*teetypes.LLMProcessorResult, client.Cursor, error) {
	if c.statsCollector != nil {
		c.statsCollector.Add(workerID, stats.LLMQueries, 1)
	}

	input := args.ToLLMProcessorRequest()
	input.LLMProviderApiKey = c.llmProviderKey

	limit := uint(1) // TODO, verify you can only ever operate on one dataset at a time
	dataset, nextCursor, err := c.apifyClient.RunActorAndGetResponse(LLMActorID, input, cursor, limit)
	if err != nil {
		if c.statsCollector != nil {
			c.statsCollector.Add(workerID, stats.LLMErrors, 1)
		}
		return nil, client.EmptyCursor, err
	}

	response := make([]*teetypes.LLMProcessorResult, 0, len(dataset.Data.Items))

	for i, item := range dataset.Data.Items {
		var resp teetypes.LLMProcessorResult
		if err := json.Unmarshal(item, &resp); err != nil {
			logrus.Warnf("Failed to unmarshal llm result at index %d: %v", i, err)
			continue
		}
		response = append(response, &resp)
	}

	if c.statsCollector != nil {
		c.statsCollector.Add(workerID, stats.LLMProcessedItems, uint(len(response)))
	}

	return response, nextCursor, nil
}
