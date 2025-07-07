package verifiers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/masa-finance/tee-worker/api/types"
)

const verificationTikTokURL = "https://www.tiktok.com/@theblockrunner.com/video/7227579907361066282"
const tiktokTranscriptionEndpoint = "https://submagic-free-tools.fly.dev/api/tiktok-transcription"

// TikTokVerifier verifies the TikTok transcription capability.
type TikTokVerifier struct {
	Client *http.Client
	Config types.TikTokTranscriptionConfiguration
}

// NewTikTokVerifier creates a new TikTokVerifier.
func NewTikTokVerifier() *TikTokVerifier {
	// Replicating the configuration setup from the actual job handler
	config := types.TikTokTranscriptionConfiguration{
		TranscriptionEndpoint: tiktokTranscriptionEndpoint,
		APIOrigin:             "https://submagic-free-tools.fly.dev",
		APIReferer:            "https://submagic-free-tools.fly.dev/tiktok-transcription",
		APIUserAgent:          "Mozilla/5.0 (Linux; Android 6.0; Nexus 5 Build/MRA58N) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/135.0.0.0 Mobile Safari/537.36",
		DefaultLanguage:       "eng-US",
	}

	return &TikTokVerifier{
		Client: &http.Client{
			Timeout: 30 * time.Second,
		},
		Config: config,
	}
}

// Verify attempts to fetch a transcript for a known video.
func (v *TikTokVerifier) Verify(ctx context.Context) (bool, error) {
	apiRequestBody := map[string]string{"url": verificationTikTokURL}
	jsonBody, err := json.Marshal(apiRequestBody)
	if err != nil {
		return false, fmt.Errorf("failed to marshal verification request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", v.Config.TranscriptionEndpoint, bytes.NewBuffer(jsonBody))
	if err != nil {
		return false, fmt.Errorf("failed to create verification request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", v.Config.APIOrigin)
	req.Header.Set("Referer", v.Config.APIReferer)
	req.Header.Set("User-Agent", v.Config.APIUserAgent)

	resp, err := v.Client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("verification failed with status code: %d", resp.StatusCode)
	}

	var parsedAPIResponse types.TikTokAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsedAPIResponse); err != nil {
		return false, fmt.Errorf("failed to parse verification response: %w", err)
	}

	if parsedAPIResponse.Error != "" {
		return false, fmt.Errorf("verification API returned an error: %s", parsedAPIResponse.Error)
	}

	if len(parsedAPIResponse.Transcripts) == 0 {
		return false, fmt.Errorf("verification response contained no transcripts")
	}

	return true, nil
}
