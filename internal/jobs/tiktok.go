package jobs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	teeargs "github.com/masa-finance/tee-types/args"
	teetypes "github.com/masa-finance/tee-types/types"
	"github.com/masa-finance/tee-worker/api/types"
	"github.com/masa-finance/tee-worker/internal/jobs/stats"
	"github.com/masa-finance/tee-worker/internal/jobs/tiktokapify"
	"github.com/masa-finance/tee-worker/pkg/client"
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
	ApifyApiKey           string `json:"apify_api_key,omitempty"`
}

// TikTokTranscriber is the main job struct for handling TikTok transcriptions.
type TikTokTranscriber struct {
	configuration TikTokTranscriptionConfiguration
	stats         *stats.StatsCollector
	httpClient    *http.Client
}

// GetStructuredCapabilities returns the structured capabilities supported by the TikTok transcriber
func (t *TikTokTranscriber) GetStructuredCapabilities() teetypes.WorkerCapabilities {
	caps := make([]teetypes.Capability, 0, len(teetypes.AlwaysAvailableTiktokCaps)+len(teetypes.TiktokSearchCaps))
	caps = append(caps, teetypes.AlwaysAvailableTiktokCaps...)
	if t.configuration.ApifyApiKey != "" {
		caps = append(caps, teetypes.TiktokSearchCaps...)
	}
	return teetypes.WorkerCapabilities{
		teetypes.TiktokJob: caps,
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
		logrus.WithError(err).Warn("failed to unmarshal TikTokTranscriptionConfiguration from JobConfiguration, using defaults where applicable")
	}
	// Ensure Apify key aligns with Twitter's pattern (explicit getter wins)
	config.ApifyApiKey = jc.GetString("apify_api_key", config.ApifyApiKey)
	if config.ApifyApiKey != "" {
		if c, err := tiktokapify.NewTikTokApifyClient(config.ApifyApiKey); err != nil {
			logrus.Errorf("Failed to create Apify client at startup: %v", err)
		} else if err := c.ValidateApiKey(); err != nil {
			logrus.Errorf("Apify API key validation failed at startup: %v", err)
		} else {
			logrus.Infof("Apify API key validated successfully at startup")
		}
	}

	// Note: APIUserAgent is optional, it can be set later or use a default
	if config.APIUserAgent == "" {
		config.APIUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"
	}

	// If a default language is set in the configuration, use it
	if config.DefaultLanguage == "" {
		config.DefaultLanguage = "eng-US"
	}

	return &TikTokTranscriber{
		configuration: config,
		stats:         statsCollector,
		httpClient:    &http.Client{Timeout: 30 * time.Second},
	}
}

// NewTikTokScraper is an alias constructor to align with Twitter's naming pattern
func NewTikTokScraper(jc types.JobConfiguration, statsCollector *stats.StatsCollector) *TikTokTranscriber {
	return NewTikTokTranscriber(jc, statsCollector)
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
	logrus.WithField("job_uuid", j.UUID).Info("Starting ExecuteJob for TikTok job")

	// Use the centralized type-safe unmarshaller
	jobArgs, err := teeargs.UnmarshalJobArguments(teetypes.JobType(j.Type), map[string]any(j.Arguments))
	if err != nil {
		return types.JobResult{Error: "Failed to unmarshal job arguments"}, fmt.Errorf("unmarshal job arguments: %w", err)
	}

	// Branch by argument type (transcription vs search)
	if transcriptionArgs, ok := teeargs.AsTikTokTranscriptionArguments(jobArgs); ok {
		return ttt.executeTranscription(j, transcriptionArgs)
	}
	if searchByQueryArgs, ok := teeargs.AsTikTokSearchByQueryArguments(jobArgs); ok {
		return ttt.executeSearchByQuery(j, searchByQueryArgs)
	}
	if searchByTrendingArgs, ok := teeargs.AsTikTokSearchByTrendingArguments(jobArgs); ok {
		return ttt.executeSearchByTrending(j, searchByTrendingArgs)
	}

	// Fallback: treat as searchbyquery (default capability)
	searchByQueryArgs, ok := teeargs.AsTikTokSearchByQueryArguments(jobArgs)
	if !ok {
		return types.JobResult{Error: "invalid argument type for TikTok job"}, fmt.Errorf("invalid argument type")
	}
	return ttt.executeSearchByQuery(j, searchByQueryArgs)
}

// executeTranscription calls the external transcription service and returns a normalized result
func (ttt *TikTokTranscriber) executeTranscription(j types.Job, a *teeargs.TikTokTranscriptionArguments) (types.JobResult, error) {
	if ttt.configuration.TranscriptionEndpoint == "" {
		ttt.stats.Add(j.WorkerID, stats.TikTokTranscriptionErrors, 1)
		return types.JobResult{Error: "TikTok transcription endpoint is not configured for the worker"}, fmt.Errorf("tiktok transcription endpoint not configured")
	}

	reqBody := map[string]any{
		"url": a.GetVideoURL(),
	}
	if a.HasLanguagePreference() {
		reqBody["language"] = a.GetLanguageCode()
	}

	payload, _ := json.Marshal(reqBody)
	req, err := http.NewRequest(http.MethodPost, ttt.configuration.TranscriptionEndpoint, bytes.NewReader(payload))
	if err != nil {
		return types.JobResult{Error: "Failed to create request"}, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if ttt.configuration.APIOrigin != "" {
		req.Header.Set("Origin", ttt.configuration.APIOrigin)
	}
	if ttt.configuration.APIReferer != "" {
		req.Header.Set("Referer", ttt.configuration.APIReferer)
	}
	if ttt.configuration.APIUserAgent != "" {
		req.Header.Set("User-Agent", ttt.configuration.APIUserAgent)
	}

	resp, err := ttt.httpClient.Do(req)
	if err != nil {
		ttt.stats.Add(j.WorkerID, stats.TikTokTranscriptionErrors, 1)
		return types.JobResult{Error: "Failed to call transcription endpoint"}, fmt.Errorf("http call: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		ttt.stats.Add(j.WorkerID, stats.TikTokTranscriptionErrors, 1)
		return types.JobResult{Error: fmt.Sprintf("transcription endpoint returned status %d", resp.StatusCode)}, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	var apiResp APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		ttt.stats.Add(j.WorkerID, stats.TikTokTranscriptionErrors, 1)
		return types.JobResult{Error: "Failed to parse response"}, fmt.Errorf("decode response: %w", err)
	}
	if apiResp.Error != "" {
		ttt.stats.Add(j.WorkerID, stats.TikTokTranscriptionErrors, 1)
		return types.JobResult{Error: apiResp.Error}, fmt.Errorf("api error: %s", apiResp.Error)
	}

	// Pick transcript language
	chosenLang := a.GetLanguageCode()
	transcriptVTT, ok := apiResp.Transcripts[chosenLang]
	if !ok {
		for lang, v := range apiResp.Transcripts {
			chosenLang = lang
			transcriptVTT = v
			break
		}
	}
	if transcriptVTT == "" {
		ttt.stats.Add(j.WorkerID, stats.TikTokTranscriptionErrors, 1)
		return types.JobResult{Error: "no transcripts available in response"}, fmt.Errorf("no transcripts available")
	}

	text, err := convertVTTToPlainText(transcriptVTT)
	if err != nil {
		ttt.stats.Add(j.WorkerID, stats.TikTokTranscriptionErrors, 1)
		return types.JobResult{Error: "failed to parse transcript"}, fmt.Errorf("parse vtt: %w", err)
	}

	result := teetypes.TikTokTranscriptionResult{
		TranscriptionText: text,
		DetectedLanguage:  chosenLang,
		VideoTitle:        apiResp.VideoTitle,
		OriginalURL:       a.GetVideoURL(),
		ThumbnailURL:      apiResp.ThumbnailURL,
	}

	data, err := json.Marshal(result)
	if err != nil {
		return types.JobResult{Error: "failed to marshal result"}, fmt.Errorf("marshal result: %w", err)
	}

	ttt.stats.Add(j.WorkerID, stats.TikTokTranscriptionSuccess, 1)
	return types.JobResult{Data: data}, nil
}

// executeSearchByQuery runs the epctex/tiktok-search-scraper actor and returns results
func (ttt *TikTokTranscriber) executeSearchByQuery(j types.Job, a *teeargs.TikTokSearchByQueryArguments) (types.JobResult, error) {
	if ttt.configuration.ApifyApiKey == "" {
		ttt.stats.Add(j.WorkerID, stats.TikTokErrors, 1)
		return types.JobResult{Error: "Apify API key not configured for searchbyquery"}, fmt.Errorf("missing Apify API key")
	}

	c, err := tiktokapify.NewTikTokApifyClient(ttt.configuration.ApifyApiKey)
	if err != nil {
		ttt.stats.Add(j.WorkerID, stats.TikTokErrors, 1)
		return types.JobResult{Error: "Failed to create Apify client"}, fmt.Errorf("apify client: %w", err)
	}

	limit := a.MaxItems
	if limit <= 0 {
		limit = 20
	}

	items, next, err := c.SearchByQuery(*a, client.EmptyCursor, limit)
	if err != nil {
		ttt.stats.Add(j.WorkerID, stats.TikTokErrors, 1)
		return types.JobResult{Error: err.Error()}, err
	}

	data, err := json.Marshal(items)
	if err != nil {
		// Do not increment error stats for marshal errors; not the worker's fault
		return types.JobResult{Error: "Failed to marshal results"}, fmt.Errorf("marshal results: %w", err)
	}

	// Increment returned videos based on the number of items
	ttt.stats.Add(j.WorkerID, stats.TikTokVideos, uint(len(items)))
	return types.JobResult{Data: data, NextCursor: next.String()}, nil
}

// executeSearchByTrending runs the lexis-solutions/tiktok-trending-videos-scraper actor and returns results
func (ttt *TikTokTranscriber) executeSearchByTrending(j types.Job, a *teeargs.TikTokSearchByTrendingArguments) (types.JobResult, error) {
	if ttt.configuration.ApifyApiKey == "" {
		ttt.stats.Add(j.WorkerID, stats.TikTokErrors, 1)
		return types.JobResult{Error: "Apify API key not configured for searchbytrending"}, fmt.Errorf("missing Apify API key")
	}

	c, err := tiktokapify.NewTikTokApifyClient(ttt.configuration.ApifyApiKey)
	if err != nil {
		ttt.stats.Add(j.WorkerID, stats.TikTokErrors, 1)
		return types.JobResult{Error: "Failed to create Apify client"}, fmt.Errorf("apify client: %w", err)
	}

	limit := a.MaxItems
	if limit <= 0 {
		limit = 20
	}

	items, next, err := c.SearchByTrending(*a, client.EmptyCursor, limit)
	if err != nil {
		ttt.stats.Add(j.WorkerID, stats.TikTokErrors, 1)
		return types.JobResult{Error: err.Error()}, err
	}

	data, err := json.Marshal(items)
	if err != nil {
		// Do not increment error stats for marshal errors; not the worker's fault
		return types.JobResult{Error: "Failed to marshal results"}, fmt.Errorf("marshal results: %w", err)
	}

	// Increment returned videos based on the number of items
	ttt.stats.Add(j.WorkerID, stats.TikTokVideos, uint(len(items)))
	return types.JobResult{Data: data, NextCursor: next.String()}, nil
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
