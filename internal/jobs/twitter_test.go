package jobs_test

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	twitterscraper "github.com/imperatrona/twitter-scraper"
	"github.com/masa-finance/tee-worker/api/types"
	. "github.com/masa-finance/tee-worker/internal/jobs"
)

var _ = Describe("Twitter Scraper", func() {

	var twitterScraper *TwitterScraper
	var tempDir string
	var err error

	BeforeEach(func() {
		CIDir := os.Getenv("TEST_COOKIE_DIR")
		if CIDir != "" {
			tempDir = CIDir
		} else {
			tempDir, err = os.MkdirTemp("", "twitter")
			Expect(err).NotTo(HaveOccurred())
		}

		account := os.Getenv("TWITTER_TEST_ACCOUNT")

		if account == "" {
			Skip("TWITTER_TEST_ACCOUNT is not set")
		}

		twitterScraper = NewTwitterScraper(types.JobConfiguration{
			"twitter_accounts": []string{account},
			"data_dir":         tempDir,
		})
	})

	AfterEach(func() {
		os.RemoveAll(tempDir)
	})

	It("should scrape tweets with a search query", func() {
		res, err := twitterScraper.ExecuteJob(types.Job{
			Type: TwitterScraperType,
			Arguments: map[string]interface{}{
				"type":  "searchbyquery",
				"query": "Jimmy Kimmel",
				"count": 1,
			},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(res.Error).To(BeEmpty())

		var results []*TweetResult
		res.Unmarshal(&results)
		Expect(err).NotTo(HaveOccurred())

		Expect(len(results)).ToNot(BeZero())

		Expect(results[0].Tweet.Text).ToNot(BeEmpty())
	})

	It("should scrape a profile", func() {
		res, err := twitterScraper.ExecuteJob(types.Job{
			Type: TwitterScraperType,
			Arguments: map[string]interface{}{
				"type":  "searchbyprofile",
				"query": "NASA_Marshall",
				"count": 1,
			},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(res.Error).To(BeEmpty())

		var results []*twitterscraper.Profile
		res.Unmarshal(&results)
		Expect(err).NotTo(HaveOccurred())

		Expect(len(results)).ToNot(BeZero())

		Expect(results[0].Website).To(ContainSubstring("nasa.gov"))
	})

	It("should scrape tweets with a search query", func() {
		res, err := twitterScraper.ExecuteJob(types.Job{
			Type: TwitterScraperType,
			Arguments: map[string]interface{}{
				"type":  "searchfollowers",
				"query": "NASA_Marshall",
				"count": 1,
			},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(res.Error).To(BeEmpty())

		var results []*twitterscraper.Profile
		res.Unmarshal(&results)
		Expect(err).NotTo(HaveOccurred())

		Expect(len(results)).ToNot(BeZero())

		Expect(results[0].Username).ToNot(BeEmpty())
	})

	It("should get tweet by ID", func() {
		res, err := twitterScraper.ExecuteJob(types.Job{
			Type: TwitterScraperType,
			Arguments: map[string]interface{}{
				"type":  "getbyid",
				"query": "1881258110712492142",
			},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(res.Error).To(BeEmpty())

		var tweet *twitterscraper.Tweet
		res.Unmarshal(&tweet)
		Expect(err).NotTo(HaveOccurred())
		Expect(tweet).NotTo(BeNil())
		Expect(tweet.ID).To(Equal("1234567890"))
		Expect(tweet.Text).NotTo(BeEmpty())
	})

	It("should fetch tweet replies", func() {
		res, err := twitterScraper.ExecuteJob(types.Job{
			Type: TwitterScraperType,
			Arguments: map[string]interface{}{
				"type":  "getreplies",
				"query": "1234567890",
			},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(res.Error).To(BeEmpty())

		var replies []*TweetResult
		res.Unmarshal(&replies)
		Expect(len(replies)).ToNot(BeZero())
		Expect(replies[0].Tweet.Text).ToNot(BeEmpty())
	})

	It("should fetch tweet retweeters", func() {
		res, err := twitterScraper.ExecuteJob(types.Job{
			Type: TwitterScraperType,
			Arguments: map[string]interface{}{
				"type":  "getretweeters",
				"query": "1234567890",
				"count": 5,
			},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(res.Error).To(BeEmpty())

		var retweeters []*twitterscraper.Profile
		res.Unmarshal(&retweeters)
		Expect(len(retweeters)).ToNot(BeZero())
		Expect(retweeters[0].Username).ToNot(BeEmpty())
	})

	It("should fetch user tweets", func() {
		res, err := twitterScraper.ExecuteJob(types.Job{
			Type: TwitterScraperType,
			Arguments: map[string]interface{}{
				"type":  "gettweets",
				"query": "NASA",
				"count": 5,
			},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(res.Error).To(BeEmpty())

		var tweets []*TweetResult
		res.Unmarshal(&tweets)
		Expect(len(tweets)).ToNot(BeZero())
		Expect(tweets[0].Tweet.Text).ToNot(BeEmpty())
	})

	It("should fetch user media", func() {
		res, err := twitterScraper.ExecuteJob(types.Job{
			Type: TwitterScraperType,
			Arguments: map[string]interface{}{
				"type":  "getmedia",
				"query": "NASA",
				"count": 5,
			},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(res.Error).To(BeEmpty())

		var media []*TweetResult
		res.Unmarshal(&media)
		Expect(len(media)).ToNot(BeZero())
		Expect(len(media[0].Tweet.Photos) > 0 || len(media[0].Tweet.Videos) > 0).To(BeTrue())
	})

	It("should fetch bookmarks", func() {
		res, err := twitterScraper.ExecuteJob(types.Job{
			Type: TwitterScraperType,
			Arguments: map[string]interface{}{
				"type":  "getbookmarks",
				"count": 5,
			},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(res.Error).To(BeEmpty())

		var bookmarks []*TweetResult
		res.Unmarshal(&bookmarks)
		Expect(len(bookmarks)).ToNot(BeZero())
		Expect(bookmarks[0].Tweet.Text).ToNot(BeEmpty())
	})

	It("should fetch home tweets", func() {
		res, err := twitterScraper.ExecuteJob(types.Job{
			Type: TwitterScraperType,
			Arguments: map[string]interface{}{
				"type":  "gethometweets",
				"count": 5,
			},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(res.Error).To(BeEmpty())

		var tweets []*TweetResult
		res.Unmarshal(&tweets)
		Expect(len(tweets)).ToNot(BeZero())
		Expect(tweets[0].Tweet.Text).ToNot(BeEmpty())
	})

	It("should fetch for you tweets", func() {
		res, err := twitterScraper.ExecuteJob(types.Job{
			Type: TwitterScraperType,
			Arguments: map[string]interface{}{
				"type":  "getforyoutweets",
				"count": 5,
			},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(res.Error).To(BeEmpty())

		var tweets []*TweetResult
		res.Unmarshal(&tweets)
		Expect(len(tweets)).ToNot(BeZero())
		Expect(tweets[0].Tweet.Text).ToNot(BeEmpty())
	})

	It("should fetch profile by ID", func() {
		res, err := twitterScraper.ExecuteJob(types.Job{
			Type: TwitterScraperType,
			Arguments: map[string]interface{}{
				"type":  "getprofilebyid",
				"query": "44196397", // NASA's ID
			},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(res.Error).To(BeEmpty())

		var profile *twitterscraper.Profile
		res.Unmarshal(&profile)
		Expect(profile.Username).To(Equal("NASA"))
	})

	It("should fetch space", func() {
		res, err := twitterScraper.ExecuteJob(types.Job{
			Type: TwitterScraperType,
			Arguments: map[string]interface{}{
				"type":  "getspace",
				"query": "1YpKkZEWlBaxj",
			},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(res.Error).To(BeEmpty())

		var space *twitterscraper.Space
		res.Unmarshal(&space)
		Expect(space.ID).ToNot(BeEmpty())
	})

	It("should fetch following", func() {
		res, err := twitterScraper.ExecuteJob(types.Job{
			Type: TwitterScraperType,
			Arguments: map[string]interface{}{
				"type":  "getfollowing",
				"query": "NASA",
				"count": 5,
			},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(res.Error).To(BeEmpty())

		var following []*twitterscraper.Profile
		res.Unmarshal(&following)
		Expect(len(following)).ToNot(BeZero())
		Expect(following[0].Username).ToNot(BeEmpty())
	})

	It("should fetch tweets with cursor", func() {
		res, err := twitterScraper.ExecuteJob(types.Job{
			Type: TwitterScraperType,
			Arguments: map[string]interface{}{
				"type":        "fetchusertweets",
				"query":       "NASA",
				"count":       5,
				"next_cursor": "",
			},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(res.Error).To(BeEmpty())
		Expect(res.NextCursor).ToNot(BeEmpty())

		var tweets []*twitterscraper.Tweet
		res.Unmarshal(&tweets)
		Expect(len(tweets)).ToNot(BeZero())
		Expect(tweets[0].Text).ToNot(BeEmpty())
	})

	It("should fetch media with cursor", func() {
		res, err := twitterScraper.ExecuteJob(types.Job{
			Type: TwitterScraperType,
			Arguments: map[string]interface{}{
				"type":        "fetchusermedia",
				"query":       "NASA",
				"count":       5,
				"next_cursor": "",
			},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(res.Error).To(BeEmpty())
		Expect(res.NextCursor).ToNot(BeEmpty())

		var tweets []*twitterscraper.Tweet
		res.Unmarshal(&tweets)
		Expect(len(tweets)).ToNot(BeZero())
		Expect(tweets[0].Text).ToNot(BeEmpty())
	})
})
