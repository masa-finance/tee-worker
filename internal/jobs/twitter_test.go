package jobs_test

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	teetypes "github.com/masa-finance/tee-types/types"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"

	twitterscraper "github.com/imperatrona/twitter-scraper"
	"github.com/masa-finance/tee-worker/api/types"
	. "github.com/masa-finance/tee-worker/internal/jobs"
	"github.com/masa-finance/tee-worker/internal/jobs/stats"
	"github.com/masa-finance/tee-worker/internal/jobs/twitterx"
)

// parseTwitterAccounts parses TWITTER_ACCOUNTS environment variable like production does
func parseTwitterAccounts() []string {
	accountsEnv := os.Getenv("TWITTER_ACCOUNTS")
	if accountsEnv == "" {
		return []string{}
	}

	accounts := strings.Split(accountsEnv, ",")
	for i, account := range accounts {
		accounts[i] = strings.TrimSpace(account)
	}
	return accounts
}

// parseTwitterApiKeys parses TWITTER_API_KEYS environment variable like production does
func parseTwitterApiKeys() []string {
	apiKeysEnv := os.Getenv("TWITTER_API_KEYS")
	if apiKeysEnv == "" {
		return []string{}
	}

	apiKeys := strings.Split(apiKeysEnv, ",")
	for i, apiKey := range apiKeys {
		apiKeys[i] = strings.TrimSpace(apiKey)
	}
	return apiKeys
}

var _ = Describe("Twitter Scraper", func() {
	var twitterScraper *TwitterScraper
	var statsCollector *stats.StatsCollector
	var tempDir string
	var err error
	var twitterAccounts []string
	var twitterApiKeys []string

	BeforeEach(func() {
		logrus.SetLevel(logrus.DebugLevel)
		os.Setenv("LOG_LEVEL", "debug")

		tempDir = os.Getenv("DATA_DIR")
		if tempDir == "" {
			tempDir = ".masa"
		}
		err = os.MkdirAll(tempDir, 0755)
		Expect(err).NotTo(HaveOccurred())

		twitterAccounts = parseTwitterAccounts()
		twitterApiKeys = parseTwitterApiKeys()

		// Skip all tests if neither auth method is available
		if len(twitterAccounts) == 0 && len(twitterApiKeys) == 0 {
			Skip("Neither TWITTER_ACCOUNTS nor TWITTER_API_KEYS are set... not possible to scrape!")
		}

		// Configure the stats collector with the same configuration that TwitterScraper needs
		// This ensures capability detection works correctly
		testConfig := types.JobConfiguration{
			"twitter_accounts": twitterAccounts,
			"twitter_api_keys": twitterApiKeys,
			"data_dir":         tempDir,
		}

		statsCollector = stats.StartCollector(128, testConfig)
		twitterScraper = NewTwitterScraper(testConfig, statsCollector)
	})

	AfterEach(func() {
		// Keep files in .masa directory for testing purposes
		// os.RemoveAll(tempDir)
	})

	// --- Tests for specialized job types with specific auth requirements ---
	Context("Specialized Twitter Scraper Job Types", func() {
		It("should use credentials for twitter-credential-scraper", func() {
			if len(twitterAccounts) == 0 {
				Skip("TWITTER_ACCOUNTS is not set")
			}
			scraper := NewTwitterScraper(types.JobConfiguration{
				"twitter_accounts": twitterAccounts,
				"data_dir":         tempDir,
			}, statsCollector)
			res, err := scraper.ExecuteJob(types.Job{
				Type: teetypes.TwitterCredentialJob,
				Arguments: map[string]interface{}{
					"type":        "searchbyquery",
					"query":       "NASA",
					"max_results": 1,
				},
				Timeout: 10 * time.Second,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(res.Error).To(BeEmpty())
			var results []*teetypes.TweetResult
			err = res.Unmarshal(&results)
			Expect(err).NotTo(HaveOccurred())
			Expect(results).ToNot(BeEmpty())
		})

		It("should use API key for twitter-api-scraper with searchbyquery", func() {
			if len(twitterApiKeys) == 0 {
				Skip("TWITTER_API_KEYS is not set")
			}
			scraper := NewTwitterScraper(types.JobConfiguration{
				"twitter_api_keys": twitterApiKeys,
				"data_dir":         tempDir,
			}, statsCollector)
			res, err := scraper.ExecuteJob(types.Job{
				Type: teetypes.TwitterApiJob,
				Arguments: map[string]interface{}{
					"type":        "searchbyquery",
					"query":       "NASA",
					"max_results": 1,
				},
				Timeout: 10 * time.Second,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(res.Error).To(BeEmpty())
			var results []*teetypes.TweetResult
			err = res.Unmarshal(&results)
			Expect(err).NotTo(HaveOccurred())
			Expect(results).ToNot(BeEmpty())
		})

		It("should error if wrong auth method for job type", func() {
			if len(twitterApiKeys) == 0 {
				Skip("TWITTER_API_KEYS is not set")
			}
			scraper := NewTwitterScraper(types.JobConfiguration{
				"twitter_api_keys": twitterApiKeys,
				"data_dir":         tempDir,
			}, statsCollector)
			// Try to run credential-only job with only API key
			res, err := scraper.ExecuteJob(types.Job{
				Type: teetypes.TwitterCredentialJob,
				Arguments: map[string]interface{}{
					"type":        "searchbyquery",
					"query":       "NASA",
					"max_results": 1,
				},
				Timeout: 10 * time.Second,
			})
			Expect(err).To(HaveOccurred())
			Expect(res.Error).NotTo(BeEmpty())
		})

		It("should prefer credentials if both are present for twitter-scraper", func() {
			if len(twitterAccounts) == 0 || len(twitterApiKeys) == 0 {
				Skip("TWITTER_ACCOUNTS or TWITTER_API_KEYS is not set")
			}
			scraper := NewTwitterScraper(types.JobConfiguration{
				"twitter_accounts": twitterAccounts,
				"twitter_api_keys": twitterApiKeys,
				"data_dir":         tempDir,
			}, statsCollector)
			res, err := scraper.ExecuteJob(types.Job{
				Type: teetypes.TwitterJob,
				Arguments: map[string]interface{}{
					"type":        "searchbyquery",
					"query":       "nasa",
					"max_results": 10,
				},
				Timeout: 10 * time.Second,
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
				Type: teetypes.TwitterApiJob,
				Arguments: map[string]interface{}{
					"type":        "searchbyquery",
					"query":       "NASA",
					"max_results": 1,
				},
				Timeout: 10 * time.Second,
			})
			Expect(err).To(HaveOccurred())
			Expect(res.Error).NotTo(BeEmpty())
		})

		It("should use API key for twitter-api-scraper with searchbyfullarchive if elevated key available", func() {
			if len(twitterApiKeys) == 0 {
				Skip("TWITTER_API_KEYS is not set")
			}
			scraper := NewTwitterScraper(types.JobConfiguration{
				"twitter_api_keys": twitterApiKeys,
				"data_dir":         tempDir,
			}, statsCollector)
			res, err := scraper.ExecuteJob(types.Job{
				Type: teetypes.TwitterApiJob,
				Arguments: map[string]interface{}{
					"type":        "searchbyfullarchive",
					"query":       "NASA",
					"max_results": 1,
				},
				Timeout: 10 * time.Second,
			})
			// This test may fail if API key is not elevated, but that's expected behavior
			if err != nil && strings.Contains(err.Error(), "base/Basic key") {
				Skip("API key does not have elevated/Pro access for full archive search")
			}
			Expect(err).NotTo(HaveOccurred())
			Expect(res.Error).To(BeEmpty())
			var results []*teetypes.TweetResult
			err = res.Unmarshal(&results)
			Expect(err).NotTo(HaveOccurred())
			Expect(results).ToNot(BeEmpty())
		})
	})

	// --- General Twitter scraper tests (uses best available auth method) ---
	Context("General Twitter Scraper Tests", func() {
		It("should scrape tweets with a search query", func() {
			j := types.Job{
				Type: teetypes.TwitterJob,
				Arguments: map[string]interface{}{
					"type":        "searchbyquery",
					"query":       "nasa",
					"max_results": 10,
				},
				Timeout: 10 * time.Second,
			}
			res, err := twitterScraper.ExecuteJob(j)
			Expect(err).NotTo(HaveOccurred())
			Expect(res.Error).To(BeEmpty())

			var results []*teetypes.TweetResult
			err = res.Unmarshal(&results)
			Expect(err).NotTo(HaveOccurred())
			Expect(results).ToNot(BeEmpty())

			// Wait briefly for asynchronous stats processing to complete
			time.Sleep(100 * time.Millisecond)

			Expect(results[0].Text).ToNot(BeEmpty())
			Expect(statsCollector.Stats.Stats[j.WorkerID][stats.TwitterScrapes]).To(BeNumerically("==", 1))
			Expect(statsCollector.Stats.Stats[j.WorkerID][stats.TwitterTweets]).To(BeNumerically("==", uint(len(results))))
		})

		It("should scrape a profile", func() {
			if len(twitterAccounts) == 0 {
				Skip("TWITTER_ACCOUNTS is not set")
			}
			j := types.Job{
				Type: teetypes.TwitterCredentialJob,
				Arguments: map[string]interface{}{
					"type":  "searchbyprofile",
					"query": "NASA_Marshall",
				},
				Timeout: 10 * time.Second,
			}
			res, err := twitterScraper.ExecuteJob(j)
			Expect(err).NotTo(HaveOccurred())
			Expect(res.Error).To(BeEmpty())

			var result *twitterscraper.Profile
			err = res.Unmarshal(&result)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).NotTo(BeNil())

			Expect(result.Website).To(ContainSubstring("nasa.gov"))

			// Wait briefly for asynchronous stats processing to complete
			time.Sleep(100 * time.Millisecond)

			Expect(statsCollector.Stats.Stats[j.WorkerID][stats.TwitterScrapes]).To(BeNumerically("==", 1))
			Expect(statsCollector.Stats.Stats[j.WorkerID][stats.TwitterProfiles]).To(BeNumerically("==", 1))
		})

		It("should get tweet by ID", func() {
			res, err := twitterScraper.ExecuteJob(types.Job{
				Type: teetypes.TwitterJob,
				Arguments: map[string]interface{}{
					"type":  "getbyid",
					"query": "1881258110712492142",
				},
				Timeout: 10 * time.Second,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(res.Error).To(BeEmpty())

			var tweet *teetypes.TweetResult
			err = res.Unmarshal(&tweet)
			Expect(err).NotTo(HaveOccurred())
			Expect(tweet).NotTo(BeNil())
			Expect(tweet.TweetID).To(Equal("1881258110712492142")) // Use TweetID field, not ID
			Expect(tweet.Text).NotTo(BeEmpty())
		})

		It("should fetch tweet replies", func() {
			if len(twitterAccounts) == 0 {
				Skip("TWITTER_ACCOUNTS is not set")
			}
			j := types.Job{
				Type: teetypes.TwitterCredentialJob,
				Arguments: map[string]interface{}{
					"type":  "getreplies",
					"query": "1234567890",
				},
				Timeout: 10 * time.Second,
			}
			res, err := twitterScraper.ExecuteJob(j)
			Expect(err).NotTo(HaveOccurred())
			Expect(res.Error).To(BeEmpty())

			var replies []*teetypes.TweetResult
			err = res.Unmarshal(&replies)
			Expect(err).NotTo(HaveOccurred())
			Expect(replies).ToNot(BeEmpty())
			Expect(replies[0].Text).ToNot(BeEmpty())

			// Wait briefly for asynchronous stats processing to complete
			time.Sleep(100 * time.Millisecond)

			Expect(statsCollector.Stats.Stats[j.WorkerID][stats.TwitterScrapes]).To(BeNumerically("==", 1))
			Expect(statsCollector.Stats.Stats[j.WorkerID][stats.TwitterTweets]).To(BeNumerically("==", uint(len(replies))))
		})

		It("should fetch tweet retweeters", func() {
			if len(twitterAccounts) == 0 {
				Skip("TWITTER_ACCOUNTS is not set")
			}
			j := types.Job{
				Type: teetypes.TwitterCredentialJob,
				Arguments: map[string]interface{}{
					"type":        "getretweeters",
					"query":       "1234567890",
					"max_results": 5,
				},
				Timeout: 10 * time.Second,
			}
			res, err := twitterScraper.ExecuteJob(j)
			Expect(err).NotTo(HaveOccurred())
			Expect(res.Error).To(BeEmpty())

			var retweeters []*twitterscraper.Profile
			err = res.Unmarshal(&retweeters)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(retweeters)).ToNot(BeZero())
			Expect(retweeters[0].Username).ToNot(BeEmpty())

			// Wait briefly for asynchronous stats processing to complete
			time.Sleep(100 * time.Millisecond)

			Expect(statsCollector.Stats.Stats[j.WorkerID][stats.TwitterScrapes]).To(BeNumerically("==", 1))
			Expect(statsCollector.Stats.Stats[j.WorkerID][stats.TwitterProfiles]).To(BeNumerically("==", uint(len(retweeters))))
		})

		It("should fetch user tweets", func() {
			if len(twitterAccounts) == 0 {
				Skip("TWITTER_ACCOUNTS is not set")
			}
			j := types.Job{
				Type: teetypes.TwitterCredentialJob,
				Arguments: map[string]interface{}{
					"type":        "gettweets",
					"query":       "NASA",
					"max_results": 5,
				},
				Timeout: 10 * time.Second,
			}
			res, err := twitterScraper.ExecuteJob(j)
			Expect(err).NotTo(HaveOccurred())
			Expect(res.Error).To(BeEmpty())

			var tweets []*teetypes.TweetResult
			err = res.Unmarshal(&tweets)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(tweets)).ToNot(BeZero())
			Expect(tweets[0].Text).ToNot(BeEmpty())

			// Wait briefly for asynchronous stats processing to complete
			time.Sleep(100 * time.Millisecond)

			Expect(statsCollector.Stats.Stats[j.WorkerID][stats.TwitterScrapes]).To(BeNumerically("==", 1))
			Expect(statsCollector.Stats.Stats[j.WorkerID][stats.TwitterTweets]).To(BeNumerically("==", uint(len(tweets))))
		})

		It("should fetch user media", func() {
			if len(twitterAccounts) == 0 {
				Skip("TWITTER_ACCOUNTS is not set")
			}
			res, err := twitterScraper.ExecuteJob(types.Job{
				Type: teetypes.TwitterCredentialJob,
				Arguments: map[string]interface{}{
					"type":        "getmedia",
					"query":       "NASA",
					"max_results": 5,
				},
				Timeout: 10 * time.Second,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(res.Error).To(BeEmpty())

			var media []*teetypes.TweetResult
			err = res.Unmarshal(&media)
			Expect(err).NotTo(HaveOccurred())
			Expect(media).ToNot(BeEmpty())
			Expect(len(media[0].Photos) + len(media[0].Videos)).ToNot(BeZero())
		})

		It("should fetch home tweets", func() {
			if len(twitterAccounts) == 0 {
				Skip("TWITTER_ACCOUNTS is not set")
			}
			j := types.Job{
				Type: teetypes.TwitterCredentialJob,
				Arguments: map[string]interface{}{
					"type":        "gethometweets",
					"max_results": 5,
				},
				Timeout: 10 * time.Second,
			}
			res, err := twitterScraper.ExecuteJob(j)
			Expect(err).NotTo(HaveOccurred())
			Expect(res.Error).To(BeEmpty())

			var tweets []*teetypes.TweetResult
			err = res.Unmarshal(&tweets)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(tweets)).ToNot(BeZero())
			Expect(tweets[0].Text).ToNot(BeEmpty())

			// Wait briefly for asynchronous stats processing to complete
			time.Sleep(100 * time.Millisecond)

			Expect(statsCollector.Stats.Stats[j.WorkerID][stats.TwitterScrapes]).To(BeNumerically("==", 1))
			Expect(statsCollector.Stats.Stats[j.WorkerID][stats.TwitterTweets]).To(BeNumerically("==", uint(len(tweets))))
		})

		It("should fetch for you tweets", func() {
			if len(twitterAccounts) == 0 {
				Skip("TWITTER_ACCOUNTS is not set")
			}
			j := types.Job{
				Type: teetypes.TwitterCredentialJob,
				Arguments: map[string]interface{}{
					"type":        "getforyoutweets",
					"max_results": 5,
				},
				Timeout: 10 * time.Second,
			}
			res, err := twitterScraper.ExecuteJob(j)

			Expect(err).NotTo(HaveOccurred())
			Expect(res.Error).To(BeEmpty())

			var tweets []*teetypes.TweetResult
			err = res.Unmarshal(&tweets)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(tweets)).ToNot(BeZero())
			Expect(tweets).ToNot(BeEmpty())
			Expect(tweets[0].Text).ToNot(BeEmpty())

			// Wait briefly for asynchronous stats processing to complete
			time.Sleep(100 * time.Millisecond)

			Expect(statsCollector.Stats.Stats[j.WorkerID][stats.TwitterScrapes]).To(BeNumerically("==", 1))
			Expect(statsCollector.Stats.Stats[j.WorkerID][stats.TwitterTweets]).To(BeNumerically("==", uint(len(tweets))))
		})

		It("should fetch profile by ID", func() {
			if len(twitterAccounts) == 0 {
				Skip("TWITTER_ACCOUNTS is not set")
			}
			j := types.Job{
				Type: teetypes.TwitterCredentialJob,
				Arguments: map[string]interface{}{
					"type":  "getprofilebyid",
					"query": "44196397", // Elon Musk's Twitter ID
				},
				Timeout: 10 * time.Second,
			}
			res, err := twitterScraper.ExecuteJob(j)
			Expect(err).NotTo(HaveOccurred())
			Expect(res.Error).To(BeEmpty())

			var profile *twitterscraper.Profile
			err = res.Unmarshal(&profile)
			Expect(err).NotTo(HaveOccurred())
			Expect(profile.Username).To(Equal("elonmusk"))

			// Wait briefly for asynchronous stats processing to complete
			time.Sleep(100 * time.Millisecond)

			Expect(statsCollector.Stats.Stats[j.WorkerID][stats.TwitterScrapes]).To(BeNumerically("==", 1))
			Expect(statsCollector.Stats.Stats[j.WorkerID][stats.TwitterProfiles]).To(BeNumerically("==", 1))
		})

		It("should fetch following", func() {
			if len(twitterAccounts) == 0 {
				Skip("TWITTER_ACCOUNTS is not set")
			}
			j := types.Job{
				Type: teetypes.TwitterCredentialJob,
				Arguments: map[string]interface{}{
					"type":        "getfollowing",
					"query":       "NASA",
					"max_results": 5,
				},
				Timeout: 10 * time.Second,
			}
			res, err := twitterScraper.ExecuteJob(j)
			Expect(err).NotTo(HaveOccurred())
			Expect(res.Error).To(BeEmpty())

			var following []*twitterscraper.Profile
			err = res.Unmarshal(&following)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(following)).ToNot(BeZero())
			Expect(following[0].Username).ToNot(BeEmpty())

			// Wait briefly for asynchronous stats processing to complete
			time.Sleep(100 * time.Millisecond)

			Expect(statsCollector.Stats.Stats[j.WorkerID][stats.TwitterScrapes]).To(BeNumerically("==", 1))
			Expect(statsCollector.Stats.Stats[j.WorkerID][stats.TwitterProfiles]).To(BeNumerically("==", uint(len(following))))
		})

		It("should scrape followers from a profile", func() {
			if len(twitterAccounts) == 0 {
				Skip("TWITTER_ACCOUNTS is not set")
			}
			j := types.Job{
				Type: teetypes.TwitterCredentialJob,
				Arguments: map[string]interface{}{
					"type":  "getfollowers",
					"query": "NASA",
				},
				Timeout: 10 * time.Second,
			}
			res, err := twitterScraper.ExecuteJob(j)
			Expect(err).NotTo(HaveOccurred())
			Expect(res.Error).To(BeEmpty())

			var results []*twitterscraper.Profile
			err = res.Unmarshal(&results)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(results)).ToNot(BeZero())
			Expect(results[0].Username).ToNot(BeEmpty())

			// Wait briefly for asynchronous stats processing to complete
			time.Sleep(100 * time.Millisecond)

			// Cannot predetermine the amount of scrapes needed to get followers
			// Expect(statsCollector.Stats.Stats[j.WorkerID][stats.TwitterScrapes]).To(BeNumerically("==", 1))
			Expect(statsCollector.Stats.Stats[j.WorkerID][stats.TwitterProfiles]).To(BeNumerically("==", uint(len(results))))
		})

		It("should get trends", func() {
			if len(twitterAccounts) == 0 {
				Skip("TWITTER_ACCOUNTS is not set")
			}
			j := types.Job{
				Type: teetypes.TwitterCredentialJob,
				Arguments: map[string]interface{}{
					"type": "gettrends",
				},
				Timeout: 10 * time.Second,
			}
			res, err := twitterScraper.ExecuteJob(j)
			Expect(err).NotTo(HaveOccurred())
			Expect(res.Error).To(BeEmpty())

			var result json.RawMessage
			err = res.Unmarshal(&result)

			Expect(err).NotTo(HaveOccurred())
			Expect(result).ToNot(BeEmpty())
			Expect(len(result)).ToNot(BeZero())
			fmt.Println(string(result))
		})

		It("should use API key for twitter-api with getbyid", func() {
			if len(twitterApiKeys) == 0 {
				Skip("TWITTER_API_KEYS is not set")
			}
			scraper := NewTwitterScraper(types.JobConfiguration{
				"twitter_api_keys": twitterApiKeys,
				"data_dir":         tempDir,
			}, statsCollector)
			res, err := scraper.ExecuteJob(types.Job{
				Type: teetypes.TwitterApiJob,
				Arguments: map[string]interface{}{
					"type":  "getbyid",
					"query": "1881258110712492142",
				},
				Timeout: 10 * time.Second,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(res.Error).To(BeEmpty())

			// Use the proper TweetResult type (the API converts TwitterXTweetData to TweetResult)
			var tweet *teetypes.TweetResult
			err = res.Unmarshal(&tweet)
			Expect(err).NotTo(HaveOccurred())
			Expect(tweet).NotTo(BeNil())

			// Now we have structured access to all tweet data
			fmt.Printf("Tweet: %s (ID: %s)\n", tweet.Text, tweet.TweetID)
			fmt.Printf("Author: %s (ID: %s)\n", tweet.Username, tweet.AuthorID)
			fmt.Printf("Metrics: %d likes, %d retweets, %d replies\n",
				tweet.PublicMetrics.LikeCount,
				tweet.PublicMetrics.RetweetCount,
				tweet.PublicMetrics.ReplyCount)
			fmt.Printf("Created: %s, Language: %s\n", tweet.CreatedAt.Format(time.RFC3339), tweet.Lang)

			// Verify the expected data
			Expect(tweet.TweetID).To(Equal("1881258110712492142"))
			Expect(tweet.Text).NotTo(BeEmpty())
			Expect(tweet.AuthorID).To(Equal("1659764713616441344"))
			Expect(tweet.PublicMetrics.LikeCount).To(BeNumerically(">", 10000)) // Over 10k likes
			Expect(tweet.CreatedAt).NotTo(BeZero())
		})

		It("should use API key for twitter-api with getprofilebyid", func() {
			if len(twitterApiKeys) == 0 {
				Skip("TWITTER_API_KEYS is not set")
			}
			scraper := NewTwitterScraper(types.JobConfiguration{
				"twitter_api_keys": twitterApiKeys,
				"data_dir":         tempDir,
			}, statsCollector)
			res, err := scraper.ExecuteJob(types.Job{
				Type: teetypes.TwitterApiJob,
				Arguments: map[string]interface{}{
					"type":  "getprofilebyid",
					"query": "44196397", // Elon Musk's Twitter ID
				},
				Timeout: 10 * time.Second,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(res.Error).To(BeEmpty())

			// Import the twitterx package for structured types
			var profile *twitterx.TwitterXProfileResponse
			err = res.Unmarshal(&profile)
			Expect(err).NotTo(HaveOccurred())
			Expect(profile).NotTo(BeNil())

			// Now we have structured access to all profile data
			fmt.Printf("Profile: %s (@%s)\n", profile.Data.Name, profile.Data.Username)
			fmt.Printf("Followers: %d, Following: %d\n", profile.Data.PublicMetrics.FollowersCount, profile.Data.PublicMetrics.FollowingCount)
			fmt.Printf("Created: %s, Verified: %t\n", profile.Data.CreatedAt, profile.Data.Verified)

			// Verify the expected data
			Expect(profile.Data.Username).To(Equal("elonmusk"))
			Expect(profile.Data.Name).To(Equal("Elon Musk"))
			Expect(profile.Data.ID).To(Equal("44196397"))
			Expect(profile.Data.PublicMetrics.FollowersCount).To(BeNumerically(">", 200000000)) // Over 200M followers
		})

		It("should fetch space", func() {
			Skip("Needs to be constructed to fetch live spaces first - hard to test with hardcoded IDs")

			res, err := twitterScraper.ExecuteJob(types.Job{
				Type: teetypes.TwitterJob,
				Arguments: map[string]interface{}{
					"type":  "getspace",
					"query": "1YpKkZEWlBaxj",
				},
				Timeout: 10 * time.Second,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(res.Error).To(BeEmpty())

			var space *twitterscraper.Space
			err = res.Unmarshal(&space)
			Expect(err).NotTo(HaveOccurred())
			Expect(space.ID).ToNot(BeEmpty())
		})

		It("should fetch bookmarks", func() {
			Skip("Returns 'job result is empty' even when account has bookmarks")

			j := types.Job{
				Type: teetypes.TwitterJob,
				Arguments: map[string]interface{}{
					"type":        "getbookmarks",
					"max_results": 5,
				},
				Timeout: 10 * time.Second,
			}
			res, err := twitterScraper.ExecuteJob(j)
			Expect(err).NotTo(HaveOccurred())
			Expect(res.Error).To(BeEmpty())

			var bookmarks []*teetypes.TweetResult
			err = res.Unmarshal(&bookmarks)
			Expect(err).NotTo(HaveOccurred())
			Expect(res.Error).To(BeEmpty())

			// Wait briefly for asynchronous stats processing to complete
			time.Sleep(100 * time.Millisecond)

			Expect(statsCollector.Stats.Stats[j.WorkerID][stats.TwitterScrapes]).To(BeNumerically("==", 1))
			Expect(statsCollector.Stats.Stats[j.WorkerID][stats.TwitterTweets]).To(BeNumerically("==", uint(len(bookmarks))))
		})

		It("should scrape tweets with full archive", func() {
			Skip("Needs full archive key in TWITTER_API_KEYS to run")

			j := types.Job{
				Type: teetypes.TwitterApiJob,
				Arguments: map[string]interface{}{
					"type":        "searchbyfullarchive",
					"query":       "AI",
					"max_results": 2,
				},
				Timeout: 10 * time.Second,
			}
			res, err := twitterScraper.ExecuteJob(j)
			Expect(err).NotTo(HaveOccurred())
			Expect(res.Error).To(BeEmpty())

			var results []*teetypes.TweetResult
			err = res.Unmarshal(&results)
			Expect(err).NotTo(HaveOccurred())
			Expect(results).ToNot(BeEmpty())

			// Wait briefly for asynchronous stats processing to complete
			time.Sleep(100 * time.Millisecond)

			Expect(results[0].Text).ToNot(BeEmpty())
			Expect(statsCollector.Stats.Stats[j.WorkerID][stats.TwitterScrapes]).To(BeNumerically("==", 1))
			Expect(statsCollector.Stats.Stats[j.WorkerID][stats.TwitterTweets]).To(BeNumerically("==", uint(len(results))))
		})

		It("should scrape tweets with a search by full archive", func() {
			Skip("Needs full archive key (elevated) in TWITTER_API_KEYS to run")

			j := types.Job{
				Type: teetypes.TwitterCredentialJob,
				Arguments: map[string]interface{}{
					"type":        "searchbyfullarchive",
					"query":       "#AI",
					"max_results": 2,
				},
				Timeout: 10 * time.Second,
			}
			res, err := twitterScraper.ExecuteJob(j)
			Expect(err).NotTo(HaveOccurred())
			Expect(res.Error).To(BeEmpty())

			var results []*teetypes.TweetResult
			err = res.Unmarshal(&results)
			Expect(err).NotTo(HaveOccurred())
			Expect(results).ToNot(BeEmpty())

			// Wait briefly for asynchronous stats processing to complete
			time.Sleep(100 * time.Millisecond)

			Expect(results[0].Text).ToNot(BeEmpty())
			Expect(statsCollector.Stats.Stats[j.WorkerID][stats.TwitterScrapes]).To(BeNumerically("==", 1))
			Expect(statsCollector.Stats.Stats[j.WorkerID][stats.TwitterTweets]).To(BeNumerically("==", uint(len(results))))
		})
	})
})
