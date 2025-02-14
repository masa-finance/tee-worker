package twitterx

import (
	"encoding/json"
	"github.com/masa-finance/tee-worker/pkg/client"
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

func NewTwitterXScraper(client *client.TwitterXClient) *TwitterXScraper {
	return &TwitterXScraper{
		twitterXClient: client,
	}
}

func (s *TwitterXScraper) ScrapeTweetsByQuery(query string) (*TwitterXSearchQueryResult, error) {

	// initialize the client
	client := s.twitterXClient

	// run the search
	response, err := client.Get("/tweets/search/recent?query=" + query)

	if err != nil {
		return nil, nil
	}

	defer response.Body.Close()

	// unmarshal the response
	var result TwitterXSearchQueryResult
	err = json.NewDecoder(response.Body).Decode(&result)
	if err != nil {
		return nil, err
	}

	return &result, nil

}
