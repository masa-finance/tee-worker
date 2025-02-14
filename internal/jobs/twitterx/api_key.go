package twitterx

import (
	"sync"
)

// Client represents a Twitter API client
type TwitterXApiKey struct {
	Key string
}

type TwitterXApiKeyManager struct {
	apiKeys []*TwitterXApiKey
	index   int
	mutex   sync.Mutex
}

func NewTwitterApiKeyManager(twitterApiKeys []*TwitterXApiKey) *TwitterXApiKeyManager {
	return &TwitterXApiKeyManager{
		apiKeys: twitterApiKeys,
	}
}

func (manager *TwitterXApiKeyManager) GetNextApiKey() *TwitterXApiKey {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()
	if len(manager.apiKeys) == 0 {
		return nil
	}
	key := manager.apiKeys[manager.index]
	manager.index = (manager.index + 1) % len(manager.apiKeys)
	return key
}

func (manager *TwitterXApiKeyManager) AddApiKey(apiKey *TwitterXApiKey) {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()
	manager.apiKeys = append(manager.apiKeys, apiKey)
}
