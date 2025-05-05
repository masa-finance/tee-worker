package twitter

import (
	"fmt"
	"github.com/masa-finance/tee-worker/pkg/client"
	"strings"
	"sync"
	"time"
)

type TwitterAccount struct {
	Username         string
	Password         string
	TwoFACode        string
	RateLimitedUntil time.Time
}

type TwitterApiKeyType string

const (
	TwitterApiKeyTypeBase       TwitterApiKeyType = "base"
	TwitterApiKeyTypeElevated   TwitterApiKeyType = "elevated"
	TwitterApiKeyTypeCredential TwitterApiKeyType = "credential"
	TwitterApiKeyTypeUnknown    TwitterApiKeyType = "unknown"
)

type TwitterApiKey struct {
	Key  string
	Type TwitterApiKeyType // "base" or "elevated"
}

type TwitterAccountManager struct {
	accounts []*TwitterAccount
	apiKeys  []*TwitterApiKey
	index    int
	mutex    sync.Mutex
}

func NewTwitterAccountManager(accounts []*TwitterAccount, apiKeys []*TwitterApiKey) *TwitterAccountManager {
	return &TwitterAccountManager{
		accounts: accounts,
		apiKeys:  apiKeys,
		index:    0,
	}
}

func (manager *TwitterAccountManager) GetNextAccount() *TwitterAccount {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()
	for i := 0; i < len(manager.accounts); i++ {
		account := manager.accounts[manager.index]
		manager.index = (manager.index + 1) % len(manager.accounts)
		if time.Now().After(account.RateLimitedUntil) {
			return account
		}
	}
	return nil
}

// DetectAllApiKeyTypes checks and sets the Type for all apiKeys in the manager.
func (manager *TwitterAccountManager) DetectAllApiKeyTypes() {
	for _, key := range manager.apiKeys {
		err := key.SetKeyType()
		if err != nil {
			key.Type = TwitterApiKeyTypeUnknown
		}
	}
}

// GetApiKeys returns all api keys managed by this manager
func (manager *TwitterAccountManager) GetApiKeys() []*TwitterApiKey {
	return manager.apiKeys
}
func (manager *TwitterAccountManager) GetNextApiKey() *TwitterApiKey {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()
	if len(manager.apiKeys) == 0 {
		return nil
	}
	key := manager.apiKeys[manager.index]
	manager.index = (manager.index + 1) % len(manager.apiKeys)
	return key
}

func (manager *TwitterAccountManager) MarkAccountRateLimited(account *TwitterAccount) {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()
	account.RateLimitedUntil = time.Now().Add(GetRateLimitDuration())
}

func detectTwitterKeyType(apiKey string) (TwitterApiKeyType, error) {
	if strings.Contains(apiKey, ":") {
		return TwitterApiKeyTypeCredential, nil
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
		return TwitterApiKeyTypeElevated, nil
	case 401, 403:
		return TwitterApiKeyTypeBase, nil
	default:
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
}

func (k *TwitterApiKey) SetKeyType() error {
	typeStr, err := detectTwitterKeyType(k.Key)
	if err != nil {
		return err
	}
	k.Type = typeStr
	return nil
}
