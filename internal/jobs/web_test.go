package jobs_test

import (
	"encoding/json"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/masa-finance/tee-worker/api/types"
	"github.com/masa-finance/tee-worker/internal/config"
	"github.com/masa-finance/tee-worker/internal/jobs"
	"github.com/masa-finance/tee-worker/internal/jobs/stats"
	"github.com/masa-finance/tee-worker/pkg/client"

	teeargs "github.com/masa-finance/tee-types/args"
	teetypes "github.com/masa-finance/tee-types/types"
)

// MockWebApifyClient is a mock implementation of the WebApifyClient.
type MockWebApifyClient struct {
	ScrapeFunc func(args teeargs.WebArguments) ([]*teetypes.WebScraperResult, client.Cursor, error)
}

func (m *MockWebApifyClient) Scrape(_ string, args teeargs.WebArguments, _ client.Cursor) ([]*teetypes.WebScraperResult, client.Cursor, error) {
	if m != nil && m.ScrapeFunc != nil {
		res, next, err := m.ScrapeFunc(args)
		return res, next, err
	}
	return nil, client.EmptyCursor, nil
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

			mockClient.ScrapeFunc = func(args teeargs.WebArguments) ([]*teetypes.WebScraperResult, client.Cursor, error) {
				Expect(args.URL).To(Equal("https://example.com"))
				return []*teetypes.WebScraperResult{{URL: "https://example.com", Markdown: "# Hello"}}, client.Cursor("next-cursor"), nil
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
			mockClient.ScrapeFunc = func(args teeargs.WebArguments) ([]*teetypes.WebScraperResult, client.Cursor, error) {
				return nil, client.EmptyCursor, expectedErr
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
})
