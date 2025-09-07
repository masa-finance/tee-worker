package jobs

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/sirupsen/logrus"

	"github.com/masa-finance/tee-worker/api/types"
	"github.com/masa-finance/tee-worker/internal/config"
	"github.com/masa-finance/tee-worker/internal/jobs/stats"
	"github.com/masa-finance/tee-worker/internal/jobs/webapify"
	"github.com/masa-finance/tee-worker/pkg/client"

	teeargs "github.com/masa-finance/tee-types/args"
	teetypes "github.com/masa-finance/tee-types/types"
)

// WebApifyClient defines the interface for the Web Apify client to allow mocking in tests
type WebApifyClient interface {
	Scrape(workerID string, args teeargs.WebArguments, cursor client.Cursor) ([]*teetypes.WebScraperResult, client.Cursor, error)
}

// NewWebApifyClient is a function variable that can be replaced in tests.
// It defaults to the actual implementation.
var NewWebApifyClient = func(apiKey string, statsCollector *stats.StatsCollector) (WebApifyClient, error) {
	return webapify.NewClient(apiKey, statsCollector)
}

type WebScraper struct {
	configuration  config.WebConfig
	statsCollector *stats.StatsCollector
	capabilities   []teetypes.Capability
}

func NewWebScraper(jc config.JobConfiguration, statsCollector *stats.StatsCollector) *WebScraper {
	cfg := jc.GetWebConfig()
	logrus.Info("Web scraper via Apify initialized")
	return &WebScraper{
		configuration:  cfg,
		statsCollector: statsCollector,
		capabilities:   teetypes.WebCaps,
	}
}

func (w *WebScraper) ExecuteJob(j types.Job) (types.JobResult, error) {
	logrus.WithField("job_uuid", j.UUID).Info("Starting ExecuteJob for Web scrape")

	jobArgs, err := teeargs.UnmarshalJobArguments(teetypes.JobType(j.Type), map[string]any(j.Arguments))
	if err != nil {
		msg := fmt.Errorf("failed to unmarshal job arguments: %w", err)
		return types.JobResult{Error: msg.Error()}, msg
	}

	webArgs, ok := jobArgs.(*teeargs.WebArguments)
	if !ok {
		return types.JobResult{Error: "invalid argument type for Web job"}, errors.New("invalid argument type")
	}
	logrus.Debugf("web job args: %+v", *webArgs)

	webClient, err := NewWebApifyClient(w.configuration.ApifyApiKey, w.statsCollector)
	if err != nil {
		return types.JobResult{Error: "error while scraping Web"}, fmt.Errorf("error creating Web Apify client: %w", err)
	}

	resp, cursor, err := webClient.Scrape(j.WorkerID, *webArgs, client.EmptyCursor)
	if err != nil {
		return types.JobResult{Error: fmt.Sprintf("error while scraping Web: %s", err.Error())}, fmt.Errorf("error scraping Web: %w", err)
	}

	data, err := json.Marshal(resp)
	if err != nil {
		return types.JobResult{Error: fmt.Sprintf("error marshalling Web response")}, fmt.Errorf("error marshalling Web response: %w", err)
	}

	// TODO is this where we add the LLM processor?

	return types.JobResult{
		Data:       data,
		Job:        j,
		NextCursor: cursor.String(),
	}, nil
}

// GetStructuredCapabilities returns the structured capabilities supported by the Web scraper
// based on the available credentials and API keys
func (ws *WebScraper) GetStructuredCapabilities() teetypes.WorkerCapabilities {
	capabilities := make(teetypes.WorkerCapabilities)

	if ws.configuration.ApifyApiKey != "" && ws.configuration.GeminiApiKey != "" {
		capabilities[teetypes.WebJob] = teetypes.WebCaps
	}

	return capabilities
}
