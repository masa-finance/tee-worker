package twitter

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// Client represents a Twitter API client
type Client struct {
	apiKey     string
	httpClient *http.Client
	baseURL    string
}

// NewClient creates a new Twitter API client
func NewClient(apiKey string) *Client {
	return &Client{
		apiKey:     apiKey,
		httpClient: &http.Client{},
		baseURL:    "https://api.x.com/2",
	}
}

// Tweet represents a Twitter post
type Tweet struct {
	ID            string                 `json:"id"`
	Text          string                 `json:"text"`
	CreatedAt     string                 `json:"created_at"`
	AuthorID      string                 `json:"author_id"`
	PublicMetrics map[string]interface{} `json:"public_metrics"`
}

type tweetResponse struct {
	Data []Tweet `json:"data"`
	Meta struct {
		NextToken string `json:"next_token"`
	} `json:"meta"`
}

// SearchTweets searches for tweets matching the query
func (c *Client) SearchTweets(ctx context.Context, query string, maxResults int) ([]Tweet, string, error) {
	endpoint := fmt.Sprintf("%s/tweets/search/recent", c.baseURL)

	params := url.Values{}
	params.Add("query", query)
	params.Add("max_results", fmt.Sprintf("%d", maxResults))
	params.Add("tweet.fields", "created_at,author_id,public_metrics")

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint+"?"+params.Encode(), nil)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var tweetResp tweetResponse
	if err := json.NewDecoder(resp.Body).Decode(&tweetResp); err != nil {
		return nil, "", fmt.Errorf("failed to decode response: %w", err)
	}

	return tweetResp.Data, tweetResp.Meta.NextToken, nil
}
