package jobs_test

import (
	"os"
	"strings"

	teetypes "github.com/masa-finance/tee-types/types"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"

	twitterscraper "github.com/imperatrona/twitter-scraper"
	"github.com/masa-finance/tee-worker/api/types"
	. "github.com/masa-finance/tee-worker/internal/jobs"
	"github.com/masa-finance/tee-worker/internal/jobs/stats"
)

// parseTwitterAccounts parses TWITTER_ACCOUNTS environment variable like production does
func parseTwitterAccounts() []string {
	accountsEnv := os.Getenv("TWITTER_ACCOUNTS")
	if accountsEnv == "" {
		return nil
	}

	accounts := strings.Split(accountsEnv, ",")
	for i, account := range accounts {
		accounts[i] = strings.TrimSpace(account)
	}
	return accounts
}

var _ = Describe("Twitter Scraper", func() {

	// --- New tests for specialized job types ---
	Context("Specialized Twitter Scraper Job Types", func() {
		var statsCollector *stats.StatsCollector
		var tempDir string
		var err error
		var twitterAccounts []string
		var apiKey string

		BeforeEach(func() {
			logrus.SetLevel(logrus.DebugLevel)
			os.Setenv("LOG_LEVEL", "debug")

			tempDir = ".masa"
			err = os.MkdirAll(tempDir, 0755)
			Expect(err).NotTo(HaveOccurred())

			twitterAccounts = parseTwitterAccounts()
			apiKey = os.Getenv("TWITTER_TEST_API_KEY")
			statsCollector = stats.StartCollector(128, types.JobConfiguration{})
		})

		AfterEach(func() {
			// Don't remove .masa directory as it's used by production
		})

		It("should use credentials for twitter-credential-scraper", func() {
			if len(twitterAccounts) == 0 {
				Skip("TWITTER_ACCOUNTS is not set")
			}
			scraper := NewTwitterScraper(types.JobConfiguration{
				"twitter_accounts": twitterAccounts,
				"data_dir":         tempDir,
			}, statsCollector)
			res, err := scraper.ExecuteJob(types.Job{
				Type: TwitterCredentialScraperType,
				Arguments: map[string]interface{}{
					"type":  "searchbyquery",
					"query": "NASA",
					"count": 1,
				},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(res.Error).To(BeEmpty())
			var results []*teetypes.TweetResult
			err = res.Unmarshal(&results)
			Expect(err).NotTo(HaveOccurred())
			Expect(results).ToNot(BeEmpty())
		})

		It("should use API key for twitter-api-scraper", func() {
			if apiKey == "" {
				Skip("TWITTER_TEST_API_KEY is not set")
			}
			scraper := NewTwitterScraper(types.JobConfiguration{
				"twitter_api_keys": []string{apiKey},
				"data_dir":         tempDir,
			}, statsCollector)
			res, err := scraper.ExecuteJob(types.Job{
				Type: TwitterApiScraperType,
				Arguments: map[string]interface{}{
					"type":  "searchbyquery",
					"query": "NASA",
					"count": 1,
				},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(res.Error).To(BeEmpty())
			var results []*teetypes.TweetResult
			err = res.Unmarshal(&results)
			Expect(err).NotTo(HaveOccurred())
			Expect(results).ToNot(BeEmpty())
		})

		It("should error if wrong auth method for job type", func() {
			if apiKey == "" {
				Skip("TWITTER_TEST_API_KEY is not set")
			}
			scraper := NewTwitterScraper(types.JobConfiguration{
				"twitter_api_keys": []string{apiKey},
				"data_dir":         tempDir,
			}, statsCollector)
			// Try to run credential-only job with only API key
			res, err := scraper.ExecuteJob(types.Job{
				Type: TwitterCredentialScraperType,
				Arguments: map[string]interface{}{
					"type":  "searchbyquery",
					"query": "NASA",
					"count": 1,
				},
			})
			Expect(err).To(HaveOccurred())
			Expect(res.Error).NotTo(BeEmpty())
		})

		It("should prefer credentials if both are present for twitter-scraper", func() {
			if len(twitterAccounts) == 0 || apiKey == "" {
				Skip("TWITTER_ACCOUNTS or TWITTER_TEST_API_KEY is not set")
			}
			scraper := NewTwitterScraper(types.JobConfiguration{
				"twitter_accounts": twitterAccounts,
				"twitter_api_keys": []string{apiKey},
				"data_dir":         tempDir,
			}, statsCollector)
			res, err := scraper.ExecuteJob(types.Job{
				Type: TwitterScraperType,
				Arguments: map[string]interface{}{
					"type":  "searchbyquery",
					"query": "NASA",
					"count": 1,
				},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(res.Error).To(BeEmpty())
			var results []*teetypes.TweetResult
			err = res.Unmarshal(&results)
			Expect(err).NotTo(HaveOccurred())
			Expect(results).ToNot(BeEmpty())
		})

		It("should error if neither credentials nor API key are present", func() {
			scraper := NewTwitterScraper(types.JobConfiguration{
				"data_dir": tempDir,
			}, statsCollector)
			res, err := scraper.ExecuteJob(types.Job{
				Type: TwitterApiScraperType,
				Arguments: map[string]interface{}{
					"type":  "searchbyquery",
					"query": "NASA",
					"count": 1,
				},
			})
			Expect(err).To(HaveOccurred())
			Expect(res.Error).NotTo(BeEmpty())
		})
	})

	var twitterScraper *TwitterScraper
	var statsCollector *stats.StatsCollector
	var tempDir string
	var err error

	BeforeEach(func() {
		tempDir = ".masa"
		err = os.MkdirAll(tempDir, 0755)
		Expect(err).NotTo(HaveOccurred())

		twitterAccounts := parseTwitterAccounts()

		if len(twitterAccounts) == 0 {
			Skip("TWITTER_ACCOUNTS is not set")
		}

		statsCollector = stats.StartCollector(128, types.JobConfiguration{})

		twitterScraper = NewTwitterScraper(types.JobConfiguration{
			"twitter_accounts": twitterAccounts,
			"data_dir":         tempDir,
		}, statsCollector)
	})

	AfterEach(func() {
		// Don't remove .masa directory as it's used by production
	})

	It("should scrape tweets with a search query", func() {
		j := types.Job{
			Type: TwitterScraperType,
			Arguments: map[string]interface{}{
				"type":  "searchbyquery",
				"query": "AI",
				"count": 1,
			},
		}
		res, err := twitterScraper.ExecuteJob(j)
		Expect(err).NotTo(HaveOccurred())
		Expect(res.Error).To(BeEmpty())

		var results []*teetypes.TweetResult
		err = res.Unmarshal(&results)
		Expect(err).NotTo(HaveOccurred())
		Expect(results).ToNot(BeEmpty())

		Expect(results[0].Text).ToNot(BeEmpty())
		Expect(statsCollector.Stats.Stats[j.WorkerID][stats.TwitterScrapes]).To(BeNumerically("==", 1))
		Expect(statsCollector.Stats.Stats[j.WorkerID][stats.TwitterTweets]).To(BeNumerically("==", uint(len(results))))
	})

	It("should scrape a profile", func() {
		j := types.Job{
			Type: TwitterScraperType,
			Arguments: map[string]interface{}{
				"type":  "searchbyprofile",
				"query": "NASA_Marshall",
				"count": 1,
			},
			WorkerID: "foo",
		}
		res, err := twitterScraper.ExecuteJob(j)
		Expect(err).NotTo(HaveOccurred())
		Expect(res.Error).To(BeEmpty())

		var results []*twitterscraper.Profile
		err = res.Unmarshal(&results)
		Expect(err).NotTo(HaveOccurred())
		Expect(len(results)).ToNot(BeZero())

		Expect(results[0].Website).To(ContainSubstring("nasa.gov"))

		Expect(statsCollector.Stats.Stats[j.WorkerID][stats.TwitterScrapes]).To(BeNumerically("==", 0))
		Expect(statsCollector.Stats.Stats[j.WorkerID][stats.TwitterProfiles]).To(BeNumerically("==", uint(len(results))))
	})

	It("should scrape tweets with a search query", func() {
		j := types.Job{
			Type: TwitterScraperType,
			Arguments: map[string]interface{}{
				"type":  "searchfollowers",
				"query": "NASA_Marshall",
				"count": 1,
			},
			WorkerID: "foo",
		}
		res, err := twitterScraper.ExecuteJob(j)
		Expect(err).NotTo(HaveOccurred())
		Expect(res.Error).To(BeEmpty())

		var results []*twitterscraper.Profile
		err = res.Unmarshal(&results)
		Expect(err).NotTo(HaveOccurred())
		Expect(len(results)).ToNot(BeZero())
		Expect(results[0].Username).ToNot(BeEmpty())

		Expect(statsCollector.Stats.Stats[j.WorkerID][stats.TwitterScrapes]).To(BeNumerically("==", 1))
		Expect(statsCollector.Stats.Stats[j.WorkerID][stats.TwitterProfiles]).To(BeNumerically("==", uint(len(results))))
	})

	FIt("should get tweet by ID", func() {
		logrus.SetLevel(logrus.DebugLevel) // Ensure debug logs are visible

		res, err := twitterScraper.ExecuteJob(types.Job{
			Type: TwitterScraperType,
			Arguments: map[string]interface{}{
				"type":  "getbyid",
				"query": "1881258110712492142",
			},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(res.Error).To(BeEmpty())

		// Debug: Print the raw response using logrus for visibility
		logrus.Infof("Raw response Data length: %d", len(res.Data))
		logrus.Infof("Raw response Error: %s", res.Error)
		logrus.Infof("Raw response NextCursor: %s", res.NextCursor)

		// Try unmarshaling to a generic interface first for debugging
		var rawResult interface{}
		err = res.Unmarshal(&rawResult)
		Expect(err).NotTo(HaveOccurred())
		logrus.Infof("Unmarshaled generic result type: %T", rawResult)

		// Now try the correct type - should be a single TweetResult, not twitterscraper.Tweet
		var tweet *teetypes.TweetResult
		err = res.Unmarshal(&tweet)
		Expect(err).NotTo(HaveOccurred())
		Expect(tweet).NotTo(BeNil())
		Expect(tweet.TweetID).To(Equal("1881258110712492142")) // Use TweetID field, not ID
		Expect(tweet.Text).NotTo(BeEmpty())

		logrus.Infof("Successfully unmarshaled tweet: ID=%s, Text=%s", tweet.TweetID, tweet.Text)
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

		var replies []*teetypes.TweetResult
		err = res.Unmarshal(&replies)
		Expect(err).NotTo(HaveOccurred())
		Expect(replies).ToNot(BeEmpty())
		Expect(replies[0].Text).ToNot(BeEmpty())
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
		err = res.Unmarshal(&retweeters)
		Expect(err).NotTo(HaveOccurred())
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

		var tweets []*teetypes.TweetResult
		err = res.Unmarshal(&tweets)
		Expect(err).NotTo(HaveOccurred())
		Expect(len(tweets)).ToNot(BeZero())
		Expect(tweets[0].Text).ToNot(BeEmpty())
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

		var media []*teetypes.TweetResult
		err = res.Unmarshal(&media)
		Expect(err).NotTo(HaveOccurred())
		Expect(media).ToNot(BeEmpty())
		Expect(len(media[0].Photos) + len(media[0].Videos)).ToNot(BeZero())
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

		var bookmarks []*teetypes.TweetResult
		err = res.Unmarshal(&bookmarks)
		Expect(err).NotTo(HaveOccurred())
		Expect(len(bookmarks)).ToNot(BeZero())
		Expect(bookmarks[0].Text).ToNot(BeEmpty())
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

		var tweets []*teetypes.TweetResult
		err = res.Unmarshal(&tweets)
		Expect(err).NotTo(HaveOccurred())
		Expect(len(tweets)).ToNot(BeZero())
		Expect(tweets[0].Text).ToNot(BeEmpty())
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

		var tweets []*teetypes.TweetResult
		err = res.Unmarshal(&tweets)
		Expect(err).NotTo(HaveOccurred())
		Expect(len(tweets)).ToNot(BeZero())
		Expect(tweets).ToNot(BeEmpty())
		Expect(tweets[0].Text).ToNot(BeEmpty())
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
		err = res.Unmarshal(&profile)
		Expect(err).NotTo(HaveOccurred())
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
		err = res.Unmarshal(&space)
		Expect(err).NotTo(HaveOccurred())
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
		err = res.Unmarshal(&following)
		Expect(err).NotTo(HaveOccurred())
		Expect(len(following)).ToNot(BeZero())
		Expect(following[0].Username).ToNot(BeEmpty())
	})

})
