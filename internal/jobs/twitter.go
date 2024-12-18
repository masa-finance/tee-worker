package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	twitterscraper "github.com/imperatrona/twitter-scraper"

	"github.com/masa-finance/tee-worker/api/types"
	"github.com/masa-finance/tee-worker/internal/jobs/twitter"

	"github.com/sirupsen/logrus"
)

type TweetResult struct {
	Tweet *twitterscraper.Tweet
	Error error
}

func parseAccounts(accountPairs []string) []*twitter.TwitterAccount {
	return filterMap(accountPairs, func(pair string) (*twitter.TwitterAccount, bool) {
		credentials := strings.Split(pair, ":")
		if len(credentials) != 2 {
			logrus.Warnf("invalid account credentials: %s", pair)
			return nil, false
		}
		return &twitter.TwitterAccount{
			Username: strings.TrimSpace(credentials[0]),
			Password: strings.TrimSpace(credentials[1]),
		}, true
	})
}

func (ts *TwitterScraper) getAuthenticatedScraper(baseDir string) (*twitter.Scraper, *twitter.TwitterAccount, error) {
	account := ts.accountManager.GetNextAccount()
	if account == nil {
		return nil, nil, fmt.Errorf("all accounts are rate-limited")
	}
	scraper := twitter.NewScraper(account, baseDir)
	if scraper == nil {
		logrus.Errorf("Authentication failed for %s", account.Username)
		return nil, account, fmt.Errorf("twitter authentication failed for %s", account.Username)
	}
	return scraper, account, nil
}

func (ts *TwitterScraper) handleRateLimit(err error, account *twitter.TwitterAccount) bool {
	if strings.Contains(err.Error(), "Rate limit exceeded") {
		ts.accountManager.MarkAccountRateLimited(account)
		logrus.Warnf("rate limited: %s", account.Username)
		return true
	}
	return false
}

func filterMap[T any, R any](slice []T, f func(T) (R, bool)) []R {
	result := make([]R, 0, len(slice))
	for _, v := range slice {
		if r, ok := f(v); ok {
			result = append(result, r)
		}
	}
	return result
}

func (ts *TwitterScraper) ScrapeFollowersForProfile(baseDir string, username string, count int) ([]*twitterscraper.Profile, error) {
	scraper, account, err := ts.getAuthenticatedScraper(baseDir)
	if err != nil {
		return nil, err
	}

	followingResponse, errString, _ := scraper.FetchFollowers(username, count, "")
	if errString != "" {
		err := fmt.Errorf("rate limited: %s", errString)
		if ts.handleRateLimit(err, account) {
			return nil, err
		}

		logrus.Errorf("[-] Error fetching followers: %s", errString)
		return nil, fmt.Errorf("error fetching followers: %s", errString)
	}
	account.LastScraped = time.Now()
	return followingResponse, nil
}

func (ts *TwitterScraper) ScrapeTweetsProfile(baseDir string, username string) (twitterscraper.Profile, error) {
	scraper, account, err := ts.getAuthenticatedScraper(baseDir)
	if err != nil {
		return twitterscraper.Profile{}, err
	}

	profile, err := scraper.GetProfile(username)
	if err != nil {
		if ts.handleRateLimit(err, account) {
			return twitterscraper.Profile{}, err
		}
		return twitterscraper.Profile{}, err
	}
	account.LastScraped = time.Now()
	return profile, nil
}

func (ts *TwitterScraper) ScrapeTweetsByQuery(baseDir string, query string, count int) ([]*TweetResult, error) {
	scraper, account, err := ts.getAuthenticatedScraper(baseDir)
	if err != nil {
		return nil, err
	}

	var tweets []*TweetResult
	ctx := context.Background()
	scraper.SetSearchMode(twitterscraper.SearchLatest)
	for tweet := range scraper.SearchTweets(ctx, query, count) {
		if tweet.Error != nil {
			if ts.handleRateLimit(tweet.Error, account) {
				return nil, tweet.Error
			}
			return nil, tweet.Error
		}
		tweets = append(tweets, &TweetResult{Tweet: &tweet.Tweet})
	}
	account.LastScraped = time.Now()
	return tweets, nil
}

// End of adapted code from masa-oracle (commit: bf277c646d44c49cc387bc5219c900e96b06dc02)

const TwitterScraperType = "twitter-scraper"

type TwitterScraper struct {
	configuration  TwitterScraperConfiguration
	accountManager *twitter.TwitterAccountManager
}

type TwitterScraperConfiguration struct {
	Accounts []string `json:"twitter_accounts"`
	DataDir  string   `json:"data_dir"`
}

type TwitterScraperArgs struct {
	SearchType string `json:"type"`
	Query      string `json:"query"`
	Count      int    `json:"count"`
}

func NewTwitterScraper(jc types.JobConfiguration) *TwitterScraper {
	config := TwitterScraperConfiguration{}
	err := jc.Unmarshal(&config)
	if err != nil {
		logrus.Errorf("error unmarshalling twitter configuration: %v", err)
		return nil
	}

	accounts := parseAccounts(config.Accounts)
	accountManager := twitter.NewTwitterAccountManager(accounts)

	return &TwitterScraper{configuration: config, accountManager: accountManager}
}

func (ts *TwitterScraper) ExecuteJob(j types.Job) (types.JobResult, error) {
	args := &TwitterScraperArgs{}
	err := j.Arguments.Unmarshal(args)
	if err != nil {
		logrus.Errorf("error unmarshalling twitter job arguments: %v", err)
	}

	switch strings.ToLower(args.SearchType) {
	case "searchbyquery":
		tweets, err := ts.ScrapeTweetsByQuery(ts.configuration.DataDir, args.Query, args.Count)
		if err != nil {
			return types.JobResult{Error: err.Error()}, err
		}
		dat, err := json.Marshal(tweets)
		return types.JobResult{
			Data: dat,
		}, err

	case "searchbyprofile":
		profile, err := ts.ScrapeTweetsProfile(ts.configuration.DataDir, args.Query)
		if err != nil {
			return types.JobResult{Error: err.Error()}, err
		}
		dat, err := json.Marshal(profile)
		return types.JobResult{
			Data: dat,
		}, err

	case "searchfollowers":
		followers, err := ts.ScrapeFollowersForProfile(ts.configuration.DataDir, args.Query, args.Count)
		if err != nil {
			return types.JobResult{Error: err.Error()}, err
		}
		dat, err := json.Marshal(followers)
		return types.JobResult{
			Data: dat,
		}, err
	}

	// Do the web scraping here
	// For now, just return the URL
	return types.JobResult{
		Error: "invalid search type",
	}, fmt.Errorf("invalid search type")
}

func (ts *TwitterScraper) Status() string {
	states := ts.accountManager.GetAccountStates()
	jsn, err := json.Marshal(states)
	if err != nil {
		return fmt.Sprintf("error marshalling account states: %v", err)
	}
	return string(jsn)
}
