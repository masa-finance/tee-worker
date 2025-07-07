package twitter

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/masa-finance/tee-worker/api/types"
	"github.com/masa-finance/tee-worker/pkg/client"
)

type TwitterAccountManager struct {
	accounts []*types.TwitterAccount
	apiKeys  []*types.TwitterApiKey
	index    int
	mutex    sync.Mutex
}

func NewTwitterAccountManager(accounts []*types.TwitterAccount, apiKeys []*types.TwitterApiKey) *TwitterAccountManager {
	return &TwitterAccountManager{
		accounts: accounts,
		apiKeys:  apiKeys,
		index:    0,
	}
}

func (manager *TwitterAccountManager) GetNextAccount() *types.TwitterAccount {
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
		err := DetectKeyType(key)
		if err != nil {
			key.Type = types.TwitterApiKeyTypeUnknown
		}
	}
}

// GetApiKeys returns all api keys managed by this manager
func (manager *TwitterAccountManager) GetApiKeys() []*types.TwitterApiKey {
	return manager.apiKeys
}
func (manager *TwitterAccountManager) GetNextApiKey() *types.TwitterApiKey {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()
	if len(manager.apiKeys) == 0 {
		return nil
	}
	key := manager.apiKeys[manager.index]
	manager.index = (manager.index + 1) % len(manager.apiKeys)
	return key
}

func (manager *TwitterAccountManager) MarkAccountRateLimited(account *types.TwitterAccount) {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()
	account.RateLimitedUntil = time.Now().Add(GetRateLimitDuration())
}

func detectTwitterKeyType(apiKey string) (types.TwitterApiKeyType, error) {
	if strings.Contains(apiKey, ":") {
		return types.TwitterApiKeyTypeCredential, nil
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
		return types.TwitterApiKeyTypeElevated, nil
	case 401, 403:
		return types.TwitterApiKeyTypeBase, nil
	default:
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
}

// DetectKeyType determines the type of a twitter API key and sets it.
func DetectKeyType(k *types.TwitterApiKey) error {
	typeStr, err := detectTwitterKeyType(k.Key)
	if err != nil {
		return err
	}
	k.Type = typeStr
	return nil
}
