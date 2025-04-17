package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/masa-finance/tee-worker/internal/jobs/twitterx"
	"github.com/masa-finance/tee-worker/pkg/client"

	twitterscraper "github.com/imperatrona/twitter-scraper"
	"github.com/masa-finance/tee-worker/api/types"
	"github.com/masa-finance/tee-worker/internal/jobs/stats"
	"github.com/masa-finance/tee-worker/internal/jobs/twitter"

	"github.com/sirupsen/logrus"
)

type TweetResult struct {
	ID             int64 `json:"id"`
	TweetID        string
	ConversationID string
	UserID         string
	Text           string
	CreatedAt      time.Time
	Timestamp      int64

	ThreadCursor struct {
		FocalTweetID string
		ThreadID     string
		Cursor       string
		CursorType   string
	}
	IsQuoted     bool
	IsPin        bool
	IsReply      bool
	IsRetweet    bool
	IsSelfThread bool
	Likes        int
	Hashtags     []string
	HTML         string
	Replies      int
	Retweets     int
	URLs         []string
	Username     string

	Photos []Photo

	// Video type.
	Videos []Video

	RetweetedStatusID string
	Views             int
	SensitiveContent  bool

	// from twitterx
	AuthorID          string
	PublicMetrics     PublicMetrics
	PossiblySensitive bool
	Lang              string
	NewestID          string
	OldestID          string
	ResultCount       int

	Error error
}

type PublicMetrics struct {
	RetweetCount    int
	ReplyCount      int
	LikeCount       int
	QuoteCount      int
	BookmarkCount   int
	ImpressionCount int
}
type Photo struct {
	ID  string
	URL string
}

type Video struct {
	ID      string
	Preview string
	URL     string
	HLSURL  string
}

func (ts *TwitterScraper) convertTwitterScraperTweetToTweetResult(tweet twitterscraper.Tweet) TweetResult {

	id, err := strconv.ParseInt(tweet.ID, 10, 64)
	if err != nil {
		logrus.Warnf("failed to convert tweet ID to int64: %s", tweet.ID)
		id = 0 // set to 0 if conversion fails
	}

	// int64 timestamp to time.Time
	createdAt := time.Unix(tweet.Timestamp, 0).UTC()

	logrus.Info("Tweet ID: ", id)
	return TweetResult{
		ID:             id,
		TweetID:        tweet.ID,
		ConversationID: tweet.ConversationID,
		UserID:         tweet.UserID,
		Text:           tweet.Text,
		CreatedAt:      createdAt,
		Timestamp:      tweet.Timestamp,
		IsQuoted:       tweet.IsQuoted,
		IsPin:          tweet.IsPin,
		IsReply:        tweet.IsPin,
		IsRetweet:      tweet.IsRetweet,
		IsSelfThread:   tweet.IsSelfThread,
		Likes:          tweet.Likes,
		Hashtags:       tweet.Hashtags,
		HTML:           tweet.HTML,
		Replies:        tweet.Replies,
		Retweets:       tweet.Retweets,
		URLs:           tweet.URLs,
		Username:       tweet.Username,
		Photos: func() []Photo {
			var photos []Photo
			for _, photo := range tweet.Photos {
				photos = append(photos, Photo{
					ID:  photo.ID,
					URL: photo.URL,
				})
			}
			return photos
		}(),
		Videos: func() []Video {
			var videos []Video
			for _, video := range tweet.Videos {
				videos = append(videos, Video{
					ID:      video.ID,
					Preview: video.Preview,
					URL:     video.URL,
					HLSURL:  video.HLSURL,
				})
			}
			return videos
		}(),
		RetweetedStatusID: tweet.RetweetedStatusID,
		Views:             tweet.Views,
		SensitiveContent:  tweet.SensitiveContent,
	}
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

func (ts *TwitterScraper) getAuthenticatedScraper(baseDir string, jobType string) (*twitter.Scraper, *twitter.TwitterAccount, *twitter.TwitterApiKey, error) {
	// If baseDir is empty, use the default data directory
	if baseDir == "" {
		baseDir = ts.configuration.DataDir
	}

	var account *twitter.TwitterAccount
	var apiKey *twitter.TwitterApiKey
	var scraper *twitter.Scraper

	// Select authentication method based on job type
	switch jobType {
	case TwitterCredentialScraperType:
		// Only use credentials
		account = ts.accountManager.GetNextAccount()
		if account == nil {
			ts.statsCollector.Add(stats.TwitterAuthErrors, 1)
			return nil, nil, nil, fmt.Errorf("no Twitter credentials available for credential-based scraping")
		}
	case TwitterApiScraperType:
		// Only use API keys
		apiKey = ts.accountManager.GetNextApiKey()
		if apiKey == nil {
			ts.statsCollector.Add(stats.TwitterAuthErrors, 1)
			return nil, nil, nil, fmt.Errorf("no Twitter API keys available for API-based scraping")
		}
	default:
		// Standard Twitter scraper - prefer credentials if available
		account = ts.accountManager.GetNextAccount()
		apiKey = ts.accountManager.GetNextApiKey()
		if account == nil && apiKey == nil {
			ts.statsCollector.Add(stats.TwitterAuthErrors, 1)
			return nil, nil, nil, fmt.Errorf("no Twitter accounts or API keys available")
		}
	}

	// Initialize the scraper if credentials are available
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

// ScrapeFollowersForProfile scrapes the followers of a given Twitter profile.
//
// It takes a base directory, a username and a count as parameters, and returns
// a slice of pointers to twitterscraper.Profile and an error. It increments the
// TwitterScrapes and TwitterProfiles stats counters.
//
// If the scraper fails, it calls handleError with the error and the account,
// and returns the error.
func (ts *TwitterScraper) ScrapeFollowersForProfile(baseDir string, username string, count int) ([]*twitterscraper.Profile, error) {
	scraper, account, _, err := ts.getAuthenticatedScraper(baseDir, TwitterScraperType)
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

// ScrapeTweetsProfile retrieves a Twitter profile by username.
//
// It takes a base directory and a username as parameters, and returns a
// twitterscraper.Profile and an error. It increments the TwitterScrapes and
// TwitterProfiles stats counters.
//
// If the scraper fails, it calls handleError with the error and the account,
// and returns the error.
func (ts *TwitterScraper) ScrapeTweetsProfile(baseDir string, username string) (twitterscraper.Profile, error) {
	scraper, account, _, err := ts.getAuthenticatedScraper(baseDir, TwitterScraperType)
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

// ScrapeTweetsByFullArchiveSearchQuery scrapes tweets by a full archive search query.
//
// It takes a base directory, a query string and a count as parameters, and returns
// a slice of pointer to TweetResult.
func (ts *TwitterScraper) ScrapeTweetsByFullArchiveSearchQuery(baseDir string, query string, count int) ([]*TweetResult, error) {
	return ts.scrapeTweetsByQuery(twitterx.TweetsAll, baseDir, query, count)
}

// ScrapeTweetsByRecentSearchQuery scrapes tweets by a search query using the TwitterX API.
//
// It takes a base directory, a query string and a count as parameters, and returns
// a slice of pointer to TweetResult.
func (ts *TwitterScraper) ScrapeTweetsByRecentSearchQuery(baseDir string, query string, count int) ([]*TweetResult, error) {
	return ts.scrapeTweetsByQuery(twitterx.TweetsSearchRecent, baseDir, query, count)
}

// scrapeTweetsByQuery scrapes tweets by a search query using the TwitterX API
// if a TwitterX API key is available. Otherwise, it uses the default scraper.
//
// It takes a base query endpoint, a base directory, a query string and a count
// as parameters, and returns a slice of pointers to TweetResult and an error.
// It increments the TwitterScrapes and TwitterTweets stats counters.
//
// If the scraper fails, it calls handleError with the error and the account,
// and returns the error.
func (ts *TwitterScraper) scrapeTweetsByQuery(baseQueryEndpoint string, baseDir string, query string, count int) ([]*TweetResult, error) {
	scraper, account, apiKey, err := ts.getAuthenticatedScraper(baseDir, TwitterScraperType)
	if err != nil {
		return nil, err
	}

	ts.statsCollector.Add(stats.TwitterScrapes, 1)
	var tweets []*TweetResult

	// Check if we have a TwitterX API key
	if apiKey != nil {

		client := client.NewTwitterXClient(apiKey.Key)
		twitterXScraper := twitterx.NewTwitterXScraper(client)

		// Scrape tweets using the TwitterX API
		var result *twitterx.TwitterXSearchQueryResult
		switch baseQueryEndpoint {
		case twitterx.TweetsAll:
			result, err = twitterXScraper.ScrapeTweetsByFullTextSearchQuery(query, count)
			if err != nil {
				return nil, err
			}
		case twitterx.TweetsSearchRecent:
			result, err = twitterXScraper.ScrapeTweetsByQuery(query, count)
			if err != nil {
				return nil, err
			}
		}

		if result == nil {
			return nil, fmt.Errorf("no tweets found")
		}

		for _, tweet := range result.Data {

			// Append the tweet to the list of tweet result
			var newTweetResult TweetResult

			// convert id string to int64
			id, err := strconv.ParseInt(tweet.ID, 10, 64)
			if err != nil {
				return nil, err
			}

			newTweetResult.ID = id
			newTweetResult.TweetID = tweet.ID
			newTweetResult.AuthorID = tweet.AuthorID
			newTweetResult.Text = tweet.Text
			newTweetResult.ConversationID = tweet.ConversationID
			newTweetResult.UserID = tweet.AuthorID
			newTweetResult.CreatedAt = tweet.CreatedAt

			newTweetPublicMetrics := PublicMetrics{
				BookmarkCount: tweet.PublicMetrics.BookmarkCount,
				LikeCount:     tweet.PublicMetrics.LikeCount,
				QuoteCount:    tweet.PublicMetrics.QuoteCount,
				ReplyCount:    tweet.PublicMetrics.ReplyCount,
				RetweetCount:  tweet.PublicMetrics.RetweetCount,
			}

			newTweetResult.PublicMetrics = newTweetPublicMetrics

			newTweetResult.Lang = tweet.Lang
			newTweetResult.NewestID = result.Meta.NewestID
			newTweetResult.OldestID = result.Meta.OldestID
			newTweetResult.ResultCount = result.Meta.ResultCount

			tweets = append(tweets, &newTweetResult)
		}
		logrus.Info("Scraped tweets - post:  ", len(tweets))
		ts.statsCollector.Add(stats.TwitterTweets, uint(len(tweets)))

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

		newTweetResult := ts.convertTwitterScraperTweetToTweetResult(tweet.Tweet)
		tweets = append(tweets, &newTweetResult)
	}

	ts.statsCollector.Add(stats.TwitterTweets, uint(len(tweets)))
	return tweets, nil
}

// scrapeTweetsByQueryWithCredentials performs tweet scraping using only Twitter credentials
// This method is specifically for queries that require credential-based access
func (ts *TwitterScraper) scrapeTweetsByQueryWithCredentials(baseDir string, query string, count int) ([]*TweetResult, error) {
	scraper, account, _, err := ts.getAuthenticatedScraper(baseDir, TwitterCredentialScraperType)
	if err != nil {
		return nil, err
	}

	ts.statsCollector.Add(stats.TwitterScrapes, 1)
	var tweets []*TweetResult

	// Use credential-based scraper to search
	ctx := context.Background()
	scraper.SetSearchMode(twitterscraper.SearchLatest)

	for tweet := range scraper.SearchTweets(ctx, query, count) {
		if tweet.Error != nil {
			_ = ts.handleError(tweet.Error, account)
			return nil, tweet.Error
		}

		newTweetResult := ts.convertTwitterScraperTweetToTweetResult(tweet.Tweet)
		tweets = append(tweets, &newTweetResult)
	}

	ts.statsCollector.Add(stats.TwitterTweets, uint(len(tweets)))
	return tweets, nil
}

// scrapeTweetsByQueryWithApiKey performs tweet scraping using only Twitter API keys
// This method is specifically for queries that require API-based access
func (ts *TwitterScraper) scrapeTweetsByQueryWithApiKey(baseQueryEndpoint string, baseDir string, query string, count int) ([]*TweetResult, error) {
	_, _, apiKey, err := ts.getAuthenticatedScraper(baseDir, TwitterApiScraperType)
	if err != nil {
		return nil, err
	}

	ts.statsCollector.Add(stats.TwitterScrapes, 1)
	var tweets []*TweetResult

	// Use API-based scraper
	client := client.NewTwitterXClient(apiKey.Key)
	twitterXScraper := twitterx.NewTwitterXScraper(client)

	// Scrape tweets using the TwitterX API
	var result *twitterx.TwitterXSearchQueryResult
	switch baseQueryEndpoint {
	case twitterx.TweetsAll:
		result, err = twitterXScraper.ScrapeTweetsByFullTextSearchQuery(query, count)
		if err != nil {
			return nil, err
		}
	case twitterx.TweetsSearchRecent:
		result, err = twitterXScraper.ScrapeTweetsByQuery(query, count)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported query endpoint: %s", baseQueryEndpoint)
	}

	if result == nil {
		return nil, fmt.Errorf("no tweets found")
	}

	for _, tweet := range result.Data {
		// Convert to tweet result
		var newTweetResult TweetResult

		// convert id string to int64
		id, err := strconv.ParseInt(tweet.ID, 10, 64)
		if err != nil {
			return nil, err
		}

		newTweetResult.ID = id
		newTweetResult.TweetID = tweet.ID
		newTweetResult.AuthorID = tweet.AuthorID
		newTweetResult.Text = tweet.Text
		newTweetResult.ConversationID = tweet.ConversationID
		newTweetResult.UserID = tweet.AuthorID
		newTweetResult.CreatedAt = tweet.CreatedAt

		newTweetPublicMetrics := PublicMetrics{
			BookmarkCount: tweet.PublicMetrics.BookmarkCount,
			LikeCount:     tweet.PublicMetrics.LikeCount,
			QuoteCount:    tweet.PublicMetrics.QuoteCount,
			ReplyCount:    tweet.PublicMetrics.ReplyCount,
			RetweetCount:  tweet.PublicMetrics.RetweetCount,
		}

		newTweetResult.PublicMetrics = newTweetPublicMetrics

		newTweetResult.Lang = tweet.Lang
		newTweetResult.NewestID = result.Meta.NewestID
		newTweetResult.OldestID = result.Meta.OldestID
		newTweetResult.ResultCount = result.Meta.ResultCount

		tweets = append(tweets, &newTweetResult)
	}
	
	logrus.Infof("Scraped %d tweets using API key", len(tweets))
	ts.statsCollector.Add(stats.TwitterTweets, uint(len(tweets)))

	return tweets, nil
}

// ScrapeTweetByID scrapes a tweet by ID
//
// It takes a base directory and a tweet ID as parameters, and returns
// a pointer to a TweetResult and an error. It increments the
// TwitterScrapes and TwitterTweets stats counters.
//
// If the scraper fails, it calls handleError with the error and the
// account, and returns the error.
func (ts *TwitterScraper) ScrapeTweetByID(baseDir string, tweetID string) (*TweetResult, error) {
	ts.statsCollector.Add(stats.TwitterScrapes, 1)

	scraper, account, _, err := ts.getAuthenticatedScraper(baseDir, TwitterScraperType)
	if err != nil {
		return nil, err
	}

	tweet, err := scraper.GetTweet(tweetID)
	if err != nil {
		_ = ts.handleError(err, account)
		return nil, err
	}

	tweetResult := ts.convertTwitterScraperTweetToTweetResult(*tweet)

	ts.statsCollector.Add(stats.TwitterTweets, 1)
	return &tweetResult, nil
}

// GetTweet retrieves a tweet by ID.
//
// It takes a base directory and a tweet ID as parameters, and returns
// a pointer to a TweetResult and an error. It increments the
// TwitterScrapes and TwitterTweets stats counters.
//
// If the scraper fails, it calls handleError with the error and the
// account, and returns the error.
func (ts *TwitterScraper) GetTweet(baseDir, tweetID string) (*TweetResult, error) {
	scraper, account, _, err := ts.getAuthenticatedScraper(baseDir, TwitterScraperType)
	if err != nil {
		return nil, err
	}

	ts.statsCollector.Add(stats.TwitterScrapes, 1)
	tweet, err := scraper.GetTweet(tweetID)
	if err != nil {
		_ = ts.handleError(err, account)
		return nil, err
	}

	tweetResult := ts.convertTwitterScraperTweetToTweetResult(*tweet)

	ts.statsCollector.Add(stats.TwitterTweets, 1)
	return &tweetResult, nil
}

// GetTweetReplies retrieves replies to a tweet.
//
// It takes a base directory, a tweet ID and a cursor as parameters,
// and returns a slice of pointers to TweetResult and an error.
// It increments the TwitterScrapes and TwitterTweets stats counters.
//
// If the scraper fails, it calls handleError with the error and the
// account, and returns the error.
func (ts *TwitterScraper) GetTweetReplies(baseDir, tweetID string, cursor string) ([]TweetResult, error) {
	scraper, account, _, err := ts.getAuthenticatedScraper(baseDir, TwitterScraperType)
	if err != nil {
		return nil, err
	}

	ts.statsCollector.Add(stats.TwitterScrapes, 1)
	var replies []TweetResult
	tweets, threadCursor, err := scraper.GetTweetReplies(tweetID, cursor)

	for i, tweet := range tweets {
		if err != nil {
			_ = ts.handleError(err, account)
			return nil, err
		}

		newTweetResult := ts.convertTwitterScraperTweetToTweetResult(*tweet)
		newTweetResult.ThreadCursor.Cursor = threadCursor[i].Cursor
		newTweetResult.ThreadCursor.CursorType = threadCursor[i].CursorType
		newTweetResult.ThreadCursor.FocalTweetID = threadCursor[i].FocalTweetID
		newTweetResult.ThreadCursor.ThreadID = threadCursor[i].ThreadID
		newTweetResult.Error = err

		replies = append(replies, newTweetResult)

	}

	ts.statsCollector.Add(stats.TwitterTweets, uint(len(replies)))
	return replies, nil
}

// GetTweetRetweeters retrieves the retweeters of a tweet.
//
// It takes a base directory, a tweet ID, a count and a cursor as parameters,
// and returns a slice of pointers to twitterscraper.Profile and an error.
// It increments the TwitterScrapes and TwitterProfiles stats counters.
//
// If the scraper fails, it calls handleError with the error and the
// account, and returns the error.
func (ts *TwitterScraper) GetTweetRetweeters(baseDir, tweetID string, count int, cursor string) ([]*twitterscraper.Profile, error) {
	scraper, account, _, err := ts.getAuthenticatedScraper(baseDir, TwitterScraperType)
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

// GetUserTweets retrieves the tweets of a user.
//
// It takes a base directory, a username, a count and a cursor as parameters,
// and returns a slice of pointers to TweetResult, a next cursor string and an error.
// It increments the TwitterScrapes and TwitterTweets stats counters.
//
// If the cursor is empty, it uses the streaming method to retrieve tweets
// without a cursor. Otherwise, it uses the fetch method with the given cursor.
// If the streaming method is used, it sets the next cursor to the last tweet's
// ID if available. If the fetch method is used, the next cursor is set to the
// cursor returned by the fetch method.
func (ts *TwitterScraper) GetUserTweets(baseDir, username string, count int, cursor string) ([]TweetResult, string, error) {
	scraper, account, _, err := ts.getAuthenticatedScraper(baseDir, TwitterScraperType)
	if err != nil {
		return nil, "", err
	}

	ts.statsCollector.Add(stats.TwitterScrapes, 1)

	var tweets []TweetResult
	var nextCursor string

	if cursor != "" {
		// Use fetch method with cursor
		fetchedTweets, fetchCursor, err := scraper.FetchTweets(username, count, cursor)
		if err != nil {
			_ = ts.handleError(err, account)
			return nil, "", err
		}

		for _, tweet := range fetchedTweets {
			newTweetResult := ts.convertTwitterScraperTweetToTweetResult(*tweet)
			tweets = append(tweets, newTweetResult)
		}
		nextCursor = fetchCursor
	} else {
		// Use streaming method without cursor
		for tweet := range scraper.GetTweets(context.Background(), username, count) {
			if tweet.Error != nil {
				_ = ts.handleError(tweet.Error, account)
				return nil, "", tweet.Error
			}
			newTweetResult := ts.convertTwitterScraperTweetToTweetResult(tweet.Tweet)
			tweets = append(tweets, newTweetResult)
		}

		// Set next cursor to last tweet's ID if available
		if len(tweets) > 0 {
			nextCursor = strconv.FormatInt(tweets[len(tweets)-1].ID, 10)
		}
	}

	ts.statsCollector.Add(stats.TwitterTweets, uint(len(tweets)))
	return tweets, nextCursor, nil
}

func (ts *TwitterScraper) GetUserMedia(baseDir, username string, count int, cursor string) ([]TweetResult, string, error) {
	scraper, account, _, err := ts.getAuthenticatedScraper(baseDir, TwitterScraperType)
	if err != nil {
		return nil, "", err
	}

	ts.statsCollector.Add(stats.TwitterScrapes, 1)

	var media []TweetResult
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
				newTweetResult := ts.convertTwitterScraperTweetToTweetResult(*tweet)
				media = append(media, newTweetResult)
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
				newTweetResult := ts.convertTwitterScraperTweetToTweetResult(tweet.Tweet)
				media = append(media, newTweetResult)
				//media = append(media, &TweetResult{Tweet: &tweet.Tweet})
			}
		}

		// Set next cursor to last tweet's ID if available
		if len(media) > 0 {
			nextCursor = strconv.FormatInt(media[len(media)-1].ID, 10)
		}
	}

	ts.statsCollector.Add(stats.TwitterOther, uint(len(media)))
	return media, nextCursor, nil
}

func (ts *TwitterScraper) GetHomeTweets(baseDir string, count int, cursor string) ([]TweetResult, string, error) {
	scraper, account, _, err := ts.getAuthenticatedScraper(baseDir, TwitterScraperType)
	if err != nil {
		return nil, "", err
	}

	ts.statsCollector.Add(stats.TwitterScrapes, 1)

	var tweets []TweetResult
	var nextCursor string

	if cursor != "" {
		// Use fetch method with cursor
		fetchedTweets, fetchCursor, err := scraper.FetchHomeTweets(count, cursor)
		if err != nil {
			_ = ts.handleError(err, account)
			return nil, "", err
		}

		for _, tweet := range fetchedTweets {
			newTweetResult := ts.convertTwitterScraperTweetToTweetResult(*tweet)
			tweets = append(tweets, newTweetResult)
			//tweets = append(tweets, &TweetResult{Tweet: tweet})
		}
		nextCursor = fetchCursor
	} else {
		// Use streaming method without cursor
		for tweet := range scraper.GetHomeTweets(context.Background(), count) {
			if tweet.Error != nil {
				_ = ts.handleError(tweet.Error, account)
				return nil, "", tweet.Error
			}
			newTweetResult := ts.convertTwitterScraperTweetToTweetResult(tweet.Tweet)
			tweets = append(tweets, newTweetResult)
			//tweets = append(tweets, &TweetResult{Tweet: &tweet.Tweet})
			if len(tweets) >= count {
				break
			}
		}

		// Set next cursor to last tweet's ID if available
		if len(tweets) > 0 {
			nextCursor = strconv.FormatInt(tweets[len(tweets)-1].ID, 10)
		}
	}

	ts.statsCollector.Add(stats.TwitterTweets, uint(len(tweets)))
	return tweets, nextCursor, nil
}

func (ts *TwitterScraper) GetForYouTweets(baseDir string, count int, cursor string) ([]TweetResult, string, error) {
	scraper, account, _, err := ts.getAuthenticatedScraper(baseDir, TwitterScraperType)
	if err != nil {
		return nil, "", err
	}

	ts.statsCollector.Add(stats.TwitterScrapes, 1)

	var tweets []TweetResult
	var nextCursor string

	if cursor != "" {
		// Use fetch method with cursor
		fetchedTweets, fetchCursor, err := scraper.FetchForYouTweets(count, cursor)
		if err != nil {
			_ = ts.handleError(err, account)
			return nil, "", err
		}

		for _, tweet := range fetchedTweets {
			newTweetResult := ts.convertTwitterScraperTweetToTweetResult(*tweet)
			tweets = append(tweets, newTweetResult)
			//tweets = append(tweets, &TweetResult{Tweet: tweet})
		}
		nextCursor = fetchCursor
	} else {
		// Use streaming method without cursor
		for tweet := range scraper.GetForYouTweets(context.Background(), count) {
			if tweet.Error != nil {
				_ = ts.handleError(tweet.Error, account)
				return nil, "", tweet.Error
			}
			newTweetResult := ts.convertTwitterScraperTweetToTweetResult(tweet.Tweet)
			tweets = append(tweets, newTweetResult)
			//tweets = append(tweets, &TweetResult{Tweet: &tweet.Tweet})
			if len(tweets) >= count {
				break
			}
		}

		// Set next cursor to last tweet's ID if available
		if len(tweets) > 0 {
			nextCursor = strconv.FormatInt(tweets[len(tweets)-1].ID, 10)
		}
	}

	ts.statsCollector.Add(stats.TwitterTweets, uint(len(tweets)))
	return tweets, nextCursor, nil
}

// GetBookmarks retrieves the bookmarks of a user.
//
// It takes a base directory, a count and a cursor as parameters,
// and returns a slice of pointers to TweetResult and an error.
// It increments the TwitterScrapes and TwitterTweets stats counters.
//
// If the scraper fails, it calls handleError with the error and the
// account, and returns the error.
func (ts *TwitterScraper) GetBookmarks(baseDir string, count int, cursor string) ([]TweetResult, string, error) {
	scraper, account, _, err := ts.getAuthenticatedScraper(baseDir, TwitterScraperType)
	if err != nil {
		return nil, "", err
	}

	ts.statsCollector.Add(stats.TwitterScrapes, 1)
	var bookmarks []TweetResult

	ctx := context.Background()
	// Convert cursor to integer if it's not empty
	var cursorInt int = 0
	if cursor != "" {
		var err error
		cursorInt, err = strconv.Atoi(cursor)
		if err != nil {
			logrus.Warnf("Invalid cursor value '%s', using default: %v", cursor, err)
		}
	}
	for tweet := range scraper.GetBookmarks(ctx, cursorInt) {
		if tweet.Error != nil {
			_ = ts.handleError(tweet.Error, account)
			return nil, "", tweet.Error
		}

		if len(bookmarks) >= count {
			break
		}

		newTweetResult := ts.convertTwitterScraperTweetToTweetResult(tweet.Tweet)
		bookmarks = append(bookmarks, newTweetResult)
	}

	var nextCursor string
	if len(bookmarks) > 0 {
		nextCursor = bookmarks[len(bookmarks)-1].TweetID
	}

	ts.statsCollector.Add(stats.TwitterTweets, uint(len(bookmarks)))
	return bookmarks, nextCursor, nil
}

func (ts *TwitterScraper) GetProfileByID(baseDir, userID string) (*twitterscraper.Profile, error) {
	scraper, account, _, err := ts.getAuthenticatedScraper(baseDir, TwitterScraperType)
	if err != nil {
		return nil, err
	}

	ts.statsCollector.Add(stats.TwitterScrapes, 1)
	profile, err := scraper.GetProfileByID(userID)
	if err != nil {
		_ = ts.handleError(err, account)
		return nil, err
	}

	ts.statsCollector.Add(stats.TwitterOther, 1)
	return &profile, nil
}

func (ts *TwitterScraper) SearchProfile(query string, count int) ([]*twitterscraper.ProfileResult, error) {
	scraper, _, _, err := ts.getAuthenticatedScraper(ts.configuration.DataDir, TwitterScraperType)
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

func (ts *TwitterScraper) GetTrends(baseDir string) ([]string, error) {
	scraper, account, _, err := ts.getAuthenticatedScraper(baseDir, TwitterScraperType)
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

func (ts *TwitterScraper) GetFollowers(baseDir, user string, count int, cursor string) ([]*twitterscraper.Profile, string, error) {
	scraper, account, _, err := ts.getAuthenticatedScraper(baseDir, TwitterScraperType)
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

func (ts *TwitterScraper) GetFollowing(baseDir, username string, count int) ([]*twitterscraper.Profile, error) {
	// get the authenticated scraper
	scraper, account, _, err := ts.getAuthenticatedScraper(baseDir, TwitterScraperType)
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

func (ts *TwitterScraper) GetSpace(baseDir, spaceID string) (*twitterscraper.Space, error) {
	// get the authenticated scraper
	scraper, account, _, err := ts.getAuthenticatedScraper(baseDir, TwitterScraperType)
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

// Job type constants for Twitter scrapers
const (
	// TwitterScraperType is the original Twitter scraper that can use either credentials or API keys
	TwitterScraperType = "twitter-scraper"
	// TwitterCredentialScraperType is specifically for scraping with Twitter credentials only
	TwitterCredentialScraperType = "twitter-credential-scraper"
	// TwitterApiScraperType is specifically for scraping with Twitter API keys only
	TwitterApiScraperType = "twitter-api-scraper"
)

type TwitterScraper struct {
	configuration  TwitterScraperConfiguration
	accountManager *twitter.TwitterAccountManager
	statsCollector *stats.StatsCollector
	capabilities   map[string]bool // Map of supported search types
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
	MaxResults int    `json:"max_results"`
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
		capabilities: map[string]bool{
			"searchbyquery":       true,
			"searchbyfullarchive": true,
			"searchbyprofile":     true,
			"searchfollowers":     true,
			"getbyid":             true,
			"getreplies":          true,
			"getretweeters":       true,
			"gettweets":           true,
			"getmedia":            true,
			"gethometweets":       true,
			"getforyoutweets":     true,
			"getbookmarks":        true,
			"getprofilebyid":      true,
			"gettrends":           true,
			"getfollowing":        true,
			"getfollowers":        true,
			"getspace":            true,
		},
	}
}

func (ws *TwitterScraper) ExecuteJob(j types.Job) (types.JobResult, error) {
	args := &TwitterScraperArgs{}
	j.Arguments.Unmarshal(args)
	
	// Determine which implementation to use based on job type
	var jobType string = j.Type
	
	switch strings.ToLower(args.SearchType) {
	case "searchbyquery":
		var tweets []*TweetResult
		var err error
		
		// Select implementation based on job type
		switch jobType {
		case TwitterCredentialScraperType:
			// Use credential-only implementation
			logrus.Infof("Using credential-only implementation for query: %s", args.Query)
			tweets, err = ws.scrapeTweetsByQueryWithCredentials(ws.configuration.DataDir, args.Query, args.MaxResults)
		case TwitterApiScraperType:
			// Use API key-only implementation
			logrus.Infof("Using API key-only implementation for query: %s", args.Query)
			tweets, err = ws.scrapeTweetsByQueryWithApiKey(twitterx.TweetsSearchRecent, ws.configuration.DataDir, args.Query, args.MaxResults)
		default:
			// Use standard implementation that can use either credentials or API keys
			logrus.Infof("Using standard implementation for query: %s", args.Query)
			tweets, err = ws.ScrapeTweetsByRecentSearchQuery(ws.configuration.DataDir, args.Query, args.MaxResults)
		}
		
		if err != nil {
			return types.JobResult{Error: err.Error()}, err
		}
		dat, err := json.Marshal(tweets)
		return types.JobResult{
			Data: dat,
		}, err

	case "searchbyfullarchive":
		var tweets []*TweetResult
		var err error
		
		// Select implementation based on job type
		switch jobType {
		case TwitterCredentialScraperType:
			// Use credential-only implementation (though not ideal for full archive)
			logrus.Warn("Full archive search with credential-only implementation may have limited results")
			tweets, err = ws.scrapeTweetsByQueryWithCredentials(ws.configuration.DataDir, args.Query, args.MaxResults)
		case TwitterApiScraperType:
			// Use API key-only implementation
			tweets, err = ws.scrapeTweetsByQueryWithApiKey(twitterx.TweetsAll, ws.configuration.DataDir, args.Query, args.MaxResults)
		default:
			// Use standard implementation
			tweets, err = ws.ScrapeTweetsByFullArchiveSearchQuery(ws.configuration.DataDir, args.Query, args.MaxResults)
		}
		
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

func (ts *TwitterScraper) FetchHomeTweets(baseDir string, count int, cursor string) ([]*twitterscraper.Tweet, string, error) {
	scraper, account, _, err := ts.getAuthenticatedScraper(baseDir, TwitterScraperType)
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

func (ts *TwitterScraper) FetchForYouTweets(baseDir string, count int, cursor string) ([]*twitterscraper.Tweet, string, error) {
	scraper, account, _, err := ts.getAuthenticatedScraper(baseDir, TwitterScraperType)
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
