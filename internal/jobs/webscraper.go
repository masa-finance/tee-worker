package jobs

import (
	"github.com/masa-finance/tee-worker/api/types"
)

type WebScraper struct {
}

type WebScraperArgs struct {
	URL string `json:"url"`
}

func NewWebScraper() *WebScraper {
	return &WebScraper{}
}

func (ws *WebScraper) ExecuteJob(j types.Job) (types.JobResult, error) {
	args := &WebScraperArgs{}
	j.Arguments.Unmarshal(args)

	// Do the web scraping here
	// For now, just return the URL
	return types.JobResult{
		Data: args.URL,
	}, nil
}
