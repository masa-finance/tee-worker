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

// parseTwitterApiKeys parses TWITTER_API_KEYS environment variable like production does
func parseTwitterApiKeys() []string {
	apiKeysEnv := os.Getenv("TWITTER_API_KEYS")
	if apiKeysEnv == "" {
		return nil
	}

	apiKeys := strings.Split(apiKeysEnv, ",")
	for i, apiKey := range apiKeys {
		apiKeys[i] = strings.TrimSpace(apiKey)
	}
	return apiKeys
}

var _ = Describe("Twitter Scraper", func() {

	// --- New tests for specialized job types ---
	Context("Specialized Twitter Scraper Job Types", func() {
		var statsCollector *stats.StatsCollector
		var tempDir string
		var err error
		var twitterAccounts []string
		var twitterApiKeys []string

		BeforeEach(func() {
			logrus.SetLevel(logrus.DebugLevel)
			os.Setenv("LOG_LEVEL", "debug")

			tempDir = ".masa"
			err = os.MkdirAll(tempDir, 0755)
			Expect(err).NotTo(HaveOccurred())

			twitterAccounts = parseTwitterAccounts()
			twitterApiKeys = parseTwitterApiKeys()
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
				Type: string(teetypes.TwitterCredentialJob),
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

		It("should use API key for twitter-api-scraper", func() {
			if len(twitterApiKeys) == 0 {
				Skip("TWITTER_API_KEYS is not set")
			}
			scraper := NewTwitterScraper(types.JobConfiguration{
				"twitter_api_keys": twitterApiKeys,
				"data_dir":         tempDir,
			}, statsCollector)
			res, err := scraper.ExecuteJob(types.Job{
				Type: string(teetypes.TwitterApiJob),
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
				Type: string(teetypes.TwitterCredentialJob),
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
				Type: string(teetypes.TwitterJob),
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

		It("should error if neither credentials nor API key are present", func() {
			scraper := NewTwitterScraper(types.JobConfiguration{
				"data_dir": tempDir,
			}, statsCollector)
			res, err := scraper.ExecuteJob(types.Job{
				Type: string(teetypes.TwitterApiJob),
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
	})

	var twitterScraper *TwitterScraper
	var statsCollector *stats.StatsCollector
	var tempDir string
	var err error

	BeforeEach(func() {
		logrus.SetLevel(logrus.DebugLevel)
		os.Setenv("LOG_LEVEL", "debug")

		tempDir = ".masa"
		err = os.MkdirAll(tempDir, 0755)
		Expect(err).NotTo(HaveOccurred())

		twitterAccounts := parseTwitterAccounts()
		twitterApiKeys := parseTwitterApiKeys()

		if len(twitterAccounts) == 0 && len(twitterApiKeys) == 0 {
			Skip("TWITTER_ACCOUNTS and TWITTER_API_KEYS not set... not possible to scrape!")
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

	FIt("should scrape tweets with a search query", func() {
		j := types.Job{
			Type: string(teetypes.TwitterJob),
			Arguments: map[string]interface{}{
				"type":        "searchbyquery",
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

	It("should scrape a profile", func() {
		j := types.Job{
			Type: string(teetypes.TwitterJob),
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
			Type: string(teetypes.TwitterJob),
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
		j := types.Job{
			Type: string(teetypes.TwitterJob),
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
		j := types.Job{
			Type: string(teetypes.TwitterJob),
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
		j := types.Job{
			Type: string(teetypes.TwitterJob),
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
		res, err := twitterScraper.ExecuteJob(types.Job{
			Type: string(teetypes.TwitterJob),
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
		j := types.Job{
			Type: string(teetypes.TwitterJob),
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
		j := types.Job{
			Type: string(teetypes.TwitterJob),
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
		j := types.Job{
			Type: string(teetypes.TwitterJob),
			Arguments: map[string]interface{}{
				"type":  "getprofilebyid",
				"query": "44196397", //
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
		j := types.Job{
			Type: string(teetypes.TwitterJob),
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
		j := types.Job{
			Type: string(teetypes.TwitterJob),
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

		// Expect(statsCollector.Stats.Stats[j.WorkerID][stats.TwitterScrapes]).To(BeNumerically("==", 1)) // note, cannot predetermine amount of scrapes are needed to get followers
		Expect(statsCollector.Stats.Stats[j.WorkerID][stats.TwitterProfiles]).To(BeNumerically("==", uint(len(results))))
	})

	It("should get trends", func() {
		j := types.Job{
			Type: string(teetypes.TwitterJob),
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

	// TODO add additional API key tests for sub type capabilities...

	// TODO verify why cookie based auth all the sudden is getting DenyLoginSubtask?

	// note, needs to be constructed to fetch live spaces first... hard to test hardcoded ids
	// It("should fetch space", func() {
	// 	res, err := twitterScraper.ExecuteJob(types.Job{
	// 		Type: string(teetypes.TwitterJob),
	// 		Arguments: map[string]interface{}{
	// 			"type":  "getspace",
	// 			"query": "1YpKkZEWlBaxj",
	// 		},
	// 		Timeout: 10 * time.Second,
	// 	})
	// 	Expect(err).NotTo(HaveOccurred())
	// 	Expect(res.Error).To(BeEmpty())

	// 	var space *twitterscraper.Space
	// 	err = res.Unmarshal(&space)
	// 	Expect(err).NotTo(HaveOccurred())
	// 	Expect(space.ID).ToNot(BeEmpty())
	// })

	// note, returning "job result is empty" even when account has bookmarks
	// It("should fetch bookmarks", func() {
	// 	j := types.Job{
	// 		Type: string(teetypes.TwitterJob),
	// 		Arguments: map[string]interface{}{
	// 			"type":        "getbookmarks",
	// 			"max_results": 5,
	// 		},
	// 		Timeout: 10 * time.Second,
	// 	}
	// 	res, err := twitterScraper.ExecuteJob(j)
	// 	Expect(err).NotTo(HaveOccurred())
	// 	Expect(res.Error).To(BeEmpty())

	// 	var bookmarks []*teetypes.TweetResult
	// 	err = res.Unmarshal(&bookmarks)
	// 	Expect(err).NotTo(HaveOccurred())
	// 	Expect(res.Error).To(BeEmpty())

	// 	// Wait briefly for asynchronous stats processing to complete
	// 	time.Sleep(100 * time.Millisecond)

	// 	Expect(statsCollector.Stats.Stats[j.WorkerID][stats.TwitterScrapes]).To(BeNumerically("==", 1))
	// 	Expect(statsCollector.Stats.Stats[j.WorkerID][stats.TwitterTweets]).To(BeNumerically("==", uint(len(bookmarks))))
	// })

	// note, needs full archive key in TWITTER_API_KEYS to run...
	// It("should scrape tweets with full archive", func() {
	// 	j := types.Job{
	// 		Type: string(teetypes.TwitterApiJob),
	// 		Arguments: map[string]interface{}{
	// 			"type":        "searchbyfullarchive",
	// 			"query":       "AI",
	// 			"max_results": 2,
	// 		},
	// 		Timeout: 10 * time.Second,
	// 	}
	// 	res, err := twitterScraper.ExecuteJob(j)
	// 	Expect(err).NotTo(HaveOccurred())
	// 	Expect(res.Error).To(BeEmpty())

	// 	var results []*teetypes.TweetResult
	// 	err = res.Unmarshal(&results)
	// 	Expect(err).NotTo(HaveOccurred())
	// 	Expect(results).ToNot(BeEmpty())

	// 	// Wait briefly for asynchronous stats processing to complete
	// 	time.Sleep(100 * time.Millisecond)

	// 	Expect(results[0].Text).ToNot(BeEmpty())
	// 	Expect(statsCollector.Stats.Stats[j.WorkerID][stats.TwitterScrapes]).To(BeNumerically("==", 1))
	// 	Expect(statsCollector.Stats.Stats[j.WorkerID][stats.TwitterTweets]).To(BeNumerically("==", uint(len(results))))
	// })

	// note, needs full archive key (elevated) in TWITTER_API_KEYS to run...
	// It("should scrape tweets with a search by full archive", func() {
	// 	j := types.Job{
	// 		Type: string(teetypes.TwitterCredentialJob),
	// 		Arguments: map[string]interface{}{
	// 			"type":        "searchbyfullarchive",
	// 			"query":       "#AI",
	// 			"max_results": 2,
	// 		},
	// 		Timeout: 10 * time.Second,
	// 	}
	// 	res, err := twitterScraper.ExecuteJob(j)
	// 	Expect(err).NotTo(HaveOccurred())
	// 	Expect(res.Error).To(BeEmpty())

	// 	var results []*teetypes.TweetResult
	// 	err = res.Unmarshal(&results)
	// 	Expect(err).NotTo(HaveOccurred())
	// 	Expect(results).ToNot(BeEmpty())

	// 	// Wait briefly for asynchronous stats processing to complete
	// 	time.Sleep(100 * time.Millisecond)

	// 	Expect(results[0].Text).ToNot(BeEmpty())
	// 	Expect(statsCollector.Stats.Stats[j.WorkerID][stats.TwitterScrapes]).To(BeNumerically("==", 1))
	// 	Expect(statsCollector.Stats.Stats[j.WorkerID][stats.TwitterTweets]).To(BeNumerically("==", uint(len(results))))
	// })
})
