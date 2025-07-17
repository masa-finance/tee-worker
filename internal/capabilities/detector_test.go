package capabilities

import (
	"reflect"
	"testing"

	"github.com/masa-finance/tee-worker/api/types"
)

// MockJobServer implements JobServerInterface for testing
type MockJobServer struct {
	capabilities types.WorkerCapabilities
}

func (m *MockJobServer) GetWorkerCapabilities() types.WorkerCapabilities {
	return m.capabilities
}

func TestDetectCapabilities(t *testing.T) {
	tests := []struct {
		name      string
		jc        types.JobConfiguration
		jobServer JobServerInterface
		expected  types.WorkerCapabilities
	}{
		{
			name: "With JobServer - gets capabilities from workers",
			jc:   types.JobConfiguration{},
			jobServer: &MockJobServer{
				capabilities: types.WorkerCapabilities{
					{Scraper: "web", Capabilities: []types.Capability{"web-scraper"}},
					{Scraper: "telemetry", Capabilities: []types.Capability{"telemetry"}},
					{Scraper: "tiktok", Capabilities: []types.Capability{"tiktok-transcription"}},
					{Scraper: "twitter", Capabilities: []types.Capability{"searchbyquery", "getbyid", "getprofilebyid"}},
				},
			},
			expected: types.WorkerCapabilities{
				{Scraper: "web", Capabilities: []types.Capability{"web-scraper"}},
				{Scraper: "telemetry", Capabilities: []types.Capability{"telemetry"}},
				{Scraper: "tiktok", Capabilities: []types.Capability{"tiktok-transcription"}},
				{Scraper: "twitter", Capabilities: []types.Capability{"searchbyquery", "getbyid", "getprofilebyid"}},
			},
		},
		{
			name:      "Without JobServer - basic capabilities only",
			jc:        types.JobConfiguration{},
			jobServer: nil,
			expected: types.WorkerCapabilities{
				{Scraper: "web", Capabilities: []types.Capability{"web-scraper"}},
				{Scraper: "telemetry", Capabilities: []types.Capability{"telemetry"}},
				{Scraper: "tiktok", Capabilities: []types.Capability{"tiktok-transcription"}},
			},
		},
		{
			name: "Without JobServer - with Twitter accounts",
			jc: types.JobConfiguration{
				"twitter_accounts": []string{"user1:pass1"},
			},
			jobServer: nil,
			expected: types.WorkerCapabilities{
				{Scraper: "web", Capabilities: []types.Capability{"web-scraper"}},
				{Scraper: "telemetry", Capabilities: []types.Capability{"telemetry"}},
				{Scraper: "tiktok", Capabilities: []types.Capability{"tiktok-transcription"}},
				{Scraper: "twitter-credential", Capabilities: []types.Capability{
					"searchbyquery", "searchbyfullarchive", "searchbyprofile", "searchfollowers",
					"getbyid", "getreplies", "getretweeters", "gettweets", "getmedia",
					"gethometweets", "getforyoutweets", "getbookmarks", "getprofilebyid",
					"gettrends", "getfollowing", "getfollowers", "getspace",
				}},
				{Scraper: "twitter", Capabilities: []types.Capability{
					"searchbyquery", "searchbyfullarchive", "searchbyprofile", "searchfollowers",
					"getbyid", "getreplies", "getretweeters", "gettweets", "getmedia",
					"gethometweets", "getforyoutweets", "getbookmarks", "getprofilebyid",
					"gettrends", "getfollowing", "getfollowers", "getspace",
				}},
			},
		},
		{
			name: "Without JobServer - with Twitter API keys",
			jc: types.JobConfiguration{
				"twitter_api_keys": []string{"key1"},
			},
			jobServer: nil,
			expected: types.WorkerCapabilities{
				{Scraper: "web", Capabilities: []types.Capability{"web-scraper"}},
				{Scraper: "telemetry", Capabilities: []types.Capability{"telemetry"}},
				{Scraper: "tiktok", Capabilities: []types.Capability{"tiktok-transcription"}},
				{Scraper: "twitter-api", Capabilities: []types.Capability{"searchbyquery", "getbyid", "getprofilebyid"}},
				{Scraper: "twitter", Capabilities: []types.Capability{"searchbyquery", "getbyid", "getprofilebyid"}},
			},
		},
		{
			name: "Without JobServer - with both accounts and API keys",
			jc: types.JobConfiguration{
				"twitter_accounts": []string{"user1:pass1"},
				"twitter_api_keys": []string{"key1"},
			},
			jobServer: nil,
			expected: types.WorkerCapabilities{
				{Scraper: "web", Capabilities: []types.Capability{"web-scraper"}},
				{Scraper: "telemetry", Capabilities: []types.Capability{"telemetry"}},
				{Scraper: "tiktok", Capabilities: []types.Capability{"tiktok-transcription"}},
				{Scraper: "twitter-credential", Capabilities: []types.Capability{
					"searchbyquery", "searchbyfullarchive", "searchbyprofile", "searchfollowers",
					"getbyid", "getreplies", "getretweeters", "gettweets", "getmedia",
					"gethometweets", "getforyoutweets", "getbookmarks", "getprofilebyid",
					"gettrends", "getfollowing", "getfollowers", "getspace",
				}},
				{Scraper: "twitter", Capabilities: []types.Capability{
					"searchbyquery", "searchbyfullarchive", "searchbyprofile", "searchfollowers",
					"getbyid", "getreplies", "getretweeters", "gettweets", "getmedia",
					"gethometweets", "getforyoutweets", "getbookmarks", "getprofilebyid",
					"gettrends", "getfollowing", "getfollowers", "getspace",
				}},
				{Scraper: "twitter-api", Capabilities: []types.Capability{"searchbyquery", "getbyid", "getprofilebyid"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectCapabilities(tt.jc, tt.jobServer)

			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("DetectCapabilities() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// Helper function to find a scraper capability by name
func findScraperCapability(capabilities types.WorkerCapabilities, scraperName string) *types.ScraperCapability {
	for _, cap := range capabilities {
		if cap.Scraper == scraperName {
			return &cap
		}
	}
	return nil
}

func TestDetectCapabilities_ScraperTypes(t *testing.T) {
	tests := []struct {
		name         string
		jc           types.JobConfiguration
		expectedKeys []string // scraper names we expect
	}{
		{
			name: "With accounts only",
			jc: types.JobConfiguration{
				"twitter_accounts": []string{"user:pass"},
			},
			expectedKeys: []string{"web", "telemetry", "tiktok", "twitter-credential", "twitter"},
		},
		{
			name: "With API keys only",
			jc: types.JobConfiguration{
				"twitter_api_keys": []string{"key123"},
			},
			expectedKeys: []string{"web", "telemetry", "tiktok", "twitter-api", "twitter"},
		},
		{
			name: "With both accounts and keys",
			jc: types.JobConfiguration{
				"twitter_accounts": []string{"user:pass"},
				"twitter_api_keys": []string{"key123"},
			},
			expectedKeys: []string{"web", "telemetry", "tiktok", "twitter-credential", "twitter", "twitter-api"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			caps := DetectCapabilities(tt.jc, nil)

			scraperNames := make([]string, len(caps))
			for i, cap := range caps {
				scraperNames[i] = cap.Scraper
			}

			// Check that all expected keys are present
			for _, expectedKey := range tt.expectedKeys {
				found := false
				for _, scraperName := range scraperNames {
					if scraperName == expectedKey {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected scraper %s not found in %v", expectedKey, scraperNames)
				}
			}

			// Check that no unexpected keys are present
			for _, scraperName := range scraperNames {
				found := false
				for _, expectedKey := range tt.expectedKeys {
					if scraperName == expectedKey {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Unexpected scraper %s found in %v", scraperName, scraperNames)
				}
			}
		})
	}
}
