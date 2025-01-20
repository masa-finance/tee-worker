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
})
