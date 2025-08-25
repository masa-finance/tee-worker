package jobs_test

import (
	"encoding/json"
	"errors"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"

	"github.com/masa-finance/tee-worker/api/types"
	"github.com/masa-finance/tee-worker/api/types/reddit"
	"github.com/masa-finance/tee-worker/internal/jobs"
	"github.com/masa-finance/tee-worker/internal/jobs/redditapify"
	"github.com/masa-finance/tee-worker/internal/jobs/stats"
	"github.com/masa-finance/tee-worker/pkg/client"

	teetypes "github.com/masa-finance/tee-types/types"
)

// MockRedditApifyClient is a mock implementation of the RedditApifyClient.
type MockRedditApifyClient struct {
	ScrapeUrlsFunc        func(urls []teetypes.RedditStartURL, after time.Time, args redditapify.CommonArgs, cursor client.Cursor, maxResults uint) ([]*reddit.Response, client.Cursor, error)
	SearchPostsFunc       func(queries []string, after time.Time, args redditapify.CommonArgs, cursor client.Cursor, maxResults uint) ([]*reddit.Response, client.Cursor, error)
	SearchCommunitiesFunc func(queries []string, args redditapify.CommonArgs, cursor client.Cursor, maxResults uint) ([]*reddit.Response, client.Cursor, error)
	SearchUsersFunc       func(queries []string, skipPosts bool, args redditapify.CommonArgs, cursor client.Cursor, maxResults uint) ([]*reddit.Response, client.Cursor, error)
}

func (m *MockRedditApifyClient) ScrapeUrls(_ string, urls []teetypes.RedditStartURL, after time.Time, args redditapify.CommonArgs, cursor client.Cursor, maxResults uint) ([]*reddit.Response, client.Cursor, error) {
	if m != nil && m.ScrapeUrlsFunc != nil {
		res, cursor, err := m.ScrapeUrlsFunc(urls, after, args, cursor, maxResults)
		for i, r := range res {
			logrus.Debugf("Scrape URLs result %d: %+v", i, r)
		}
		return res, cursor, err
	}
	return nil, "", nil
}

func (m *MockRedditApifyClient) SearchPosts(_ string, queries []string, after time.Time, args redditapify.CommonArgs, cursor client.Cursor, maxResults uint) ([]*reddit.Response, client.Cursor, error) {
	if m != nil && m.SearchPostsFunc != nil {
		return m.SearchPostsFunc(queries, after, args, cursor, maxResults)
	}
	return nil, "", nil
}

func (m *MockRedditApifyClient) SearchCommunities(_ string, queries []string, args redditapify.CommonArgs, cursor client.Cursor, maxResults uint) ([]*reddit.Response, client.Cursor, error) {
	if m != nil && m.SearchCommunitiesFunc != nil {
		return m.SearchCommunitiesFunc(queries, args, cursor, maxResults)
	}
	return nil, "", nil
}

func (m *MockRedditApifyClient) SearchUsers(_ string, queries []string, skipPosts bool, args redditapify.CommonArgs, cursor client.Cursor, maxResults uint) ([]*reddit.Response, client.Cursor, error) {
	if m != nil && m.SearchUsersFunc != nil {
		return m.SearchUsersFunc(queries, skipPosts, args, cursor, maxResults)
	}
	return nil, "", nil
}

var _ = Describe("RedditScraper", func() {
	var (
		scraper        *jobs.RedditScraper
		statsCollector *stats.StatsCollector
		job            types.Job
		mockClient     *MockRedditApifyClient
	)

	BeforeEach(func() {
		statsCollector = stats.StartCollector(128, types.JobConfiguration{})
		cfg := types.JobConfiguration{
			"apify_api_key": "test-key",
		}
		scraper = jobs.NewRedditScraper(cfg, statsCollector)
		mockClient = &MockRedditApifyClient{}

		// Replace the client creation function with one that returns the mock
		jobs.NewRedditApifyClient = func(apiKey string, _ *stats.StatsCollector) (jobs.RedditApifyClient, error) {
			return mockClient, nil
		}

		job = types.Job{
			UUID: "test-uuid",
			Type: teetypes.RedditJob,
		}
	})

	Context("ExecuteJob", func() {
		It("should return an error for invalid arguments", func() {
			job.Arguments = map[string]any{"invalid": "args"}
			result, err := scraper.ExecuteJob(job)
			Expect(err).To(HaveOccurred())
			Expect(result.Error).To(ContainSubstring("failed to unmarshal job arguments"))
		})

		It("should call ScrapeUrls for the correct QueryType", func() {
			job.Arguments = map[string]any{
				"type": teetypes.RedditScrapeUrls,
				"urls": []teetypes.RedditStartURL{{URL: "https://www.reddit.com/u/zaphod/", Method: "GET"}},
			}

			mockClient.ScrapeUrlsFunc = func(urls []teetypes.RedditStartURL, after time.Time, cArgs redditapify.CommonArgs, cursor client.Cursor, maxResults uint) ([]*reddit.Response, client.Cursor, error) {
				Expect(urls).To(HaveLen(1))
				Expect(urls[0].URL).To(Equal("https://www.reddit.com/u/zaphod/"))
				return []*reddit.Response{{TypeSwitch: &reddit.TypeSwitch{Type: reddit.UserResponse}, User: &reddit.User{ID: "user1", DataType: string(reddit.UserResponse)}}}, "next", nil
			}

			result, err := scraper.ExecuteJob(job)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.NextCursor).To(Equal("next"))
			var resp []*reddit.Response
			err = json.Unmarshal(result.Data, &resp)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp).To(HaveLen(1))
			Expect(resp[0]).NotTo(BeNil())
			Expect(resp[0].User).NotTo(BeNil())
			Expect(resp[0].User.ID).To(Equal("user1"))
		})

		It("should call SearchUsers for the correct QueryType", func() {
			job.Arguments = map[string]any{
				"type":    teetypes.RedditSearchUsers,
				"queries": []string{"user-query"},
			}

			mockClient.SearchUsersFunc = func(queries []string, skipPosts bool, cArgs redditapify.CommonArgs, cursor client.Cursor, maxResults uint) ([]*reddit.Response, client.Cursor, error) {
				Expect(queries).To(Equal([]string{"user-query"}))
				return []*reddit.Response{{TypeSwitch: &reddit.TypeSwitch{Type: reddit.UserResponse}, User: &reddit.User{ID: "user2", DataType: string(reddit.UserResponse)}}}, "next-user", nil
			}

			result, err := scraper.ExecuteJob(job)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.NextCursor).To(Equal("next-user"))
			var resp []*reddit.Response
			err = json.Unmarshal(result.Data, &resp)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp).To(HaveLen(1))
			Expect(resp[0]).NotTo(BeNil())
			Expect(resp[0].User).NotTo(BeNil())
			Expect(resp[0].User.ID).To(Equal("user2"))
		})

		It("should call SearchPosts for the correct QueryType", func() {
			job.Arguments = map[string]any{
				"type":    teetypes.RedditSearchPosts,
				"queries": []string{"post-query"},
			}

			mockClient.SearchPostsFunc = func(queries []string, after time.Time, cArgs redditapify.CommonArgs, cursor client.Cursor, maxResults uint) ([]*reddit.Response, client.Cursor, error) {
				Expect(queries).To(Equal([]string{"post-query"}))
				return []*reddit.Response{{TypeSwitch: &reddit.TypeSwitch{Type: reddit.PostResponse}, Post: &reddit.Post{ID: "post1", DataType: string(reddit.PostResponse)}}}, "next-post", nil
			}

			result, err := scraper.ExecuteJob(job)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.NextCursor).To(Equal("next-post"))
			var resp []*reddit.Response
			err = json.Unmarshal(result.Data, &resp)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp).To(HaveLen(1))
			Expect(resp[0]).NotTo(BeNil())
			Expect(resp[0].Post).NotTo(BeNil())
			Expect(resp[0].Post.ID).To(Equal("post1"))
		})

		It("should call SearchCommunities for the correct QueryType", func() {
			job.Arguments = map[string]any{
				"type":    teetypes.RedditSearchCommunities,
				"queries": []string{"community-query"},
			}

			mockClient.SearchCommunitiesFunc = func(queries []string, cArgs redditapify.CommonArgs, cursor client.Cursor, maxResults uint) ([]*reddit.Response, client.Cursor, error) {
				Expect(queries).To(Equal([]string{"community-query"}))
				return []*reddit.Response{{TypeSwitch: &reddit.TypeSwitch{Type: reddit.CommunityResponse}, Community: &reddit.Community{ID: "comm1", DataType: string(reddit.CommunityResponse)}}}, "next-comm", nil
			}

			result, err := scraper.ExecuteJob(job)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.NextCursor).To(Equal("next-comm"))
			var resp []*reddit.Response
			err = json.Unmarshal(result.Data, &resp)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp).To(HaveLen(1))
			Expect(resp[0]).NotTo(BeNil())
			Expect(resp[0].Community).NotTo(BeNil())
			Expect(resp[0].Community.ID).To(Equal("comm1"))
		})

		It("should return an error for an invalid QueryType", func() {
			job.Arguments = map[string]any{
				"type": "invalid-type",
			}

			result, err := scraper.ExecuteJob(job)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(ContainSubstring("invalid type")))
			Expect(result.Error).To(ContainSubstring("invalid type"))
		})

		It("should handle errors from the reddit client", func() {
			job.Arguments = map[string]any{
				"type":    teetypes.RedditSearchPosts,
				"queries": []string{"post-query"},
			}

			expectedErr := errors.New("client error")
			mockClient.SearchPostsFunc = func(queries []string, after time.Time, cArgs redditapify.CommonArgs, cursor client.Cursor, maxResults uint) ([]*reddit.Response, client.Cursor, error) {
				return nil, "", expectedErr
			}

			result, err := scraper.ExecuteJob(job)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(ContainSubstring("client error")))
			Expect(result.Error).To(ContainSubstring("error while scraping Reddit: client error"))
		})

		It("should handle errors when creating the client", func() {
			jobs.NewRedditApifyClient = func(apiKey string, _ *stats.StatsCollector) (jobs.RedditApifyClient, error) {
				return nil, errors.New("client creation failed")
			}
			job.Arguments = map[string]any{
				"type":    teetypes.RedditSearchPosts,
				"queries": []string{"post-query"},
			}

			result, err := scraper.ExecuteJob(job)
			Expect(err).To(HaveOccurred())
			Expect(result.Error).To(Equal("error while scraping Reddit"))
		})
	})
})
