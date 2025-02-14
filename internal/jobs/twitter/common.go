package twitter

import (
	"fmt"
	"github.com/masa-finance/tee-worker/internal/jobs/twitterx"
	"os"
	"strings"
	"sync"

	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
)

var (
	accountManager *TwitterAccountManager
	apiKeyManager  *twitterx.TwitterXApiKeyManager
	once           sync.Once
)

func initializeAccountManager() {
	accounts := loadAccountsFromConfig()
	apiKeys := loadApiKeysFromConfig()
	accountManager = NewTwitterAccountManager(accounts)
	apiKeyManager = twitterx.NewTwitterApiKeyManager(apiKeys)
}

func loadAccountsFromConfig() []*TwitterAccount {
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

func loadApiKeysFromConfig() []*twitterx.TwitterXApiKey {
	err := godotenv.Load()
	if err != nil {
		logrus.Fatalf("error loading .env file: %v", err)
	}

	apiKeysEnv := os.Getenv("TWITTER_API_KEYS")
	if apiKeysEnv == "" {
		logrus.Fatal("TWITTER_API_KEYS not set in .env file")
	}

	return parseApiKeys(strings.Split(apiKeysEnv, ","))
}

func parseApiKeys(apiKeys []string) []*twitterx.TwitterXApiKey {
	return filterMap(apiKeys, func(key string) (*twitterx.TwitterXApiKey, bool) {
		return &twitterx.TwitterXApiKey{
			Key: strings.TrimSpace(key),
		}, true
	})
}

func parseAccounts(accountPairs []string) []*TwitterAccount {
	return filterMap(accountPairs, func(pair string) (*TwitterAccount, bool) {
		credentials := strings.Split(pair, ":")
		if len(credentials) != 2 {
			logrus.Warnf("invalid account credentials: %s", pair)
			return nil, false
		}
		return &TwitterAccount{
			Username: strings.TrimSpace(credentials[0]),
			Password: strings.TrimSpace(credentials[1]),
		}, true
	})
}

func getAuthenticatedScraper(baseDir string) (*Scraper, *TwitterAccount, error) {
	once.Do(initializeAccountManager)

	apiKey := apiKeyManager.GetNextApiKey()
	if apiKey != nil {

	}

	account := accountManager.GetNextAccount()
	if account == nil {
		return nil, nil, fmt.Errorf("all accounts are rate-limited")
	}

	authConfig := AuthConfig{
		Account: account,
		BaseDir: baseDir,
	}

	scraper := NewScraper(authConfig)
	if scraper == nil {
		logrus.Errorf("Authentication failed for %s", account.Username)
		return nil, account, fmt.Errorf("Twitter authentication failed for %s", account.Username)
	}
	return scraper, account, nil
}

func handleError(err error, account *TwitterAccount) bool {
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
