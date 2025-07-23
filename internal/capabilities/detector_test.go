package capabilities

import (
	"reflect"
	"testing"

	teetypes "github.com/masa-finance/tee-types/types"
	"github.com/masa-finance/tee-worker/api/types"
)

// MockJobServer implements JobServerInterface for testing
type MockJobServer struct {
	capabilities teetypes.WorkerCapabilities
}

func (m *MockJobServer) GetWorkerCapabilities() teetypes.WorkerCapabilities {
	return m.capabilities
}

func TestDetectCapabilities(t *testing.T) {
	tests := []struct {
		name      string
		jc        types.JobConfiguration
		jobServer JobServerInterface
		expected  teetypes.WorkerCapabilities
	}{
		{
			name: "With JobServer - gets capabilities from workers",
			jc:   types.JobConfiguration{},
			jobServer: &MockJobServer{
				capabilities: teetypes.WorkerCapabilities{
					{JobType: string(teetypes.WebJob), Capabilities: []teetypes.Capability{"web-scraper"}},
					{JobType: string(teetypes.TelemetryJob), Capabilities: []teetypes.Capability{"telemetry"}},
					{JobType: string(teetypes.TiktokJob), Capabilities: []teetypes.Capability{"tiktok-transcription"}},
					{JobType: string(teetypes.TwitterJob), Capabilities: []teetypes.Capability{"searchbyquery", "getbyid", "getprofilebyid"}},
				},
			},
			expected: teetypes.WorkerCapabilities{
				{JobType: string(teetypes.WebJob), Capabilities: []teetypes.Capability{"web-scraper"}},
				{JobType: string(teetypes.TelemetryJob), Capabilities: []teetypes.Capability{"telemetry"}},
				{JobType: string(teetypes.TiktokJob), Capabilities: []teetypes.Capability{"tiktok-transcription"}},
				{JobType: string(teetypes.TwitterJob), Capabilities: []teetypes.Capability{"searchbyquery", "getbyid", "getprofilebyid"}},
			},
		},
		{
			name:      "Without JobServer - basic capabilities only",
			jc:        types.JobConfiguration{},
			jobServer: nil,
			expected: teetypes.WorkerCapabilities{
				{JobType: string(teetypes.WebJob), Capabilities: []teetypes.Capability{"web-scraper"}},
				{JobType: string(teetypes.TelemetryJob), Capabilities: []teetypes.Capability{"telemetry"}},
				{JobType: string(teetypes.TiktokJob), Capabilities: []teetypes.Capability{"tiktok-transcription"}},
			},
		},
		{
			name: "Without JobServer - with Twitter accounts",
			jc: types.JobConfiguration{
				"twitter_accounts": []string{"user1:pass1"},
			},
			jobServer: nil,
			expected: teetypes.WorkerCapabilities{
				{JobType: string(teetypes.WebJob), Capabilities: []teetypes.Capability{"web-scraper"}},
				{JobType: string(teetypes.TelemetryJob), Capabilities: []teetypes.Capability{"telemetry"}},
				{JobType: string(teetypes.TiktokJob), Capabilities: []teetypes.Capability{"tiktok-transcription"}},
				{JobType: string(teetypes.TwitterCredentialJob), Capabilities: []teetypes.Capability{
					"searchbyquery", "searchbyfullarchive", "searchbyprofile",
					"getbyid", "getreplies", "getretweeters", "gettweets", "getmedia",
					"gethometweets", "getforyoutweets", "getprofilebyid",
					"gettrends", "getfollowing", "getfollowers", "getspace",
				}},
				{JobType: string(teetypes.TwitterJob), Capabilities: []teetypes.Capability{
					"searchbyquery", "searchbyfullarchive", "searchbyprofile",
					"getbyid", "getreplies", "getretweeters", "gettweets", "getmedia",
					"gethometweets", "getforyoutweets", "getprofilebyid",
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
			expected: teetypes.WorkerCapabilities{
				{JobType: string(teetypes.WebJob), Capabilities: []teetypes.Capability{"web-scraper"}},
				{JobType: string(teetypes.TelemetryJob), Capabilities: []teetypes.Capability{"telemetry"}},
				{JobType: string(teetypes.TiktokJob), Capabilities: []teetypes.Capability{"tiktok-transcription"}},
				{JobType: string(teetypes.TwitterApiJob), Capabilities: []teetypes.Capability{"searchbyquery", "getbyid", "getprofilebyid"}},
				{JobType: string(teetypes.TwitterJob), Capabilities: []teetypes.Capability{"searchbyquery", "getbyid", "getprofilebyid"}},
			},
		},
		{
			name: "Without JobServer - with both accounts and API keys",
			jc: types.JobConfiguration{
				"twitter_accounts": []string{"user1:pass1"},
				"twitter_api_keys": []string{"key1"},
			},
			jobServer: nil,
			expected: teetypes.WorkerCapabilities{
				{JobType: string(teetypes.WebJob), Capabilities: []teetypes.Capability{"web-scraper"}},
				{JobType: string(teetypes.TelemetryJob), Capabilities: []teetypes.Capability{"telemetry"}},
				{JobType: string(teetypes.TiktokJob), Capabilities: []teetypes.Capability{"tiktok-transcription"}},
				{JobType: string(teetypes.TwitterCredentialJob), Capabilities: []teetypes.Capability{
					"searchbyquery", "searchbyfullarchive", "searchbyprofile",
					"getbyid", "getreplies", "getretweeters", "gettweets", "getmedia",
					"gethometweets", "getforyoutweets", "getprofilebyid",
					"gettrends", "getfollowing", "getfollowers", "getspace",
				}},
				{JobType: string(teetypes.TwitterJob), Capabilities: []teetypes.Capability{
					"searchbyquery", "searchbyfullarchive", "searchbyprofile",
					"getbyid", "getreplies", "getretweeters", "gettweets", "getmedia",
					"gethometweets", "getforyoutweets", "getprofilebyid",
					"gettrends", "getfollowing", "getfollowers", "getspace",
				}},
				{JobType: string(teetypes.TwitterApiJob), Capabilities: []teetypes.Capability{"searchbyquery", "getbyid", "getprofilebyid"}},
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

// Helper function to find a job capability by name
func findJobCapability(capabilities teetypes.WorkerCapabilities, jobName string) *teetypes.JobCapability {
	for _, cap := range capabilities {
		if cap.JobType == jobName {
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
			expectedKeys: []string{string(teetypes.WebJob), string(teetypes.TelemetryJob), string(teetypes.TiktokJob), string(teetypes.TwitterCredentialJob), string(teetypes.TwitterJob)},
		},
		{
			name: "With API keys only",
			jc: types.JobConfiguration{
				"twitter_api_keys": []string{"key123"},
			},
			expectedKeys: []string{string(teetypes.WebJob), string(teetypes.TelemetryJob), string(teetypes.TiktokJob), string(teetypes.TwitterApiJob), string(teetypes.TwitterJob)},
		},
		{
			name: "With both accounts and keys",
			jc: types.JobConfiguration{
				"twitter_accounts": []string{"user:pass"},
				"twitter_api_keys": []string{"key123"},
			},
			expectedKeys: []string{string(teetypes.WebJob), string(teetypes.TelemetryJob), string(teetypes.TiktokJob), string(teetypes.TwitterCredentialJob), string(teetypes.TwitterJob), string(teetypes.TwitterApiJob)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			caps := DetectCapabilities(tt.jc, nil)

			jobNames := make([]string, len(caps))
			for i, cap := range caps {
				jobNames[i] = cap.JobType
			}

			// Check that all expected keys are present
			for _, expectedKey := range tt.expectedKeys {
				found := false
				for _, jobName := range jobNames {
					if jobName == expectedKey {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected scraper %s not found in %v", expectedKey, jobNames)
				}
			}

			// Check that no unexpected keys are present
			for _, jobName := range jobNames {
				found := false
				for _, expectedKey := range tt.expectedKeys {
					if jobName == expectedKey {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Unexpected scraper %s found in %v", jobName, jobNames)
				}
			}
		})
	}
}
