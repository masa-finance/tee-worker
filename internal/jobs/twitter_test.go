package jobs_test

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/masa-finance/tee-worker/api/types"
	. "github.com/masa-finance/tee-worker/internal/jobs"
)

var _ = Describe("Twitter Scraper", func() {

	var twitterScraper *TwitterScraper
	var tempDir string
	var err error

	BeforeEach(func() {
		tempDir, err = os.MkdirTemp("", "twitter")
		Expect(err).NotTo(HaveOccurred())

		twitterScraper = NewTwitterScraper(types.JobConfiguration{
			"twitter_accounts": []string{os.Getenv("TWITTER_TEST_ACCOUNT")},
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
})
