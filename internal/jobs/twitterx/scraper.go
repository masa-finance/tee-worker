package twitterx

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/masa-finance/tee-worker/pkg/client"
	"github.com/sirupsen/logrus"
)

const (
	TweetsSearchRecent = "tweets/search/recent"
	TweetsAll          = "tweets/search/all"
)

type TwitterXScraper struct {
	twitterXClient *client.TwitterXClient
}

type TwitterXData struct {
	AuthorID string `json:"author_id"`
	Username string `json:"username,omitempty"` // Added username field
	Entities struct {
		Urls []struct {
			Start       int    `json:"start"`
			End         int    `json:"end"`
			URL         string `json:"url"`
			ExpandedURL string `json:"expanded_url"`
			DisplayURL  string `json:"display_url"`
		} `json:"urls"`
		Annotations []struct {
			Start          int     `json:"start"`
			End            int     `json:"end"`
			Probability    float64 `json:"probability"`
			Type           string  `json:"type"`
			NormalizedText string  `json:"normalized_text"`
		} `json:"annotations"`
	} `json:"entities"`
	ID                string `json:"id"`
	PossiblySensitive bool   `json:"possibly_sensitive"`
	ReplySettings     string `json:"reply_settings"`
	ConversationID    string `json:"conversation_id"`
	PublicMetrics     struct {
		RetweetCount    int `json:"retweet_count"`
		ReplyCount      int `json:"reply_count"`
		LikeCount       int `json:"like_count"`
		QuoteCount      int `json:"quote_count"`
		BookmarkCount   int `json:"bookmark_count"`
		ImpressionCount int `json:"impression_count"`
	} `json:"public_metrics"`
	EditControls struct {
		EditsRemaining int       `json:"edits_remaining"`
		IsEditEligible bool      `json:"is_edit_eligible"`
		EditableUntil  time.Time `json:"editable_until"`
	} `json:"edit_controls"`
	Text               string `json:"text"`
	ContextAnnotations []struct {
		Domain struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			Description string `json:"description"`
		} `json:"domain"`
		Entity struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"entity"`
	} `json:"context_annotations"`
	CreatedAt           time.Time `json:"created_at"`
	DisplayTextRange    []int     `json:"display_text_range"`
	Lang                string    `json:"lang"`
	EditHistoryTweetIds []string  `json:"edit_history_tweet_ids"`
	InReplyToUserID     string    `json:"in_reply_to_user_id,omitempty"`
	ReferencedTweets    []struct {
		Type string `json:"type"`
		ID   string `json:"id"`
	} `json:"referenced_tweets,omitempty"`
}

type TwitterMeta struct {
	NewestID    string `json:"newest_id"`
	OldestID    string `json:"oldest_id"`
	ResultCount int    `json:"result_count"`
	NextCursor  string `json:"next_token"`
}

// UserLookupResponse structure for the user lookup endpoint
type UserLookupResponse struct {
	Data struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		Username string `json:"username"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
		Code    int    `json:"code"`
		Title   string `json:"title"`
	} `json:"errors,omitempty"`
}
type TwitterXSearchQueryResult struct {
	Data   []TwitterXData `json:"data"`
	Meta   TwitterMeta    `json:"meta"`
	Errors []struct {
		Detail string `json:"detail"`
		Status int    `json:"status"`
		Title  string `json:"title"`
		Type   string `json:"type"`
	}
}

// SearchParams holds all possible search parameters
type SearchParams struct {
	Query       string   // The search query
	MaxResults  int      // Maximum number of results to return
	NextToken   string   // Token for getting the next page of results
	SinceID     string   // Returns results with a Tweet ID greater than this ID
	UntilID     string   // Returns results with a Tweet ID less than this ID
	TweetFields []string // Additional tweet fields to include
}

func NewTwitterXScraper(client *client.TwitterXClient) *TwitterXScraper {
	return &TwitterXScraper{
		twitterXClient: client,
	}
}

func (s *TwitterXScraper) ScrapeTweetsByQuery(baseQueryEndpoint string, query string, count int, cursor string) (*TwitterXSearchQueryResult, error) {
	switch baseQueryEndpoint {
	case TweetsAll:
		count = min(max(count, 10), 499)
	case TweetsSearchRecent:
		count = min(max(count, 10), 100)
	default:
		return nil, fmt.Errorf("Unsupported base query endpoint: %s", baseQueryEndpoint)
	}

	// Initialize the client
	client := s.twitterXClient

	// Create url.Values to handle all query parameters
	params := url.Values{}

	// Check if query has special characters and add quotes if needed
	if s.containsSpecialChars(query) && !strings.HasPrefix(query, "\"") && !strings.HasSuffix(query, "\"") {
		// Add quotes around the query
		query = fmt.Sprintf("\"%s\"", query)
		logrus.Debugf("Added quotes to query with special characters: %s", query)
	}

	// Add the query parameter (will be properly encoded)
	params.Add("query", query)

	params.Add("max_results", strconv.Itoa(count))

	// Add cursor if provided
	if cursor != "" {
		params.Add("next_token", cursor)
	}

	// Add tweet fields
	params.Add("tweet.fields", "created_at,author_id,public_metrics,context_annotations,geo,lang,possibly_sensitive,source,withheld,attachments,entities,conversation_id,in_reply_to_user_id,referenced_tweets,reply_settings,media_metadata,note_tweet,display_text_range,edit_controls,edit_history_tweet_ids,article,card_uri,community_id")

	// Add user fields
	params.Add("user.fields", "username,affiliation,connection_status,description,entities,id,is_identity_verified,location,most_recent_tweet_id,name,parody,pinned_tweet_id,profile_banner_url,profile_image_url,protected,public_metrics,receives_your_dm,subscription,subscription_type,url,verified,verified_followers_count,verified_type,withheld")

	// Add place fields
	params.Add("place.fields", "contained_within,country,country_code,full_name,geo,id,name,place_type")

	// Construct the final URL with all encoded parameters
	endpoint := baseQueryEndpoint + "?" + params.Encode()

	logrus.Debugf("Making request to endpoint: %s", endpoint)

	// Run the search
	response, err := client.Get(endpoint)
	if err != nil {
		logrus.Error("failed to execute search query: %w", err)
		return nil, fmt.Errorf("failed to execute search query: %w", err)
	}
	defer response.Body.Close()

	// Read the response body
	body, err := io.ReadAll(response.Body)
	if err != nil {
		logrus.Error("failed to read response body: %w", err)
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Check response status
	if response.StatusCode != http.StatusOK {
		logrus.Errorf("unexpected status code %d: %s", response.StatusCode, string(body))
		return nil, fmt.Errorf("unexpected status code %d: %s", response.StatusCode, string(body))
	}

	// Unmarshal the response
	var result TwitterXSearchQueryResult
	if err := json.Unmarshal(body, &result); err != nil {
		logrus.WithError(err).Error("failed to unmarshal response")
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Fetch usernames for each tweet author if there are results
	if len(result.Data) > 0 {
		if err := s.fetchUsernames(&result); err != nil {
			logrus.WithError(err).Warn("failed to fetch some usernames")
			// We'll continue even if username lookup fails for some users
		}
	}

	logrus.WithFields(logrus.Fields{
		"result_count": result.Meta.ResultCount,
		"newest_id":    result.Meta.NewestID,
		"oldest_id":    result.Meta.OldestID,
	}).Info("Successfully scraped tweets by query")

	return &result, nil
}

// Helper function to check if a string contains special characters
func (s *TwitterXScraper) containsSpecialChars(str string) bool {
	return strings.ContainsAny(str, "$@#!%^&*()+={}[]:;'\"\\|<>,.?/~` ")
}

// fetchUsernames retrieves the username for each author_id in the search results
func (s *TwitterXScraper) fetchUsernames(result *TwitterXSearchQueryResult) error {
	// Early return if no results
	if len(result.Data) == 0 {
		return nil
	}

	logrus.Infof("Fetching usernames for %d tweets", len(result.Data))

	// Create a map to track which author IDs we've already processed
	// to avoid duplicate lookups for the same author
	processedAuthors := make(map[string]string)

	// For each tweet in the result
	for i, tweet := range result.Data {
		// Skip if author ID is empty
		if tweet.AuthorID == "" {
			continue
		}

		// Check if we've already looked up this author
		if username, exists := processedAuthors[tweet.AuthorID]; exists {
			// Use the cached username
			result.Data[i].Username = username
			continue
		}

		// Look up the user by ID
		username, err := s.lookupUserByID(tweet.AuthorID)
		if err != nil {
			logrus.Warnf("Failed to lookup user ID %s: %v", tweet.AuthorID, err)
			continue
		}

		// Store the username in the tweet data
		result.Data[i].Username = username

		// Cache the username for potential reuse
		processedAuthors[tweet.AuthorID] = username

		// Add a small delay to avoid hitting rate limits
		time.Sleep(50 * time.Millisecond)
	}

	logrus.Infof("Successfully fetched usernames for tweets")
	return nil
}

// ScrapeTweetsByQueryExtended Example extended version that supports pagination and additional parameters
// lookupUserByID fetches user information by user ID
func (s *TwitterXScraper) lookupUserByID(userID string) (string, error) {
	logrus.Infof("Looking up user with ID: %s", userID)

	// Construct endpoint URL
	endpoint := fmt.Sprintf("users/%s", userID)

	// Make the request
	resp, err := s.twitterXClient.Get(endpoint)
	if err != nil {
		logrus.Errorf("Error looking up user: %v", err)
		return "", fmt.Errorf("error looking up user: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logrus.Errorf("Error reading response body: %v", err)
		return "", fmt.Errorf("error reading response body: %w", err)
	}

	// Parse response
	var userResp UserLookupResponse
	if err := json.Unmarshal(body, &userResp); err != nil {
		logrus.Errorf("Error parsing response: %v", err)
		return "", fmt.Errorf("error parsing response: %w", err)
	}

	// Check for errors
	if len(userResp.Errors) > 0 {
		logrus.Errorf("API error: %s (code: %d)", userResp.Errors[0].Message, userResp.Errors[0].Code)
		return "", fmt.Errorf("API error: %s (code: %d)", userResp.Errors[0].Message, userResp.Errors[0].Code)
	}

	// Check response status
	switch resp.StatusCode {
	case http.StatusOK:
		return userResp.Data.Username, nil
	case http.StatusUnauthorized:
		return "", fmt.Errorf("invalid API key")
	case http.StatusTooManyRequests:
		return "", fmt.Errorf("rate limit exceeded")
	case http.StatusNotFound:
		return "", fmt.Errorf("user not found")
	default:
		return "", fmt.Errorf("API user lookup failed with status: %d", resp.StatusCode)
	}
}
