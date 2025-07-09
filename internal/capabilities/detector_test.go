package capabilities

import (
	"reflect"
	"slices"
	"testing"

	"github.com/masa-finance/tee-worker/api/types"
)

// MockJobServer implements JobServerInterface for testing
type MockJobServer struct {
	capabilities map[string][]types.Capability
}

func (m *MockJobServer) GetWorkerCapabilities() map[string][]types.Capability {
	return m.capabilities
}

func TestDetectCapabilities(t *testing.T) {
	tests := []struct {
		name      string
		jc        types.JobConfiguration
		jobServer JobServerInterface
		expected  []types.Capability
	}{
		{
			name: "With JobServer - gets capabilities from workers",
			jc:   types.JobConfiguration{},
			jobServer: &MockJobServer{
				capabilities: map[string][]types.Capability{
					"web-scraper":          {"web-scraper"},
					"telemetry":            {"telemetry"},
					"tiktok-transcription": {"tiktok-transcription"},
					"twitter-scraper":      {"searchbyquery", "getbyid", "getprofilebyid"},
				},
			},
			expected: []types.Capability{
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
			expected: []types.Capability{
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
			expected: []types.Capability{
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
			expected: []types.Capability{
				"web-scraper",
				"telemetry",
				"tiktok-transcription",
				"searchbyquery",
				"getbyid",
				"getprofilebyid",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectCapabilities(tt.jc, tt.jobServer)

			// Sort both slices for comparison
			slices.Sort(got)
			slices.Sort(tt.expected)

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
		detected []types.Capability
		expected []types.Capability
	}{
		{
			name:     "Empty manual, some detected",
			manual:   "",
			detected: []types.Capability{"web-scraper", "telemetry"},
			expected: []types.Capability{"web-scraper", "telemetry"},
		},
		{
			name:     "Manual 'all' with detected",
			manual:   "all",
			detected: []types.Capability{"web-scraper", "telemetry", "searchbyquery"},
			expected: []types.Capability{"all", "web-scraper", "telemetry", "searchbyquery"},
		},
		{
			name:     "Manual specific capabilities with detected",
			manual:   "searchbyquery,getbyid",
			detected: []types.Capability{"web-scraper", "telemetry", "searchbyprofile"},
			expected: []types.Capability{"searchbyquery", "getbyid", "web-scraper", "telemetry", "searchbyprofile"},
		},
		{
			name:     "Overlapping manual and detected",
			manual:   "web-scraper,custom-cap",
			detected: []types.Capability{"web-scraper", "telemetry"},
			expected: []types.Capability{"web-scraper", "custom-cap", "telemetry"},
		},
		{
			name:     "Manual with spaces",
			manual:   "cap1, cap2 , cap3",
			detected: []types.Capability{"cap4"},
			expected: []types.Capability{"cap1", "cap2", "cap3", "cap4"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MergeCapabilities(tt.manual, tt.detected)

			// Sort for consistent comparison since map iteration is random
			slices.Sort(got)
			slices.Sort(tt.expected)

			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("MergeCapabilities() = %v, want %v", got, tt.expected)
			}
		})
	}
}
