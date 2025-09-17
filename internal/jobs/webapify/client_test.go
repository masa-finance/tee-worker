package webapify_test

import (
	"encoding/json"
	"errors"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/masa-finance/tee-worker/internal/jobs/webapify"
	"github.com/masa-finance/tee-worker/pkg/client"

	teeargs "github.com/masa-finance/tee-types/args"
)

// MockApifyClient is a mock implementation of the ApifyClient.
type MockApifyClient struct {
	RunActorAndGetResponseFunc func(actorID string, input any, cursor client.Cursor, limit uint) (*client.DatasetResponse, client.Cursor, error)
	ValidateApiKeyFunc         func() error
}

func (m *MockApifyClient) RunActorAndGetResponse(actorID string, input any, cursor client.Cursor, limit uint) (*client.DatasetResponse, client.Cursor, error) {
	if m.RunActorAndGetResponseFunc != nil {
		return m.RunActorAndGetResponseFunc(actorID, input, cursor, limit)
	}
	return nil, "", errors.New("RunActorAndGetResponseFunc not defined")
}

func (m *MockApifyClient) ValidateApiKey() error {
	if m.ValidateApiKeyFunc != nil {
		return m.ValidateApiKeyFunc()
	}
	return errors.New("ValidateApiKeyFunc not defined")
}

var _ = Describe("WebApifyClient", func() {
	var (
		mockClient *MockApifyClient
		webClient  *webapify.ApifyClient
		apifyKey   string
		geminiKey  string
	)

	BeforeEach(func() {
		apifyKey = os.Getenv("APIFY_API_KEY")
		geminiKey = os.Getenv("GEMINI_API_KEY")
		mockClient = &MockApifyClient{}
		// Replace the client creation function with one that returns the mock
		webapify.NewInternalClient = func(apiKey string) (client.Apify, error) {
			return mockClient, nil
		}
		var err error
		webClient, err = webapify.NewClient("test-token", nil)
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("Scrape", func() {
		It("should construct the correct actor input", func() {
			args := teeargs.WebArguments{
				URL:      "https://example.com",
				MaxDepth: 1,
				MaxPages: 2,
			}

			mockClient.RunActorAndGetResponseFunc = func(actorID string, input any, cursor client.Cursor, limit uint) (*client.DatasetResponse, client.Cursor, error) {
				Expect(actorID).To(Equal(webapify.ActorID))
				Expect(limit).To(Equal(uint(2)))
				return &client.DatasetResponse{Data: client.ApifyDatasetData{Items: []json.RawMessage{}}}, "next", nil
			}

			_, _, _, err := webClient.Scrape("test-worker", args, client.EmptyCursor)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should handle errors from the apify client", func() {
			expectedErr := errors.New("apify error")
			mockClient.RunActorAndGetResponseFunc = func(actorID string, input any, cursor client.Cursor, limit uint) (*client.DatasetResponse, client.Cursor, error) {
				return nil, "", expectedErr
			}

			args := teeargs.WebArguments{
				URL:      "https://example.com",
				MaxDepth: 0,
				MaxPages: 1,
			}
			_, _, _, err := webClient.Scrape("test-worker", args, client.EmptyCursor)
			Expect(err).To(MatchError(expectedErr))
		})

		It("should handle JSON unmarshalling errors gracefully", func() {
			invalidJSON := []byte(`{"url": "test", "markdown": 123}`) // markdown should be a string
			dataset := &client.DatasetResponse{
				Data: client.ApifyDatasetData{
					Items: []json.RawMessage{invalidJSON},
				},
			}
			mockClient.RunActorAndGetResponseFunc = func(actorID string, input any, cursor client.Cursor, limit uint) (*client.DatasetResponse, client.Cursor, error) {
				return dataset, "next", nil
			}

			args := teeargs.WebArguments{
				URL:      "https://example.com",
				MaxDepth: 0,
				MaxPages: 1,
			}
			results, _, _, err := webClient.Scrape("test-worker", args, client.EmptyCursor)
			Expect(err).NotTo(HaveOccurred())
			Expect(results).To(BeEmpty()) // The invalid item should be skipped
		})

		It("should correctly unmarshal valid items", func() {
			webResultJSON, _ := json.Marshal(map[string]any{
				"url":      "https://example.com",
				"markdown": "# Hello World",
				"title":    "Example",
			})
			dataset := &client.DatasetResponse{
				Data: client.ApifyDatasetData{
					Items: []json.RawMessage{webResultJSON},
				},
			}
			mockClient.RunActorAndGetResponseFunc = func(actorID string, input any, cursor client.Cursor, limit uint) (*client.DatasetResponse, client.Cursor, error) {
				return dataset, "next", nil
			}

			args := teeargs.WebArguments{
				URL:      "https://example.com",
				MaxDepth: 0,
				MaxPages: 1,
			}
			results, _, cursor, err := webClient.Scrape("test-worker", args, client.EmptyCursor)
			Expect(err).NotTo(HaveOccurred())
			Expect(cursor).To(Equal(client.Cursor("next")))
			Expect(results).To(HaveLen(1))
			Expect(results[0].URL).To(Equal("https://example.com"))
			Expect(results[0].Markdown).To(Equal("# Hello World"))
		})
	})

	Describe("ValidateApiKey", func() {
		It("should validate the API key", func() {
			mockClient.ValidateApiKeyFunc = func() error {
				return nil
			}
			Expect(webClient.ValidateApiKey()).To(Succeed())
		})

		It("should return error when validation fails", func() {
			expectedErr := errors.New("invalid key")
			mockClient.ValidateApiKeyFunc = func() error {
				return expectedErr
			}
			Expect(webClient.ValidateApiKey()).To(MatchError(expectedErr))
		})
	})

	// Integration tests that use the real client
	Context("Integration tests", func() {
		It("should validate API key with real client when APIFY_API_KEY is set", func() {
			if apifyKey == "" || geminiKey == "" {
				Skip("APIFY_API_KEY and GEMINI_API_KEY required to run web integration tests")
			}

			// Reset to use real client
			webapify.NewInternalClient = func(apiKey string) (client.Apify, error) {
				return client.NewApifyClient(apiKey)
			}

			realClient, err := webapify.NewClient(apifyKey, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(realClient.ValidateApiKey()).To(Succeed())
		})

		It("should scrape a real URL when APIFY_API_KEY is set", func() {
			if apifyKey == "" || geminiKey == "" {
				Skip("APIFY_API_KEY and GEMINI_API_KEY required to run web integration tests")
			}

			// Reset to use real client
			webapify.NewInternalClient = func(apiKey string) (client.Apify, error) {
				return client.NewApifyClient(apiKey)
			}

			realClient, err := webapify.NewClient(apifyKey, nil)
			Expect(err).NotTo(HaveOccurred())

			args := teeargs.WebArguments{
				URL:      "https://example.com",
				MaxDepth: 0,
				MaxPages: 1,
			}

			results, datasetId, cursor, err := realClient.Scrape("test-worker", args, client.EmptyCursor)
			Expect(err).NotTo(HaveOccurred())
			Expect(datasetId).NotTo(BeEmpty())
			Expect(results).NotTo(BeEmpty())
			Expect(results[0]).NotTo(BeNil())
			Expect(results[0].URL).To(Equal("https://example.com/"))
			Expect(cursor).NotTo(BeEmpty())
		})
	})
})
