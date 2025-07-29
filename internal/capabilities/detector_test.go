package capabilities

import (
	"reflect"
	"slices"
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
					teetypes.WebJob:       {teetypes.CapWebScraper},
					teetypes.TelemetryJob: {teetypes.CapTelemetry},
					teetypes.TiktokJob:    {teetypes.CapTiktokTranscription},
					teetypes.TwitterJob:   {teetypes.CapSearchByQuery, teetypes.CapGetById, teetypes.CapGetProfileById},
				},
			},
			expected: teetypes.WorkerCapabilities{
				teetypes.WebJob:       {teetypes.CapWebScraper},
				teetypes.TelemetryJob: {teetypes.CapTelemetry},
				teetypes.TiktokJob:    {teetypes.CapTiktokTranscription},
				teetypes.TwitterJob:   {teetypes.CapSearchByQuery, teetypes.CapGetById, teetypes.CapGetProfileById},
			},
		},
		{
			name:      "Without JobServer - basic capabilities only",
			jc:        types.JobConfiguration{},
			jobServer: nil,
			expected: teetypes.WorkerCapabilities{
				teetypes.WebJob:       {teetypes.CapWebScraper},
				teetypes.TelemetryJob: {teetypes.CapTelemetry},
				teetypes.TiktokJob:    {teetypes.CapTiktokTranscription},
			},
		},
		{
			name: "With Twitter accounts - adds credential capabilities",
			jc: types.JobConfiguration{
				"twitter_accounts": []string{"account1", "account2"},
			},
			jobServer: nil,
			expected: teetypes.WorkerCapabilities{
				teetypes.WebJob:               {teetypes.CapWebScraper},
				teetypes.TelemetryJob:         {teetypes.CapTelemetry},
				teetypes.TiktokJob:            {teetypes.CapTiktokTranscription},
				teetypes.TwitterCredentialJob: teetypes.TwitterCredentialCaps,
				teetypes.TwitterJob:           teetypes.TwitterCredentialCaps,
			},
		},
		{
			name: "With Twitter API keys - adds API capabilities",
			jc: types.JobConfiguration{
				"twitter_api_keys": []string{"key1", "key2"},
			},
			jobServer: nil,
			expected: teetypes.WorkerCapabilities{
				teetypes.WebJob:        {teetypes.CapWebScraper},
				teetypes.TelemetryJob:  {teetypes.CapTelemetry},
				teetypes.TiktokJob:     {teetypes.CapTiktokTranscription},
				teetypes.TwitterApiJob: teetypes.TwitterAPICaps,
				teetypes.TwitterJob:    teetypes.TwitterAPICaps,
			},
		},
		{
			name: "With mock elevated Twitter API keys - only basic capabilities detected",
			jc: types.JobConfiguration{
				"twitter_api_keys": []string{"Bearer abcd1234-ELEVATED"},
			},
			jobServer: nil,
			expected: teetypes.WorkerCapabilities{
				teetypes.WebJob:       {teetypes.CapWebScraper},
				teetypes.TelemetryJob: {teetypes.CapTelemetry},
				teetypes.TiktokJob:    {teetypes.CapTiktokTranscription},
				// Note: Mock elevated keys will be detected as basic since we can't make real API calls in tests
				teetypes.TwitterApiJob: teetypes.TwitterAPICaps,
				teetypes.TwitterJob:    teetypes.TwitterAPICaps,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectCapabilities(tt.jc, tt.jobServer)

			// Extract job type keys and sort for consistent comparison
			gotKeys := make([]string, 0, len(got))
			for jobType := range got {
				gotKeys = append(gotKeys, jobType.String())
			}

			expectedKeys := make([]string, 0, len(tt.expected))
			for jobType := range tt.expected {
				expectedKeys = append(expectedKeys, jobType.String())
			}

			// Sort both slices for comparison
			slices.Sort(gotKeys)
			slices.Sort(expectedKeys)

			// Compare the sorted slices
			if !reflect.DeepEqual(gotKeys, expectedKeys) {
				t.Errorf("DetectCapabilities() job types = %v, want %v", gotKeys, expectedKeys)
			}
		})
	}
}

// Helper function to check if a job type exists in capabilities
func hasJobType(capabilities teetypes.WorkerCapabilities, jobName string) bool {
	_, exists := capabilities[teetypes.JobType(jobName)]
	return exists
}

func TestDetectCapabilities_ScraperTypes(t *testing.T) {
	tests := []struct {
		name         string
		jc           types.JobConfiguration
		expectedKeys []string // scraper names we expect
	}{
		{
			name:         "Basic scrapers only",
			jc:           types.JobConfiguration{},
			expectedKeys: []string{"web", "telemetry", "tiktok"},
		},
		{
			name: "With Twitter accounts",
			jc: types.JobConfiguration{
				"twitter_accounts": []string{"user1:pass1"},
			},
			expectedKeys: []string{"web", "telemetry", "tiktok", "twitter", "twitter-credential"},
		},
		{
			name: "With Twitter API keys",
			jc: types.JobConfiguration{
				"twitter_api_keys": []string{"key1"},
			},
			expectedKeys: []string{"web", "telemetry", "tiktok", "twitter", "twitter-api"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			caps := DetectCapabilities(tt.jc, nil)

			jobNames := make([]string, 0, len(caps))
			for jobType := range caps {
				jobNames = append(jobNames, jobType.String())
			}

			// Sort both slices for comparison
			slices.Sort(jobNames)
			expectedSorted := make([]string, len(tt.expectedKeys))
			copy(expectedSorted, tt.expectedKeys)
			slices.Sort(expectedSorted)

			// Compare the sorted slices
			if !reflect.DeepEqual(jobNames, expectedSorted) {
				t.Errorf("Expected capabilities %v, got %v", expectedSorted, jobNames)
			}
		})
	}
}
