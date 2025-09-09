package jobs_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/masa-finance/tee-worker/api/types"
	"github.com/masa-finance/tee-worker/internal/config"
	"github.com/masa-finance/tee-worker/internal/jobs"
	"github.com/masa-finance/tee-worker/internal/jobs/stats"
	"github.com/masa-finance/tee-worker/internal/jobs/webapify"
	"github.com/masa-finance/tee-worker/pkg/client"

	teeargs "github.com/masa-finance/tee-types/args"
	teetypes "github.com/masa-finance/tee-types/types"
)

// MockWebApifyClient is a mock implementation of the WebApifyClient.
type MockWebApifyClient struct {
	ScrapeFunc func(args teeargs.WebArguments) ([]*teetypes.WebScraperResult, string, client.Cursor, error)
}

func (m *MockWebApifyClient) Scrape(_ string, args teeargs.WebArguments, _ client.Cursor) ([]*teetypes.WebScraperResult, string, client.Cursor, error) {
	if m != nil && m.ScrapeFunc != nil {
		res, datasetId, next, err := m.ScrapeFunc(args)
		return res, datasetId, next, err
	}
	return nil, "", client.EmptyCursor, nil
}

var _ = Describe("WebScraper", func() {
	var (
		scraper        *jobs.WebScraper
		statsCollector *stats.StatsCollector
		job            types.Job
		mockClient     *MockWebApifyClient
	)

	BeforeEach(func() {
		statsCollector = stats.StartCollector(128, config.JobConfiguration{})
		cfg := config.JobConfiguration{
			"apify_api_key": "test-key",
		}
		scraper = jobs.NewWebScraper(cfg, statsCollector)
		mockClient = &MockWebApifyClient{}

		// Replace the client creation function with one that returns the mock
		jobs.NewWebApifyClient = func(apiKey string, _ *stats.StatsCollector) (jobs.WebApifyClient, error) {
			return mockClient, nil
		}

		job = types.Job{
			UUID: "test-uuid",
			Type: teetypes.WebJob,
		}
	})

	Context("ExecuteJob", func() {
		It("should return an error for invalid arguments", func() {
			job.Arguments = map[string]any{"invalid": "args"}
			result, err := scraper.ExecuteJob(job)
			Expect(err).To(HaveOccurred())
			Expect(result.Error).To(ContainSubstring("failed to unmarshal job arguments"))
		})

		It("should call Scrape and return data and next cursor", func() {
			job.Arguments = map[string]any{
				"type":      teetypes.WebScraper,
				"url":       "https://example.com",
				"max_depth": 1,
				"max_pages": 2,
			}

			mockClient.ScrapeFunc = func(args teeargs.WebArguments) ([]*teetypes.WebScraperResult, string, client.Cursor, error) {
				Expect(args.URL).To(Equal("https://example.com"))
				return []*teetypes.WebScraperResult{{URL: "https://example.com", Markdown: "# Hello"}}, "dataset-123", client.Cursor("next-cursor"), nil
			}

			result, err := scraper.ExecuteJob(job)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.NextCursor).To(Equal("next-cursor"))

			var resp []*teetypes.WebScraperResult
			err = json.Unmarshal(result.Data, &resp)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp).To(HaveLen(1))
			Expect(resp[0]).NotTo(BeNil())
			Expect(resp[0].URL).To(Equal("https://example.com"))
		})

		It("should handle errors from the web client", func() {
			job.Arguments = map[string]any{
				"type":      teetypes.WebScraper,
				"url":       "https://example.com",
				"max_depth": 0,
				"max_pages": 1,
			}

			expectedErr := errors.New("client error")
			mockClient.ScrapeFunc = func(args teeargs.WebArguments) ([]*teetypes.WebScraperResult, string, client.Cursor, error) {
				return nil, "", client.EmptyCursor, expectedErr
			}

			result, err := scraper.ExecuteJob(job)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(ContainSubstring("client error")))
			Expect(result.Error).To(ContainSubstring("error while scraping Web: client error"))
		})

		It("should handle errors when creating the client", func() {
			jobs.NewWebApifyClient = func(apiKey string, _ *stats.StatsCollector) (jobs.WebApifyClient, error) {
				return nil, errors.New("client creation failed")
			}
			job.Arguments = map[string]any{
				"type":      teetypes.WebScraper,
				"url":       "https://example.com",
				"max_depth": 0,
				"max_pages": 1,
			}

			result, err := scraper.ExecuteJob(job)
			Expect(err).To(HaveOccurred())
			Expect(result.Error).To(Equal("error while scraping Web"))
		})
	})

	// Integration tests that use the real client
	Context("Integration tests", func() {
		var (
			apifyKey  string
			geminiKey string
		)

		BeforeEach(func() {
			apifyKey = os.Getenv("APIFY_API_KEY")
			geminiKey = os.Getenv("GEMINI_API_KEY")

			// Reset to use real client for integration tests
			jobs.NewWebApifyClient = func(apiKey string, s *stats.StatsCollector) (jobs.WebApifyClient, error) {
				return webapify.NewClient(apiKey, s)
			}
		})

		FIt("should execute a real web scraping job when APIFY_API_KEY is set", func() {
			if apifyKey == "" {
				Skip("APIFY_API_KEY is not set")
			}

			cfg := config.JobConfiguration{
				"apify_api_key":  apifyKey,
				"gemini_api_key": geminiKey,
			}
			integrationStatsCollector := stats.StartCollector(128, cfg)
			integrationScraper := jobs.NewWebScraper(cfg, integrationStatsCollector)

			job := types.Job{
				UUID: "integration-test-uuid",
				Type: teetypes.WebJob,
				Arguments: map[string]any{
					"type":      teetypes.WebScraper,
					"url":       "https://en.wikipedia.org/wiki/Bitcoin",
					"max_depth": 0,
					"max_pages": 1,
				},
			}

			result, err := integrationScraper.ExecuteJob(job)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Error).To(BeEmpty())
			Expect(result.Data).NotTo(BeEmpty())

			var resp []*teetypes.WebScraperResult
			err = json.Unmarshal(result.Data, &resp)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp).NotTo(BeEmpty())
			Expect(resp[0]).NotTo(BeNil())
			Expect(resp[0].URL).To(Equal("https://en.wikipedia.org/wiki/Bitcoin/"))
			Expect(resp[0].LLMResponse).NotTo(BeEmpty())

			prettyJSON, err := json.MarshalIndent(resp, "", "  ")
			Expect(err).NotTo(HaveOccurred())
			fmt.Println(string(prettyJSON))
		})

		It("should expose capabilities only when both APIFY and GEMINI keys are present", func() {
			cfg := config.JobConfiguration{
				"apify_api_key":  apifyKey,
				"gemini_api_key": geminiKey,
			}
			integrationStatsCollector := stats.StartCollector(128, cfg)
			integrationScraper := jobs.NewWebScraper(cfg, integrationStatsCollector)

			caps := integrationScraper.GetStructuredCapabilities()
			if apifyKey != "" && geminiKey != "" {
				Expect(caps[teetypes.WebJob]).NotTo(BeEmpty())
			} else {
				// Expect no capabilities when either key is missing
				_, ok := caps[teetypes.WebJob]
				Expect(ok).To(BeFalse())
			}
		})
	})
})
