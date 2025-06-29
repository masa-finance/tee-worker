package capabilities

import (
	"reflect"
	"sort"
	"testing"

	"github.com/masa-finance/tee-worker/api/types"
)

// MockJobServer implements JobServerInterface for testing
type MockJobServer struct {
	capabilities map[string][]string
}

func (m *MockJobServer) GetWorkerCapabilities() map[string][]string {
	return m.capabilities
}

func TestDetectCapabilities(t *testing.T) {
	tests := []struct {
		name      string
		jc        types.JobConfiguration
		jobServer JobServerInterface
		expected  []ScraperCapabilities
	}{
		{
			name: "With JobServer - gets capabilities from workers",
			jc:   types.JobConfiguration{},
			jobServer: &MockJobServer{
				capabilities: map[string][]string{
					"web-scraper":          {"web-scraper"},
					"telemetry":            {"telemetry"},
					"tiktok-transcription": {"tiktok-transcription"},
					"twitter-scraper":      {"searchbyquery", "getbyid", "getprofilebyid"},
					"linkedin-scraper":     {"searchbyquery", "getprofile"},
				},
			},
			expected: []ScraperCapabilities{
				{Scraper: "web-scraper", Capabilities: []string{"web-scraper"}},
				{Scraper: "telemetry", Capabilities: []string{"telemetry"}},
				{Scraper: "tiktok-transcription", Capabilities: []string{"tiktok-transcription"}},
				{Scraper: "twitter-scraper", Capabilities: []string{"searchbyquery", "getbyid", "getprofilebyid"}},
				{Scraper: "linkedin-scraper", Capabilities: []string{"searchbyquery", "getprofile"}},
			},
		},
		{
			name:      "Without JobServer - basic capabilities only",
			jc:        types.JobConfiguration{},
			jobServer: nil,
			expected: []ScraperCapabilities{
				{Scraper: "web-scraper", Capabilities: []string{"web-scraper"}},
				{Scraper: "telemetry", Capabilities: []string{"telemetry"}},
				{Scraper: "tiktok-transcription", Capabilities: []string{"tiktok-transcription"}},
			},
		},
		{
			name: "Without JobServer - with Twitter accounts",
			jc: types.JobConfiguration{
				"twitter_accounts": []string{"user1:pass1"},
			},
			jobServer: nil,
			expected: []ScraperCapabilities{
				{Scraper: "web-scraper", Capabilities: []string{"web-scraper"}},
				{Scraper: "telemetry", Capabilities: []string{"telemetry"}},
				{Scraper: "tiktok-transcription", Capabilities: []string{"tiktok-transcription"}},
				{Scraper: "twitter-scraper", Capabilities: []string{
					"searchbyquery", "searchbyfullarchive", "searchbyprofile", "searchfollowers",
					"getbyid", "getreplies", "getretweeters", "gettweets", "getmedia",
					"gethometweets", "getforyoutweets", "getbookmarks", "getprofilebyid",
					"gettrends", "getfollowing", "getfollowers", "getspace",
				}},
			},
		},
		{
			name: "With manual capabilities",
			jc: types.JobConfiguration{
				"capabilities": "custom-cap1,custom-cap2",
			},
			jobServer: &MockJobServer{
				capabilities: map[string][]string{
					"web-scraper": {"web-scraper"},
				},
			},
			expected: []ScraperCapabilities{
				{Scraper: "web-scraper", Capabilities: []string{"web-scraper", "custom-cap1", "custom-cap2"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectCapabilities(tt.jc, tt.jobServer)

			// Sort scrapers for consistent comparison
			sort.Slice(got, func(i, j int) bool {
				return got[i].Scraper < got[j].Scraper
			})
			sort.Slice(tt.expected, func(i, j int) bool {
				return tt.expected[i].Scraper < tt.expected[j].Scraper
			})

			// Sort capabilities within each scraper
			for i := range got {
				sort.Strings(got[i].Capabilities)
			}
			for i := range tt.expected {
				sort.Strings(tt.expected[i].Capabilities)
			}

			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("DetectCapabilities() = %v, want %v", got, tt.expected)
			}
		})
	}
}
