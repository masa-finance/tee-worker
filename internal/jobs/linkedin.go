package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	linkedinscraper "github.com/masa-finance/linkedin-scraper"
	"github.com/masa-finance/tee-types/args"
	teetypes "github.com/masa-finance/tee-types/types"
	"github.com/masa-finance/tee-worker/api/types"
	"github.com/masa-finance/tee-worker/internal/jobs/stats"
	"github.com/sirupsen/logrus"
)

const (
	LinkedInScraperType = "linkedin-scraper"
)

type LinkedInScraper struct {
	configuration  LinkedInScraperConfiguration
	statsCollector *stats.StatsCollector
}

type LinkedInScraperConfiguration struct {
	Credentials []LinkedInCredential `json:"linkedin_credentials"`
	DataDir     string               `json:"data_dir"`
}

type LinkedInCredential struct {
	LiAtCookie string `json:"li_at_cookie"`
	CSRFToken  string `json:"csrf_token"`
	JSESSIONID string `json:"jsessionid"`
}

func NewLinkedInScraper(jc types.JobConfiguration, c *stats.StatsCollector) *LinkedInScraper {
	config := LinkedInScraperConfiguration{}
	if err := jc.Unmarshal(&config); err != nil {
		logrus.Errorf("Error unmarshalling LinkedIn scraper configuration: %v", err)
		return nil
	}

	// Parse credentials from environment variables if not in config
	if len(config.Credentials) == 0 {
		liAtCookie, _ := jc["linkedin_li_at_cookie"].(string)
		csrfToken, _ := jc["linkedin_csrf_token"].(string)
		jsessionID, _ := jc["linkedin_jsessionid"].(string)

		if liAtCookie != "" && csrfToken != "" {
			config.Credentials = append(config.Credentials, LinkedInCredential{
				LiAtCookie: liAtCookie,
				CSRFToken:  csrfToken,
				JSESSIONID: jsessionID,
			})
		}
	}

	return &LinkedInScraper{
		configuration:  config,
		statsCollector: c,
	}
}

// GetCapabilities returns the capabilities supported by the LinkedIn scraper
func (ls *LinkedInScraper) GetCapabilities() []string {
	return []string{"searchbyquery"}
}

func (ls *LinkedInScraper) ExecuteJob(j types.Job) (types.JobResult, error) {
	jobArgs := &args.LinkedInSearchArguments{}
	if err := j.Arguments.Unmarshal(jobArgs); err != nil {
		logrus.Errorf("Error while unmarshalling job arguments for job ID %s, type %s: %v", j.UUID, j.Type, err)
		return types.JobResult{Error: "error unmarshalling job arguments"}, err
	}

	// Validate we have credentials
	if len(ls.configuration.Credentials) == 0 {
		ls.statsCollector.Add(j.WorkerID, stats.LinkedInAuthErrors, 1)
		return types.JobResult{Error: "no LinkedIn credentials available"}, fmt.Errorf("no LinkedIn credentials configured")
	}

	// Get the first available credential (in future, implement rotation)
	cred := ls.configuration.Credentials[0]

	// Create LinkedIn client
	authCreds := linkedinscraper.AuthCredentials{
		LiAtCookie: cred.LiAtCookie,
		CSRFToken:  cred.CSRFToken,
		JSESSIONID: cred.JSESSIONID,
	}

	cfg, err := linkedinscraper.NewConfig(authCreds)
	if err != nil {
		ls.statsCollector.Add(j.WorkerID, stats.LinkedInAuthErrors, 1)
		return types.JobResult{Error: "failed to create LinkedIn config"}, err
	}

	client, err := linkedinscraper.NewClient(cfg)
	if err != nil {
		ls.statsCollector.Add(j.WorkerID, stats.LinkedInAuthErrors, 1)
		return types.JobResult{Error: "failed to create LinkedIn client"}, err
	}

	ls.statsCollector.Add(j.WorkerID, stats.LinkedInScrapes, 1)

	switch strings.ToLower(jobArgs.QueryType) {
	case "searchbyquery":
		return ls.searchProfiles(j, client, jobArgs)
	default:
		return types.JobResult{Error: "invalid search type: " + jobArgs.QueryType}, fmt.Errorf("invalid search type: %s", jobArgs.QueryType)
	}
}

func (ls *LinkedInScraper) searchProfiles(j types.Job, client *linkedinscraper.Client, args *args.LinkedInSearchArguments) (types.JobResult, error) {
	// Validate query is not empty
	if args.Query == "" {
		ls.statsCollector.Add(j.WorkerID, stats.LinkedInErrors, 1)
		return types.JobResult{Error: "query is required"}, fmt.Errorf("query is required")
	}

	searchArgs := linkedinscraper.ProfileSearchArgs{
		Keywords:       args.Query,
		NetworkFilters: args.NetworkFilters,
		Start:          args.Start,
		Count:          args.MaxResults,
	}

	// Set defaults if not provided
	if searchArgs.Count == 0 {
		searchArgs.Count = 10
	}
	if len(searchArgs.NetworkFilters) == 0 {
		searchArgs.NetworkFilters = []string{"F", "S", "O"} // All networks
	}

	ctx, cancel := context.WithTimeout(context.Background(), j.Timeout)
	defer cancel()

	profiles, err := client.SearchProfiles(ctx, searchArgs)
	if err != nil {
		// Check for specific error types
		if strings.Contains(err.Error(), "unauthorized") || strings.Contains(err.Error(), "401") {
			ls.statsCollector.Add(j.WorkerID, stats.LinkedInAuthErrors, 1)
		} else if strings.Contains(err.Error(), "rate limit") || strings.Contains(err.Error(), "429") {
			ls.statsCollector.Add(j.WorkerID, stats.LinkedInRateErrors, 1)
		} else {
			ls.statsCollector.Add(j.WorkerID, stats.LinkedInErrors, 1)
		}
		return types.JobResult{Error: fmt.Sprintf("failed to search profiles: %v", err)}, err
	}

	// Convert to our result type
	var results []teetypes.LinkedInProfileResult
	for _, profile := range profiles {
		result := teetypes.LinkedInProfileResult{
			PublicIdentifier: profile.PublicIdentifier,
			URN:              profile.URN,
			FullName:         profile.FullName,
			Headline:         profile.Headline,
			Location:         profile.Location,
			ProfileURL:       profile.ProfileURL,
			// Degree field will be empty for now since BadgeText is not available
			Degree: "",
		}
		results = append(results, result)
	}

	ls.statsCollector.Add(j.WorkerID, stats.LinkedInProfiles, uint(len(results)))

	data, err := json.Marshal(results)
	if err != nil {
		return types.JobResult{Error: "failed to marshal results"}, err
	}

	return types.JobResult{Data: data}, nil
}
