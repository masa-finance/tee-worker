package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	twitterscraper "github.com/imperatrona/twitter-scraper"
	"github.com/masa-finance/tee-worker/api/types"
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
	return tweets, nil
}

// End of adapted code from masa-oracle (commit: bf277c646d44c49cc387bc5219c900e96b06dc02)

// GetTweet retrieves a tweet by ID
func (ts *TwitterScraper) GetTweet(baseDir, tweetID string) (*TweetResult, error) {

	// if baseDir is empty, use the default data directory
	if baseDir == "" {
		baseDir = ts.configuration.DataDir
	}
	// get the authenticated scraper
	scraper, account, err := ts.getAuthenticatedScraper(baseDir)
	if err != nil {
		return nil, err
	}

	tweet, err := scraper.GetTweet(tweetID)
	if err != nil {
		if ts.handleRateLimit(err, account) {
			return nil, err
		}
		return nil, err
	}
	return &TweetResult{Tweet: tweet}, nil
}

// GetTweetReplies retrieves replies to a tweet
func (ts *TwitterScraper) GetTweetReplies(baseDir, tweetID string, cursor string) ([]*TweetResult, error) {
	if baseDir == "" {
		baseDir = ts.configuration.DataDir
	}
	// get the authenticated scraper
	scraper, account, err := ts.getAuthenticatedScraper(baseDir)
	if err != nil {
		return nil, err
	}

	// handle rate limit
	if ts.handleRateLimit(err, account) {
		return nil, err
	}

	var replies []*TweetResult
	tweets, threadCursor, err := scraper.GetTweetReplies(tweetID, cursor)
	for i, tweet := range tweets {
		if err != nil {

			replies = append(replies, &TweetResult{Tweet: tweet, ThreadCursor: threadCursor[i], Error: err})
		}
	}
	return replies, nil
}

// GetTweetRetweeters retrieves users who retweeted a tweet
func (ts *TwitterScraper) GetTweetRetweeters(baseDir, tweetID string, count int, cursor string) ([]*twitterscraper.Profile, error) {
	if baseDir == "" {
		baseDir = ts.configuration.DataDir
	}
	// get the authenticated scraper
	scraper, account, err := ts.getAuthenticatedScraper(ts.configuration.DataDir)
	if err != nil {
		return nil, err
	}

	retweeters, nextCursor, err := scraper.GetTweetRetweeters(tweetID, count, cursor)
	fmt.Sprintf("Next cursor: %s", nextCursor)
	if err != nil {
		if ts.handleRateLimit(err, account) {
			return nil, err
		}
		return nil, err
	}
	return retweeters, nil
}

// GetUserTweets retrieves tweets from a user
func (ts *TwitterScraper) GetUserTweets(baseDir, username string, count int) ([]*TweetResult, error) {
	if baseDir == "" {
		baseDir = ts.configuration.DataDir
	}
	// get the authenticated scraper
	scraper, account, err := ts.getAuthenticatedScraper(baseDir)
	if err != nil {
		return nil, err
	}

	var tweets []*TweetResult
	for tweet := range scraper.GetTweets(context.Background(), username, count) {
		if tweet.Error != nil {
			if ts.handleRateLimit(tweet.Error, account) {
				return nil, tweet.Error
			}
			return nil, tweet.Error
		}
		tweets = append(tweets, &TweetResult{Tweet: &tweet.Tweet})
	}
	return tweets, nil
}

// FetchUserTweets retrieves tweets from a user
func (ts *TwitterScraper) FetchUserTweets(baseDir, username string, count int, cursor string) ([]*twitterscraper.Tweet, string, error) {
	if baseDir == "" {
		baseDir = ts.configuration.DataDir
	}
	// get the authenticated scraper
	scraper, account, err := ts.getAuthenticatedScraper(baseDir)
	if err != nil {
		return nil, "", err
	}

	tweets, nextCursor, err := scraper.FetchTweets(username, count, cursor)
	if err != nil {
		if ts.handleRateLimit(err, account) {
			return nil, "", err
		}
		return nil, "", err
	}
	return tweets, nextCursor, nil
}

// GetUserMedia retrieves media tweets from a user
func (ts *TwitterScraper) GetUserMedia(baseDir, username string, count int) ([]*TweetResult, error) {
	if baseDir == "" {
		baseDir = ts.configuration.DataDir
	}
	// get the authenticated scraper
	scraper, account, err := ts.getAuthenticatedScraper(baseDir)
	if err != nil {
		return nil, err
	}

	var media []*TweetResult
	for tweet := range scraper.GetTweetsAndReplies(context.Background(), username, count) {
		if tweet.Error != nil {
			if ts.handleRateLimit(tweet.Error, account) {
				return nil, tweet.Error
			}
			continue
		}
		if len(tweet.Tweet.Photos) > 0 || len(tweet.Tweet.Videos) > 0 {
			media = append(media, &TweetResult{Tweet: &tweet.Tweet})
		}
	}
	return media, nil
}

// FetchUserMedia retrieves media tweets from a user
func (ts *TwitterScraper) FetchUserMedia(baseDir, username string, count int, cursor string) ([]*twitterscraper.Tweet, string, error) {
	if baseDir == "" {
		baseDir = ts.configuration.DataDir
	}
	// get the authenticated scraper
	scraper, account, err := ts.getAuthenticatedScraper(baseDir)
	if err != nil {
		return nil, "", err
	}

	tweets, nextCursor, err := scraper.FetchTweetsAndReplies(username, count, cursor)
	if err != nil {
		if ts.handleRateLimit(err, account) {
			return nil, "", err
		}
		return nil, "", err
	}
	return tweets, nextCursor, nil
}

// GetBookmarks retrieves user's bookmarked tweets
func (ts *TwitterScraper) GetBookmarks(baseDir string, count int) ([]*TweetResult, error) {
	if baseDir == "" {
		baseDir = ts.configuration.DataDir
	}
	// get the authenticated scraper
	scraper, account, err := ts.getAuthenticatedScraper(baseDir)
	if err != nil {
		return nil, err
	}

	var bookmarks []*TweetResult
	for tweet := range scraper.GetBookmarks(context.Background(), count) {
		if tweet.Error != nil {
			if ts.handleRateLimit(tweet.Error, account) {
				return nil, tweet.Error
			}
			return nil, tweet.Error
		}
		bookmarks = append(bookmarks, &TweetResult{Tweet: &tweet.Tweet})
		if len(bookmarks) >= count {
			break
		}
	}
	return bookmarks, nil
}

// FetchBookmarks retrieves user's bookmarked tweets
func (ts *TwitterScraper) FetchBookmarks(baseDir string, count int, cursor string) ([]*twitterscraper.Tweet, string, error) {
	if baseDir == "" {
		baseDir = ts.configuration.DataDir
	}
	// get the authenticated scraper
	scraper, account, err := ts.getAuthenticatedScraper(baseDir)
	if err != nil {
		return nil, "", err
	}

	tweets, nextCursor, err := scraper.FetchBookmarks(count, cursor)
	if err != nil {
		if ts.handleRateLimit(err, account) {
			return nil, "", err
		}
		return nil, "", err
	}
	return tweets, nextCursor, nil
}

// GetHomeTweets retrieves tweets from user's home timeline
func (ts *TwitterScraper) GetHomeTweets(baseDir string, count int) ([]*TweetResult, error) {
	if baseDir == "" {
		baseDir = ts.configuration.DataDir
	}
	// get the authenticated scraper
	scraper, account, err := ts.getAuthenticatedScraper(baseDir)
	if err != nil {
		return nil, err
	}

	var tweets []*TweetResult
	for tweet := range scraper.GetHomeTweets(context.Background(), count) {
		if tweet.Error != nil {
			if ts.handleRateLimit(tweet.Error, account) {
				return nil, tweet.Error
			}
			return nil, tweet.Error
		}
		tweets = append(tweets, &TweetResult{Tweet: &tweet.Tweet})
		if len(tweets) >= count {
			break
		}
	}
	return tweets, nil
}

// FetchHomeTweets retrieves tweets from user's home timeline
func (ts *TwitterScraper) FetchHomeTweets(baseDir string, count int, cursor string) ([]*twitterscraper.Tweet, string, error) {
	if baseDir == "" {
		baseDir = ts.configuration.DataDir
	}
	// get the authenticated scraper
	scraper, account, err := ts.getAuthenticatedScraper(baseDir)
	if err != nil {
		return nil, "", err
	}

	tweets, nextCursor, err := scraper.FetchHomeTweets(count, cursor)
	if err != nil {
		if ts.handleRateLimit(err, account) {
			return nil, "", err
		}
		return nil, "", err
	}
	return tweets, nextCursor, nil
}

// GetForYouTweets retrieves tweets from For You timeline
func (ts *TwitterScraper) GetForYouTweets(baseDir string, count int) ([]*TweetResult, error) {
	if baseDir == "" {
		baseDir = ts.configuration.DataDir
	}
	// get the authenticated scraper
	scraper, account, err := ts.getAuthenticatedScraper(baseDir)
	if err != nil {
		return nil, err
	}

	var tweets []*TweetResult
	for tweet := range scraper.GetForYouTweets(context.Background(), count) {
		if tweet.Error != nil {
			if ts.handleRateLimit(tweet.Error, account) {
				return nil, tweet.Error
			}
			return nil, tweet.Error
		}
		tweets = append(tweets, &TweetResult{Tweet: &tweet.Tweet})
		if len(tweets) >= count {
			break
		}
	}
	return tweets, nil
}

// FetchForYouTweets retrieves tweets from For You timeline
func (ts *TwitterScraper) FetchForYouTweets(baseDir string, count int, cursor string) ([]*twitterscraper.Tweet, string, error) {
	if baseDir == "" {
		baseDir = ts.configuration.DataDir
	}
	// get the authenticated scraper
	scraper, account, err := ts.getAuthenticatedScraper(baseDir)
	if err != nil {
		return nil, "", err
	}

	tweets, nextCursor, err := scraper.FetchForYouTweets(count, cursor)
	if err != nil {
		if ts.handleRateLimit(err, account) {
			return nil, "", err
		}
		return nil, "", err
	}
	return tweets, nextCursor, nil
}

// GetProfileByID retrieves a user profile by ID
func (ts *TwitterScraper) GetProfileByID(baseDir, userID string) (*twitterscraper.Profile, error) {
	if baseDir == "" {
		baseDir = ts.configuration.DataDir
	}
	// get the authenticated scraper
	scraper, account, err := ts.getAuthenticatedScraper(baseDir)
	if err != nil {
		return nil, err
	}

	profile, err := scraper.GetProfileByID(userID)
	if err != nil {
		if ts.handleRateLimit(err, account) {
			return nil, err
		}
		return nil, err
	}
	return &profile, nil
}

// SearchProfile searches for user profiles
func (ts *TwitterScraper) SearchProfile(query string, count int) ([]*twitterscraper.ProfileResult, error) {
	scraper, _, err := ts.getAuthenticatedScraper(ts.configuration.DataDir)
	if err != nil {
		return nil, err
	}

	var profiles []*twitterscraper.ProfileResult
	for profile := range scraper.SearchProfiles(context.Background(), query, count) {
		if len(profiles) >= count {
			break
		}
		profiles = append(profiles, profile)
	}
	return profiles, nil
}

// GetTrends retrieves current trending topics
func (ts *TwitterScraper) GetTrends(baseDir string) ([]string, error) {
	if baseDir == "" {
		baseDir = ts.configuration.DataDir
	}
	// get the authenticated scraper
	scraper, account, err := ts.getAuthenticatedScraper(baseDir)
	if err != nil {
		return nil, err
	}
	trends, err := scraper.GetTrends()
	if err != nil {
		if ts.handleRateLimit(err, account) {
			return nil, err
		}
		return nil, err
	}
	return trends, nil
}

// GetFollowers retrieves users that follow a user
func (ts *TwitterScraper) GetFollowers(baseDir, user string, count int, cursor string) ([]*twitterscraper.Profile, string, error) {
	if baseDir == "" {
		baseDir = ts.configuration.DataDir
	}
	// get the authenticated scraper
	scraper, account, err := ts.getAuthenticatedScraper(baseDir)
	if err != nil {
		return nil, "", err
	}

	followers, nextCursor, err := scraper.FetchFollowers(user, count, cursor)
	if err != nil {
		if ts.handleRateLimit(err, account) {
			return nil, "", err
		}
		return nil, "", err
	}
	return followers, nextCursor, nil
}

// GetFollowing retrieves users that a user follows
func (ts *TwitterScraper) GetFollowing(baseDir, username string, count int) ([]*twitterscraper.Profile, error) {
	if baseDir == "" {
		baseDir = ts.configuration.DataDir
	}
	// get the authenticated scraper
	scraper, account, err := ts.getAuthenticatedScraper(baseDir)
	if err != nil {
		return nil, err
	}

	following, errString, _ := scraper.FetchFollowing(username, count, "")
	if errString != "" {
		err := fmt.Errorf("error fetching following: %s", errString)
		if ts.handleRateLimit(err, account) {
			return nil, err
		}
		return nil, err
	}
	return following, nil
}

// GetSpace retrieves space information by ID
func (ts *TwitterScraper) GetSpace(baseDir, spaceID string) (*twitterscraper.Space, error) {
	if baseDir == "" {
		baseDir = ts.configuration.DataDir
	}
	// get the authenticated scraper
	scraper, account, err := ts.getAuthenticatedScraper(baseDir)
	if err != nil {
		return nil, err
	}

	space, err := scraper.GetSpace(spaceID)
	if err != nil {
		if ts.handleRateLimit(err, account) {
			return nil, err
		}
		return nil, err
	}
	return space, nil
}

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
	NextCursor string `json:"next_cursor"`
}

func NewTwitterScraper(jc types.JobConfiguration) *TwitterScraper {
	config := TwitterScraperConfiguration{}
	jc.Unmarshal(&config)

	accounts := parseAccounts(config.Accounts)
	accountManager := twitter.NewTwitterAccountManager(accounts)

	return &TwitterScraper{configuration: config, accountManager: accountManager}
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
		tweet, err := twitter.ScrapeTweetByID(ws.configuration.DataDir, args.Query)
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
		tweets, err := ws.GetUserTweets(ws.configuration.DataDir, args.Query, args.Count)
		if err != nil {
			return types.JobResult{Error: err.Error()}, err
		}
		dat, err := json.Marshal(tweets)
		return types.JobResult{
			Data: dat,
		}, err

	case "getmedia":
		media, err := ws.GetUserMedia(ws.configuration.DataDir, args.Query, args.Count)
		if err != nil {
			return types.JobResult{Error: err.Error()}, err
		}
		dat, err := json.Marshal(media)
		return types.JobResult{
			Data: dat,
		}, err

	case "getbookmarks":
		bookmarks, err := ws.GetBookmarks(ws.configuration.DataDir, args.Count)
		if err != nil {
			return types.JobResult{Error: err.Error()}, err

		}
		dat, err := json.Marshal(bookmarks)
		return types.JobResult{
			Data: dat,
		}, err

	case "gethometweets":
		tweets, err := ws.GetHomeTweets(ws.configuration.DataDir, args.Count)
		if err != nil {
			return types.JobResult{Error: err.Error()}, err
		}
		dat, err := json.Marshal(tweets)
		return types.JobResult{
			Data: dat,
		}, err

	case "getforyoutweets":
		tweets, err := ws.GetForYouTweets(ws.configuration.DataDir, args.Count)
		if err != nil {
			return types.JobResult{Error: err.Error()}, err
		}
		dat, err := json.Marshal(tweets)
		return types.JobResult{
			Data: dat,
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

	case "fetchusertweets":
		tweets, nextCursor, err := ws.FetchUserTweets(ws.configuration.DataDir, args.Query, args.Count, args.NextCursor)
		if err != nil {
			return types.JobResult{Error: err.Error()}, err
		}
		dat, err := json.Marshal(tweets)
		return types.JobResult{
			Data:       dat,
			NextCursor: nextCursor,
		}, err

	case "fetchusermedia":
		tweets, nextCursor, err := ws.FetchUserMedia(ws.configuration.DataDir, args.Query, args.Count, args.NextCursor)
		if err != nil {
			return types.JobResult{Error: err.Error()}, err
		}
		dat, err := json.Marshal(tweets)
		return types.JobResult{
			Data:       dat,
			NextCursor: nextCursor,
		}, err

	case "fetchbookmarks":
		tweets, nextCursor, err := ws.FetchBookmarks(ws.configuration.DataDir, args.Count, args.NextCursor)
		if err != nil {
			return types.JobResult{Error: err.Error()}, err
		}
		dat, err := json.Marshal(tweets)
		return types.JobResult{
			Data:       dat,
			NextCursor: nextCursor,
		}, err

	case "fetchhometweets":
		tweets, nextCursor, err := ws.FetchHomeTweets(ws.configuration.DataDir, args.Count, args.NextCursor)
		if err != nil {
			return types.JobResult{Error: err.Error()}, err
		}
		dat, err := json.Marshal(tweets)
		return types.JobResult{
			Data:       dat,
			NextCursor: nextCursor,
		}, err

	case "fetchforyoutweets":
		tweets, nextCursor, err := ws.FetchForYouTweets(ws.configuration.DataDir, args.Count, args.NextCursor)
		if err != nil {
			return types.JobResult{Error: err.Error()}, err
		}

		dat, err := json.Marshal(tweets)
		return types.JobResult{
			Data:       dat,
			NextCursor: nextCursor,
		}, err
	}

	// Do the web scraping here
	// For now, just return the URL
	return types.JobResult{
		Error: "invalid search type",
	}, fmt.Errorf("invalid search type")
}
