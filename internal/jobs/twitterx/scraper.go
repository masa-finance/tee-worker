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

type TwitterXSearchQueryResult struct {
	Data []struct {
		AuthorID  string `json:"author_id"`
		CreatedAt string `json:"created_at"`
		ID        string `json:"id"`
		Text      string `json:"text"`
		Username  string `json:"username"`
	} `json:"data"`
	Errors []struct {
		Detail string `json:"detail"`
		Status int    `json:"status"`
		Title  string `json:"title"`
		Type   string `json:"type"`
	} `json:"errors"`
	Includes struct {
		Media []struct {
			Height   int    `json:"height"`
			MediaKey string `json:"media_key"`
			Type     string `json:"type"`
			Width    int    `json:"width"`
		} `json:"media"`
		Places []struct {
			ContainedWithin []string `json:"contained_within"`
			Country         string   `json:"country"`
			CountryCode     string   `json:"country_code"`
			FullName        string   `json:"full_name"`
			Geo             struct {
				Bbox     []float64 `json:"bbox"`
				Geometry struct {
					Coordinates []float64 `json:"coordinates"`
					Type        string    `json:"type"`
				} `json:"geometry"`
				Properties struct {
				} `json:"properties"`
				Type string `json:"type"`
			} `json:"geo"`
			ID        string `json:"id"`
			Name      string `json:"name"`
			PlaceType string `json:"place_type"`
		} `json:"places"`
		Polls []struct {
			DurationMinutes int       `json:"duration_minutes"`
			EndDatetime     time.Time `json:"end_datetime"`
			ID              string    `json:"id"`
			Options         []struct {
				Label    string `json:"label"`
				Position int    `json:"position"`
				Votes    int    `json:"votes"`
			} `json:"options"`
			VotingStatus string `json:"voting_status"`
		} `json:"polls"`
		Topics []struct {
			Description string `json:"description"`
			ID          string `json:"id"`
			Name        string `json:"name"`
		} `json:"topics"`
		Tweets []struct {
			AuthorID  string `json:"author_id"`
			CreatedAt string `json:"created_at"`
			ID        string `json:"id"`
			Text      string `json:"text"`
			Username  string `json:"username"`
		} `json:"tweets"`
		Users []struct {
			CreatedAt time.Time `json:"created_at"`
			ID        string    `json:"id"`
			Name      string    `json:"name"`
			Protected bool      `json:"protected"`
			Username  string    `json:"username"`
		} `json:"users"`
	} `json:"includes"`
	Meta struct {
		NewestID    string `json:"newest_id"`
		NextToken   string `json:"next_token"`
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

// NewTwitterXScraper creates a new TwitterXScraper with the given TwitterXClient.
//
// A TwitterXScraper is used to scrape tweets from Twitter using the Twitter API v2.
// The TwitterXClient is used to make requests to the API.
//
// After creating a TwitterXScraper, you can use its methods to scrape tweets.
// For example, you can use ScrapeTweetsByQuery to scrape tweets by query.
func NewTwitterXScraper(client *client.TwitterXClient) *TwitterXScraper {
	return &TwitterXScraper{
		twitterXClient: client,
	}
}

// ScrapeTweetsByQuery scrapes tweets by query using the Twitter API v2.
//
// ScrapeTweetsByQuery makes a GET request to the Twitter API v2's "tweets/search/recent" endpoint.
// It uses the provided query parameter to construct the request URL.
// The query parameter is added to the URL without encoding or escaping.
// This preserves Twitter search operators and special characters.
//
// ScrapeTweetsByQuery returns a pointer to a TwitterXSearchQueryResult, which contains the results of the search.
// The TwitterXSearchQueryResult contains a Meta field with information about the search results.
// The Meta field has the following fields:
// - NewestID: The newest tweet ID in the results
// - NextToken: The next token to get the next page of results
// - OldestID: The oldest tweet ID in the results
// - ResultCount: The number of results returned
//
// If there is an error making the request or processing the response, ScrapeTweetsByQuery returns an error.
//
// Example:
//
//	scraper := NewTwitterXScraper(client)
//	result, err := scraper.ScrapeTweetsByQuery("from:twitterdev")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	log.Printf("newest_id: %s, oldest_id: %s, result_count: %d", result.Meta.NewestID, result.Meta.OldestID, result.Meta.ResultCount)
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

// ScrapeTweetsByQueryExtended scrapes tweets by query using the Twitter API v2.
//
// ScrapeTweetsByQueryExtended makes a GET request to the Twitter API v2's "tweets/search/recent" endpoint.
// It uses the provided SearchParams to construct the request URL.
// The query parameter is added to the URL without encoding or escaping.
// This preserves Twitter search operators and special characters.
//
// ScrapeTweetsByQueryExtended returns a pointer to a TwitterXSearchQueryResult, which contains the results of the search.
// The TwitterXSearchQueryResult contains a Meta field with information about the search results.
// The Meta field has the following fields:
// - NewestID: The newest tweet ID in the results
// - NextToken: The next token to get the next page of results
// - OldestID: The oldest tweet ID in the results
// - ResultCount: The number of results returned
//
// If there is an error making the request or processing the response, ScrapeTweetsByQueryExtended returns an error.
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
