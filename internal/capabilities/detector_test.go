package capabilities

import (
	"context"
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
		expected  []string
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
				},
			},
			expected: []string{
				"web-scraper",
				"telemetry",
				"tiktok-transcription",
				"searchbyquery",
				"getbyid",
				"getprofilebyid",
			},
		},
		{
			name:      "Without JobServer - basic capabilities only",
			jc:        types.JobConfiguration{},
			jobServer: nil,
			expected: []string{
				"web-scraper",
				"telemetry",
				"tiktok-transcription",
			},
		},
		{
			name: "Without JobServer - with Twitter accounts",
			jc: types.JobConfiguration{
				"twitter_accounts": []string{"user1:pass1"},
			},
			jobServer: nil,
			expected: []string{
				"web-scraper",
				"telemetry",
				"tiktok-transcription",
				"searchbyquery",
				"getbyid",
				"getprofilebyid",
			},
		},
		{
			name: "Without JobServer - with Twitter API keys",
			jc: types.JobConfiguration{
				"twitter_api_keys": []string{"key1"},
			},
			jobServer: nil,
			expected: []string{
				"web-scraper",
				"telemetry",
				"tiktok-transcription",
				"searchbyquery",
				"getbyid",
				"getprofilebyid",
			},
		},
		{
			name: "Without JobServer - with LinkedIn credentials array",
			jc: types.JobConfiguration{
				"linkedin_credentials": []interface{}{
					map[string]interface{}{
						"li_at_cookie": "cookie1",
						"csrf_token":   "token1",
						"jsessionid":   "session1",
					},
				},
			},
			jobServer: nil,
			expected: []string{
				"web-scraper",
				"telemetry",
				"tiktok-transcription",
				"searchbyquery",
				"getprofile",
			},
		},
		{
			name: "Without JobServer - with LinkedIn individual credentials",
			jc: types.JobConfiguration{
				"linkedin_li_at_cookie": "cookie1",
				"linkedin_csrf_token":   "token1",
				"linkedin_jsessionid":   "session1",
			},
			jobServer: nil,
			expected: []string{
				"web-scraper",
				"telemetry",
				"tiktok-transcription",
				"searchbyquery",
				"getprofile",
			},
		},
		{
			name: "Without JobServer - with both Twitter and LinkedIn credentials",
			jc: types.JobConfiguration{
				"twitter_accounts":      []string{"user1:pass1"},
				"linkedin_li_at_cookie": "cookie1",
				"linkedin_csrf_token":   "token1",
				"linkedin_jsessionid":   "session1",
			},
			jobServer: nil,
			expected: []string{
				"web-scraper",
				"telemetry",
				"tiktok-transcription",
				"searchbyquery",
				"getbyid",
				"getprofilebyid",
				"getprofile",
			},
		},
		{
			name: "Without JobServer - with incomplete LinkedIn credentials",
			jc: types.JobConfiguration{
				"linkedin_li_at_cookie": "cookie1",
				"linkedin_csrf_token":   "token1",
				// Missing linkedin_jsessionid
			},
			jobServer: nil,
			expected: []string{
				"web-scraper",
				"telemetry",
				"tiktok-transcription",
			},
		},
		{
			name: "Without JobServer - with incomplete LinkedIn credentials (missing csrf_token)",
			jc: types.JobConfiguration{
				"linkedin_li_at_cookie": "cookie1",
				"linkedin_jsessionid":   "session1",
				// Missing linkedin_csrf_token
			},
			jobServer: nil,
			expected: []string{
				"web-scraper",
				"telemetry",
				"tiktok-transcription",
			},
		},
		{
			name: "Without JobServer - with incomplete LinkedIn credentials (missing jsessionid)",
			jc: types.JobConfiguration{
				"linkedin_li_at_cookie": "cookie1",
				"linkedin_csrf_token":   "token1",
				// Missing linkedin_jsessionid
			},
			jobServer: nil,
			expected: []string{
				"web-scraper",
				"telemetry",
				"tiktok-transcription",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectCapabilities(context.Background(), tt.jc, tt.jobServer)

			// Sort both slices for comparison
			sort.Strings(got)
			sort.Strings(tt.expected)

			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("DetectCapabilities() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestMergeCapabilities(t *testing.T) {
	tests := []struct {
		name     string
		manual   string
		detected []string
		expected []string
	}{
		{
			name:     "Empty manual, some detected",
			manual:   "",
			detected: []string{"web-scraper", "telemetry"},
			expected: []string{"web-scraper", "telemetry"},
		},
		{
			name:     "Manual 'all' with detected",
			manual:   "all",
			detected: []string{"web-scraper", "telemetry", "searchbyquery"},
			expected: []string{"all", "web-scraper", "telemetry", "searchbyquery"},
		},
		{
			name:     "Manual specific capabilities with detected",
			manual:   "searchbyquery,getbyid",
			detected: []string{"web-scraper", "telemetry", "searchbyprofile"},
			expected: []string{"searchbyquery", "getbyid", "web-scraper", "telemetry", "searchbyprofile"},
		},
		{
			name:     "Overlapping manual and detected",
			manual:   "web-scraper,custom-cap",
			detected: []string{"web-scraper", "telemetry"},
			expected: []string{"web-scraper", "custom-cap", "telemetry"},
		},
		{
			name:     "Manual with spaces",
			manual:   "cap1, cap2 , cap3",
			detected: []string{"cap4"},
			expected: []string{"cap1", "cap2", "cap3", "cap4"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MergeCapabilities(tt.manual, tt.detected)

			// Sort for consistent comparison since map iteration is random
			sort.Strings(got)
			sort.Strings(tt.expected)

			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("MergeCapabilities() = %v, want %v", got, tt.expected)
			}
		})
	}
}
