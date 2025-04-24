package twitter

import (
	"fmt"
	"strings"
	"github.com/masa-finance/tee-worker/pkg/client"
)

// DetectTwitterKeyType tries to determine if the key is base, elevated, or credential.
func DetectTwitterKeyType(apiKey string) (string, error) {
	if strings.Contains(apiKey, ":") {
		return "credential", nil
	}
	// Try a harmless full archive search (tweets/search/all)
	tx := client.NewTwitterXClient(apiKey)
	endpoint := "tweets/search/all?query=from:twitterdev&max_results=10"
	resp, err := tx.Get(endpoint)
	if err != nil {
		return "", fmt.Errorf("request error: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case 200:
		return "elevated", nil
	case 401, 403:
		return "base", nil
	default:
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
}

// SetKeyType sets the Type field on the TwitterApiKey struct
func (k *TwitterApiKey) SetKeyType() error {
	typeStr, err := DetectTwitterKeyType(k.Key)
	if err != nil {
		return err
	}
	k.Type = typeStr
	return nil
}
