package redditapify_test

import (
	"encoding/json"
	"errors"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/masa-finance/tee-worker/internal/apify"
	"github.com/masa-finance/tee-worker/internal/jobs/redditapify"
	"github.com/masa-finance/tee-worker/pkg/client"

	teeargs "github.com/masa-finance/tee-types/args"
	teetypes "github.com/masa-finance/tee-types/types"
)

// MockApifyClient is a mock implementation of the ApifyClient.
type MockApifyClient struct {
	RunActorAndGetResponseFunc func(actorID string, input any, cursor client.Cursor, limit uint) (*client.DatasetResponse, client.Cursor, error)
	ValidateApiKeyFunc         func() error
	ProbeActorAccessFunc       func(actorID string, input map[string]any) (bool, error)
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

func (m *MockApifyClient) ProbeActorAccess(actorID string, input map[string]any) (bool, error) {
	if m.ProbeActorAccessFunc != nil {
		return m.ProbeActorAccessFunc(actorID, input)
	}
	return false, errors.New("ProbeActorAccessFunc not defined")
}

var _ = Describe("RedditApifyClient", func() {
	var (
		mockClient   *MockApifyClient
		redditClient *redditapify.RedditApifyClient
	)

	BeforeEach(func() {
		mockClient = &MockApifyClient{}
		// Replace the client creation function with one that returns the mock
		redditapify.NewInternalClient = func(apiKey string) (client.Apify, error) {
			return mockClient, nil
		}
		var err error
		redditClient, err = redditapify.NewClient("test-token", nil)
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("ScrapeUrls", func() {
		It("should construct the correct actor input", func() {
			urls := []teetypes.RedditStartURL{{URL: "http://reddit.com/r/golang"}}
			after := time.Now()
			args := redditapify.CommonArgs{MaxPosts: 10}

			mockClient.RunActorAndGetResponseFunc = func(actorID string, input any, cursor client.Cursor, limit uint) (*client.DatasetResponse, client.Cursor, error) {
				Expect(actorID).To(Equal(apify.Actors.RedditScraper))
				req := input.(redditapify.RedditActorRequest)
				Expect(req.StartUrls).To(Equal(urls))
				Expect(*req.PostDateLimit).To(BeTemporally("~", after, time.Second))
				Expect(req.Searches).To(BeNil())
				Expect(req.SearchUsers).To(BeTrue())
				Expect(req.SearchPosts).To(BeTrue())
				Expect(req.SearchCommunities).To(BeTrue())
				Expect(req.SkipUserPosts).To(BeFalse())
				Expect(req.MaxPostCount).To(Equal(uint(10)))
				return &client.DatasetResponse{Data: client.ApifyDatasetData{Items: []json.RawMessage{}}}, "next", nil
			}

			_, _, err := redditClient.ScrapeUrls("", urls, after, args, "", 100)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("SearchPosts", func() {
		It("should construct the correct actor input", func() {
			queries := []string{"golang"}
			after := time.Now()
			args := redditapify.CommonArgs{MaxComments: 5}

			mockClient.RunActorAndGetResponseFunc = func(actorID string, input any, cursor client.Cursor, limit uint) (*client.DatasetResponse, client.Cursor, error) {
				Expect(actorID).To(Equal(apify.Actors.RedditScraper))
				req := input.(redditapify.RedditActorRequest)
				Expect(req.Searches).To(Equal(queries))
				Expect(req.StartUrls).To(BeNil())
				Expect(*req.PostDateLimit).To(BeTemporally("~", after, time.Second))
				Expect(req.Type).To(Equal(teetypes.RedditQueryType("posts")))
				Expect(req.SearchPosts).To(BeTrue())
				Expect(req.SkipComments).To(BeFalse())
				Expect(req.MaxComments).To(Equal(uint(5)))
				return &client.DatasetResponse{Data: client.ApifyDatasetData{Items: []json.RawMessage{}}}, "next", nil
			}

			_, _, err := redditClient.SearchPosts("", queries, after, args, "", 100)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("SearchCommunities", func() {
		It("should construct the correct actor input", func() {
			queries := []string{"golang"}
			args := redditapify.CommonArgs{}

			mockClient.RunActorAndGetResponseFunc = func(actorID string, input any, cursor client.Cursor, limit uint) (*client.DatasetResponse, client.Cursor, error) {
				Expect(actorID).To(Equal(apify.Actors.RedditScraper))
				req := input.(redditapify.RedditActorRequest)
				Expect(req.Searches).To(Equal(queries))
				Expect(req.StartUrls).To(BeNil())
				Expect(req.Type).To(Equal(teetypes.RedditQueryType("communities")))
				Expect(req.SearchCommunities).To(BeTrue())
				return &client.DatasetResponse{Data: client.ApifyDatasetData{Items: []json.RawMessage{}}}, "next", nil
			}

			_, _, err := redditClient.SearchCommunities("", queries, args, "", 100)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("SearchUsers", func() {
		It("should construct the correct actor input", func() {
			queries := []string{"gopher"}
			args := redditapify.CommonArgs{}

			mockClient.RunActorAndGetResponseFunc = func(actorID string, input any, cursor client.Cursor, limit uint) (*client.DatasetResponse, client.Cursor, error) {
				Expect(actorID).To(Equal(apify.Actors.RedditScraper))
				req := input.(redditapify.RedditActorRequest)
				Expect(req.Searches).To(Equal(queries))
				Expect(req.StartUrls).To(BeNil())
				Expect(req.Type).To(Equal(teetypes.RedditQueryType("users")))
				Expect(req.SearchUsers).To(BeTrue())
				Expect(req.SkipUserPosts).To(BeTrue())
				return &client.DatasetResponse{Data: client.ApifyDatasetData{Items: []json.RawMessage{}}}, "next", nil
			}

			_, _, err := redditClient.SearchUsers("", queries, true, args, "", 100)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("queryReddit", func() {
		It("should handle errors from the apify client", func() {
			expectedErr := errors.New("apify error")
			mockClient.RunActorAndGetResponseFunc = func(actorID string, input any, cursor client.Cursor, limit uint) (*client.DatasetResponse, client.Cursor, error) {
				return nil, "", expectedErr
			}
			_, _, err := redditClient.SearchUsers("", []string{"test"}, false, redditapify.CommonArgs{}, "", 10)
			Expect(err).To(MatchError(expectedErr))
		})

		It("should handle JSON unmarshalling errors gracefully", func() {
			invalidJSON := []byte(`{"type": "user", "id": 123}`) // id should be a string
			dataset := &client.DatasetResponse{
				Data: client.ApifyDatasetData{
					Items: []json.RawMessage{invalidJSON},
				},
			}
			mockClient.RunActorAndGetResponseFunc = func(actorID string, input any, cursor client.Cursor, limit uint) (*client.DatasetResponse, client.Cursor, error) {
				return dataset, "next", nil
			}

			// This is a bit of a hack to test the private queryReddit method
			// We call a public method that uses it
			profiles, _, err := redditClient.SearchUsers("", []string{"test"}, false, redditapify.CommonArgs{}, "", 10)
			Expect(err).NotTo(HaveOccurred())
			Expect(profiles).To(BeEmpty()) // The invalid item should be skipped
		})

		It("should correctly unmarshal valid items", func() {
			userJSON, _ := json.Marshal(map[string]any{"type": "user", "id": "u1", "username": "testuser"})
			dataset := &client.DatasetResponse{
				Data: client.ApifyDatasetData{
					Items: []json.RawMessage{userJSON},
				},
			}
			mockClient.RunActorAndGetResponseFunc = func(actorID string, input any, cursor client.Cursor, limit uint) (*client.DatasetResponse, client.Cursor, error) {
				return dataset, "next", nil
			}

			profiles, cursor, err := redditClient.SearchUsers("", []string{"test"}, false, redditapify.CommonArgs{}, "", 10)
			Expect(err).NotTo(HaveOccurred())
			Expect(cursor).To(Equal(client.Cursor("next")))
			Expect(profiles).To(HaveLen(1))
			Expect(profiles[0].User).NotTo(BeNil())
			Expect(profiles[0].User.ID).To(Equal("u1"))
		})
	})

	Describe("CommonArgs", func() {
		It("should copy from RedditArguments correctly", func() {
			redditArgs := &teeargs.RedditArguments{
				Sort:           teetypes.RedditSortTop,
				IncludeNSFW:    true,
				MaxItems:       1,
				MaxPosts:       2,
				MaxComments:    3,
				MaxCommunities: 4,
				MaxUsers:       5,
			}
			commonArgs := redditapify.CommonArgs{}
			commonArgs.CopyFromArgs(redditArgs)

			Expect(commonArgs.Sort).To(Equal(teetypes.RedditSortTop))
			Expect(commonArgs.IncludeNSFW).To(BeTrue())
			Expect(commonArgs.MaxItems).To(Equal(uint(1)))
			Expect(commonArgs.MaxPosts).To(Equal(uint(2)))
			Expect(commonArgs.MaxComments).To(Equal(uint(3)))
			Expect(commonArgs.MaxCommunities).To(Equal(uint(4)))
			Expect(commonArgs.MaxUsers).To(Equal(uint(5)))
		})

		It("should convert to RedditActorRequest correctly", func() {
			commonArgs := redditapify.CommonArgs{
				Sort:           teetypes.RedditSortNew,
				IncludeNSFW:    true,
				MaxItems:       10,
				MaxPosts:       20,
				MaxComments:    30,
				MaxCommunities: 40,
				MaxUsers:       50,
			}
			actorReq := commonArgs.ToActorRequest()

			Expect(actorReq.Sort).To(Equal(teetypes.RedditSortNew))
			Expect(actorReq.IncludeNSFW).To(BeTrue())
			Expect(actorReq.MaxItems).To(Equal(uint(10)))
			Expect(actorReq.MaxPostCount).To(Equal(uint(20)))
			Expect(actorReq.MaxComments).To(Equal(uint(30)))
			Expect(actorReq.MaxCommunitiesCount).To(Equal(uint(40)))
			Expect(actorReq.MaxUserCount).To(Equal(uint(50)))
		})
	})
})
