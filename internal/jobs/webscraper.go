package jobs

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/gocolly/colly"
	"github.com/masa-finance/tee-worker/api/types"
	"github.com/masa-finance/tee-worker/internal/jobs/stats"
	"github.com/sirupsen/logrus"
)

const WebScraperType = "web-scraper"

type WebScraper struct {
	configuration WebScraperConfiguration
	stats         *stats.StatsCollector
}

type WebScraperConfiguration struct {
	Blacklist []string `json:"webscraper_blacklist"`
}

type WebScraperArgs struct {
	URL   string `json:"url"`
	Depth int    `json:"depth"`
}

func NewWebScraper(jc types.JobConfiguration, statsCollector *stats.StatsCollector) *WebScraper {
	config := WebScraperConfiguration{}
	jc.Unmarshal(&config)
	return &WebScraper{
		configuration: config,
		stats:         statsCollector,
	}
}

func (ws *WebScraper) ExecuteJob(j types.Job) (types.JobResult, error) {
	logrus.Info("Starting ExecuteJob for web scraper")

	// Step 1: Unmarshal arguments
	args := &WebScraperArgs{}
	logrus.Info("Unmarshaling job arguments")
	if err := j.Arguments.Unmarshal(args); err != nil {
		logrus.Errorf("Failed to unmarshal job arguments: %v", err)
		return types.JobResult{Error: fmt.Sprintf("Invalid arguments: %v", err)}, err
	}
	logrus.Infof("Job arguments unmarshaled successfully: %+v", args)

	// Step 2: Validate URL against blacklist
	logrus.Info("Validating URL against blacklist")
	for _, u := range ws.configuration.Blacklist {
		logrus.Debugf("Checking if URL contains blacklisted term: %s", u)
		if strings.Contains(args.URL, u) {
			logrus.Warnf("URL %s is blacklisted due to term: %s", args.URL, u)
			ws.stats.Add(j.WorkerID, stats.WebInvalid, 1)
			logrus.Errorf("Blacklisted URL: %s", args.URL)
			return types.JobResult{
				Error: fmt.Sprintf("URL blacklisted: %s", args.URL),
			}, nil
		}
	}
	logrus.Infof("URL %s passed blacklist validation", args.URL)

	// Step 3: Perform web scraping
	logrus.Infof("Initiating web scraping for URL: %s with depth: %d", args.URL, args.Depth)
	result, err := scrapeWeb([]string{args.URL}, args.Depth)
	if err != nil {
		logrus.Errorf("Web scraping failed for URL %s: %v", args.URL, err)
		ws.stats.Add(j.WorkerID, stats.WebErrors, 1)
		return types.JobResult{Error: err.Error()}, err
	}
	logrus.Infof("Web scraping succeeded for URL %s: %v", args.URL, result)

	// Step 4: Process result and return
	logrus.Info("Updating statistics for successful web scraping")
	ws.stats.Add(j.WorkerID, stats.WebSuccess, 1)
	logrus.Infof("Returning web scraping result for URL %s", args.URL)
	return types.JobResult{
		Data: result,
	}, nil
}

// Section represents a distinct part of a scraped webpage, typically defined by a heading.
// It contains a Title, representing the heading of the section, and Paragraphs, a slice of strings
// containing the text content found within that section.
type Section struct {
	Title      string   `json:"title"`      // Title is the heading text of the section.
	Paragraphs []string `json:"paragraphs"` // Paragraphs contains all the text content of the section.
	Images     []string `json:"images"`     // Images storing base64 - maybe!!?
}

// CollectedData represents the aggregated result of the scraping process.
// It contains a slice of Section structs, each representing a distinct part of a scraped webpage.
type CollectedData struct {
	Sections []Section `json:"sections"` // Sections is a collection of webpage sections that have been scraped.
	Pages    []string  `json:"pages"`
}

// scrapeWeb initiates the scraping process for the given list of URIs.
// It returns a CollectedData struct containing the scraped sections from each URI,
// and an error if any occurred during the scraping process.
//
// Parameters:
//   - uri: []string - list of URLs to scrape
//   - depth: int - depth of how many subpages to scrape
//
// Returns:
//   - []byte - JSON representation of the collected data
//   - error - any error that occurred during the scraping process
//
// Example usage:
//
//	go func() {
//		res, err := scraper.scrapeWeb([]string{"https://en.wikipedia.org/wiki/Maize"}, 5)
//		if err != nil {
//			logrus.WithError(err).Error("Error collecting data")
//			return
//		}
//		logrus.WithField("result", string(res)).Info("Scraping completed")
//	}()
func scrapeWeb(uri []string, depth int) ([]byte, error) {
	logrus.Infof("Starting scrapeWeb with parameters: URIs=%v, Depth=%d", uri, depth)
	// Set default depth to 1 if 0 is provided
	if depth <= 0 {
		logrus.Infof("Invalid depth (%d) provided, setting default depth to 1", depth)
		depth = 1
	}

	logrus.Info("Initializing CollectedData struct")
	var collectedData CollectedData

	logrus.Info("Creating new Colly collector")
	c := colly.NewCollector(
		colly.Async(true), // Enable asynchronous requests
		colly.AllowURLRevisit(),
		colly.IgnoreRobotsTxt(),
		colly.MaxDepth(depth),
	)
	logrus.Info("Colly collector created successfully")

	// Adjust the parallelism and delay based on your needs and server capacity
	logrus.Info("Setting scraping limits with parallelism and delay")
	limitRule := colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: 4,                      // Increased parallelism
		Delay:       500 * time.Millisecond, // Reduced delay
	}
	logrus.Info("Applying scraping limits to the collector")
	if err := c.Limit(&limitRule); err != nil {
		logrus.Errorf("[-] Unable to set scraper limit. Using default. Error: %v", err)
	}

	// Increase the timeout slightly if necessary
	logrus.Info("Setting request timeout to 240 seconds")
	c.SetRequestTimeout(240 * time.Second)

	// Initialize a backoff strategy
	logrus.Info("Initializing exponential backoff strategy")
	backoffStrategy := backoff.NewExponentialBackOff()

	logrus.Info("Registering OnError callback to handle request errors")
	c.OnError(func(r *colly.Response, err error) {
		logrus.Errorf("Error occurred during request to URL: %s. StatusCode: %d, Error: %v", r.Request.URL, r.StatusCode, err)
		if r.StatusCode == http.StatusTooManyRequests {
			// Parse the Retry-After header (in seconds)
			retryAfter, convErr := strconv.Atoi(r.Headers.Get("Retry-After"))
			if convErr != nil {
				// If not in seconds, it might be a date. Handle accordingly.
				logrus.Warnf("Retry-After header is present but unrecognized format: %s", r.Headers.Get("Retry-After"))
			}
			// Calculate the next delay
			nextDelay := backoffStrategy.NextBackOff()
			if retryAfter > 0 {
				nextDelay = time.Duration(retryAfter) * time.Second
			}
			logrus.Warnf("Rate limited for URL: %s. Retrying after %v", r.Request.URL, nextDelay)
			time.Sleep(nextDelay)
			// Retry the request
			logrus.Info("Retrying the request")
			_ = r.Request.Retry()

		} else {
			logrus.Errorf("Request failed for URL: %s with error: %v", r.Request.URL, err)
			logrus.Errorf("[-] Request URL: %s failed with error: %v", r.Request.URL, err)
		}
	})

	logrus.Info("Registering OnHTML callback for h1, h2 elements (titles)")
	c.OnHTML("h1, h2", func(e *colly.HTMLElement) {
		logrus.Infof("Title (h1/h2) found: %s", e.Text)
		// Directly append a new Section to collectedData.Sections
		collectedData.Sections = append(collectedData.Sections, Section{Title: e.Text})
	})

	logrus.Info("Registering OnHTML callback for paragraph elements")
	c.OnHTML("p", func(e *colly.HTMLElement) {
		logrus.Infof("Paragraph detected: %s", e.Text)
		// Check if there are any sections to append paragraphs to
		if len(collectedData.Sections) > 0 {
			// Get a reference to the last section
			lastSection := &collectedData.Sections[len(collectedData.Sections)-1]
			// Append the paragraph to the last section
			// Check for duplicate paragraphs before appending
			isDuplicate := false
			for _, paragraph := range lastSection.Paragraphs {
				if paragraph == e.Text {
					isDuplicate = true
					break
				}
			}
			// Handle dupes
			if !isDuplicate {
				lastSection.Paragraphs = append(lastSection.Paragraphs, e.Text)
			}
		}
	})

	logrus.Info("Registering OnHTML callback for image elements")
	c.OnHTML("img", func(e *colly.HTMLElement) {
		logrus.Infof("Image detected with source URL: %s", e.Attr("src"))
		imageURL := e.Request.AbsoluteURL(e.Attr("src"))
		if len(collectedData.Sections) > 0 {
			lastSection := &collectedData.Sections[len(collectedData.Sections)-1]
			lastSection.Images = append(lastSection.Images, imageURL)
		}
	})

	logrus.Info("Registering OnHTML callback for anchor elements")
	c.OnHTML("a", func(e *colly.HTMLElement) {
		logrus.Infof("Link detected: %s", e.Attr("href"))
		pageURL := e.Request.AbsoluteURL(e.Attr("href"))
		// Check if the URL protocol is supported (http or https)
		if strings.HasPrefix(pageURL, "http://") || strings.HasPrefix(pageURL, "https://") {
			collectedData.Pages = append(collectedData.Pages, pageURL)
			_ = e.Request.Visit(pageURL)
		}
	})

	logrus.Infof("Starting to visit URLs: %v", uri)
	for _, u := range uri {
		err := c.Visit(u)
		if err != nil {
			logrus.Errorf("Failed to visit URL: %s. Error: %v", u, err)
			continue
		}
		logrus.Infof("Visiting URL: %s", u)
		err = c.Visit(u)
		if err != nil {
			logrus.Errorf("Failed to visit URL: %s. Error: %v", u, err)
			return nil, err
		}
	}

	// Wait for all requests to finish
	logrus.Info("Waiting for all requests to complete")
	c.Wait()

	logrus.Info("Scraping completed, marshaling collected data into JSON format")
	j, _ := json.Marshal(collectedData)

	logrus.Infof("Scraping successful. Returning data for URIs: %v", uri)
	return j, nil
}
