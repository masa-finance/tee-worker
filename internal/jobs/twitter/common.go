package twitter

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/joho/godotenv"
	"github.com/masa-finance/tee-worker/api/types"
	"github.com/sirupsen/logrus"
)

var (
	accountManager *TwitterAccountManager
	once           sync.Once
)

func initializeAccountManager() {
	accounts := loadAccountsFromConfig()
	apiKeys := loadApiKeysFromConfig()
	accountManager = NewTwitterAccountManager(accounts, apiKeys)
}

func loadAccountsFromConfig() []*types.TwitterAccount {
	err := godotenv.Load()
	if err != nil {
		logrus.Fatalf("error loading .env file: %v", err)
	}

	accountsEnv := os.Getenv("TWITTER_ACCOUNTS")
	if accountsEnv == "" {
		logrus.Fatal("TWITTER_ACCOUNTS not set in .env file")
	}

	return parseAccounts(strings.Split(accountsEnv, ","))
}

func loadApiKeysFromConfig() []*types.TwitterApiKey {
	err := godotenv.Load()
	if err != nil {
		logrus.Fatalf("error loading .env file: %v", err)
	}

	apiKeysEnv := os.Getenv("TWITTER_API_KEYS")
	if apiKeysEnv == "" {
		// optional
		logrus.Info("TWITTER_API_KEYS not set in .env file")
		return nil
	}

	return parseApiKeys(strings.Split(apiKeysEnv, ","))
}

func parseApiKeys(apiKeys []string) []*types.TwitterApiKey {
	return filterMap(apiKeys, func(key string) (*types.TwitterApiKey, bool) {
		return &types.TwitterApiKey{
			Key: strings.TrimSpace(key),
		}, true
	})
}

func parseAccounts(accountPairs []string) []*types.TwitterAccount {
	return filterMap(accountPairs, func(pair string) (*types.TwitterAccount, bool) {
		credentials := strings.Split(pair, ":")
		if len(credentials) != 2 {
			logrus.Warnf("invalid account credentials: %s", pair)
			return nil, false
		}
		return &types.TwitterAccount{
			Username: strings.TrimSpace(credentials[0]),
			Password: strings.TrimSpace(credentials[1]),
		}, true
	})
}

func getAuthenticatedScraper(baseDir string) (*Scraper, *types.TwitterAccount, error) {
	once.Do(initializeAccountManager)

	account := accountManager.GetNextAccount()
	if account == nil {
		return nil, nil, fmt.Errorf("all accounts are rate-limited")
	}

	// Check if we should skip login verification from environment
	skipVerification := os.Getenv("TWITTER_SKIP_LOGIN_VERIFICATION") == "true"

	authConfig := AuthConfig{
		Account:               account,
		BaseDir:               baseDir,
		SkipLoginVerification: skipVerification,
	}

	scraper := NewScraper(authConfig)
	if scraper == nil {
		logrus.Errorf("Authentication failed for %s", account.Username)
		return nil, account, fmt.Errorf("twitter authentication failed for %s", account.Username)
	}
	return scraper, account, nil
}

func handleError(err error, account *types.TwitterAccount) bool {
	if strings.Contains(err.Error(), "Rate limit exceeded") {
		accountManager.MarkAccountRateLimited(account)
		logrus.Warnf("rate limited: %s", account.Username)
		return true
	}
	return false
}

func filterMap[T any, R any](slice []T, f func(T) (R, bool)) []R {
	result := make([]R, 0, len(slice))
	for _, v := range slice {
		if r, ok := f(v); ok {
			result = append(result, r)
		}
	}
	return result
}
