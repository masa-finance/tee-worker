package twitter

import (
	"sync"
)

// Client represents a Twitter API client
type TwitterApiKey struct {
	Key string
}

type TwitterApiKeyManager struct {
	apiKeys []*TwitterApiKey
	index   int
	mutex   sync.Mutex
}

func NewTwitterApiKeyManager(twitterApiKeys []*TwitterApiKey) *TwitterApiKeyManager {
	return &TwitterApiKeyManager{
		apiKeys: twitterApiKeys,
	}
}

func (manager *TwitterApiKeyManager) GetNextApiKey() *TwitterApiKey {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()
	if len(manager.apiKeys) == 0 {
		return nil
	}
	key := manager.apiKeys[manager.index]
	manager.index = (manager.index + 1) % len(manager.apiKeys)
	return key
}

func (manager *TwitterApiKeyManager) AddApiKey(apiKey *TwitterApiKey) {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()
	manager.apiKeys = append(manager.apiKeys, apiKey)
}
