package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/masa-finance/tee-types/args"
	teetypes "github.com/masa-finance/tee-types/types"
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

func (ts *TwitterScraper) convertTwitterScraperTweetToTweetResult(tweet twitterscraper.Tweet) *teetypes.TweetResult {
	id, err := strconv.ParseInt(tweet.ID, 10, 64)
	if err != nil {
		logrus.Warnf("failed to convert tweet ID to int64: %s", tweet.ID)
		id = 0 // set to 0 if conversion fails
	}

	createdAt := time.Unix(tweet.Timestamp, 0).UTC()

	logrus.Debug("Converting Tweet ID: ", id) // Changed to Debug
	return &teetypes.TweetResult{
		ID:             id,
		TweetID:        tweet.ID,
		ConversationID: tweet.ConversationID,
		UserID:         tweet.UserID,
		Text:           tweet.Text,
		CreatedAt:      createdAt,
		Timestamp:      tweet.Timestamp,
		IsQuoted:       tweet.IsQuoted,
		IsPin:          tweet.IsPin,
		IsReply:        tweet.IsReply, // Corrected from tweet.IsPin
		IsRetweet:      tweet.IsRetweet,
		IsSelfThread:   tweet.IsSelfThread,
		Likes:          tweet.Likes,
		Hashtags:       tweet.Hashtags,
		HTML:           tweet.HTML,
		Replies:        tweet.Replies,
		Retweets:       tweet.Retweets,
		URLs:           tweet.URLs,
		Username:       tweet.Username,
		Photos: func() []teetypes.Photo {
			var photos []teetypes.Photo
			for _, photo := range tweet.Photos {
				photos = append(photos, teetypes.Photo{
					ID:  photo.ID,
					URL: photo.URL,
				})
			}
			return photos
		}(),
		Videos: func() []teetypes.Video {
			var videos []teetypes.Video
			for _, video := range tweet.Videos {
				videos = append(videos, teetypes.Video{
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
	if baseDir == "" {
		baseDir = ts.configuration.DataDir
	}

	var account *twitter.TwitterAccount
	var apiKey *twitter.TwitterApiKey
	var scraper *twitter.Scraper

	switch jobType {
	case TwitterCredentialScraperType:
		account = ts.accountManager.GetNextAccount()
		if account == nil {
			ts.statsCollector.Add(j.WorkerID, stats.TwitterAuthErrors, 1)
			return nil, nil, nil, fmt.Errorf("no Twitter credentials available for credential-based scraping")
		}
	case TwitterApiScraperType:
		apiKey = ts.accountManager.GetNextApiKey()
		if apiKey == nil {
			ts.statsCollector.Add(j.WorkerID, stats.TwitterAuthErrors, 1)
			return nil, nil, nil, fmt.Errorf("no Twitter API keys available for API-based scraping")
		}
	default: // TwitterScraperType
		logrus.Debug("Using standard Twitter scraper - prefer credentials if available")
		account = ts.accountManager.GetNextAccount()
		if account == nil {
			apiKey = ts.accountManager.GetNextApiKey()
			if apiKey == nil {
				ts.statsCollector.Add(j.WorkerID, stats.TwitterAuthErrors, 1)
				return nil, nil, nil, fmt.Errorf("no Twitter accounts or API keys available")
			}
		}
	}

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
		logrus.Info("Using API key only for this request")
	} else {
		return nil, nil, nil, fmt.Errorf("no authentication method available after selection logic")
	}
	return scraper, account, apiKey, nil
}

func (ts *TwitterScraper) handleError(j types.Job, err error, account *twitter.TwitterAccount) bool {
	if strings.Contains(err.Error(), "Rate limit exceeded") || strings.Contains(err.Error(), "status code 429") {
		ts.statsCollector.Add(j.WorkerID, stats.TwitterRateErrors, 1)
		if account != nil {
			ts.accountManager.MarkAccountRateLimited(account)
			logrus.Warnf("rate limited: %s", account.Username)
		} else {
			logrus.Warn("Rate limited (API Key or no specific account)")
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

func (ts *TwitterScraper) ScrapeFollowersForProfile(j types.Job, baseDir string, username string, count int) ([]*twitterscraper.Profile, error) {
	scraper, account, _, err := ts.getAuthenticatedScraper(j, baseDir, TwitterScraperType)
	if err != nil {
		return nil, err
	}
	if scraper == nil {
		return nil, fmt.Errorf("scraper not initialized for ScrapeFollowersForProfile")
	}

	ts.statsCollector.Add(j.WorkerID, stats.TwitterScrapes, 1)
	followingResponse, errString, _ := scraper.FetchFollowers(username, count, "")
	if errString != "" {
		fetchErr := fmt.Errorf("error fetching followers: %s", errString)
		if ts.handleError(j, fetchErr, account) {
			return nil, fetchErr
		}
		logrus.Errorf("[-] Error fetching followers: %s", errString)
		return nil, fetchErr
	}

	ts.statsCollector.Add(j.WorkerID, stats.TwitterProfiles, uint(len(followingResponse)))
	return followingResponse, nil
}

func (ts *TwitterScraper) ScrapeTweetsProfile(j types.Job, baseDir string, username string) (twitterscraper.Profile, error) {
	scraper, account, _, err := ts.getAuthenticatedScraper(j, baseDir, TwitterScraperType)
	if err != nil {
		return twitterscraper.Profile{}, err
	}
	if scraper == nil {
		return twitterscraper.Profile{}, fmt.Errorf("scraper not initialized for ScrapeTweetsProfile")
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

func (ts *TwitterScraper) ScrapeTweetsByFullArchiveSearchQuery(j types.Job, baseDir string, query string, count int) ([]*teetypes.TweetResult, error) {
	return ts.queryTweets(j, twitterx.TweetsAll, baseDir, query, count)
}

func (ts *TwitterScraper) ScrapeTweetsByRecentSearchQuery(j types.Job, baseDir string, query string, count int) ([]*teetypes.TweetResult, error) {
	return ts.queryTweets(j, twitterx.TweetsSearchRecent, baseDir, query, count)
}

func (ts *TwitterScraper) queryTweets(j types.Job, baseQueryEndpoint string, baseDir string, query string, count int) ([]*teetypes.TweetResult, error) {
	scraper, account, apiKey, err := ts.getAuthenticatedScraper(j, baseDir, TwitterScraperType)
	if err != nil {
		return nil, err
	}

	if account != nil && scraper != nil {
		return ts.scrapeTweetsWithCredentials(j, query, count, scraper, account)
	} else if apiKey != nil {
		return ts.scrapeTweetsWithApiKey(j, baseQueryEndpoint, query, count, apiKey)
	}
	return nil, fmt.Errorf("no valid authentication method (credentials or API key) found by getAuthenticatedScraper for queryTweets")
}

func (ts *TwitterScraper) queryTweetsWithCredentials(j types.Job, baseDir string, query string, count int) ([]*teetypes.TweetResult, error) {
	scraper, account, _, err := ts.getAuthenticatedScraper(j, baseDir, TwitterCredentialScraperType)
	if err != nil {
		return nil, err
	}
	if scraper == nil {
		return nil, fmt.Errorf("scraper not initialized for queryTweetsWithCredentials")
	}
	return ts.scrapeTweetsWithCredentials(j, query, count, scraper, account)
}

func (ts *TwitterScraper) queryTweetsWithApiKey(j types.Job, baseQueryEndpoint string, baseDir string, query string, count int) ([]*teetypes.TweetResult, error) {
	_, _, apiKey, err := ts.getAuthenticatedScraper(j, baseDir, TwitterApiScraperType)
	if err != nil {
		return nil, err
	}
	if apiKey == nil {
		return nil, fmt.Errorf("API key not available for queryTweetsWithApiKey")
	}
	return ts.scrapeTweetsWithApiKey(j, baseQueryEndpoint, query, count, apiKey)
}

func (ts *TwitterScraper) scrapeTweetsWithCredentials(j types.Job, query string, count int, scraper *twitter.Scraper, account *twitter.TwitterAccount) ([]*teetypes.TweetResult, error) {
	ts.statsCollector.Add(j.WorkerID, stats.TwitterScrapes, 1)
	tweets := make([]*teetypes.TweetResult, 0, count)

	ctx, cancel := context.WithTimeout(context.Background(), j.Timeout)
	defer cancel()

	scraper.SetSearchMode(twitterscraper.SearchLatest)

	for tweetScraped := range scraper.SearchTweets(ctx, query, count) {
		if tweetScraped.Error != nil {
			_ = ts.handleError(j, tweetScraped.Error, account)
			return nil, tweetScraped.Error
		}
		newTweetResult := ts.convertTwitterScraperTweetToTweetResult(tweetScraped.Tweet)
		tweets = append(tweets, newTweetResult)
	}

	ts.statsCollector.Add(j.WorkerID, stats.TwitterTweets, uint(len(tweets)))
	return tweets, nil
}

func (ts *TwitterScraper) scrapeTweetsWithApiKey(j types.Job, baseQueryEndpoint string, query string, count int, apiKey *twitter.TwitterApiKey) ([]*teetypes.TweetResult, error) {
	ts.statsCollector.Add(j.WorkerID, stats.TwitterScrapes, 1)

	if baseQueryEndpoint == twitterx.TweetsAll && apiKey.Type == twitter.TwitterApiKeyTypeBase {
		return nil, fmt.Errorf("this API key is a base/Basic key and does not have access to full archive search. Please use an elevated/Pro API key")
	}

	apiClient := client.NewTwitterXClient(apiKey.Key)
	twitterXScraper := twitterx.NewTwitterXScraper(apiClient)
	tweets := make([]*teetypes.TweetResult, 0, count)

	cursor := ""
	deadline := time.Now().Add(j.Timeout)

	for len(tweets) < count && time.Now().Before(deadline) {
		numToFetch := count - len(tweets)
		if numToFetch <= 0 {
			break
		}

		result, err := twitterXScraper.ScrapeTweetsByQuery(baseQueryEndpoint, query, numToFetch, cursor)
		if err != nil {
			if ts.handleError(j, err, nil) {
				if len(tweets) > 0 {
					logrus.Warnf("Rate limit hit, returning partial results (%d tweets) for query: %s", len(tweets), query)
					break
				}
			}
			return nil, err
		}

		if result == nil || len(result.Data) == 0 {
			if len(tweets) == 0 && err == nil {
				logrus.Infof("No tweets found for query: %s with API key.", query)
			}
			break
		}

		for _, tX := range result.Data {
			tweetIDInt, convErr := strconv.ParseInt(tX.ID, 10, 64)
			if convErr != nil {
				logrus.Errorf("Failed to convert tweet ID from twitterx '%s' to int64: %v", tX.ID, convErr)
				return nil, fmt.Errorf("failed to parse tweet ID '%s' from twitterx: %w", tX.ID, convErr)
			}

			newTweet := &teetypes.TweetResult{
				ID:             tweetIDInt,
				TweetID:        tX.ID,
				AuthorID:       tX.AuthorID,
				Text:           tX.Text,
				ConversationID: tX.ConversationID,
				UserID:         tX.AuthorID,
				CreatedAt:      tX.CreatedAt,
				Username:       tX.Username,
				Lang:           tX.Lang,
			}
			//if result.Meta != nil {
			newTweet.NewestID = result.Meta.NewestID
			newTweet.OldestID = result.Meta.OldestID
			newTweet.ResultCount = result.Meta.ResultCount
			//}

			//if tX.PublicMetrics != nil {
			newTweet.PublicMetrics = teetypes.PublicMetrics{
				RetweetCount:  tX.PublicMetrics.RetweetCount,
				ReplyCount:    tX.PublicMetrics.ReplyCount,
				LikeCount:     tX.PublicMetrics.LikeCount,
				QuoteCount:    tX.PublicMetrics.QuoteCount,
				BookmarkCount: tX.PublicMetrics.BookmarkCount,
			}
			//}
			// if tX.PossiblySensitive is available in twitterx.TweetData and teetypes.TweetResult has PossiblySensitive:
			// newTweet.PossiblySensitive = tX.PossiblySensitive
			// Also, fields like IsQuoted, Photos, Videos etc. would need to be populated if tX provides them.
			// Currently, this mapping is simpler than convertTwitterScraperTweetToTweetResult.

			tweets = append(tweets, newTweet)
			if len(tweets) >= count {
				goto EndLoop
			}
		}

		if result.Meta.NextCursor != "" {
			cursor = result.Meta.NextCursor
		} else {
			cursor = ""
		}

		if cursor == "" {
			break
		}
	}
EndLoop:

	logrus.Infof("Scraped %d tweets (target: %d) using API key for query: %s", len(tweets), count, query)
	ts.statsCollector.Add(j.WorkerID, stats.TwitterTweets, uint(len(tweets)))
	return tweets, nil
}

func (ts *TwitterScraper) ScrapeTweetByID(j types.Job, baseDir string, tweetID string) (*teetypes.TweetResult, error) {
	ts.statsCollector.Add(j.WorkerID, stats.TwitterScrapes, 1)

	scraper, account, _, err := ts.getAuthenticatedScraper(j, baseDir, TwitterScraperType)
	if err != nil {
		return nil, err
	}
	if scraper == nil {
		return nil, fmt.Errorf("scraper not initialized for ScrapeTweetByID")
	}

	tweet, err := scraper.GetTweet(tweetID)
	if err != nil {
		_ = ts.handleError(j, err, account)
		return nil, err
	}
	if tweet == nil {
		return nil, fmt.Errorf("tweet not found or error occurred, but error was nil")
	}

	tweetResult := ts.convertTwitterScraperTweetToTweetResult(*tweet)
	ts.statsCollector.Add(j.WorkerID, stats.TwitterTweets, 1)
	return tweetResult, nil
}

func (ts *TwitterScraper) GetTweet(j types.Job, baseDir, tweetID string) (*teetypes.TweetResult, error) {
	scraper, account, _, err := ts.getAuthenticatedScraper(j, baseDir, TwitterScraperType)
	if err != nil {
		return nil, err
	}
	if scraper == nil {
		return nil, fmt.Errorf("scraper not initialized for GetTweet")
	}

	ts.statsCollector.Add(j.WorkerID, stats.TwitterScrapes, 1)
	scrapedTweet, err := scraper.GetTweet(tweetID)
	if err != nil {
		_ = ts.handleError(j, err, account)
		return nil, err
	}
	if scrapedTweet == nil {
		return nil, fmt.Errorf("scrapedTweet not found or error occurred, but error was nil")
	}
	tweetResult := ts.convertTwitterScraperTweetToTweetResult(*scrapedTweet)
	ts.statsCollector.Add(j.WorkerID, stats.TwitterTweets, 1)
	return tweetResult, nil
}

func (ts *TwitterScraper) GetTweetReplies(j types.Job, baseDir, tweetID string, cursor string) ([]*teetypes.TweetResult, error) {
	scraper, account, _, err := ts.getAuthenticatedScraper(j, baseDir, TwitterScraperType)
	if err != nil {
		return nil, err
	}
	if scraper == nil {
		return nil, fmt.Errorf("scraper not initialized for GetTweetReplies")
	}

	ts.statsCollector.Add(j.WorkerID, stats.TwitterScrapes, 1)
	var replies []*teetypes.TweetResult

	scrapedTweets, threadEntries, err := scraper.GetTweetReplies(tweetID, cursor)
	if err != nil {
		_ = ts.handleError(j, err, account)
		return nil, err
	}

	for i, scrapedTweet := range scrapedTweets {
		newTweetResult := ts.convertTwitterScraperTweetToTweetResult(*scrapedTweet)
		if i < len(threadEntries) {
			// Assuming teetypes.TweetResult has a ThreadCursor field (struct, not pointer)
			newTweetResult.ThreadCursor.Cursor = threadEntries[i].Cursor
			newTweetResult.ThreadCursor.CursorType = threadEntries[i].CursorType
			newTweetResult.ThreadCursor.FocalTweetID = threadEntries[i].FocalTweetID
			newTweetResult.ThreadCursor.ThreadID = threadEntries[i].ThreadID
		}
		// Removed newTweetResult.Error = err as err is for the GetTweetReplies operation itself.
		replies = append(replies, newTweetResult)
	}

	ts.statsCollector.Add(j.WorkerID, stats.TwitterTweets, uint(len(replies)))
	return replies, nil
}

func (ts *TwitterScraper) GetTweetRetweeters(j types.Job, baseDir, tweetID string, count int, cursor string) ([]*twitterscraper.Profile, error) {
	scraper, account, _, err := ts.getAuthenticatedScraper(j, baseDir, TwitterScraperType)
	if err != nil {
		return nil, err
	}
	if scraper == nil {
		return nil, fmt.Errorf("scraper not initialized for GetTweetRetweeters")
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

func (ts *TwitterScraper) GetUserTweets(j types.Job, baseDir, username string, count int, cursor string) ([]*teetypes.TweetResult, string, error) {
	scraper, account, _, err := ts.getAuthenticatedScraper(j, baseDir, TwitterScraperType)
	if err != nil {
		return nil, "", err
	}
	if scraper == nil {
		return nil, "", fmt.Errorf("scraper not initialized for GetUserTweets")
	}
	ts.statsCollector.Add(j.WorkerID, stats.TwitterScrapes, 1)

	var tweets []*teetypes.TweetResult
	var nextCursor string

	if cursor != "" {
		fetchedTweets, fetchCursor, fetchErr := scraper.FetchTweets(username, count, cursor)
		if fetchErr != nil {
			_ = ts.handleError(j, fetchErr, account)
			return nil, "", fetchErr
		}
		for _, tweet := range fetchedTweets {
			newTweetResult := ts.convertTwitterScraperTweetToTweetResult(*tweet)
			tweets = append(tweets, newTweetResult)
		}
		nextCursor = fetchCursor
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), j.Timeout)
		defer cancel()
		for tweetScraped := range scraper.GetTweets(ctx, username, count) {
			if tweetScraped.Error != nil {
				_ = ts.handleError(j, tweetScraped.Error, account)
				return nil, "", tweetScraped.Error
			}
			newTweetResult := ts.convertTwitterScraperTweetToTweetResult(tweetScraped.Tweet)
			tweets = append(tweets, newTweetResult)
		}
		if len(tweets) > 0 {
			nextCursor = strconv.FormatInt(tweets[len(tweets)-1].ID, 10)
		}
	}
	ts.statsCollector.Add(j.WorkerID, stats.TwitterTweets, uint(len(tweets)))
	return tweets, nextCursor, nil
}

func (ts *TwitterScraper) GetUserMedia(j types.Job, baseDir, username string, count int, cursor string) ([]*teetypes.TweetResult, string, error) {
	scraper, account, _, err := ts.getAuthenticatedScraper(j, baseDir, TwitterScraperType)
	if err != nil {
		return nil, "", err
	}
	if scraper == nil {
		return nil, "", fmt.Errorf("scraper not initialized for GetUserMedia")
	}
	ts.statsCollector.Add(j.WorkerID, stats.TwitterScrapes, 1)

	var media []*teetypes.TweetResult
	var nextCursor string
	ctx, cancel := context.WithTimeout(context.Background(), j.Timeout)
	defer cancel()

	if cursor != "" {
		fetchedTweets, fetchCursor, fetchErr := scraper.FetchTweetsAndReplies(username, count, cursor)
		if fetchErr != nil {
			_ = ts.handleError(j, fetchErr, account)
			return nil, "", fetchErr
		}
		for _, tweet := range fetchedTweets {
			if len(tweet.Photos) > 0 || len(tweet.Videos) > 0 {
				newTweetResult := ts.convertTwitterScraperTweetToTweetResult(*tweet)
				media = append(media, newTweetResult)
			}
			if len(media) >= count {
				break
			}
		}
		nextCursor = fetchCursor
	} else {
		// Fetch more tweets initially as GetTweetsAndReplies doesn't guarantee 'count' media items.
		// Adjust multiplier as needed; it's a heuristic.
		initialFetchCount := count * 5
		if initialFetchCount == 0 && count > 0 { // handle count=0 case for initialFetchCount if count is very small
			initialFetchCount = 100 // a reasonable default if count is tiny but non-zero
		} else if count == 0 {
			initialFetchCount = 0 // if specifically asking for 0 media items
		}

		for tweetScraped := range scraper.GetTweetsAndReplies(ctx, username, initialFetchCount) {
			if tweetScraped.Error != nil {
				if ts.handleError(j, tweetScraped.Error, account) {
					return nil, "", tweetScraped.Error
				}
				continue
			}
			if len(tweetScraped.Tweet.Photos) > 0 || len(tweetScraped.Tweet.Videos) > 0 {
				newTweetResult := ts.convertTwitterScraperTweetToTweetResult(tweetScraped.Tweet)
				media = append(media, newTweetResult)
				if len(media) >= count && count > 0 { // ensure count > 0 for breaking
					break
				}
			}
		}
		if len(media) > 0 {
			nextCursor = strconv.FormatInt(media[len(media)-1].ID, 10)
		}
	}
	ts.statsCollector.Add(j.WorkerID, stats.TwitterOther, uint(len(media)))
	return media, nextCursor, nil
}

func (ts *TwitterScraper) GetHomeTweets(j types.Job, baseDir string, count int, cursor string) ([]*teetypes.TweetResult, string, error) {
	scraper, account, _, err := ts.getAuthenticatedScraper(j, baseDir, TwitterScraperType)
	if err != nil {
		return nil, "", err
	}
	if scraper == nil {
		return nil, "", fmt.Errorf("scraper not initialized for GetHomeTweets")
	}
	ts.statsCollector.Add(j.WorkerID, stats.TwitterScrapes, 1)

	var tweets []*teetypes.TweetResult
	var nextCursor string

	if cursor != "" {
		fetchedTweets, fetchCursor, fetchErr := scraper.FetchHomeTweets(count, cursor)
		if fetchErr != nil {
			_ = ts.handleError(j, fetchErr, account)
			return nil, "", fetchErr
		}
		for _, tweet := range fetchedTweets {
			newTweetResult := ts.convertTwitterScraperTweetToTweetResult(*tweet)
			tweets = append(tweets, newTweetResult)
		}
		nextCursor = fetchCursor
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), j.Timeout)
		defer cancel()
		for tweetScraped := range scraper.GetHomeTweets(ctx, count) {
			if tweetScraped.Error != nil {
				_ = ts.handleError(j, tweetScraped.Error, account)
				return nil, "", tweetScraped.Error
			}
			newTweetResult := ts.convertTwitterScraperTweetToTweetResult(tweetScraped.Tweet)
			tweets = append(tweets, newTweetResult)
			if len(tweets) >= count && count > 0 {
				break
			}
		}
		if len(tweets) > 0 {
			nextCursor = strconv.FormatInt(tweets[len(tweets)-1].ID, 10)
		}
	}
	ts.statsCollector.Add(j.WorkerID, stats.TwitterTweets, uint(len(tweets)))
	return tweets, nextCursor, nil
}

func (ts *TwitterScraper) GetForYouTweets(j types.Job, baseDir string, count int, cursor string) ([]*teetypes.TweetResult, string, error) {
	scraper, account, _, err := ts.getAuthenticatedScraper(j, baseDir, TwitterScraperType)
	if err != nil {
		return nil, "", err
	}
	if scraper == nil {
		return nil, "", fmt.Errorf("scraper not initialized for GetForYouTweets")
	}
	ts.statsCollector.Add(j.WorkerID, stats.TwitterScrapes, 1)

	var tweets []*teetypes.TweetResult
	var nextCursor string

	if cursor != "" {
		fetchedTweets, fetchCursor, fetchErr := scraper.FetchForYouTweets(count, cursor)
		if fetchErr != nil {
			_ = ts.handleError(j, fetchErr, account)
			return nil, "", fetchErr
		}
		for _, tweet := range fetchedTweets {
			newTweetResult := ts.convertTwitterScraperTweetToTweetResult(*tweet)
			tweets = append(tweets, newTweetResult)
		}
		nextCursor = fetchCursor
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), j.Timeout)
		defer cancel()
		for tweetScraped := range scraper.GetForYouTweets(ctx, count) {
			if tweetScraped.Error != nil {
				_ = ts.handleError(j, tweetScraped.Error, account)
				return nil, "", tweetScraped.Error
			}
			newTweetResult := ts.convertTwitterScraperTweetToTweetResult(tweetScraped.Tweet)
			tweets = append(tweets, newTweetResult)
			if len(tweets) >= count && count > 0 {
				break
			}
		}
		if len(tweets) > 0 {
			nextCursor = strconv.FormatInt(tweets[len(tweets)-1].ID, 10)
		}
	}
	ts.statsCollector.Add(j.WorkerID, stats.TwitterTweets, uint(len(tweets)))
	return tweets, nextCursor, nil
}

func (ts *TwitterScraper) GetBookmarks(j types.Job, baseDir string, count int, cursor string) ([]*teetypes.TweetResult, string, error) {
	scraper, account, _, err := ts.getAuthenticatedScraper(j, baseDir, TwitterScraperType)
	if err != nil {
		return nil, "", err
	}
	if scraper == nil {
		return nil, "", fmt.Errorf("scraper not initialized for GetBookmarks")
	}
	ts.statsCollector.Add(j.WorkerID, stats.TwitterScrapes, 1)
	var bookmarks []*teetypes.TweetResult

	ctx, cancel := context.WithTimeout(context.Background(), j.Timeout)
	defer cancel()
	cursorInt := 0
	if cursor != "" {
		var parseErr error
		cursorInt, parseErr = strconv.Atoi(cursor)
		if parseErr != nil {
			logrus.Warnf("Invalid cursor value for GetBookmarks '%s', using default 0: %v", cursor, parseErr)
			cursorInt = 0 // Ensure it's reset if parse fails
		}
	}
	for tweetScraped := range scraper.GetBookmarks(ctx, cursorInt) {
		if tweetScraped.Error != nil {
			_ = ts.handleError(j, tweetScraped.Error, account)
			return nil, "", tweetScraped.Error
		}
		newTweetResult := ts.convertTwitterScraperTweetToTweetResult(tweetScraped.Tweet)
		bookmarks = append(bookmarks, newTweetResult)
		if len(bookmarks) >= count && count > 0 {
			break
		}
	}

	var nextCursor string
	if len(bookmarks) > 0 {
		// The twitterscraper GetBookmarks cursor is an offset.
		// The next cursor should be the current offset + number of items fetched in this batch.
		nextCursor = strconv.Itoa(cursorInt + len(bookmarks))
	} else if cursor != "" {
		// If no bookmarks were fetched but a cursor was provided, retain it or signal no change
		nextCursor = cursor
	}

	ts.statsCollector.Add(j.WorkerID, stats.TwitterTweets, uint(len(bookmarks)))
	return bookmarks, nextCursor, nil
}

func (ts *TwitterScraper) GetProfileByID(j types.Job, baseDir, userID string) (*twitterscraper.Profile, error) {
	scraper, account, _, err := ts.getAuthenticatedScraper(j, baseDir, TwitterScraperType)
	if err != nil {
		return nil, err
	}
	if scraper == nil {
		return nil, fmt.Errorf("scraper not initialized for GetProfileByID")
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
	if scraper == nil {
		return nil, fmt.Errorf("scraper not initialized for SearchProfile")
	}

	ts.statsCollector.Add(j.WorkerID, stats.TwitterScrapes, 1)
	var profiles []*twitterscraper.ProfileResult
	ctx, cancel := context.WithTimeout(context.Background(), j.Timeout)
	defer cancel()

	for profile := range scraper.SearchProfiles(ctx, query, count) {
		profiles = append(profiles, profile)
		if len(profiles) >= count && count > 0 {
			break
		}
	}
	ts.statsCollector.Add(j.WorkerID, stats.TwitterProfiles, uint(len(profiles)))
	return profiles, nil
}

func (ts *TwitterScraper) GetTrends(j types.Job, baseDir string) ([]string, error) {
	scraper, account, _, err := ts.getAuthenticatedScraper(j, baseDir, TwitterScraperType)
	if err != nil {
		return nil, err
	}
	if scraper == nil {
		return nil, fmt.Errorf("scraper not initialized for GetTrends")
	}

	ts.statsCollector.Add(j.WorkerID, stats.TwitterScrapes, 1)
	trends, err := scraper.GetTrends()
	if err != nil {
		_ = ts.handleError(j, err, account)
		return nil, err
	}
	ts.statsCollector.Add(j.WorkerID, stats.TwitterOther, uint(len(trends)))
	return trends, nil
}

func (ts *TwitterScraper) GetFollowers(j types.Job, baseDir, user string, count int, cursor string) ([]*twitterscraper.Profile, string, error) {
	scraper, account, _, err := ts.getAuthenticatedScraper(j, baseDir, TwitterScraperType)
	if err != nil {
		return nil, "", err
	}
	if scraper == nil {
		return nil, "", fmt.Errorf("scraper not initialized for GetFollowers")
	}

	ts.statsCollector.Add(j.WorkerID, stats.TwitterScrapes, 1)
	followers, nextCursor, fetchErr := scraper.FetchFollowers(user, count, cursor)
	if fetchErr != nil {
		_ = ts.handleError(j, fetchErr, account)
		return nil, "", fetchErr
	}
	ts.statsCollector.Add(j.WorkerID, stats.TwitterProfiles, uint(len(followers)))
	return followers, nextCursor, nil
}

func (ts *TwitterScraper) GetFollowing(j types.Job, baseDir, username string, count int) ([]*twitterscraper.Profile, error) {
	scraper, account, _, err := ts.getAuthenticatedScraper(j, baseDir, TwitterScraperType)
	if err != nil {
		return nil, err
	}
	if scraper == nil {
		return nil, fmt.Errorf("scraper not initialized for GetFollowing")
	}

	ts.statsCollector.Add(j.WorkerID, stats.TwitterScrapes, 1)
	following, _, fetchErr := scraper.FetchFollowing(username, count, "")
	if fetchErr != nil {
		_ = ts.handleError(j, fetchErr, account) // Assuming FetchFollowing returns error, not errString
		return nil, fetchErr
	}
	ts.statsCollector.Add(j.WorkerID, stats.TwitterProfiles, uint(len(following)))
	return following, nil
}

func (ts *TwitterScraper) GetSpace(j types.Job, baseDir, spaceID string) (*twitterscraper.Space, error) {
	scraper, account, _, err := ts.getAuthenticatedScraper(j, baseDir, TwitterScraperType)
	if err != nil {
		return nil, err
	}
	if scraper == nil {
		return nil, fmt.Errorf("scraper not initialized for GetSpace")
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

const (
	TwitterScraperType           = "twitter-scraper"
	TwitterCredentialScraperType = "twitter-credential-scraper"
	TwitterApiScraperType        = "twitter-api-scraper"
)

type TwitterScraper struct {
	configuration struct {
		Accounts              []string `json:"twitter_accounts"`
		ApiKeys               []string `json:"twitter_api_keys"`
		DataDir               string   `json:"data_dir"`
		SkipLoginVerification bool     `json:"skip_login_verification,omitempty"`
	}
	accountManager *twitter.TwitterAccountManager
	statsCollector *stats.StatsCollector
	capabilities   map[string]bool
}

func NewTwitterScraper(jc types.JobConfiguration, c *stats.StatsCollector) *TwitterScraper {
	config := struct {
		Accounts              []string `json:"twitter_accounts"`
		ApiKeys               []string `json:"twitter_api_keys"`
		DataDir               string   `json:"data_dir"`
		SkipLoginVerification bool     `json:"skip_login_verification,omitempty"`
	}{}
	if err := jc.Unmarshal(&config); err != nil {
		logrus.Errorf("Error unmarshalling Twitter scraper configuration: %v", err)
		return nil
	}

	accounts := parseAccounts(config.Accounts)
	apiKeys := parseApiKeys(config.ApiKeys)
	accountManager := twitter.NewTwitterAccountManager(accounts, apiKeys)
	accountManager.DetectAllApiKeyTypes()

	if os.Getenv("TWITTER_SKIP_LOGIN_VERIFICATION") == "true" {
		config.SkipLoginVerification = true
	}

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

type TwitterScrapeStrategy interface {
	Execute(j types.Job, ts *TwitterScraper, jobArgs *args.TwitterSearchArguments) (types.JobResult, error)
}

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

type CredentialScrapeStrategy struct{}

func (s *CredentialScrapeStrategy) Execute(j types.Job, ts *TwitterScraper, jobArgs *args.TwitterSearchArguments) (types.JobResult, error) {
	switch strings.ToLower(jobArgs.QueryType) {
	case "searchbyquery":
		tweets, err := ts.queryTweetsWithCredentials(j, ts.configuration.DataDir, jobArgs.Query, jobArgs.MaxResults)
		return processResponse(tweets, "", err)
	case "searchbyfullarchive":
		logrus.Warn("Full archive search with credential-only implementation may have limited results")
		tweets, err := ts.queryTweetsWithCredentials(j, ts.configuration.DataDir, jobArgs.Query, jobArgs.MaxResults)
		return processResponse(tweets, "", err)
	default:
		return defaultStrategyFallback(j, ts, jobArgs)
	}
}

type ApiKeyScrapeStrategy struct{}

func (s *ApiKeyScrapeStrategy) Execute(j types.Job, ts *TwitterScraper, jobArgs *args.TwitterSearchArguments) (types.JobResult, error) {
	switch strings.ToLower(jobArgs.QueryType) {
	case "searchbyquery":
		tweets, err := ts.queryTweetsWithApiKey(j, twitterx.TweetsSearchRecent, ts.configuration.DataDir, jobArgs.Query, jobArgs.MaxResults)
		return processResponse(tweets, "", err)
	case "searchbyfullarchive":
		tweets, err := ts.queryTweetsWithApiKey(j, twitterx.TweetsAll, ts.configuration.DataDir, jobArgs.Query, jobArgs.MaxResults)
		return processResponse(tweets, "", err)
	default:
		return defaultStrategyFallback(j, ts, jobArgs)
	}
}

type DefaultScrapeStrategy struct{}

func (s *DefaultScrapeStrategy) Execute(j types.Job, ts *TwitterScraper, jobArgs *args.TwitterSearchArguments) (types.JobResult, error) {
	switch strings.ToLower(jobArgs.QueryType) {
	case "searchbyquery":
		tweets, err := ts.queryTweets(j, twitterx.TweetsSearchRecent, ts.configuration.DataDir, jobArgs.Query, jobArgs.MaxResults)
		return processResponse(tweets, "", err)
	case "searchbyfullarchive":
		tweets, err := ts.queryTweets(j, twitterx.TweetsAll, ts.configuration.DataDir, jobArgs.Query, jobArgs.MaxResults)
		return processResponse(tweets, "", err)
	default:
		return defaultStrategyFallback(j, ts, jobArgs)
	}
}

func retryWithCursor[T any](
	j types.Job,
	baseDir string,
	count int,
	cursor string,
	fn func(j types.Job, baseDir string, currentCount int, currentCursor string) ([]*T, string, error),
) (types.JobResult, error) {
	records := make([]*T, 0, count)
	deadline := time.Now().Add(j.Timeout)
	currentCursor := cursor // Use 'currentCursor' to manage pagination state within the loop

	for (len(records) < count || count == 0) && time.Now().Before(deadline) { // Allow count == 0 to fetch all available up to timeout
		numToFetch := count - len(records)
		if count == 0 { // If count is 0, fetch a reasonable batch size, e.g. 100, or let fn decide
			numToFetch = 100 // Or another default batch size if fn doesn't handle count=0 well for batching
		}
		if numToFetch <= 0 && count > 0 {
			break
		}

		results, nextInternalCursor, err := fn(j, baseDir, numToFetch, currentCursor)
		if err != nil {
			if len(records) > 0 {
				logrus.Warnf("Error during paginated fetch, returning partial results. Error: %v", err)
				return processResponse(records, currentCursor, nil)
			}
			return processResponse(nil, "", err)
		}

		if len(results) > 0 {
			records = append(records, results...)
		}

		if nextInternalCursor == "" || nextInternalCursor == currentCursor { // No more pages or cursor stuck
			currentCursor = nextInternalCursor // Update to the last known cursor
			break
		}
		currentCursor = nextInternalCursor
		if count > 0 && len(records) >= count { // Check if desired count is reached
			break
		}
	}
	return processResponse(records, currentCursor, nil)
}

func retryWithCursorAndQuery[T any](
	j types.Job,
	baseDir string,
	query string,
	count int,
	cursor string,
	fn func(j types.Job, baseDir string, currentQuery string, currentCount int, currentCursor string) ([]*T, string, error),
) (types.JobResult, error) {
	return retryWithCursor(
		j,
		baseDir,
		count,
		cursor,
		func(jInner types.Job, baseDirInner string, currentCountInner int, currentCursorInner string) ([]*T, string, error) {
			return fn(jInner, baseDirInner, query, currentCountInner, currentCursorInner)
		},
	)
}

func processResponse(response any, nextCursor string, err error) (types.JobResult, error) {
	if err != nil {
		logrus.Debugf("Processing response with error: %v, NextCursor: %s", err, nextCursor)
		return types.JobResult{Error: err.Error(), NextCursor: nextCursor}, err
	}
	dat, marshalErr := json.Marshal(response)
	if marshalErr != nil {
		logrus.Errorf("Error marshalling response: %v", marshalErr)
		return types.JobResult{Error: marshalErr.Error()}, marshalErr
	}
	return types.JobResult{Data: dat, NextCursor: nextCursor}, nil
}

func defaultStrategyFallback(j types.Job, ts *TwitterScraper, jobArgs *args.TwitterSearchArguments) (types.JobResult, error) {
	switch strings.ToLower(jobArgs.QueryType) {
	case "searchbyprofile":
		profile, err := ts.ScrapeTweetsProfile(j, ts.configuration.DataDir, jobArgs.Query)
		return processResponse(profile, "", err)
	case "searchfollowers": // This is for ScrapeFollowersForProfile which is not paginated by cursor in this context
		followers, err := ts.ScrapeFollowersForProfile(j, ts.configuration.DataDir, jobArgs.Query, jobArgs.MaxResults)
		return processResponse(followers, "", err)
	case "getbyid":
		tweet, err := ts.ScrapeTweetByID(j, ts.configuration.DataDir, jobArgs.Query)
		return processResponse(tweet, "", err)
	case "getreplies":
		// GetTweetReplies takes a cursor for a specific part of a thread, not general pagination of all replies.
		// The retryWithCursor logic might not directly apply unless GetTweetReplies is adapted for broader pagination.
		replies, err := ts.GetTweetReplies(j, ts.configuration.DataDir, jobArgs.Query, jobArgs.NextCursor)
		return processResponse(replies, jobArgs.NextCursor, err) // Pass original NextCursor as it's specific
	case "getretweeters":
		// Similar to GetTweetReplies, cursor is for a specific page.
		retweeters, err := ts.GetTweetRetweeters(j, ts.configuration.DataDir, jobArgs.Query, jobArgs.MaxResults, jobArgs.NextCursor)
		// GetTweetRetweeters in twitterscraper returns (profiles, nextCursorStr, error)
		// The current ts.GetTweetRetweeters doesn't return the next cursor. This should be updated if pagination is needed here.
		// For now, assuming it fetches one batch or handles its own pagination internally up to MaxResults.
		return processResponse(retweeters, "", err) // Assuming no next cursor from this specific call structure
	case "gettweets":
		return retryWithCursorAndQuery(j, ts.configuration.DataDir, jobArgs.Query, jobArgs.MaxResults, jobArgs.NextCursor, ts.GetUserTweets)
	case "getmedia":
		return retryWithCursorAndQuery(j, ts.configuration.DataDir, jobArgs.Query, jobArgs.MaxResults, jobArgs.NextCursor, ts.GetUserMedia)
	case "gethometweets":
		return retryWithCursor(j, ts.configuration.DataDir, jobArgs.MaxResults, jobArgs.NextCursor, ts.GetHomeTweets)
	case "getforyoutweets":
		return retryWithCursor(j, ts.configuration.DataDir, jobArgs.MaxResults, jobArgs.NextCursor, ts.GetForYouTweets)
	case "getbookmarks":
		return retryWithCursor(j, ts.configuration.DataDir, jobArgs.MaxResults, jobArgs.NextCursor, ts.GetBookmarks)
	case "getprofilebyid":
		profile, err := ts.GetProfileByID(j, ts.configuration.DataDir, jobArgs.Query)
		return processResponse(profile, "", err)
	case "gettrends":
		trends, err := ts.GetTrends(j, ts.configuration.DataDir)
		return processResponse(trends, "", err)
	case "getfollowing":
		following, err := ts.GetFollowing(j, ts.configuration.DataDir, jobArgs.Query, jobArgs.MaxResults)
		return processResponse(following, "", err)
	case "getfollowers":
		return retryWithCursorAndQuery(j, ts.configuration.DataDir, jobArgs.Query, jobArgs.MaxResults, jobArgs.NextCursor, ts.GetFollowers)
	case "getspace":
		space, err := ts.GetSpace(j, ts.configuration.DataDir, jobArgs.Query)
		return processResponse(space, "", err)
	}
	return types.JobResult{Error: "invalid search type in defaultStrategyFallback: " + jobArgs.QueryType}, fmt.Errorf("invalid search type: %s", jobArgs.QueryType)
}

func (ts *TwitterScraper) ExecuteJob(j types.Job) (types.JobResult, error) {
	jobArgs := &args.TwitterSearchArguments{}
	if err := j.Arguments.Unmarshal(jobArgs); err != nil {
		logrus.Errorf("Error while unmarshalling job arguments for job ID %s, type %s: %v", j.UUID, j.Type, err)
		return types.JobResult{Error: err.Error()}, err
	}
	strategy := getScrapeStrategy(j.Type)
	return strategy.Execute(j, ts, jobArgs)
}

func (ts *TwitterScraper) FetchHomeTweets(j types.Job, baseDir string, count int, cursor string) ([]*twitterscraper.Tweet, string, error) {
	scraper, account, _, err := ts.getAuthenticatedScraper(j, baseDir, TwitterScraperType)
	if err != nil {
		return nil, "", err
	}
	if scraper == nil {
		return nil, "", fmt.Errorf("scraper not initialized for FetchHomeTweets")
	}

	ts.statsCollector.Add(j.WorkerID, stats.TwitterScrapes, 1)
	tweets, nextCursor, fetchErr := scraper.FetchHomeTweets(count, cursor)
	if fetchErr != nil {
		_ = ts.handleError(j, fetchErr, account)
		return nil, "", fetchErr
	}

	ts.statsCollector.Add(j.WorkerID, stats.TwitterTweets, uint(len(tweets)))
	return tweets, nextCursor, nil
}

func (ts *TwitterScraper) FetchForYouTweets(j types.Job, baseDir string, count int, cursor string) ([]*twitterscraper.Tweet, string, error) {
	scraper, account, _, err := ts.getAuthenticatedScraper(j, baseDir, TwitterScraperType)
	if err != nil {
		return nil, "", err
	}
	if scraper == nil {
		return nil, "", fmt.Errorf("scraper not initialized for FetchForYouTweets")
	}

	ts.statsCollector.Add(j.WorkerID, stats.TwitterScrapes, 1)
	tweets, nextCursor, fetchErr := scraper.FetchForYouTweets(count, cursor)
	if fetchErr != nil {
		_ = ts.handleError(j, fetchErr, account)
		return nil, "", fetchErr
	}

	ts.statsCollector.Add(j.WorkerID, stats.TwitterTweets, uint(len(tweets)))
	return tweets, nextCursor, nil
}
