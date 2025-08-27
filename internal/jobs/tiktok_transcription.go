package jobs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	teeargs "github.com/masa-finance/tee-types/args"
	teetypes "github.com/masa-finance/tee-types/types"
	"github.com/masa-finance/tee-worker/api/types"
	"github.com/masa-finance/tee-worker/internal/jobs/stats"
	"github.com/sirupsen/logrus"
)

// tiktokTranscriptionEndpoint is the default hardcoded endpoint for TikTok transcriptions.
const tiktokTranscriptionEndpoint = "https://submagic-free-tools.fly.dev/api/tiktok-transcription"

// TikTokTranscriptionConfiguration holds the configuration for the TikTok transcriber.
// These values are typically populated from environment variables via config.go.
type TikTokTranscriptionConfiguration struct {
	TranscriptionEndpoint string `json:"tiktok_transcription_endpoint"`
	APIOrigin             string `json:"tiktok_api_origin,omitempty"`
	APIReferer            string `json:"tiktok_api_referer,omitempty"`
	APIUserAgent          string `json:"tiktok_api_user_agent,omitempty"`
	DefaultLanguage       string `json:"tiktok_default_language,omitempty"` // e.g., "eng-US"
}

// TikTokTranscriber is the main job struct for handling TikTok transcriptions.
type TikTokTranscriber struct {
	configuration TikTokTranscriptionConfiguration
	stats         *stats.StatsCollector
	httpClient    *http.Client
}

// GetStructuredCapabilities returns the structured capabilities supported by the TikTok transcriber
func (t *TikTokTranscriber) GetStructuredCapabilities() teetypes.WorkerCapabilities {
	return teetypes.WorkerCapabilities{
		teetypes.TiktokJob: teetypes.AlwaysAvailableTiktokCaps,
	}
}

// NewTikTokTranscriber creates and initializes a new TikTokTranscriber.
// It sets default values for the API configuration.
func NewTikTokTranscriber(jc types.JobConfiguration, statsCollector *stats.StatsCollector) *TikTokTranscriber {
	config := TikTokTranscriptionConfiguration{}

	// Set default values directly
	config.TranscriptionEndpoint = tiktokTranscriptionEndpoint
	config.APIOrigin = "https://submagic-free-tools.fly.dev"
	config.APIReferer = "https://submagic-free-tools.fly.dev/tiktok-transcription"

	// Get configurable values from job configuration
	if err := jc.Unmarshal(&config); err != nil {
		logrus.WithError(err).Debug("TikTokTranscriber: Could not unmarshal job configuration, using all defaults")
	}

	// Set defaults for configurable values if not provided
	if config.DefaultLanguage == "" {
		config.DefaultLanguage = "eng-US"
	}

	if config.APIUserAgent == "" {
		config.APIUserAgent = "Mozilla/5.0 (Linux; Android 6.0; Nexus 5 Build/MRA58N) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/135.0.0.0 Mobile Safari/537.36"
	}

	// Log the actual configuration values being used
	logrus.WithFields(logrus.Fields{
		"transcription_endpoint": config.TranscriptionEndpoint,
		"api_origin":             config.APIOrigin,
		"api_referer":            config.APIReferer,
		"api_user_agent":         config.APIUserAgent,
		"default_language":       config.DefaultLanguage,
	}).Info("TikTokTranscriber initialized with configuration")

	httpClient := &http.Client{
		Timeout: 30 * time.Second, // Sensible default timeout
	}

	return &TikTokTranscriber{
		configuration: config,
		stats:         statsCollector,
		httpClient:    httpClient,
	}
}

// APIResponse is used to unmarshal the JSON response from the transcription API.
type APIResponse struct {
	VideoTitle   string            `json:"videoTitle"`
	Transcripts  map[string]string `json:"transcripts"` // map of language_code -> VTT string
	ThumbnailURL string            `json:"thumbnailUrl"`
	Error        string            `json:"error,omitempty"` // Optional error from API
}

// ExecuteJob processes a single TikTok transcription job.
func (ttt *TikTokTranscriber) ExecuteJob(j types.Job) (types.JobResult, error) {
	logrus.WithField("job_uuid", j.UUID).Info("Starting ExecuteJob for TikTok transcription")

	if ttt.configuration.TranscriptionEndpoint == "" {
		ttt.stats.Add(j.WorkerID, stats.TikTokTranscriptionErrors, 1)
		return types.JobResult{Error: "TikTok transcription endpoint is not configured for the worker"}, fmt.Errorf("tiktok transcription endpoint not configured")
	}

	// Use the centralized type-safe unmarshaller
	jobArgs, err := teeargs.UnmarshalJobArguments(teetypes.JobType(j.Type), map[string]any(j.Arguments))
	if err != nil {
		return types.JobResult{Error: "Failed to unmarshal job arguments"}, fmt.Errorf("unmarshal job arguments: %w", err)
	}

	// Type assert to TikTok arguments
	tiktokArgs, ok := jobArgs.(*teeargs.TikTokTranscriptionArguments)
	if !ok {
		return types.JobResult{Error: "invalid argument type for TikTok job"}, fmt.Errorf("invalid argument type")
	}

	// Use interface methods; no need to downcast
	logrus.WithField("job_uuid", j.UUID).Infof("TikTok arguments validated: video_url=%s, language=%s, has_language_preference=%t",
		tiktokArgs.GetVideoURL(), tiktokArgs.GetLanguageCode(), tiktokArgs.HasLanguagePreference())

	// VideoURL validation is now handled by the unmarshaller, but we check again for safety
	if tiktokArgs.GetVideoURL() == "" {
		ttt.stats.Add(j.WorkerID, stats.TikTokTranscriptionErrors, 1)
		return types.JobResult{Error: "VideoURL is required"}, fmt.Errorf("videoURL is required")
	}

	// Use the enhanced language selection logic
	selectedLanguageKey := tiktokArgs.GetLanguageCode() // This handles defaults automatically
	if tiktokArgs.HasLanguagePreference() {
		logrus.WithField("job_uuid", j.UUID).Infof("Using custom language preference: %s", selectedLanguageKey)
	} else {
		logrus.WithField("job_uuid", j.UUID).Infof("Using default language: %s", selectedLanguageKey)
	}

	// Sub-Step 3.1: Call TikTok Transcription API
	apiRequestBody := map[string]string{"url": tiktokArgs.GetVideoURL()}
	jsonBody, err := json.Marshal(apiRequestBody)
	if err != nil {
		ttt.stats.Add(j.WorkerID, stats.TikTokTranscriptionErrors, 1)
		return types.JobResult{Error: "Failed to marshal API request body"}, fmt.Errorf("marshal API request body: %w", err)
	}

	req, err := http.NewRequest("POST", ttt.configuration.TranscriptionEndpoint, bytes.NewBuffer(jsonBody))
	if err != nil {
		ttt.stats.Add(j.WorkerID, stats.TikTokTranscriptionErrors, 1)
		return types.JobResult{Error: "Failed to create API request"}, fmt.Errorf("create API request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if ttt.configuration.APIOrigin != "" {
		req.Header.Set("Origin", ttt.configuration.APIOrigin)
	}
	if ttt.configuration.APIReferer != "" {
		req.Header.Set("Referer", ttt.configuration.APIReferer)
	}
	// User-Agent is set from config or default in NewTikTokTranscriber
	req.Header.Set("User-Agent", ttt.configuration.APIUserAgent)

	logrus.WithFields(logrus.Fields{
		"job_uuid":     j.UUID,
		"url":          tiktokArgs.GetVideoURL(),
		"method":       "POST",
		"api_endpoint": ttt.configuration.TranscriptionEndpoint,
	}).Info("Calling TikTok Transcription API")

	apiResp, err := ttt.httpClient.Do(req)
	if err != nil {
		ttt.stats.Add(j.WorkerID, stats.TikTokTranscriptionErrors, 1)
		return types.JobResult{Error: "API request failed"}, fmt.Errorf("API request execution: %w", err)
	}
	defer apiResp.Body.Close()

	if apiResp.StatusCode != http.StatusOK {
		// Try to read body for more error details from API
		bodyBytes, _ := io.ReadAll(apiResp.Body)
		errMsg := fmt.Sprintf("API request failed with status code %d. Response: %s", apiResp.StatusCode, string(bodyBytes))
		logrus.WithField("job_uuid", j.UUID).Error(errMsg)
		ttt.stats.Add(j.WorkerID, stats.TikTokTranscriptionErrors, 1)
		return types.JobResult{Error: errMsg}, fmt.Errorf(errMsg)
	}

	var parsedAPIResponse APIResponse
	if err := json.NewDecoder(apiResp.Body).Decode(&parsedAPIResponse); err != nil {
		ttt.stats.Add(j.WorkerID, stats.TikTokTranscriptionErrors, 1)
		return types.JobResult{Error: "Failed to parse API response"}, fmt.Errorf("parse API response: %w", err)
	}

	if parsedAPIResponse.Error != "" {
		errMsg := fmt.Sprintf("API returned an error: %s", parsedAPIResponse.Error)
		logrus.WithField("job_uuid", j.UUID).Error(errMsg)
		ttt.stats.Add(j.WorkerID, stats.TikTokTranscriptionErrors, 1)
		return types.JobResult{Error: errMsg}, fmt.Errorf(errMsg)
	}

	// Sub-Step 3.2: Extract Transcription and Metadata
	if len(parsedAPIResponse.Transcripts) == 0 {
		errMsg := "No transcripts found in API response"
		logrus.WithField("job_uuid", j.UUID).Warn(errMsg)
		ttt.stats.Add(j.WorkerID, stats.TikTokTranscriptionErrors, 1) // Or a different stat for "no_transcript_found"
		return types.JobResult{Error: errMsg}, fmt.Errorf(errMsg)
	}

	vttText := ""

	// Directly use the requested/default language; if missing, return an error
	if transcript, ok := parsedAPIResponse.Transcripts[selectedLanguageKey]; ok && strings.TrimSpace(transcript) != "" {
		vttText = transcript
	} else {
		errMsg := fmt.Sprintf("Transcript for requested language %s not found in API response", selectedLanguageKey)
		logrus.WithFields(logrus.Fields{
			"job_uuid":       j.UUID,
			"requested_lang": selectedLanguageKey,
		}).Error(errMsg)
		ttt.stats.Add(j.WorkerID, stats.TikTokTranscriptionErrors, 1)
		return types.JobResult{Error: errMsg}, fmt.Errorf(errMsg)
	}

	if vttText == "" {
		errMsg := "Suitable transcript could not be extracted from API response"
		logrus.WithField("job_uuid", j.UUID).Error(errMsg)
		ttt.stats.Add(j.WorkerID, stats.TikTokTranscriptionErrors, 1)
		return types.JobResult{Error: errMsg}, fmt.Errorf(errMsg)
	}

	logrus.Debugf("Job %s: Raw VTT content for language %s:\n%s", j.UUID, selectedLanguageKey, vttText)

	// Convert VTT to Plain Text
	plainTextTranscription, err := convertVTTToPlainText(vttText)
	if err != nil {
		// This error is more about our parsing than the API
		errMsg := fmt.Sprintf("Failed to convert VTT to plain text: %v", err)
		logrus.WithField("job_uuid", j.UUID).Error(errMsg)
		ttt.stats.Add(j.WorkerID, stats.TikTokTranscriptionErrors, 1)
		return types.JobResult{Error: errMsg}, fmt.Errorf(errMsg)
	}

	// Process Result & Return
	resultData := teetypes.TikTokTranscriptionResult{
		TranscriptionText: plainTextTranscription,
		DetectedLanguage:  selectedLanguageKey,
		VideoTitle:        parsedAPIResponse.VideoTitle,
		OriginalURL:       tiktokArgs.GetVideoURL(),
		ThumbnailURL:      parsedAPIResponse.ThumbnailURL,
	}

	jsonData, err := json.Marshal(resultData)
	if err != nil {
		ttt.stats.Add(j.WorkerID, stats.TikTokTranscriptionErrors, 1)
		return types.JobResult{Error: "Failed to marshal result data"}, fmt.Errorf("marshal result data: %w", err)
	}

	logrus.WithFields(logrus.Fields{
		"job_uuid":          j.UUID,
		"video_title":       resultData.VideoTitle,
		"detected_language": resultData.DetectedLanguage,
	}).Info("Successfully processed TikTok transcription job")
	ttt.stats.Add(j.WorkerID, stats.TikTokTranscriptionSuccess, 1)
	return types.JobResult{Data: jsonData}, nil
}

// convertVTTToPlainText parses a VTT string and extracts the dialogue lines.
// This is a basic implementation and might need to be made more robust.
func convertVTTToPlainText(vttContent string) (string, error) {
	var plainText strings.Builder
	lines := strings.Split(strings.ReplaceAll(vttContent, "\r\n", "\n"), "\n")

	inCaptionBlock := false
	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)

		if strings.HasPrefix(trimmedLine, "WEBVTT") {
			// Attempt to extract text directly from the WEBVTT line if it's not just "WEBVTT"
			potentialText := strings.TrimSpace(strings.TrimPrefix(trimmedLine, "WEBVTT"))
			if potentialText != "" {
				cleanedLine := removeVttTags(potentialText)
				if plainText.Len() > 0 {
					plainText.WriteString(" ")
				}
				plainText.WriteString(cleanedLine)
			}
			inCaptionBlock = false // Reset/ensure false after processing WEBVTT line
			continue
		}
		if trimmedLine == "" || strings.HasPrefix(trimmedLine, "NOTE") {
			inCaptionBlock = false // Reset on these lines or empty lines
			continue
		}
		// Detect timestamp lines like "00:00:00.000 --> 00:00:05.000" or "12:34.567 --> 12:39.000"
		if strings.Contains(trimmedLine, "-->") {
			// Simple check for timestamp pattern. More robust regex could be used.
			parts := strings.Split(trimmedLine, "-->")
			if len(parts) == 2 && (strings.Contains(parts[0], ":") && strings.Contains(parts[1], ":")) {
				inCaptionBlock = true
				continue // Skip timestamp lines
			}
		}

		if inCaptionBlock && !strings.HasPrefix(trimmedLine, "<") { // Avoid VTT tags like <v Author>
			if plainText.Len() > 0 {
				plainText.WriteString(" ") // Add space between caption parts
			}
			// Further clean up any potential inline VTT tags, e.g. <c.color969696> or <00:00:26.780>
			cleanedLine := removeVttTags(trimmedLine)
			plainText.WriteString(cleanedLine)
		} else if !strings.Contains(trimmedLine, "-->") && !strings.HasPrefix(trimmedLine, "<") {
			// This handles cases where captions might not be strictly under a timestamp block
			// or if the simple inCaptionBlock logic fails.
			// This is a bit of a catch-all and might need refinement if VTT is complex.
			if plainText.Len() > 0 {
				plainText.WriteString(" ")
			}
			cleanedLine := removeVttTags(trimmedLine)
			plainText.WriteString(cleanedLine)
			inCaptionBlock = false // Assume single line caption if not under timestamp
		}
	}
	return strings.TrimSpace(plainText.String()), nil
}

// removeVttTags attempts to remove common VTT styling/timing tags from a line.
func removeVttTags(line string) string {
	// Regex to remove tags like <...> e.g. <v Author>, <c.color>, <00:00:00.000>
	// This is a simplified regex. More complex VTTs might need a more robust parser.
	// It removes content between < and >.
	var result strings.Builder
	inTag := false
	for _, r := range line {
		if r == '<' {
			inTag = true
		} else if r == '>' {
			inTag = false
		} else if !inTag {
			result.WriteRune(r)
		}
	}
	return result.String()
}
