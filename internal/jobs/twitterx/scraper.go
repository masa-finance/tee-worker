package twitterx

import (
	"encoding/json"
	"fmt"
	"github.com/masa-finance/tee-worker/pkg/client"
	"github.com/sirupsen/logrus"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	TweetsSearchRecent = "tweets/search/recent"
)

type TwitterXScraper struct {
	twitterXClient *client.TwitterXClient
}

type TwitterXData struct {
	AuthorID string `json:"author_id"`
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
type TwitterXSearchQueryResult struct {
	Data []TwitterXData `json:"data"`
	Meta struct {
		NewestID    string `json:"newest_id"`
		OldestID    string `json:"oldest_id"`
		ResultCount int    `json:"result_count"`
	} `json:"meta"`
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

// ScrapeTweetsByQuery Alternative version using url.Values for more parameters
func (s *TwitterXScraper) ScrapeTweetsByQuery(query string, count int) (*TwitterXSearchQueryResult, error) {
	// initialize the client
	client := s.twitterXClient

	// construct the base URL
	baseURL := TweetsSearchRecent

	// create url.Values to properly handle query parameters
	params := url.Values{}

	// Add the raw query - url.Values.Add() will handle the encoding
	// This preserves Twitter search operators and special characters
	params.Add("query", query)

	// construct the final URL with encoded parameters
	endpoint := baseURL + "?" + params.Encode()

	// max_results
	//if count = 0, just return the first 10 results.query parameter value [2] is not between 10 and 100
	if count == 0 || count < 10 || count > 100 {
		logrus.Error("Invalid count value. Must be between 10 and 100")
		return nil, fmt.Errorf("invalid count value. Must be between 10 and 100")
	}

	endpoint = endpoint + "&max_results=" + strconv.Itoa(count)

	// include all possible fields - but note that the twitter api does not return all fields.
	// TODO: check the response and adjust the fields as needed
	endpoint = endpoint + "&tweet.fields=tweet.fields=created_at,author_id,public_metrics,context_annotations,geo,lang,possibly_sensitive,source,withheld,attachments,entities,conversation_id,in_reply_to_user_id,referenced_tweets,reply_settings,media_metadata,note_tweet,display_text_range,edit_controls,edit_history_tweet_ids,article,card_uri,community_id"
	endpoint = endpoint + "&user.fields=username,affiliation,connection_status,created_at,description,entities,id,is_identity_verified,location,most_recent_tweet_id,name,parody,pinned_tweet_id,profile_banner_url,profile_image_url,protected,public_metrics,receives_your_dm,subscription,subscription_type,url,verified,verified_followers_count,verified_type,withheld"
	endpoint = endpoint + "&place.fields=contained_within,country,country_code,full_name,geo,id,name,place_type"

	// sample
	//https://api.x.com/2/tweets/search/recent?query=Learn+how+to+use+the+user+Tweet+timeline&tweet.fields=created_at,author_id,public_metrics,context_annotations,geo,lang,possibly_sensitive,source,withheld,attachments,entities,conversation_id,in_reply_to_user_id,referenced_tweets,reply_settings,media_metadata,note_tweet,display_text_range,edit_controls,edit_history_tweet_ids,article,card_uri,community_id&user.fields=username,affiliation,connection_status,created_at,description,entities,id,is_identity_verified,location,most_recent_tweet_id,name,parody,pinned_tweet_id,profile_banner_url,profile_image_url,protected,public_metrics,receives_your_dm,subscription,subscription_type,url,verified,verified_followers_count,verified_type,withheld&place.fields=contained_within,country,country_code,full_name,geo,id,name,place_type

	// run the search
	response, err := client.Get(endpoint)
	if err != nil {
		logrus.Error("failed to execute search query: %w", err)
		return nil, fmt.Errorf("failed to execute search query: %w", err)
	}

	defer response.Body.Close()

	// read the response body
	var body []byte
	body, err = io.ReadAll(response.Body)
	if err != nil {
		logrus.Error("failed to read response body: %w", err)
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// check response status
	if response.StatusCode != http.StatusOK {
		logrus.Errorf("unexpected status code %d", response.StatusCode)
		return nil, fmt.Errorf("unexpected status code %d", response.StatusCode)
	}

	// unmarshal the response
	var result TwitterXSearchQueryResult
	if err := json.Unmarshal(body, &result); err != nil {
		logrus.WithError(err).Error("failed to unmarshal response")
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	logrus.WithFields(logrus.Fields{
		"result_count": result.Meta.ResultCount,
		"newest_id":    result.Meta.NewestID,
		"oldest_id":    result.Meta.OldestID,
	}).Info("Successfully scraped tweets by query")

	return &result, nil
}

// ScrapeTweetsByQueryExtended Example extended version that supports pagination and additional parameters
func (s *TwitterXScraper) ScrapeTweetsByQueryExtended(params SearchParams) (*TwitterXSearchQueryResult, error) {
	// initialize the client
	client := s.twitterXClient

	// construct the base URL
	baseURL := TweetsSearchRecent

	// create url.Values for parameter encoding
	queryParams := url.Values{}

	// Add the main search query
	queryParams.Add("query", params.Query)

	// Add optional parameters if present
	if params.MaxResults > 0 {
		queryParams.Add("max_results", strconv.Itoa(params.MaxResults))
	}
	if params.NextToken != "" {
		queryParams.Add("next_token", params.NextToken)
	}
	if params.SinceID != "" {
		queryParams.Add("since_id", params.SinceID)
	}
	if params.UntilID != "" {
		queryParams.Add("until_id", params.UntilID)
	}
	if len(params.TweetFields) > 0 {
		queryParams.Add("tweet.fields", strings.Join(params.TweetFields, ","))
	}

	// construct the final URL
	endpoint := baseURL + "?" + queryParams.Encode()

	// run the search
	response, err := client.Get(endpoint)
	if err != nil {
		logrus.Errorf("failed to execute search query: %s", err)
		return nil, fmt.Errorf("failed to execute search query: %w", err)
	}
	defer response.Body.Close()

	// check response status
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(response.Body)
		logrus.Errorf("unexpected status code %d: %s", response.StatusCode, string(body))
		return nil, fmt.Errorf("unexpected status code %d: %s", response.StatusCode, string(body))
	}

	// unmarshal the response
	var result TwitterXSearchQueryResult
	err = json.NewDecoder(response.Body).Decode(&result)
	if err != nil {
		logrus.Error("failed to decode response: %w", err)
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	logrus.Info("Successfully scraped tweets by query, result count: ", result.Meta.ResultCount)
	return &result, nil
}
