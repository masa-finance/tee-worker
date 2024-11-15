package jobs

import (
	"fmt"
	"strings"

	"github.com/masa-finance/tee-worker/api/types"
)

const WebScraperType = "web-scraper"

type WebScraper struct {
	configuration WebScraperConfiguration
}

type WebScraperConfiguration struct {
	Blacklist []string `json:"webscraper_blacklist"`
}

type WebScraperArgs struct {
	URL string `json:"url"`
}

func NewWebScraper(jc types.JobConfiguration) *WebScraper {
	config := WebScraperConfiguration{}
	jc.Unmarshal(&config)
	return &WebScraper{configuration: config}
}

func (ws *WebScraper) ExecuteJob(j types.Job) (types.JobResult, error) {
	args := &WebScraperArgs{}
	j.Arguments.Unmarshal(args)

	for _, u := range ws.configuration.Blacklist {
		if strings.Contains(args.URL, u) {
			return types.JobResult{
				Error: fmt.Sprintf("URL blacklisted: %s", args.URL),
			}, nil
		}
	}

	// Do the web scraping here
	// For now, just return the URL
	return types.JobResult{
		Data: args.URL,
	}, nil
}
