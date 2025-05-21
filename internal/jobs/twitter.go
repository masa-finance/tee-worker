package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"golang.org/x/exp/slices"
	"os"
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

func (ts *TwitterScraper) convertTwitterScraperTweetToTweetResult(tweet twitterscraper.Tweet) *TweetResult {

	id, err := strconv.ParseInt(tweet.ID, 10, 64)
	if err != nil {
		logrus.Warnf("failed to convert tweet ID to int64: %s", tweet.ID)
		id = 0 // set to 0 if conversion fails
	}

	// int64 timestamp to time.Time
	createdAt := time.Unix(tweet.Timestamp, 0).UTC()

	logrus.Info("Tweet ID: ", id)
	return &TweetResult{
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

func (ts *TwitterScraper) getAuthenticatedScraper(j types.Job, baseDir string, jobType string) (*twitter.Scraper, *twitter.TwitterAccount, *twitter.TwitterApiKey, error) {
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
			ts.statsCollector.Add(j.WorkerID, stats.TwitterAuthErrors, 1)
			return nil, nil, nil, fmt.Errorf("no Twitter credentials available for credential-based scraping")
		}
	case TwitterApiScraperType:
		// Only use API keys
		apiKey = ts.accountManager.GetNextApiKey()
		if apiKey == nil {
			ts.statsCollector.Add(j.WorkerID, stats.TwitterAuthErrors, 1)
			return nil, nil, nil, fmt.Errorf("no Twitter API keys available for API-based scraping")
		}
	default:
		logrus.Debug("Using standard Twitter scraper - prefer credentials if available")
		// Standard Twitter scraper - prefer credentials if available
		account = ts.accountManager.GetNextAccount()
		// Only get API key if no credential is available
		if account == nil {
			apiKey = ts.accountManager.GetNextApiKey()
			if apiKey == nil {
				ts.statsCollector.Add(j.WorkerID, stats.TwitterAuthErrors, 1)
				return nil, nil, nil, fmt.Errorf("no Twitter accounts or API keys available")
			}
		}
	}

	// Initialize the scraper if credentials are available
	if account != nil {
		authConfig := twitter.AuthConfig{
			Account:               account,
			BaseDir:               baseDir,
			SkipLoginVerification: ts.configuration.SkipLoginVerification,
		}

		scraper = twitter.NewScraper(authConfig)
		if scraper == nil {
			ts.statsCollector.Add(j.WorkerID, stats.TwitterAuthErrors, 1)
			logrus.Errorf("Authentication failed for %s", account.Username)
			return nil, account, nil, fmt.Errorf("twitter authentication failed for %s", account.Username)
		}
	} else if apiKey != nil {
		// If we're using API key only (no credentials), we don't initialize the scraper here
		// The TwitterX client will be created in the appropriate method
		logrus.Info("Using API key only for this request")
	} else {
		// This shouldn't happen due to our earlier checks, but just in case
		return nil, nil, nil, fmt.Errorf("no authentication method available")
	}

	return scraper, account, apiKey, nil
}

// handleError handles Twitter API errors, detecting rate limits and marking accounts as rate-limited if necessary
// It returns true if the account is rate-limited, false otherwise
func (ts *TwitterScraper) handleError(j types.Job, err error, account *twitter.TwitterAccount) bool {
	if strings.Contains(err.Error(), "Rate limit exceeded") {
		ts.statsCollector.Add(j.WorkerID, stats.TwitterRateErrors, 1)
		if account != nil {
			ts.accountManager.MarkAccountRateLimited(account)
			logrus.Warnf("rate limited: %s", account.Username)
		}
		return true
	}

	ts.statsCollector.Add(j.WorkerID, stats.TwitterErrors, 1)
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
func (ts *TwitterScraper) ScrapeFollowersForProfile(j types.Job, baseDir string, username string, count int) ([]*twitterscraper.Profile, error) {
	scraper, account, _, err := ts.getAuthenticatedScraper(j, baseDir, TwitterScraperType)
	if err != nil {
		return nil, err
	}

	ts.statsCollector.Add(j.WorkerID, stats.TwitterScrapes, 1)
	followingResponse, errString, _ := scraper.FetchFollowers(username, count, "")
	if errString != "" {
		err := fmt.Errorf("rate limited: %s", errString)
		if ts.handleError(j, err, account) {
			return nil, err
		}

		logrus.Errorf("[-] Error fetching followers: %s", errString)
		return nil, fmt.Errorf("error fetching followers: %s", errString)
	}

	ts.statsCollector.Add(j.WorkerID, stats.TwitterProfiles, uint(len(followingResponse)))
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
func (ts *TwitterScraper) ScrapeTweetsProfile(j types.Job, baseDir string, username string) (twitterscraper.Profile, error) {
	scraper, account, _, err := ts.getAuthenticatedScraper(j, baseDir, TwitterScraperType)
	if err != nil {
		return twitterscraper.Profile{}, err
	}

	ts.statsCollector.Add(j.WorkerID, stats.TwitterScrapes, 1)
	profile, err := scraper.GetProfile(username)
	if err != nil {
		_ = ts.handleError(j, err, account)
		return twitterscraper.Profile{}, err
	}

	ts.statsCollector.Add(j.WorkerID, stats.TwitterProfiles, 1)
	return profile, nil
}

// queryTweets scrapes tweets by a search query using the TwitterX API
// if a TwitterX API key is available. Otherwise, it uses the default scraper.
//
// It takes a base query endpoint, a base directory, a query string and a count
// as parameters, and returns a slice of pointers to TweetResult and an error.
// It increments the TwitterScrapes and TwitterTweets stats counters.
//
// If the scraper fails, it calls handleError with the error and the account,
// and returns the error.
func (ts *TwitterScraper) queryTweets(j types.Job, baseQueryEndpoint string, baseDir string, query string, count int) ([]*TweetResult, error) {
	scraper, account, apiKey, err := ts.getAuthenticatedScraper(j, baseDir, TwitterScraperType)
	if err != nil {
		return nil, err
	}

	// Check if we have a TwitterX API key
	if apiKey == nil {
		return ts.scrapeTweetsWithCredentials(j, query, count, scraper, account)
	}
	return ts.scrapeTweetsWithApiKey(j, baseQueryEndpoint, query, count, apiKey)
}

// queryTweetsWithCredentials performs tweet scraping using only Twitter credentials
// This method is specifically for queries that require credential-based access
func (ts *TwitterScraper) queryTweetsWithCredentials(j types.Job, baseDir string, query string, count int) ([]*TweetResult, error) {
	scraper, account, _, err := ts.getAuthenticatedScraper(j, baseDir, TwitterCredentialScraperType)
	if err != nil {
		return nil, err
	}
	return ts.scrapeTweetsWithCredentials(j, query, count, scraper, account)
}

// queryTweetsWithApiKey performs tweet scraping using only Twitter API keys
// This method is specifically for queries that require API-based access
func (ts *TwitterScraper) queryTweetsWithApiKey(j types.Job, baseQueryEndpoint string, baseDir string, query string, count int) ([]*TweetResult, error) {
	_, _, apiKey, err := ts.getAuthenticatedScraper(j, baseDir, TwitterApiScraperType)
	if err != nil {
		return nil, err
	}
	return ts.scrapeTweetsWithApiKey(j, baseQueryEndpoint, query, count, apiKey)
}

// scrapeTweetsWithCredentials is the method that actually performs the work of scraping tweets using only Twitter credentials.
func (ts *TwitterScraper) scrapeTweetsWithCredentials(j types.Job, query string, count int, scraper *twitter.Scraper, account *twitter.TwitterAccount) ([]*TweetResult, error) {
	ts.statsCollector.Add(j.WorkerID, stats.TwitterScrapes, 1)

	tweets := make([]*TweetResult, 0, count)
	// Use the default scraper if no TwitterX API key is available
	ctx, cancel := context.WithTimeout(context.Background(), j.Timeout)
	defer cancel()

	scraper.SetSearchMode(twitterscraper.SearchLatest)

	// No need to loop until we get `count` tweets, scraper.SearchTweets does it for us.
	// TODO: Instead of collecting results into a slice, directly use the channel to add results as they come in
	for tweet := range scraper.SearchTweets(ctx, query, count) {
		if tweet.Error != nil {
			_ = ts.handleError(j, tweet.Error, account)
			return nil, tweet.Error
		}

		newTweetResult := ts.convertTwitterScraperTweetToTweetResult(tweet.Tweet)
		tweets = append(tweets, newTweetResult)
	}

	ts.statsCollector.Add(j.WorkerID, stats.TwitterTweets, uint(len(tweets)))
	return tweets, nil
}

// scrapeTweetsWithApiKey performs the actual work of tweet scraping using only Twitter API keys
func (ts *TwitterScraper) scrapeTweetsWithApiKey(j types.Job, baseQueryEndpoint string, query string, count int, apiKey *twitter.TwitterApiKey) ([]*TweetResult, error) {
	ts.statsCollector.Add(j.WorkerID, stats.TwitterScrapes, 1)

	// If full archive search is requested, ensure key is elevated
	if baseQueryEndpoint == twitterx.TweetsAll && apiKey.Type == twitter.TwitterApiKeyTypeBase {
		return nil, fmt.Errorf("this API key is a base/Basic key and does not have access to full archive search. Please use an elevated/Pro API key")
	}

	// Use API-based scraper
	client := client.NewTwitterXClient(apiKey.Key)
	twitterXScraper := twitterx.NewTwitterXScraper(client)

	// TODO: Instead of adding to `tweets`, incrementally store them directly into the result cache
	tweets := make([]*TweetResult, 0, count)

	cursor := ""
	deadline := time.Now().Add(j.Timeout)

	for len(tweets) < count && time.Now().Before(deadline) {
		result, err := twitterXScraper.ScrapeTweetsByQuery(baseQueryEndpoint, query, count-len(tweets), cursor)
		if err != nil {
			if len(tweets) == 0 {
				ts.handleError(j, err, nil)
				return nil, err
			}
			// Return the partial results
			break
		}
		if result == nil || len(result.Data) == 0 {
			// Reached the end of the query
			if len(tweets) == 0 {
				return nil, fmt.Errorf("no tweets found")
			}
			break
		}

		// Scrape tweets using the TwitterX API
		for _, t := range result.Data {
			// convert id string to int64
			id, err := strconv.ParseInt(t.ID, 10, 64)
			if err != nil {
				return nil, err
			}

			tweet := TweetResult{
				ID:             id,
				TweetID:        t.ID,
				AuthorID:       t.AuthorID,
				Text:           t.Text,
				ConversationID: t.ConversationID,
				UserID:         t.AuthorID,
				CreatedAt:      t.CreatedAt,
				Username:       t.Username,
				Lang:           t.Lang,
				NewestID:       result.Meta.NewestID,
				OldestID:       result.Meta.OldestID,
				ResultCount:    result.Meta.ResultCount,
				PublicMetrics: PublicMetrics{
					BookmarkCount: t.PublicMetrics.BookmarkCount,
					LikeCount:     t.PublicMetrics.LikeCount,
					QuoteCount:    t.PublicMetrics.QuoteCount,
					ReplyCount:    t.PublicMetrics.ReplyCount,
					RetweetCount:  t.PublicMetrics.RetweetCount,
				},
			}

			tweets = append(tweets, &tweet)
		}
		cursor = result.Meta.NextCursor
	}

	logrus.Infof("Scraped %d tweets using API key", len(tweets))
	ts.statsCollector.Add(j.WorkerID, stats.TwitterTweets, uint(len(tweets)))

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
func (ts *TwitterScraper) ScrapeTweetByID(j types.Job, baseDir string, tweetID string) (*TweetResult, error) {
	ts.statsCollector.Add(j.WorkerID, stats.TwitterScrapes, 1)

	scraper, account, _, err := ts.getAuthenticatedScraper(j, baseDir, TwitterScraperType)
	if err != nil {
		return nil, err
	}

	tweet, err := scraper.GetTweet(tweetID)
	if err != nil {
		_ = ts.handleError(j, err, account)
		return nil, err
	}

	tweetResult := ts.convertTwitterScraperTweetToTweetResult(*tweet)

	ts.statsCollector.Add(j.WorkerID, stats.TwitterTweets, 1)
	return tweetResult, nil
}

// GetTweet retrieves a tweet by ID.
//
// It takes a base directory and a tweet ID as parameters, and returns
// a pointer to a TweetResult and an error. It increments the
// TwitterScrapes and TwitterTweets stats counters.
//
// If the scraper fails, it calls handleError with the error and the
// account, and returns the error.
func (ts *TwitterScraper) GetTweet(j types.Job, baseDir, tweetID string) (*TweetResult, error) {
	scraper, account, _, err := ts.getAuthenticatedScraper(j, baseDir, TwitterScraperType)
	if err != nil {
		return nil, err
	}

	ts.statsCollector.Add(j.WorkerID, stats.TwitterScrapes, 1)
	tweet, err := scraper.GetTweet(tweetID)
	if err != nil {
		_ = ts.handleError(j, err, account)
		return nil, err
	}

	tweetResult := ts.convertTwitterScraperTweetToTweetResult(*tweet)

	ts.statsCollector.Add(j.WorkerID, stats.TwitterTweets, 1)
	return tweetResult, nil
}

// GetTweetReplies retrieves replies to a tweet.
//
// It takes a base directory, a tweet ID and a cursor as parameters,
// and returns a slice of pointers to TweetResult and an error.
// It increments the TwitterScrapes and TwitterTweets stats counters.
//
// If the scraper fails, it calls handleError with the error and the
// account, and returns the error.
func (ts *TwitterScraper) GetTweetReplies(j types.Job, baseDir, tweetID string, cursor string) ([]*TweetResult, error) {
	scraper, account, _, err := ts.getAuthenticatedScraper(j, baseDir, TwitterScraperType)
	if err != nil {
		return nil, err
	}

	ts.statsCollector.Add(j.WorkerID, stats.TwitterScrapes, 1)
	var replies []*TweetResult
	tweets, threadCursor, err := scraper.GetTweetReplies(tweetID, cursor)

	for i, tweet := range tweets {
		if err != nil {
			_ = ts.handleError(j, err, account)
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

	ts.statsCollector.Add(j.WorkerID, stats.TwitterTweets, uint(len(replies)))
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
func (ts *TwitterScraper) GetTweetRetweeters(j types.Job, baseDir, tweetID string, count int, cursor string) ([]*twitterscraper.Profile, error) {
	scraper, account, _, err := ts.getAuthenticatedScraper(j, baseDir, TwitterScraperType)
	if err != nil {
		return nil, err
	}

	ts.statsCollector.Add(j.WorkerID, stats.TwitterScrapes, 1)
	retweeters, _, err := scraper.GetTweetRetweeters(tweetID, count, cursor)
	if err != nil {
		_ = ts.handleError(j, err, account)
		return nil, err
	}

	ts.statsCollector.Add(j.WorkerID, stats.TwitterProfiles, uint(len(retweeters)))
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
func (ts *TwitterScraper) GetUserTweets(j types.Job, baseDir, username string, count int, cursor string) ([]*TweetResult, string, error) {
	scraper, account, _, err := ts.getAuthenticatedScraper(j, baseDir, TwitterScraperType)
	if err != nil {
		return nil, "", err
	}

	ts.statsCollector.Add(j.WorkerID, stats.TwitterScrapes, 1)

	var tweets []*TweetResult
	var nextCursor string

	if cursor != "" {
		// Use fetch method with cursor
		fetchedTweets, fetchCursor, err := scraper.FetchTweets(username, count, cursor)
		if err != nil {
			_ = ts.handleError(j, err, account)
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
				_ = ts.handleError(j, tweet.Error, account)
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

	ts.statsCollector.Add(j.WorkerID, stats.TwitterTweets, uint(len(tweets)))
	return tweets, nextCursor, nil
}

func (ts *TwitterScraper) GetUserMedia(j types.Job, baseDir, username string, count int, cursor string) ([]*TweetResult, string, error) {
	scraper, account, _, err := ts.getAuthenticatedScraper(j, baseDir, TwitterScraperType)
	if err != nil {
		return nil, "", err
	}

	ts.statsCollector.Add(j.WorkerID, stats.TwitterScrapes, 1)

	var media []*TweetResult
	var nextCursor string

	if cursor != "" {
		// Use fetch method with cursor
		fetchedTweets, fetchCursor, err := scraper.FetchTweetsAndReplies(username, count, cursor)
		if err != nil {
			_ = ts.handleError(j, err, account)
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
				if ts.handleError(j, tweet.Error, account) {
					return nil, "", tweet.Error
				}
				continue
			}
			if len(tweet.Photos) > 0 || len(tweet.Videos) > 0 {
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

	ts.statsCollector.Add(j.WorkerID, stats.TwitterOther, uint(len(media)))
	return media, nextCursor, nil
}

func (ts *TwitterScraper) GetHomeTweets(j types.Job, baseDir string, count int, cursor string) ([]*TweetResult, string, error) {
	scraper, account, _, err := ts.getAuthenticatedScraper(j, baseDir, TwitterScraperType)
	if err != nil {
		return nil, "", err
	}

	ts.statsCollector.Add(j.WorkerID, stats.TwitterScrapes, 1)

	var tweets []*TweetResult
	var nextCursor string

	if cursor != "" {
		// Use fetch method with cursor
		fetchedTweets, fetchCursor, err := scraper.FetchHomeTweets(count, cursor)
		if err != nil {
			_ = ts.handleError(j, err, account)
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
				_ = ts.handleError(j, tweet.Error, account)
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

	ts.statsCollector.Add(j.WorkerID, stats.TwitterTweets, uint(len(tweets)))
	return tweets, nextCursor, nil
}

func (ts *TwitterScraper) GetForYouTweets(j types.Job, baseDir string, count int, cursor string) ([]*TweetResult, string, error) {
	scraper, account, _, err := ts.getAuthenticatedScraper(j, baseDir, TwitterScraperType)
	if err != nil {
		return nil, "", err
	}

	ts.statsCollector.Add(j.WorkerID, stats.TwitterScrapes, 1)

	var tweets []*TweetResult
	var nextCursor string

	if cursor != "" {
		// Use fetch method with cursor
		fetchedTweets, fetchCursor, err := scraper.FetchForYouTweets(count, cursor)
		if err != nil {
			_ = ts.handleError(j, err, account)
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
				_ = ts.handleError(j, tweet.Error, account)
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

	ts.statsCollector.Add(j.WorkerID, stats.TwitterTweets, uint(len(tweets)))
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
func (ts *TwitterScraper) GetBookmarks(j types.Job, baseDir string, count int, cursor string) ([]*TweetResult, string, error) {
	scraper, account, _, err := ts.getAuthenticatedScraper(j, baseDir, TwitterScraperType)
	if err != nil {
		return nil, "", err
	}

	ts.statsCollector.Add(j.WorkerID, stats.TwitterScrapes, 1)
	var bookmarks []*TweetResult

	ctx := context.Background()
	// Convert cursor to integer if it's not empty
	cursorInt := 0
	if cursor != "" {
		var err error
		cursorInt, err = strconv.Atoi(cursor)
		if err != nil {
			logrus.Warnf("Invalid cursor value '%s', using default: %v", cursor, err)
		}
	}
	for tweet := range scraper.GetBookmarks(ctx, cursorInt) {
		if tweet.Error != nil {
			_ = ts.handleError(j, tweet.Error, account)
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

	ts.statsCollector.Add(j.WorkerID, stats.TwitterTweets, uint(len(bookmarks)))
	return bookmarks, nextCursor, nil
}

func (ts *TwitterScraper) GetProfileByID(j types.Job, baseDir, userID string) (*twitterscraper.Profile, error) {
	scraper, account, _, err := ts.getAuthenticatedScraper(j, baseDir, TwitterScraperType)
	if err != nil {
		return nil, err
	}

	ts.statsCollector.Add(j.WorkerID, stats.TwitterScrapes, 1)
	profile, err := scraper.GetProfileByID(userID)
	if err != nil {
		_ = ts.handleError(j, err, account)
		return nil, err
	}

	ts.statsCollector.Add(j.WorkerID, stats.TwitterProfiles, 1)
	return &profile, nil
}

func (ts *TwitterScraper) SearchProfile(j types.Job, query string, count int) ([]*twitterscraper.ProfileResult, error) {
	scraper, _, _, err := ts.getAuthenticatedScraper(j, ts.configuration.DataDir, TwitterScraperType)
	if err != nil {
		return nil, err
	}

	ts.statsCollector.Add(j.WorkerID, stats.TwitterScrapes, 1)
	var profiles []*twitterscraper.ProfileResult
	for profile := range scraper.SearchProfiles(context.Background(), query, count) {
		if len(profiles) >= count {
			break
		}

		profiles = append(profiles, profile)
	}

	ts.statsCollector.Add(j.WorkerID, stats.TwitterProfiles, uint(len(profiles)))
	return profiles, nil
}

func (ts *TwitterScraper) GetTrends(j types.Job, baseDir string) ([]string, error) {
	scraper, account, _, err := ts.getAuthenticatedScraper(j, baseDir, TwitterScraperType)
	if err != nil {
		return nil, err
	}

	ts.statsCollector.Add(j.WorkerID, stats.TwitterScrapes, 1)
	trends, err := scraper.GetTrends()
	if err != nil {
		_ = ts.handleError(j, err, account)
		return nil, err
	}

	// TODO: Should this be the number of topics, or 1 ?
	ts.statsCollector.Add(j.WorkerID, stats.TwitterOther, uint(len(trends)))
	return trends, nil
}

func (ts *TwitterScraper) GetFollowers(j types.Job, baseDir, user string, count int, cursor string) ([]*twitterscraper.Profile, string, error) {
	scraper, account, _, err := ts.getAuthenticatedScraper(j, baseDir, TwitterScraperType)
	if err != nil {
		return nil, "", err
	}

	ts.statsCollector.Add(j.WorkerID, stats.TwitterScrapes, 1)
	followers, nextCursor, err := scraper.FetchFollowers(user, count, cursor)
	if err != nil {
		_ = ts.handleError(j, err, account)
		return nil, "", err
	}

	ts.statsCollector.Add(j.WorkerID, stats.TwitterProfiles, uint(len(followers)))
	return followers, nextCursor, nil
}

func (ts *TwitterScraper) GetFollowing(j types.Job, baseDir, username string, count int) ([]*twitterscraper.Profile, error) {
	// get the authenticated scraper
	scraper, account, _, err := ts.getAuthenticatedScraper(j, baseDir, TwitterScraperType)
	if err != nil {
		return nil, err
	}

	ts.statsCollector.Add(j.WorkerID, stats.TwitterScrapes, 1)
	following, errString, _ := scraper.FetchFollowing(username, count, "")
	if errString != "" {
		err := fmt.Errorf("error fetching following: %s", errString)
		_ = ts.handleError(j, err, account)
		return nil, err
	}

	ts.statsCollector.Add(j.WorkerID, stats.TwitterProfiles, uint(len(following)))
	return following, nil
}

func (ts *TwitterScraper) GetSpace(j types.Job, baseDir, spaceID string) (*twitterscraper.Space, error) {
	// get the authenticated scraper
	scraper, account, _, err := ts.getAuthenticatedScraper(j, baseDir, TwitterScraperType)
	if err != nil {
		return nil, err
	}

	ts.statsCollector.Add(j.WorkerID, stats.TwitterScrapes, 1)
	space, err := scraper.GetSpace(spaceID)
	if err != nil {
		_ = ts.handleError(j, err, account)
		return nil, err
	}

	ts.statsCollector.Add(j.WorkerID, stats.TwitterOther, 1)
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
	configuration struct {
		Accounts              []string `json:"twitter_accounts"`
		ApiKeys               []string `json:"twitter_api_keys"`
		DataDir               string   `json:"data_dir"`
		SkipLoginVerification bool     `json:"skip_login_verification,omitempty"` // If true, skips Twitter's verify_credentials check (default: false)
	}
	accountManager *twitter.TwitterAccountManager
	statsCollector *stats.StatsCollector
	capabilities   map[string]bool // Map of supported search types
}

// TwitterScraperConfiguration struct is removed and inlined above.
type TwitterScraperArgs struct {
	SearchType string `json:"type"`
	Query      string `json:"query"`
	MaxResults int    `json:"max_results"`
	NextCursor string `json:"next_cursor"`
}

func NewTwitterScraper(jc types.JobConfiguration, c *stats.StatsCollector) *TwitterScraper {
	config := struct {
		Accounts              []string `json:"twitter_accounts"`
		ApiKeys               []string `json:"twitter_api_keys"`
		DataDir               string   `json:"data_dir"`
		SkipLoginVerification bool     `json:"skip_login_verification,omitempty"` // If true, skips Twitter's verify_credentials check (default: false)
	}{}
	if err := jc.Unmarshal(&config); err != nil {
		logrus.Errorf("Error unmarshalling Twitter scraper configuration: %v", err)
		return nil
	}

	accounts := parseAccounts(config.Accounts)
	apiKeys := parseApiKeys(config.ApiKeys)
	accountManager := twitter.NewTwitterAccountManager(accounts, apiKeys)

	// Detect all API key types
	accountManager.DetectAllApiKeyTypes()

	skipVerification := os.Getenv("TWITTER_SKIP_LOGIN_VERIFICATION") == "true"
	if config.SkipLoginVerification || skipVerification {
		config.SkipLoginVerification = true
	}

	// Set capabilities based on available authentication methods
	// Define all Twitter capabilities
	allCapabilities := []string{
		"searchbyquery", "searchbyfullarchive", "searchbyprofile", "searchfollowers",
		"getbyid", "getreplies", "getretweeters", "gettweets", "getmedia",
		"gethometweets", "getforyoutweets", "getbookmarks", "getprofilebyid",
		"gettrends", "getfollowing", "getfollowers", "getspace", "getspaces",
	}

	// Initialize capabilities map
	capabilities := make(map[string]bool)

	// Check for API keys with elevated access (can do full archive search)
	apiKeyList := accountManager.GetApiKeys()
	hasFullArchiveCapability := slices.ContainsFunc(apiKeyList, func(k *twitter.TwitterApiKey) bool {
		return k.Type == twitter.TwitterApiKeyTypeElevated
	})

	// Set capabilities based on authentication types
	hasAccounts := len(accounts) > 0
	hasApiKeys := len(apiKeyList) > 0

	// Enable all capabilities for everything
	for _, capability := range allCapabilities {
		// For searchbyfullarchive, only enable if we have elevated API access
		if capability == "searchbyfullarchive" {
			capabilities[capability] = hasFullArchiveCapability
		} else {
			// Enable if we have either accounts or API keys
			capabilities[capability] = hasAccounts || hasApiKeys
		}
	}

	logrus.Infof("Twitter capabilities: Accounts=%v, ApiKeys=%v, FullArchive=%v, Capabilities: %v",
		hasAccounts, hasApiKeys, hasFullArchiveCapability, capabilities)

	return &TwitterScraper{
		configuration:  config,
		accountManager: accountManager,
		statsCollector: c,
		capabilities:   capabilities,
	}
}

// TwitterScrapeStrategy defines the interface for scrape strategy
// Each job type (credential, api, default) implements this
type TwitterScrapeStrategy interface {
	Execute(j types.Job, ts *TwitterScraper, args *TwitterScraperArgs) (types.JobResult, error)
}

// Factory for strategy
func getScrapeStrategy(jobType string) TwitterScrapeStrategy {
	switch jobType {
	case TwitterCredentialScraperType:
		return &CredentialScrapeStrategy{}
	case TwitterApiScraperType:
		return &ApiKeyScrapeStrategy{}
	default:
		return &DefaultScrapeStrategy{}
	}
}

// Credential-only
type CredentialScrapeStrategy struct{}

func (s *CredentialScrapeStrategy) Execute(j types.Job, ts *TwitterScraper, args *TwitterScraperArgs) (types.JobResult, error) {
	switch strings.ToLower(args.SearchType) {
	case "searchbyquery":
		tweets, err := ts.queryTweetsWithCredentials(j, ts.configuration.DataDir, args.Query, args.MaxResults)
		if err != nil {
			return types.JobResult{Error: err.Error()}, err
		}
		dat, err := json.Marshal(tweets)
		return types.JobResult{Data: dat}, err

	case "searchbyfullarchive":
		logrus.Warn("Full archive search with credential-only implementation may have limited results")
		tweets, err := ts.queryTweetsWithCredentials(j, ts.configuration.DataDir, args.Query, args.MaxResults)
		if err != nil {
			return types.JobResult{Error: err.Error()}, err
		}
		dat, err := json.Marshal(tweets)
		return types.JobResult{Data: dat}, err

	default:
		return defaultStrategyFallback(j, ts, args)
	}
}

// API key-only
type ApiKeyScrapeStrategy struct{}

func (s *ApiKeyScrapeStrategy) Execute(j types.Job, ts *TwitterScraper, args *TwitterScraperArgs) (types.JobResult, error) {
	switch strings.ToLower(args.SearchType) {
	case "searchbyquery":
		tweets, err := ts.queryTweetsWithApiKey(j, twitterx.TweetsSearchRecent, ts.configuration.DataDir, args.Query, args.MaxResults)
		if err != nil {
			return types.JobResult{Error: err.Error()}, err
		}
		dat, err := json.Marshal(tweets)
		return types.JobResult{Data: dat}, err

	case "searchbyfullarchive":
		tweets, err := ts.queryTweetsWithApiKey(j, twitterx.TweetsAll, ts.configuration.DataDir, args.Query, args.MaxResults)
		if err != nil {
			return types.JobResult{Error: err.Error()}, err
		}
		dat, err := json.Marshal(tweets)
		return types.JobResult{Data: dat}, err

	default:
		return defaultStrategyFallback(j, ts, args)
	}
}

// Default (legacy, prefers credentials if both present)
type DefaultScrapeStrategy struct{}

func (s *DefaultScrapeStrategy) Execute(j types.Job, ts *TwitterScraper, args *TwitterScraperArgs) (types.JobResult, error) {
	switch strings.ToLower(args.SearchType) {
	case "searchbyquery":
		tweets, err := ts.queryTweets(j, twitterx.TweetsSearchRecent, ts.configuration.DataDir, args.Query, args.MaxResults)
		if err != nil {
			return types.JobResult{Error: err.Error()}, err
		}
		dat, err := json.Marshal(tweets)
		return types.JobResult{Data: dat}, err

	case "searchbyfullarchive":
		tweets, err := ts.queryTweets(j, twitterx.TweetsAll, ts.configuration.DataDir, args.Query, args.MaxResults)
		if err != nil {
			return types.JobResult{Error: err.Error()}, err
		}
		dat, err := json.Marshal(tweets)
		return types.JobResult{Data: dat}, err

	default:
		return defaultStrategyFallback(j, ts, args)
	}
}

// retryWithCursor retries a given function `fn` which fetches data in a paginated manner using a cursor. It continues to call `fn` until the desired `count` of records is fetched, the job timeout is reached, or there are no more results.
func retryWithCursor[T any](
	j types.Job,
	baseDir string,
	count int,
	cursor string,
	fn func(j types.Job, baseDir string, count int, cursor string) ([]*T, string, error),
) (types.JobResult, error) {
	records := make([]*T, 0, count)
	deadline := time.Now().Add(j.Timeout)

	var results []*T
	var err error
	var nextCursor string

	for time.Now().Before(deadline) || len(records) < count {
		results, nextCursor, err = fn(j, baseDir, count-len(records), cursor)
		if len(results) == 0 {
			// No results, probably reached the end of the query
			break
		}
		if err != nil {
			if len(records) > 0 {
				// Return the partial results
				break
			}
			return processResponse(nil, "", err)
		}
		records = append(records, results...)
		cursor = nextCursor
		if cursor == "" {
			break
		}
	}

	return processResponse(records, cursor, nil)
}

// retryWithCursor retries a given function `fn` which receives a query and fetches data in a paginated manner using a cursor. It continues to call `fn` until the desired `count` of records is fetched, the job timeout is reached, or there are no more results.
func retryWithCursorAndQuery[T any](
	j types.Job,
	baseDir string,
	query string,
	count int,
	cursor string,
	fn func(j types.Job, baseDir string, query string, count int, cursor string) ([]*T, string, error),
) (types.JobResult, error) {
	return retryWithCursor(
		j,
		baseDir,
		count,
		cursor,
		func(j types.Job, baseDir string, count int, cursor string) ([]*T, string, error) {
			return fn(j, baseDir, query, count, cursor)
		},
	)
}

// processResponse is a convenience function that takes a response object, a nextCursor string, and an error, and converts them into a types.JobResult.
func processResponse(response any, nextCursor string, err error) (types.JobResult, error) {
	if err != nil {
		return types.JobResult{Error: err.Error()}, err
	}
	dat, err := json.Marshal(response)
	if err != nil {
		return types.JobResult{Error: err.Error()}, err
	}
	return types.JobResult{Data: dat, NextCursor: nextCursor}, nil
}

// Retry the function with the cursor
// fallback for all strategies for non-query types
func defaultStrategyFallback(j types.Job, ts *TwitterScraper, args *TwitterScraperArgs) (types.JobResult, error) {
	switch strings.ToLower(args.SearchType) {
	case "searchbyprofile":
		profile, err := ts.ScrapeTweetsProfile(j, ts.configuration.DataDir, args.Query)
		return processResponse(profile, "", err)
	case "searchfollowers":
		followers, err := ts.ScrapeFollowersForProfile(j, ts.configuration.DataDir, args.Query, args.MaxResults)
		return processResponse(followers, "", err)
	case "getbyid":
		tweet, err := ts.ScrapeTweetByID(j, ts.configuration.DataDir, args.Query)
		return processResponse(tweet, "", err)
	case "getreplies":
		replies, err := ts.GetTweetReplies(j, ts.configuration.DataDir, args.Query, "")
		return processResponse(replies, "", err)
	case "getretweeters":
		retweeters, err := ts.GetTweetRetweeters(j, ts.configuration.DataDir, args.Query, args.MaxResults, "")
		return processResponse(retweeters, "", err)
	case "gettweets":
		return retryWithCursorAndQuery(j, ts.configuration.DataDir, args.Query, args.MaxResults, args.NextCursor, ts.GetUserTweets)
	case "getmedia":
		return retryWithCursorAndQuery(j, ts.configuration.DataDir, args.Query, args.MaxResults, args.NextCursor, ts.GetUserMedia)
	case "gethometweets":
		return retryWithCursor(j, ts.configuration.DataDir, args.MaxResults, args.NextCursor, ts.GetHomeTweets)
	case "getforyoutweets":
		return retryWithCursor(j, ts.configuration.DataDir, args.MaxResults, args.NextCursor, ts.GetForYouTweets)
	case "getbookmarks":
		return retryWithCursor(j, ts.configuration.DataDir, args.MaxResults, args.NextCursor, ts.GetBookmarks)
	case "getprofilebyid":
		profile, err := ts.GetProfileByID(j, ts.configuration.DataDir, args.Query)
		return processResponse(profile, "", err)
	case "gettrends":
		trends, err := ts.GetTrends(j, ts.configuration.DataDir)
		return processResponse(trends, "", err)
	case "getfollowing":
		following, err := ts.GetFollowing(j, ts.configuration.DataDir, args.Query, args.MaxResults)
		return processResponse(following, "", err)
	case "getfollowers":
		return retryWithCursorAndQuery(j, ts.configuration.DataDir, args.Query, args.MaxResults, args.NextCursor, ts.GetFollowers)
	case "getspace":
		space, err := ts.GetSpace(j, ts.configuration.DataDir, args.Query)
		return processResponse(space, "", err)
	}
	return types.JobResult{Error: "invalid search type"}, fmt.Errorf("invalid search type")
}

func (ts *TwitterScraper) ExecuteJob(j types.Job) (types.JobResult, error) {
	args := &TwitterScraperArgs{}
	if err := j.Arguments.Unmarshal(args); err != nil {
		logrus.Errorf("Error while unmarshalling job arguments: %s", err)
		return types.JobResult{Error: err.Error()}, err
	}
	strategy := getScrapeStrategy(j.Type)
	return strategy.Execute(j, ts, args)
}

func (ts *TwitterScraper) FetchHomeTweets(j types.Job, baseDir string, count int, cursor string) ([]*twitterscraper.Tweet, string, error) {
	scraper, account, _, err := ts.getAuthenticatedScraper(j, baseDir, TwitterScraperType)
	if err != nil {
		return nil, "", err
	}

	ts.statsCollector.Add(j.WorkerID, stats.TwitterScrapes, 1)
	tweets, nextCursor, err := scraper.FetchHomeTweets(count, cursor)
	if err != nil {
		_ = ts.handleError(j, err, account)
		return nil, "", err
	}

	ts.statsCollector.Add(j.WorkerID, stats.TwitterTweets, uint(len(tweets)))
	return tweets, nextCursor, nil
}

func (ts *TwitterScraper) FetchForYouTweets(j types.Job, baseDir string, count int, cursor string) ([]*twitterscraper.Tweet, string, error) {
	scraper, account, _, err := ts.getAuthenticatedScraper(j, baseDir, TwitterScraperType)
	if err != nil {
		return nil, "", err
	}

	ts.statsCollector.Add(j.WorkerID, stats.TwitterScrapes, 1)
	tweets, nextCursor, err := scraper.FetchForYouTweets(count, cursor)
	if err != nil {
		_ = ts.handleError(j, err, account)
		return nil, "", err
	}

	ts.statsCollector.Add(j.WorkerID, stats.TwitterTweets, uint(len(tweets)))
	return tweets, nextCursor, nil
}
