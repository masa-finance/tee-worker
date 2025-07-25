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
					{JobType: teetypes.WebJob, Capabilities: []teetypes.Capability{teetypes.CapWebScraper}},
					{JobType: teetypes.TelemetryJob, Capabilities: []teetypes.Capability{teetypes.CapTelemetry}},
					{JobType: teetypes.TiktokJob, Capabilities: []teetypes.Capability{teetypes.CapTiktokTranscription}},
					{JobType: teetypes.TwitterJob, Capabilities: []teetypes.Capability{teetypes.CapSearchByQuery, teetypes.CapGetById, teetypes.CapGetProfileById}},
				},
			},
			expected: teetypes.WorkerCapabilities{
				{JobType: teetypes.WebJob, Capabilities: []teetypes.Capability{teetypes.CapWebScraper}},
				{JobType: teetypes.TelemetryJob, Capabilities: []teetypes.Capability{teetypes.CapTelemetry}},
				{JobType: teetypes.TiktokJob, Capabilities: []teetypes.Capability{teetypes.CapTiktokTranscription}},
				{JobType: teetypes.TwitterJob, Capabilities: []teetypes.Capability{teetypes.CapSearchByQuery, teetypes.CapGetById, teetypes.CapGetProfileById}},
			},
		},
		{
			name:      "Without JobServer - basic capabilities only",
			jc:        types.JobConfiguration{},
			jobServer: nil,
			expected: teetypes.WorkerCapabilities{
				{JobType: teetypes.WebJob, Capabilities: []teetypes.Capability{teetypes.CapWebScraper}},
				{JobType: teetypes.TelemetryJob, Capabilities: []teetypes.Capability{teetypes.CapTelemetry}},
				{JobType: teetypes.TiktokJob, Capabilities: []teetypes.Capability{teetypes.CapTiktokTranscription}},
			},
		},
		{
			name: "Without JobServer - with Twitter accounts",
			jc: types.JobConfiguration{
				"twitter_accounts": []string{"user1:pass1"},
			},
			jobServer: nil,
			expected: teetypes.WorkerCapabilities{
				{JobType: teetypes.WebJob, Capabilities: []teetypes.Capability{teetypes.CapWebScraper}},
				{JobType: teetypes.TelemetryJob, Capabilities: []teetypes.Capability{teetypes.CapTelemetry}},
				{JobType: teetypes.TiktokJob, Capabilities: []teetypes.Capability{teetypes.CapTiktokTranscription}},
				{JobType: teetypes.TwitterCredentialJob, Capabilities: teetypes.TwitterAllCaps},
				{JobType: teetypes.TwitterJob, Capabilities: teetypes.TwitterAllCaps},
			},
		},
		{
			name: "Without JobServer - with Twitter API keys",
			jc: types.JobConfiguration{
				"twitter_api_keys": []string{"key1"},
			},
			jobServer: nil,
			expected: teetypes.WorkerCapabilities{
				{JobType: teetypes.WebJob, Capabilities: []teetypes.Capability{teetypes.CapWebScraper}},
				{JobType: teetypes.TelemetryJob, Capabilities: []teetypes.Capability{teetypes.CapTelemetry}},
				{JobType: teetypes.TiktokJob, Capabilities: []teetypes.Capability{teetypes.CapTiktokTranscription}},
				{JobType: teetypes.TwitterApiJob, Capabilities: teetypes.TwitterAPICaps},
				{JobType: teetypes.TwitterJob, Capabilities: teetypes.TwitterAPICaps},
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
				{JobType: teetypes.WebJob, Capabilities: []teetypes.Capability{teetypes.CapWebScraper}},
				{JobType: teetypes.TelemetryJob, Capabilities: []teetypes.Capability{teetypes.CapTelemetry}},
				{JobType: teetypes.TiktokJob, Capabilities: []teetypes.Capability{teetypes.CapTiktokTranscription}},
				{JobType: teetypes.TwitterCredentialJob, Capabilities: teetypes.TwitterAllCaps},
				{JobType: teetypes.TwitterApiJob, Capabilities: teetypes.TwitterAPICaps},
				{JobType: teetypes.TwitterJob, Capabilities: teetypes.TwitterAllCaps},
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
		if cap.JobType.String() == jobName {
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
			expectedKeys: []string{teetypes.WebJob.String(), teetypes.TelemetryJob.String(), teetypes.TiktokJob.String(), teetypes.TwitterCredentialJob.String(), teetypes.TwitterJob.String()},
		},
		{
			name: "With API keys only",
			jc: types.JobConfiguration{
				"twitter_api_keys": []string{"key123"},
			},
			expectedKeys: []string{teetypes.WebJob.String(), teetypes.TelemetryJob.String(), teetypes.TiktokJob.String(), teetypes.TwitterApiJob.String(), teetypes.TwitterJob.String()},
		},
		{
			name: "With both accounts and keys",
			jc: types.JobConfiguration{
				"twitter_accounts": []string{"user:pass"},
				"twitter_api_keys": []string{"key123"},
			},
			expectedKeys: []string{teetypes.WebJob.String(), teetypes.TelemetryJob.String(), teetypes.TiktokJob.String(), teetypes.TwitterCredentialJob.String(), teetypes.TwitterJob.String(), teetypes.TwitterApiJob.String()},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			caps := DetectCapabilities(tt.jc, nil)

			jobNames := make([]string, len(caps))
			for i, cap := range caps {
				jobNames[i] = cap.JobType.String()
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
