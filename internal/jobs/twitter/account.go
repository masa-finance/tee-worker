package twitter

import (
	"sync"
	"time"
)

type TwitterAccount struct {
	Username         string
	Password         string
	TwoFACode        string
	RateLimitedUntil time.Time
}

type TwitterApiKey struct {
	Key  string
	Type string // "base", "elevated", or "credential"
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

// DetectAllApiKeyTypes checks and sets the Type for all apiKeys in the manager.
func (manager *TwitterAccountManager) DetectAllApiKeyTypes() {
	for _, key := range manager.apiKeys {
		err := key.SetKeyType()
		if err != nil {
			key.Type = "unknown"
		}
	}
}

// GetApiKeys returns all api keys managed by this manager
func (manager *TwitterAccountManager) GetApiKeys() []*TwitterApiKey {
	return manager.apiKeys
}

func (manager *TwitterAccountManager) MarkAccountRateLimited(account *TwitterAccount) {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()
	account.RateLimitedUntil = time.Now().Add(GetRateLimitDuration())
}
