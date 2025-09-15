package jobs

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/sirupsen/logrus"

	"github.com/masa-finance/tee-worker/api/types"
	"github.com/masa-finance/tee-worker/internal/config"
	"github.com/masa-finance/tee-worker/internal/jobs/llmapify"
	"github.com/masa-finance/tee-worker/internal/jobs/stats"
	"github.com/masa-finance/tee-worker/internal/jobs/webapify"
	"github.com/masa-finance/tee-worker/pkg/client"

	teeargs "github.com/masa-finance/tee-types/args"
	"github.com/masa-finance/tee-types/pkg/util"
	teetypes "github.com/masa-finance/tee-types/types"
)

// WebApifyClient defines the interface for the Web Apify client to allow mocking in tests
type WebApifyClient interface {
	Scrape(workerID string, args teeargs.WebArguments, cursor client.Cursor) ([]*teetypes.WebScraperResult, string, client.Cursor, error)
}

// NewWebApifyClient is a function variable that can be replaced in tests.
// It defaults to the actual implementation.
var NewWebApifyClient = func(apiKey string, statsCollector *stats.StatsCollector) (WebApifyClient, error) {
	return webapify.NewClient(apiKey, statsCollector)
}

// LLMApify is the interface for the LLM processor client
// Only the Process method is required for this flow
type LLMApify interface {
	Process(workerID string, args teeargs.LLMProcessorArguments, cursor client.Cursor) ([]*teetypes.LLMProcessorResult, client.Cursor, error)
}

// NewLLMApifyClient is a function variable to allow injection in tests
var NewLLMApifyClient = func(apiKey string, llmConfig config.LlmConfig, statsCollector *stats.StatsCollector) (LLMApify, error) {
	return llmapify.NewClient(apiKey, llmConfig, statsCollector)
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

	// Require Gemini key for LLM processing in Web flow
	if !w.configuration.GeminiApiKey.IsValid() {
		msg := errors.New("Gemini API key is required for Web job")
		return types.JobResult{Error: msg.Error()}, msg
	}

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

	webResp, datasetId, cursor, err := webClient.Scrape(j.WorkerID, *webArgs, client.EmptyCursor)
	if err != nil {
		return types.JobResult{Error: fmt.Sprintf("error while scraping Web: %s", err.Error())}, fmt.Errorf("error scraping Web: %w", err)
	}

	// Run LLM processing and inject into results (Gemini key already validated)
	if datasetId == "" {
		return types.JobResult{Error: "missing dataset id from web scraping"}, errors.New("missing dataset id from web scraping")
	}

	llmClient, err := NewLLMApifyClient(w.configuration.ApifyApiKey, w.configuration.LlmConfig, w.statsCollector)
	if err != nil {
		return types.JobResult{Error: "error creating LLM Apify client"}, fmt.Errorf("failed to create LLM Apify client: %w", err)
	}

	llmArgs := teeargs.LLMProcessorArguments{
		DatasetId:   datasetId,
		Prompt:      "summarize the content of this webpage in plain text, focusing on keywords and topics: ${markdown}",
		MaxTokens:   teeargs.LLMDefaultMaxTokens,
		Temperature: teeargs.LLMDefaultTemperature,
		MaxPages:    webArgs.MaxPages,
	}
	llmResp, _, llmErr := llmClient.Process(j.WorkerID, llmArgs, client.EmptyCursor)
	if llmErr != nil {
		return types.JobResult{Error: fmt.Sprintf("error while processing LLM: %s", llmErr.Error())}, fmt.Errorf("error processing LLM: %w", llmErr)
	}

	max := util.Min(len(webResp), len(llmResp))
	for i := 0; i < max; i++ {
		if webResp[i] != nil {
			webResp[i].LLMResponse = llmResp[i].LLMResponse
		}
	}

	data, err := json.Marshal(webResp)
	if err != nil {
		return types.JobResult{Error: fmt.Sprintf("error marshalling Web response")}, fmt.Errorf("error marshalling Web response: %w", err)
	}

	if w.statsCollector != nil {
		w.statsCollector.Add(j.WorkerID, stats.WebProcessedPages, uint(len(llmResp)))
	}

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

	if ws.configuration.ApifyApiKey != "" && ws.configuration.GeminiApiKey.IsValid() {
		capabilities[teetypes.WebJob] = teetypes.WebCaps
	}

	return capabilities
}
