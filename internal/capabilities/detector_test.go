package capabilities

import (
	"reflect"
	"sort"
	"testing"

	"github.com/masa-finance/tee-worker/api/types"
)

func TestDetectCapabilities(t *testing.T) {
	tests := []struct {
		name     string
		jc       types.JobConfiguration
		expected []string
	}{
		{
			name: "No credentials",
			jc:   types.JobConfiguration{},
			expected: []string{
				"web-scraper",
				"telemetry",
				"tiktok-transcription",
			},
		},
		{
			name: "Twitter accounts only",
			jc: types.JobConfiguration{
				"twitter_accounts": []string{"user1:pass1", "user2:pass2"},
			},
			expected: []string{
				"web-scraper",
				"telemetry",
				"tiktok-transcription",
				"searchbyquery",
				"searchbyprofile",
				"searchfollowers",
				"getbyid",
				"getreplies",
				"getretweeters",
				"gettweets",
				"getmedia",
				"gethometweets",
				"getforyoutweets",
				"getbookmarks",
				"getprofilebyid",
				"gettrends",
				"getfollowing",
				"getfollowers",
				"getspace",
			},
		},
		{
			name: "Twitter API keys only",
			jc: types.JobConfiguration{
				"twitter_api_keys": []string{"key1", "key2"},
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
			name: "Both Twitter accounts and API keys",
			jc: types.JobConfiguration{
				"twitter_accounts": []string{"user1:pass1"},
				"twitter_api_keys": []string{"key1"},
			},
			expected: []string{
				"web-scraper",
				"telemetry",
				"tiktok-transcription",
				"searchbyquery",
				"searchbyprofile",
				"searchfollowers",
				"getbyid",
				"getreplies",
				"getretweeters",
				"gettweets",
				"getmedia",
				"gethometweets",
				"getforyoutweets",
				"getbookmarks",
				"getprofilebyid",
				"gettrends",
				"getfollowing",
				"getfollowers",
				"getspace",
			},
		},
		{
			name: "Invalid Twitter account format",
			jc: types.JobConfiguration{
				"twitter_accounts": []string{"invalid_format"},
			},
			expected: []string{
				"web-scraper",
				"telemetry",
				"tiktok-transcription",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectCapabilities(tt.jc)
			
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

func TestHasCapability(t *testing.T) {
	capabilities := []string{"searchbyquery", "getbyid", "web-scraper"}
	
	tests := []struct {
		name       string
		capability string
		expected   bool
	}{
		{
			name:       "Existing capability",
			capability: "searchbyquery",
			expected:   true,
		},
		{
			name:       "Non-existing capability",
			capability: "searchbyfullarchive",
			expected:   false,
		},
		{
			name:       "Empty capability",
			capability: "",
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasCapability(capabilities, tt.capability); got != tt.expected {
				t.Errorf("hasCapability() = %v, want %v", got, tt.expected)
			}
		})
	}
}