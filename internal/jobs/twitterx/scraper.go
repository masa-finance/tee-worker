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
)

const (
	TweetsSearchRecent = "tweets/search/recent"
)

type TwitterXScraper struct {
	twitterXClient *client.TwitterXClient
}

type TwitterXSearchQueryResult struct {
	Data []struct {
		ID                  string   `json:"id"`
		EditHistoryTweetIds []string `json:"edit_history_tweet_ids"`
		Text                string   `json:"text"`
	} `json:"data"`
	Meta struct {
		NewestID    string `json:"newest_id"`
		OldestID    string `json:"oldest_id"`
		ResultCount int    `json:"result_count"`
		NextToken   string `json:"next_token"`
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
func (s *TwitterXScraper) ScrapeTweetsByQuery(query string) (*TwitterXSearchQueryResult, error) {
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

	// run the search
	response, err := client.Get(endpoint)
	if err != nil {
		logrus.Error("failed to execute search query: %w", err)
		return nil, fmt.Errorf("failed to execute search query: %w", err)
	}
	defer response.Body.Close()

	// check response status
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(response.Body)
		logrus.Error("unexpected status code %d: %s", response.StatusCode, string(body))
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
		logrus.Error("failed to execute search query: %w", err)
		return nil, fmt.Errorf("failed to execute search query: %w", err)
	}
	defer response.Body.Close()

	// check response status
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(response.Body)
		logrus.Error("unexpected status code %d: %s", response.StatusCode, string(body))
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
