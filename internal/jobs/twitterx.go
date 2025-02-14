package jobs

import (
	"encoding/json"
	"fmt"
	"github.com/masa-finance/tee-worker/api/types"
	"github.com/masa-finance/tee-worker/internal/jobs/stats"
	"github.com/masa-finance/tee-worker/internal/jobs/twitterx"
	client2 "github.com/masa-finance/tee-worker/pkg/client"

	"strings"
)

const TwitterXScraperType = "twitter-x-scraper"

type TwitterXScraper struct {
	configuration  TwitterScraperConfiguration
	apiKeyManager  *twitterx.TwitterXApiKeyManager
	statsCollector *stats.StatsCollector
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

func NewTwitterXScraper(jc types.JobConfiguration, c *stats.StatsCollector) *TwitterXScraper {
	config := TwitterScraperConfiguration{}
	jc.Unmarshal(&config)

	apiKeys := parseApiKeys(config.ApiKeys)
	apiKeyManager := twitterx.NewTwitterApiKeyManager(apiKeys)

	return &TwitterXScraper{
		configuration:  config,
		apiKeyManager:  apiKeyManager,
		statsCollector: c,
	}
}

func (tx *TwitterXScraper) Type() string {
	return TwitterXScraperType
}

func (tx *TwitterXScraper) StatsCollector() *stats.StatsCollector {
	return tx.statsCollector
}

// ScrapeTweetsByQuery implement search by query
func (tx *TwitterXScraper) ScrapeTweetsByQuery(query string) (*TwitterXSearchQueryResult, error) {
	tx.statsCollector.Add(stats.TwitterScrapes, 1)
	apiKey := tx.apiKeyManager.GetNextApiKey()

	if apiKey == nil {
		return nil, fmt.Errorf("no api keys available")
	}

	// initialize the client
	client := client2.NewTwitterXClient(apiKey.Key)

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

	tx.statsCollector.Add(stats.TwitterXSearchQueries, 1)

	return &result, nil

}

func (tx *TwitterXScraper) ExecuteJob(j types.Job) (types.JobResult, error) {
	args := &TwitterScraperArgs{}
	j.Arguments.Unmarshal(args)

	switch strings.ToLower(args.SearchType) {
	case "searchbyquery":
		result, err := tx.ScrapeTweetsByQuery(args.Query)
		if err != nil {
			return types.JobResult{Error: err.Error()}, err
		}

		dat, err := json.Marshal(result)
		return types.JobResult{
			Data: dat,
		}, err

	}

	return types.JobResult{
		Error: "invalid search type",
	}, fmt.Errorf("invalid search type")
}

func parseApiKeys(apiKeys []string) []*twitterx.TwitterXApiKey {
	return filterMap(apiKeys, func(key string) (*twitterx.TwitterXApiKey, bool) {
		return &twitterx.TwitterXApiKey{
			Key: strings.TrimSpace(key),
		}, true
	})
}
