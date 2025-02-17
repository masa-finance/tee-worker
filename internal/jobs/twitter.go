package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/masa-finance/tee-worker/internal/jobs/twitterx"
	"github.com/masa-finance/tee-worker/pkg/client"
	"strings"

	twitterscraper "github.com/imperatrona/twitter-scraper"
	"github.com/masa-finance/tee-worker/api/types"
	"github.com/masa-finance/tee-worker/internal/jobs/stats"
	"github.com/masa-finance/tee-worker/internal/jobs/twitter"

	"github.com/sirupsen/logrus"
)

type TweetResult struct {
	Tweet        *twitterscraper.Tweet
	ThreadCursor *twitterscraper.ThreadCursor
	Error        error
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

func parseApiKeys(apiKeys []string) []*twitter.TwitterApiKey {
	return filterMap(apiKeys, func(key string) (*twitter.TwitterApiKey, bool) {
		return &twitter.TwitterApiKey{
			Key: strings.TrimSpace(key),
		}, true
	})
}

func (ts *TwitterScraper) getAuthenticatedScraper(baseDir string) (*twitter.Scraper, *twitter.TwitterAccount, *twitter.TwitterApiKey, error) {
	// if baseDir is empty, use the default data directory
	if baseDir == "" {
		baseDir = ts.configuration.DataDir
	}

	account := ts.accountManager.GetNextAccount()
	apiKey := ts.accountManager.GetNextApiKey()
	if account == nil && apiKey == nil {
		ts.statsCollector.Add(stats.TwitterAuthErrors, 1)
		return nil, nil, nil, fmt.Errorf("no accounts or API keys available")
	}

	var scraper *twitter.Scraper
	if account != nil {

		authConfig := twitter.AuthConfig{
			Account: account,
			BaseDir: baseDir,
		}

		scraper = twitter.NewScraper(authConfig)
		if scraper == nil {
			ts.statsCollector.Add(stats.TwitterAuthErrors, 1)
			logrus.Errorf("Authentication failed for %s", account.Username)
			return nil, account, nil, fmt.Errorf("twitter authentication failed for %s", account.Username)
		}
	}

	return scraper, account, apiKey, nil
}

// handleError handles Twitter API errors, detecting rate limits and marking accounts as rate-limited if necessary
// It returns true if the account is rate-limited, false otherwise
func (ts *TwitterScraper) handleError(err error, account *twitter.TwitterAccount) bool {
	if strings.Contains(err.Error(), "Rate limit exceeded") {
		ts.statsCollector.Add(stats.TwitterRateErrors, 1)
		ts.accountManager.MarkAccountRateLimited(account)
		logrus.Warnf("rate limited: %s", account.Username)
		return true
	}

	ts.statsCollector.Add(stats.TwitterErrors, 1)
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
	scraper, account, _, err := ts.getAuthenticatedScraper(baseDir)
	if err != nil {
		return nil, err
	}

	ts.statsCollector.Add(stats.TwitterScrapes, 1)
	followingResponse, errString, _ := scraper.FetchFollowers(username, count, "")
	if errString != "" {
		err := fmt.Errorf("rate limited: %s", errString)
		if ts.handleError(err, account) {
			return nil, err
		}

		logrus.Errorf("[-] Error fetching followers: %s", errString)
		return nil, fmt.Errorf("error fetching followers: %s", errString)
	}

	ts.statsCollector.Add(stats.TwitterProfiles, uint(len(followingResponse)))
	return followingResponse, nil
}

func (ts *TwitterScraper) ScrapeTweetsProfile(baseDir string, username string) (twitterscraper.Profile, error) {
	scraper, account, _, err := ts.getAuthenticatedScraper(baseDir)
	if err != nil {
		return twitterscraper.Profile{}, err
	}

	ts.statsCollector.Add(stats.TwitterScrapes, 1)
	profile, err := scraper.GetProfile(username)
	if err != nil {
		_ = ts.handleError(err, account)
		return twitterscraper.Profile{}, err
	}

	ts.statsCollector.Add(stats.TwitterProfiles, 1)
	return profile, nil
}

func (ts *TwitterScraper) ScrapeTweetsByQuery(baseDir string, query string, count int) ([]*TweetResult, error) {
	scraper, account, apiKey, err := ts.getAuthenticatedScraper(baseDir)
	if err != nil {
		return nil, err
	}

	ts.statsCollector.Add(stats.TwitterScrapes, 1)
	var tweets []*TweetResult

	// Check if we have a TwitterX API key
	if apiKey != nil {

		client := client.NewTwitterXClient(apiKey.Key)
		twitterXScraper := twitterx.NewTwitterXScraper(client)
		result, err := twitterXScraper.ScrapeTweetsByQuery(query)
		if err != nil {
			return nil, err
		}

		for _, tweet := range result.Data {
			var newTweet twitterscraper.Tweet
			newTweet.ID = tweet.ID
			newTweet.Text = tweet.Text
			tweets = append(tweets, &TweetResult{Tweet: &newTweet})
		}

		ts.statsCollector.Add(stats.TwitterTweets, uint(len(result.Data)))

		return tweets, nil

	}

	// Use the default scraper if no TwitterX API key is available
	ctx := context.Background()
	scraper.SetSearchMode(twitterscraper.SearchLatest)

	for tweet := range scraper.SearchTweets(ctx, query, count) {
		if tweet.Error != nil {
			_ = ts.handleError(tweet.Error, account)
			return nil, tweet.Error
		}
		tweets = append(tweets, &TweetResult{Tweet: &tweet.Tweet})
	}

	ts.statsCollector.Add(stats.TwitterTweets, uint(len(tweets)))
	logrus.Info("Scraped tweets: ", len(tweets))
	return tweets, nil
}

func (ts *TwitterScraper) ScrapeTweetByID(baseDir string, tweetID string) (*twitterscraper.Tweet, error) {
	ts.statsCollector.Add(stats.TwitterScrapes, 1)

	scraper, account, _, err := ts.getAuthenticatedScraper(baseDir)
	if err != nil {
		return nil, err
	}

	tweet, err := scraper.GetTweet(tweetID)
	if err != nil {
		_ = ts.handleError(err, account)
		return nil, err
	}

	ts.statsCollector.Add(stats.TwitterTweets, 1)
	return tweet, nil
}

// End of adapted code from masa-oracle (commit: bf277c646d44c49cc387bc5219c900e96b06dc02)

// GetTweet retrieves a tweet by ID
func (ts *TwitterScraper) GetTweet(baseDir, tweetID string) (*TweetResult, error) {
	scraper, account, _, err := ts.getAuthenticatedScraper(baseDir)
	if err != nil {
		return nil, err
	}

	ts.statsCollector.Add(stats.TwitterScrapes, 1)
	tweet, err := scraper.GetTweet(tweetID)
	if err != nil {
		_ = ts.handleError(err, account)
		return nil, err
	}

	ts.statsCollector.Add(stats.TwitterTweets, 1)
	return &TweetResult{Tweet: tweet}, nil
}

// GetTweetReplies retrieves replies to a tweet
func (ts *TwitterScraper) GetTweetReplies(baseDir, tweetID string, cursor string) ([]*TweetResult, error) {
	scraper, account, _, err := ts.getAuthenticatedScraper(baseDir)
	if err != nil {
		return nil, err
	}

	ts.statsCollector.Add(stats.TwitterScrapes, 1)
	var replies []*TweetResult
	tweets, threadCursor, err := scraper.GetTweetReplies(tweetID, cursor)

	for i, tweet := range tweets {
		if err != nil {
			_ = ts.handleError(err, account)
			return nil, err
		}
		replies = append(replies, &TweetResult{Tweet: tweet, ThreadCursor: threadCursor[i], Error: err})
	}

	ts.statsCollector.Add(stats.TwitterTweets, uint(len(replies)))
	return replies, nil
}

// GetTweetRetweeters retrieves users who retweeted a tweet
func (ts *TwitterScraper) GetTweetRetweeters(baseDir, tweetID string, count int, cursor string) ([]*twitterscraper.Profile, error) {
	scraper, account, _, err := ts.getAuthenticatedScraper(ts.configuration.DataDir)
	if err != nil {
		return nil, err
	}

	ts.statsCollector.Add(stats.TwitterScrapes, 1)
	retweeters, _, err := scraper.GetTweetRetweeters(tweetID, count, cursor)
	if err != nil {
		_ = ts.handleError(err, account)
		return nil, err
	}

	ts.statsCollector.Add(stats.TwitterProfiles, uint(len(retweeters)))
	return retweeters, nil
}

// GetUserTweets retrieves tweets from a user
func (ts *TwitterScraper) GetUserTweets(baseDir, username string, count int, cursor string) ([]*TweetResult, string, error) {
	scraper, account, _, err := ts.getAuthenticatedScraper(baseDir)
	if err != nil {
		return nil, "", err
	}

	ts.statsCollector.Add(stats.TwitterScrapes, 1)

	var tweets []*TweetResult
	var nextCursor string

	if cursor != "" {
		// Use fetch method with cursor
		fetchedTweets, fetchCursor, err := scraper.FetchTweets(username, count, cursor)
		if err != nil {
			_ = ts.handleError(err, account)
			return nil, "", err
		}

		for _, tweet := range fetchedTweets {
			tweets = append(tweets, &TweetResult{Tweet: tweet})
		}
		nextCursor = fetchCursor
	} else {
		// Use streaming method without cursor
		for tweet := range scraper.GetTweets(context.Background(), username, count) {
			if tweet.Error != nil {
				_ = ts.handleError(tweet.Error, account)
				return nil, "", tweet.Error
			}
			tweets = append(tweets, &TweetResult{Tweet: &tweet.Tweet})
		}

		// Set next cursor to last tweet's ID if available
		if len(tweets) > 0 {
			nextCursor = tweets[len(tweets)-1].Tweet.ID
		}
	}

	ts.statsCollector.Add(stats.TwitterTweets, uint(len(tweets)))
	return tweets, nextCursor, nil
}

func (ts *TwitterScraper) GetUserMedia(baseDir, username string, count int, cursor string) ([]*TweetResult, string, error) {
	scraper, account, _, err := ts.getAuthenticatedScraper(baseDir)
	if err != nil {
		return nil, "", err
	}

	ts.statsCollector.Add(stats.TwitterScrapes, 1)

	var media []*TweetResult
	var nextCursor string

	if cursor != "" {
		// Use fetch method with cursor
		fetchedTweets, fetchCursor, err := scraper.FetchTweetsAndReplies(username, count, cursor)
		if err != nil {
			_ = ts.handleError(err, account)
			return nil, "", err
		}

		for _, tweet := range fetchedTweets {
			if len(tweet.Photos) > 0 || len(tweet.Videos) > 0 {
				media = append(media, &TweetResult{Tweet: tweet})
			}
		}
		nextCursor = fetchCursor
	} else {
		// Use streaming method without cursor
		for tweet := range scraper.GetTweetsAndReplies(context.Background(), username, count) {
			if tweet.Error != nil {
				if ts.handleError(tweet.Error, account) {
					return nil, "", tweet.Error
				}
				continue
			}
			if len(tweet.Tweet.Photos) > 0 || len(tweet.Tweet.Videos) > 0 {
				media = append(media, &TweetResult{Tweet: &tweet.Tweet})
			}
		}

		// Set next cursor to last tweet's ID if available
		if len(media) > 0 {
			nextCursor = media[len(media)-1].Tweet.ID
		}
	}

	ts.statsCollector.Add(stats.TwitterOther, uint(len(media)))
	return media, nextCursor, nil
}

func (ts *TwitterScraper) GetHomeTweets(baseDir string, count int, cursor string) ([]*TweetResult, string, error) {
	scraper, account, _, err := ts.getAuthenticatedScraper(baseDir)
	if err != nil {
		return nil, "", err
	}

	ts.statsCollector.Add(stats.TwitterScrapes, 1)

	var tweets []*TweetResult
	var nextCursor string

	if cursor != "" {
		// Use fetch method with cursor
		fetchedTweets, fetchCursor, err := scraper.FetchHomeTweets(count, cursor)
		if err != nil {
			_ = ts.handleError(err, account)
			return nil, "", err
		}

		for _, tweet := range fetchedTweets {
			tweets = append(tweets, &TweetResult{Tweet: tweet})
		}
		nextCursor = fetchCursor
	} else {
		// Use streaming method without cursor
		for tweet := range scraper.GetHomeTweets(context.Background(), count) {
			if tweet.Error != nil {
				_ = ts.handleError(tweet.Error, account)
				return nil, "", tweet.Error
			}
			tweets = append(tweets, &TweetResult{Tweet: &tweet.Tweet})
			if len(tweets) >= count {
				break
			}
		}

		// Set next cursor to last tweet's ID if available
		if len(tweets) > 0 {
			nextCursor = tweets[len(tweets)-1].Tweet.ID
		}
	}

	ts.statsCollector.Add(stats.TwitterTweets, uint(len(tweets)))
	return tweets, nextCursor, nil
}

func (ts *TwitterScraper) GetForYouTweets(baseDir string, count int, cursor string) ([]*TweetResult, string, error) {
	scraper, account, _, err := ts.getAuthenticatedScraper(baseDir)
	if err != nil {
		return nil, "", err
	}

	ts.statsCollector.Add(stats.TwitterScrapes, 1)

	var tweets []*TweetResult
	var nextCursor string

	if cursor != "" {
		// Use fetch method with cursor
		fetchedTweets, fetchCursor, err := scraper.FetchForYouTweets(count, cursor)
		if err != nil {
			_ = ts.handleError(err, account)
			return nil, "", err
		}

		for _, tweet := range fetchedTweets {
			tweets = append(tweets, &TweetResult{Tweet: tweet})
		}
		nextCursor = fetchCursor
	} else {
		// Use streaming method without cursor
		for tweet := range scraper.GetForYouTweets(context.Background(), count) {
			if tweet.Error != nil {
				_ = ts.handleError(tweet.Error, account)
				return nil, "", tweet.Error
			}
			tweets = append(tweets, &TweetResult{Tweet: &tweet.Tweet})
			if len(tweets) >= count {
				break
			}
		}

		// Set next cursor to last tweet's ID if available
		if len(tweets) > 0 {
			nextCursor = tweets[len(tweets)-1].Tweet.ID
		}
	}

	ts.statsCollector.Add(stats.TwitterTweets, uint(len(tweets)))
	return tweets, nextCursor, nil
}

func (ts *TwitterScraper) GetBookmarks(baseDir string, count int, cursor string) ([]*TweetResult, string, error) {
	scraper, account, _, err := ts.getAuthenticatedScraper(baseDir)
	if err != nil {
		return nil, "", err
	}

	ts.statsCollector.Add(stats.TwitterScrapes, 1)

	var bookmarks []*TweetResult
	var nextCursor string

	if cursor != "" {
		// Use fetch method with cursor
		fetchedTweets, fetchCursor, err := scraper.FetchBookmarks(count, cursor)
		if err != nil {
			_ = ts.handleError(err, account)
			return nil, "", err
		}

		for _, tweet := range fetchedTweets {
			bookmarks = append(bookmarks, &TweetResult{Tweet: tweet})
		}
		nextCursor = fetchCursor
	} else {
		// Use streaming method without cursor
		for tweet := range scraper.GetBookmarks(context.Background(), count) {
			if tweet.Error != nil {
				_ = ts.handleError(tweet.Error, account)
				return nil, "", tweet.Error
			}
			bookmarks = append(bookmarks, &TweetResult{Tweet: &tweet.Tweet})
			if len(bookmarks) >= count {
				break
			}
		}

		// Set next cursor to last tweet's ID if available
		if len(bookmarks) > 0 {
			nextCursor = bookmarks[len(bookmarks)-1].Tweet.ID
		}
	}

	ts.statsCollector.Add(stats.TwitterTweets, uint(len(bookmarks)))
	return bookmarks, nextCursor, nil
}

// FetchHomeTweets retrieves tweets from user's home timeline
func (ts *TwitterScraper) FetchHomeTweets(baseDir string, count int, cursor string) ([]*twitterscraper.Tweet, string, error) {
	scraper, account, _, err := ts.getAuthenticatedScraper(baseDir)
	if err != nil {
		return nil, "", err
	}

	ts.statsCollector.Add(stats.TwitterScrapes, 1)
	tweets, nextCursor, err := scraper.FetchHomeTweets(count, cursor)
	if err != nil {
		_ = ts.handleError(err, account)
		return nil, "", err
	}

	ts.statsCollector.Add(stats.TwitterTweets, uint(len(tweets)))
	return tweets, nextCursor, nil
}

// FetchForYouTweets retrieves tweets from For You timeline
func (ts *TwitterScraper) FetchForYouTweets(baseDir string, count int, cursor string) ([]*twitterscraper.Tweet, string, error) {
	scraper, account, _, err := ts.getAuthenticatedScraper(baseDir)
	if err != nil {
		return nil, "", err
	}

	ts.statsCollector.Add(stats.TwitterScrapes, 1)
	tweets, nextCursor, err := scraper.FetchForYouTweets(count, cursor)
	if err != nil {
		_ = ts.handleError(err, account)
		return nil, "", err
	}

	ts.statsCollector.Add(stats.TwitterTweets, uint(len(tweets)))
	return tweets, nextCursor, nil
}

// GetProfileByID retrieves a user profile by ID
func (ts *TwitterScraper) GetProfileByID(baseDir, userID string) (*twitterscraper.Profile, error) {
	scraper, account, _, err := ts.getAuthenticatedScraper(baseDir)
	if err != nil {
		return nil, err
	}

	ts.statsCollector.Add(stats.TwitterScrapes, 1)
	profile, err := scraper.GetProfileByID(userID)
	if err != nil {
		_ = ts.handleError(err, account)
		return nil, err
	}

	ts.statsCollector.Add(stats.TwitterProfiles, 1)
	return &profile, nil
}

// SearchProfile searches for user profiles
func (ts *TwitterScraper) SearchProfile(query string, count int) ([]*twitterscraper.ProfileResult, error) {
	scraper, _, _, err := ts.getAuthenticatedScraper(ts.configuration.DataDir)
	if err != nil {
		return nil, err
	}

	ts.statsCollector.Add(stats.TwitterScrapes, 1)
	var profiles []*twitterscraper.ProfileResult
	for profile := range scraper.SearchProfiles(context.Background(), query, count) {
		if len(profiles) >= count {
			break
		}

		profiles = append(profiles, profile)
	}

	ts.statsCollector.Add(stats.TwitterProfiles, uint(len(profiles)))
	return profiles, nil
}

// GetTrends retrieves current trending topics
func (ts *TwitterScraper) GetTrends(baseDir string) ([]string, error) {
	scraper, account, _, err := ts.getAuthenticatedScraper(baseDir)
	if err != nil {
		return nil, err
	}

	ts.statsCollector.Add(stats.TwitterScrapes, 1)
	trends, err := scraper.GetTrends()
	if err != nil {
		_ = ts.handleError(err, account)
		return nil, err
	}

	// TODO: Should this be the number of topics, or 1 ?
	ts.statsCollector.Add(stats.TwitterOther, uint(len(trends)))
	return trends, nil
}

// GetFollowers retrieves users that follow a user
func (ts *TwitterScraper) GetFollowers(baseDir, user string, count int, cursor string) ([]*twitterscraper.Profile, string, error) {
	scraper, account, _, err := ts.getAuthenticatedScraper(baseDir)
	if err != nil {
		return nil, "", err
	}

	ts.statsCollector.Add(stats.TwitterScrapes, 1)
	followers, nextCursor, err := scraper.FetchFollowers(user, count, cursor)
	if err != nil {
		_ = ts.handleError(err, account)
		return nil, "", err
	}

	ts.statsCollector.Add(stats.TwitterProfiles, uint(len(followers)))
	return followers, nextCursor, nil
}

// GetFollowing retrieves users that a user follows
func (ts *TwitterScraper) GetFollowing(baseDir, username string, count int) ([]*twitterscraper.Profile, error) {
	// get the authenticated scraper
	scraper, account, _, err := ts.getAuthenticatedScraper(baseDir)
	if err != nil {
		return nil, err
	}

	ts.statsCollector.Add(stats.TwitterScrapes, 1)
	following, errString, _ := scraper.FetchFollowing(username, count, "")
	if errString != "" {
		err := fmt.Errorf("error fetching following: %s", errString)
		_ = ts.handleError(err, account)
		return nil, err
	}

	ts.statsCollector.Add(stats.TwitterProfiles, uint(len(following)))
	return following, nil
}

// GetSpace retrieves space information by ID
func (ts *TwitterScraper) GetSpace(baseDir, spaceID string) (*twitterscraper.Space, error) {
	// get the authenticated scraper
	scraper, account, _, err := ts.getAuthenticatedScraper(baseDir)
	if err != nil {
		return nil, err
	}

	ts.statsCollector.Add(stats.TwitterScrapes, 1)
	space, err := scraper.GetSpace(spaceID)
	if err != nil {
		_ = ts.handleError(err, account)
		return nil, err
	}

	ts.statsCollector.Add(stats.TwitterOther, 1)
	return space, nil
}

const TwitterScraperType = "twitter-scraper"

type TwitterScraper struct {
	configuration  TwitterScraperConfiguration
	accountManager *twitter.TwitterAccountManager
	statsCollector *stats.StatsCollector
}

type TwitterScraperConfiguration struct {
	Accounts []string `json:"twitter_accounts"`
	ApiKeys  []string `json:"twitter_api_keys"`
	DataDir  string   `json:"data_dir"`
}

type TwitterScraperArgs struct {
	SearchType string `json:"type"`
	Query      string `json:"query"`
	Count      int    `json:"count"`
	NextCursor string `json:"next_cursor"`
}

func NewTwitterScraper(jc types.JobConfiguration, c *stats.StatsCollector) *TwitterScraper {
	config := TwitterScraperConfiguration{}
	jc.Unmarshal(&config)

	accounts := parseAccounts(config.Accounts)
	apiKeys := parseApiKeys(config.ApiKeys)
	accountManager := twitter.NewTwitterAccountManager(accounts, apiKeys)

	return &TwitterScraper{
		configuration:  config,
		accountManager: accountManager,
		statsCollector: c,
	}
}

func (ws *TwitterScraper) ExecuteJob(j types.Job) (types.JobResult, error) {
	args := &TwitterScraperArgs{}
	j.Arguments.Unmarshal(args)

	switch strings.ToLower(args.SearchType) {
	case "searchbyquery":
		tweets, err := ws.ScrapeTweetsByQuery(ws.configuration.DataDir, args.Query, args.Count)
		if err != nil {
			return types.JobResult{Error: err.Error()}, err
		}
		dat, err := json.Marshal(tweets)
		return types.JobResult{
			Data: dat,
		}, err

	case "searchbyprofile":
		profile, err := ws.ScrapeTweetsProfile(ws.configuration.DataDir, args.Query)
		if err != nil {
			return types.JobResult{Error: err.Error()}, err
		}
		dat, err := json.Marshal(profile)
		return types.JobResult{
			Data: dat,
		}, err

	case "searchfollowers":
		followers, err := ws.ScrapeFollowersForProfile(ws.configuration.DataDir, args.Query, args.Count)
		if err != nil {
			return types.JobResult{Error: err.Error()}, err
		}
		dat, err := json.Marshal(followers)
		return types.JobResult{
			Data: dat,
		}, err

	case "getbyid":
		tweet, err := ws.ScrapeTweetByID(ws.configuration.DataDir, args.Query)
		if err != nil {
			return types.JobResult{Error: err.Error()}, err
		}
		dat, err := json.Marshal(tweet)
		return types.JobResult{
			Data: dat,
		}, err

	case "getreplies":
		replies, err := ws.GetTweetReplies(ws.configuration.DataDir, args.Query, "")
		if err != nil {
			return types.JobResult{Error: err.Error()}, err
		}
		dat, err := json.Marshal(replies)
		return types.JobResult{
			Data: dat,
		}, err

	case "getretweeters":
		retweeters, err := ws.GetTweetRetweeters(ws.configuration.DataDir, args.Query, args.Count, "")
		if err != nil {
			return types.JobResult{Error: err.Error()}, err
		}
		dat, err := json.Marshal(retweeters)
		return types.JobResult{
			Data: dat,
		}, err

	case "gettweets":
		tweets, nextCursor, err := ws.GetUserTweets(ws.configuration.DataDir, args.Query, args.Count, args.NextCursor)
		if err != nil {
			return types.JobResult{Error: err.Error()}, err
		}
		dat, err := json.Marshal(tweets)
		return types.JobResult{
			Data:       dat,
			NextCursor: nextCursor,
		}, err

	case "getmedia":
		media, nextCursor, err := ws.GetUserMedia(ws.configuration.DataDir, args.Query, args.Count, args.NextCursor)
		if err != nil {
			return types.JobResult{Error: err.Error()}, err
		}
		dat, err := json.Marshal(media)
		return types.JobResult{
			Data:       dat,
			NextCursor: nextCursor,
		}, err

	case "gethometweets":
		tweets, nextCursor, err := ws.GetHomeTweets(ws.configuration.DataDir, args.Count, args.NextCursor)
		if err != nil {
			return types.JobResult{Error: err.Error()}, err
		}
		dat, err := json.Marshal(tweets)
		return types.JobResult{
			Data:       dat,
			NextCursor: nextCursor,
		}, err

	case "getforyoutweets":
		tweets, nextCursor, err := ws.GetForYouTweets(ws.configuration.DataDir, args.Count, args.NextCursor)
		if err != nil {
			return types.JobResult{Error: err.Error()}, err
		}
		dat, err := json.Marshal(tweets)
		return types.JobResult{
			Data:       dat,
			NextCursor: nextCursor,
		}, err

	case "getbookmarks":
		bookmarks, nextCursor, err := ws.GetBookmarks(ws.configuration.DataDir, args.Count, args.NextCursor)
		if err != nil {
			return types.JobResult{Error: err.Error()}, err
		}
		dat, err := json.Marshal(bookmarks)
		return types.JobResult{
			Data:       dat,
			NextCursor: nextCursor,
		}, err

	case "getprofilebyid":
		profile, err := ws.GetProfileByID(ws.configuration.DataDir, args.Query)
		if err != nil {
			return types.JobResult{Error: err.Error()}, err
		}
		dat, err := json.Marshal(profile)
		return types.JobResult{
			Data: dat,
		}, err

	case "gettrends":
		trends, err := ws.GetTrends(ws.configuration.DataDir)
		if err != nil {
			return types.JobResult{Error: err.Error()}, err
		}
		dat, err := json.Marshal(trends)
		return types.JobResult{
			Data: dat,
		}, err

	case "getfollowing":
		following, err := ws.GetFollowing(ws.configuration.DataDir, args.Query, args.Count)
		if err != nil {
			return types.JobResult{Error: err.Error()}, err
		}
		dat, err := json.Marshal(following)
		return types.JobResult{
			Data: dat,
		}, err

	case "getfollowers":
		followers, nextCursor, err := ws.GetFollowers(ws.configuration.DataDir, args.Query, args.Count, "")
		if err != nil {
			return types.JobResult{Error: err.Error()}, err

		}
		dat, err := json.Marshal(followers)
		return types.JobResult{
			Data:       dat,
			NextCursor: nextCursor,
		}, err

	case "getspace":
		space, err := ws.GetSpace(ws.configuration.DataDir, args.Query)
		if err != nil {
			return types.JobResult{Error: err.Error()}, err
		}
		dat, err := json.Marshal(space)
		return types.JobResult{
			Data: dat,
		}, err

	}

	return types.JobResult{
		Error: "invalid search type",
	}, fmt.Errorf("invalid search type")
}
