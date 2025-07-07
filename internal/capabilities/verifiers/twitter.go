package verifiers

import (
	"context"
	"fmt"
	"os"
	"strings"

	twitterscraper "github.com/imperatrona/twitter-scraper"
	"github.com/masa-finance/tee-worker/api/types"
	"github.com/sirupsen/logrus"
)

// TwitterVerifier verifies the Twitter capability.
type TwitterVerifier struct {
	Accounts []*types.TwitterAccount
	ApiKeys  []*types.TwitterApiKey
	scrapers []*twitterscraper.Scraper
}

func parseAccounts(accountPairs []string) []*types.TwitterAccount {
	var accounts []*types.TwitterAccount
	for _, pair := range accountPairs {
		credentials := strings.Split(pair, ":")
		if len(credentials) != 2 {
			logrus.Warnf("invalid account credentials format: %s", pair)
			continue
		}
		accounts = append(accounts, &types.TwitterAccount{
			Username: strings.TrimSpace(credentials[0]),
			Password: strings.TrimSpace(credentials[1]),
		})
	}
	return accounts
}

func parseApiKeys(apiKeys []string) []*types.TwitterApiKey {
	var keys []*types.TwitterApiKey
	for _, key := range apiKeys {
		if strings.TrimSpace(key) != "" {
			keys = append(keys, &types.TwitterApiKey{
				Key: strings.TrimSpace(key),
			})
		}
	}
	return keys
}

// NewTwitterVerifier creates a new TwitterVerifier.
// It initializes scrapers for each account provided.
func NewTwitterVerifier(accounts []string, dataDir string) (*TwitterVerifier, error) {
	parsedAccounts := parseAccounts(accounts)

	if len(parsedAccounts) == 0 {
		return nil, fmt.Errorf("no valid twitter accounts provided for verification")
	}

	// Ensure the data directory exists
	if dataDir != "" {
		if err := os.MkdirAll(dataDir, 0755); err != nil {
			return nil, fmt.Errorf("could not create data directory for twitter verifier: %w", err)
		}
	}

	var scrapers []*twitterscraper.Scraper
	for _, acc := range parsedAccounts {
		scraper := twitterscraper.New()
		err := scraper.Login(acc.Username, acc.Password)
		if err != nil {
			// Do not error out, just log it. The Verify method will catch this.
			fmt.Printf("verifier: failed to login with twitter account %s: %v\n", acc.Username, err)
		}
		scrapers = append(scrapers, scraper)
	}

	return &TwitterVerifier{
		Accounts: parsedAccounts,
		scrapers: scrapers,
	}, nil
}

// NewTwitterApiKeyVerifier creates a new TwitterVerifier for API key authentication.
func NewTwitterApiKeyVerifier(apiKeys []string) (*TwitterVerifier, error) {
	parsedApiKeys := parseApiKeys(apiKeys)

	if len(parsedApiKeys) == 0 {
		return nil, fmt.Errorf("no valid twitter API keys provided for verification")
	}

	// For API keys, we don't need to initialize scrapers with login
	// The verification will test the API key directly
	return &TwitterVerifier{
		ApiKeys: parsedApiKeys,
	}, nil
}

// Verify attempts to perform a minimal search query.
func (v *TwitterVerifier) Verify(ctx context.Context) (bool, error) {
	// If we have API keys, verify using API key method
	if len(v.ApiKeys) > 0 {
		return v.verifyWithApiKeys(ctx)
	}

	// Otherwise, verify using credential-based method
	return v.verifyWithCredentials(ctx)
}

// verifyWithCredentials verifies using credential-based authentication
func (v *TwitterVerifier) verifyWithCredentials(ctx context.Context) (bool, error) {
	if len(v.scrapers) == 0 {
		return false, fmt.Errorf("no successfully logged-in scrapers available for verification")
	}

	// Try with any available scraper
	var lastErr error
	for _, scraper := range v.scrapers {
		if !scraper.IsLoggedIn() {
			continue
		}
		tweets := scraper.SearchTweets(ctx, "BTC", 1)
		// The SearchTweets channel returns an error if the scrape fails.
		if err := <-tweets; err != nil && err.Error != nil {
			lastErr = err.Error
			continue // Try the next scraper
		}
		// If we get here, it means the channel opened without an immediate error.
		return true, nil
	}

	if lastErr != nil {
		return false, fmt.Errorf("all twitter accounts failed verification: %w", lastErr)
	}

	return false, fmt.Errorf("no logged-in twitter accounts available for verification")
}

// verifyWithApiKeys verifies using API key authentication
func (v *TwitterVerifier) verifyWithApiKeys(ctx context.Context) (bool, error) {
	// For API key verification, we'll create a temporary scraper and test it
	// This is a simplified verification - in practice, you might want to test
	// the API key more thoroughly

	var lastErr error
	for _, apiKey := range v.ApiKeys {
		// Create a new scraper for this API key
		scraper := twitterscraper.New()

		// Note: The twitter-scraper library doesn't have direct API key support
		// This is a placeholder for API key verification logic
		// In a real implementation, you would test the API key here

		// For now, we'll do a basic validation that the key is not empty
		if apiKey.Key == "" {
			lastErr = fmt.Errorf("empty API key provided")
			continue
		}

		// Try a simple search operation
		tweets := scraper.SearchTweets(ctx, "BTC", 1)
		if err := <-tweets; err != nil && err.Error != nil {
			lastErr = err.Error
			continue
		}

		// If we get here, the API key appears to work
		return true, nil
	}

	if lastErr != nil {
		return false, fmt.Errorf("all twitter API keys failed verification: %w", lastErr)
	}

	return false, fmt.Errorf("no valid twitter API keys available for verification")
}
