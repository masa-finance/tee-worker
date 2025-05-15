package jobs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/masa-finance/tee-worker/api/types"
	"github.com/masa-finance/tee-worker/internal/jobs/stats"
	"github.com/sirupsen/logrus"
)

// TikTokTranscriptionType is the job type identifier for TikTok transcriptions.
const TikTokTranscriptionType = "tiktok-transcription"

// tiktokTranscriptionEndpoint is the default hardcoded endpoint for TikTok transcriptions.
const tiktokTranscriptionEndpoint = "https://submagic-free-tools.fly.dev/api/tiktok-transcription"

// TikTokTranscriptionArgs defines the arguments required for a TikTok transcription job.
type TikTokTranscriptionArgs struct {
	VideoURL string `json:"video_url"`
	Language string `json:"language,omitempty"` // e.g., "eng-US"
}

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

// TikTokTranscriptionResult defines the structure of the result data for a TikTok transcription.
type TikTokTranscriptionResult struct {
	TranscriptionText string `json:"transcription_text"`
	DetectedLanguage  string `json:"detected_language,omitempty"`
	VideoTitle        string `json:"video_title,omitempty"`
	OriginalURL       string `json:"original_url"`
	ThumbnailURL      string `json:"thumbnail_url,omitempty"`
}

// NewTikTokTranscriber creates and initializes a new TikTokTranscriber.
// It populates its configuration from the global job configuration map (jc)
// which is initially loaded by readConfig() in main.go.
func NewTikTokTranscriber(jc types.JobConfiguration, statsCollector *stats.StatsCollector) *TikTokTranscriber {
	config := TikTokTranscriptionConfiguration{}

	// Manually unmarshal from jc (map[string]interface{}) to config struct
	if err := jc.Unmarshal(&config); err != nil {
		logrus.WithError(err).Error("TikTokTranscriber: Failed to unmarshal job configuration into TikTokTranscriptionConfiguration")
		// Depending on policy, could return nil or a non-functional transcriber.
		// For now, proceed with potentially zero-value config, ExecuteJob should check critical fields.
	}

	// Hardcode the TranscriptionEndpoint, overriding any value from jc
	config.TranscriptionEndpoint = tiktokTranscriptionEndpoint
	logrus.Info("TikTokTranscriber: Using hardcoded TranscriptionEndpoint: ", config.TranscriptionEndpoint)

	if config.APIOrigin == "" {
		config.APIOrigin = "https://submagic-free-tools.fly.dev"
		logrus.Info("TikTokTranscriber: APIOrigin not configured, using default: ", config.APIOrigin)
	}

	if config.APIReferer == "" {
		config.APIReferer = "https://submagic-free-tools.fly.dev/tiktok-transcription"
		logrus.Info("TikTokTranscriber: APIReferer not configured, using default: ", config.APIReferer)
	}

	if config.APIUserAgent == "" {
		config.APIUserAgent = "Mozilla/5.0 (Linux; Android 6.0; Nexus 5 Build/MRA58N) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/135.0.0.0 Mobile Safari/537.36"
		logrus.Info("TikTokTranscriber: APIUserAgent not configured, using default.")
	}

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

	args := &TikTokTranscriptionArgs{}
	if err := j.Arguments.Unmarshal(args); err != nil {
		ttt.stats.Add(j.WorkerID, stats.TikTokTranscriptionErrors, 1)
		return types.JobResult{Error: "Failed to unmarshal job arguments"}, fmt.Errorf("unmarshal job arguments: %w", err)
	}

	if args.VideoURL == "" {
		ttt.stats.Add(j.WorkerID, stats.TikTokTranscriptionErrors, 1)
		return types.JobResult{Error: "VideoURL is required"}, fmt.Errorf("videoURL is required")
	}
	// Sanitize/Validate VideoURL further if necessary (e.g., ensure it's a TikTok URL)

	// Placeholder for language selection logic
	selectedLanguageKey := args.Language
	if selectedLanguageKey == "" {
		selectedLanguageKey = ttt.configuration.DefaultLanguage
	}
	// If still empty, a hardcoded default like "eng-US" or first available will be used later

	// Sub-Step 3.1: Call TikTok Transcription API
	apiRequestBody := map[string]string{"url": args.VideoURL}
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
		"url":          args.VideoURL,
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
		bodyBytes, _ := ReadAll(apiResp.Body)
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
	finalDetectedLanguage := ""

	// Try requested/default language
	if selectedLanguageKey != "" {
		if transcript, ok := parsedAPIResponse.Transcripts[selectedLanguageKey]; ok {
			vttText = transcript
			finalDetectedLanguage = selectedLanguageKey
		}
	}

	// If not found, try a hardcoded common default or first available
	if vttText == "" {
		commonDefault := "eng-US" // As per spec
		if transcript, ok := parsedAPIResponse.Transcripts[commonDefault]; ok {
			vttText = transcript
			finalDetectedLanguage = commonDefault
		} else { // Pick the first one available if commonDefault also not found
			for lang, transcript := range parsedAPIResponse.Transcripts {
				vttText = transcript
				finalDetectedLanguage = lang
				logrus.WithFields(logrus.Fields{
					"job_uuid":       j.UUID,
					"requested_lang": selectedLanguageKey,
					"fallback_used":  finalDetectedLanguage,
				}).Info("Requested/default language not found, using first available transcript")
				break
			}
		}
	}

	if vttText == "" {
		errMsg := "Suitable transcript could not be extracted from API response"
		logrus.WithField("job_uuid", j.UUID).Error(errMsg)
		ttt.stats.Add(j.WorkerID, stats.TikTokTranscriptionErrors, 1)
		return types.JobResult{Error: errMsg}, fmt.Errorf(errMsg)
	}

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
	resultData := TikTokTranscriptionResult{
		TranscriptionText: plainTextTranscription,
		DetectedLanguage:  finalDetectedLanguage,
		VideoTitle:        parsedAPIResponse.VideoTitle,
		OriginalURL:       args.VideoURL,
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

		if trimmedLine == "" || strings.HasPrefix(trimmedLine, "WEBVTT") || strings.HasPrefix(trimmedLine, "NOTE") {
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

// ReadAll is a helper to read an io.Reader (like http.Response.Body) fully.
// Needed because io.ReadAll is not available in older Go versions used by the project context.
func ReadAll(r io.Reader) ([]byte, error) {
	var buf bytes.Buffer
	_, err := buf.ReadFrom(r)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Ensure TikTokTranscriber implements the JobHandler interface (if one exists, common pattern)
// var _ types.JobHandler = (*TikTokTranscriber)(nil)

// Dummy types.JobArgument to make the code compile standalone until integrated.
// This should be removed once the actual types.JobArgument is confirmed.
// In the real system, j.Arguments.UnmarshalTo(args) would be provided by the types package.

// Add a simple main for standalone testing if needed (remove for production)
/*
func main() {
	// Example of how to use NewTikTokTranscriber
	mockJobConfig := types.JobConfiguration{
		"tiktok_transcription_endpoint": "https://submagic-free-tools.fly.dev/api/tiktok-transcription",
		"tiktok_api_origin":             "https://submagic-free-tools.fly.dev",
		"tiktok_api_referer":            "https://submagic-free-tools.fly.dev/tiktok-transcription",
		"tiktok_api_user_agent":         "Mozilla/5.0 (Linux; Android 6.0; Nexus 5 Build/MRA58N) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/135.0.0.0 Mobile Safari/537.36",
		"tiktok_default_language":       "eng-US",
	}
	mockStatsCollector := &stats.StatsCollector{} // Replace with actual stats collector init if possible

	transcriber := NewTikTokTranscriber(mockJobConfig, mockStatsCollector)
	if transcriber == nil {
		logrus.Fatal("Failed to create transcriber")
	}
	logrus.Infof("Transcriber created with endpoint: %s", transcriber.configuration.TranscriptionEndpoint)

	// Example of how to call ExecuteJob
	mockJob := types.Job{
		ID:       "test-job-123",
		WorkerID: "test-worker",
		Type:     TikTokTranscriptionType,
		Arguments: types.JobArgument(map[string]interface{}{ // This needs to be compatible with how JobArgument is actually defined
			"video_url": "https://www.tiktok.com/@coachty23/video/7502100651397172526",
			// "language": "cat-ES",
		}),
	}

	// To make types.JobArgument(map[string]interface{}{...}) work,
	// we need a concrete type for JobArgument or a constructor.
	// For now, let's assume JobArgument can be created from a map for testing.
	// This part is tricky without knowing the exact definition of types.JobArgument
	// and its UnmarshalTo method.

	// The following will not compile directly without a proper types.JobArgument that has UnmarshalTo.
	// For now, this main function is for conceptual testing.
	// result, err := transcriber.ExecuteJob(mockJob)
	// if err != nil {
	// 	logrus.Fatalf("ExecuteJob failed: %v, Result Error: %s", err, result.Error)
	// }
	// logrus.Infof("ExecuteJob successful. Result Data: %s", string(result.Data))
}
*/

// Placeholder for io.Reader if not directly available
type ioReader interface {
	Read(p []byte) (n int, err error)
}

// Interface for io.Closer
type ioCloser interface {
	Close() error
}

// Combined interface for io.ReadCloser
type ioReadCloser interface {
	ioReader
	ioCloser
}

// Note: The `io` package stubs (ioReader, ioCloser, ioReadCloser, ReadAll)
// and the `main` function are for ensuring the code is self-contained for review
// and basic compilation checks if the actual `io` or `types.JobArgument` isn't
// fully defined in this context. They should be removed or reconciled with the
// actual project dependencies.
// The `bytes.ReadAll` and `io.ReadAll` have been replaced with a custom `ReadAll`
// and `bytes.Buffer.ReadFrom` for broader Go version compatibility if needed.
// The actual `io.ReadAll` from the `io` package should be preferred if available.
// The placeholder `APIResponse` and `convertVTTToPlainText` are also part of fulfilling the spec.
