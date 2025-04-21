package jobserver

import (
	"os"
	"strconv"
	"sync"
	"time"
	"container/list"
	"github.com/masa-finance/tee-worker/api/types"
)

// Default values
const (
	defaultMaxSize      = 1000
	defaultMaxAgeSecs   = 600
)

type cacheEntry struct {
	key       string
	result    types.JobResult
	timestamp time.Time
	element   *list.Element // pointer to the element in the list
}

type ResultCache struct {
	lock     sync.Mutex
	entries  map[string]*cacheEntry
	order    *list.List // oldest at Front, newest at Back
	maxSize  int
	maxAge   time.Duration
}

func NewResultCacheFromEnv() *ResultCache {
	maxSize := defaultMaxSize
	maxAge := defaultMaxAgeSecs

	if s := os.Getenv("RESULT_CACHE_MAX_SIZE"); s != "" {
		if v, err := strconv.Atoi(s); err == nil && v > 0 {
			maxSize = v
		}
	}
	if s := os.Getenv("RESULT_CACHE_MAX_AGE_SECONDS"); s != "" {
		if v, err := strconv.Atoi(s); err == nil && v > 0 {
			maxAge = v
		}
	}

	rc := &ResultCache{
		entries: make(map[string]*cacheEntry),
		order:   list.New(),
		maxSize: maxSize,
		maxAge:  time.Duration(maxAge) * time.Second,
	}
	go rc.periodicCleanup()
	return rc
}

func (rc *ResultCache) Set(key string, result types.JobResult) {
	rc.lock.Lock()
	defer rc.lock.Unlock()
	if entry, exists := rc.entries[key]; exists {
		// Update and move to back
		entry.result = result
		entry.timestamp = time.Now()
		rc.order.MoveToBack(entry.element)
		return
	}
	// New entry
	entry := &cacheEntry{
		key:       key,
		result:    result,
		timestamp: time.Now(),
	}
	entry.element = rc.order.PushBack(entry)
	rc.entries[key] = entry
	// Evict if over size
	for len(rc.entries) > rc.maxSize {
		oldest := rc.order.Front()
		if oldest != nil {
			oldestEntry := oldest.Value.(*cacheEntry)
			delete(rc.entries, oldestEntry.key)
			rc.order.Remove(oldest)
		}
	}
}

func (rc *ResultCache) Get(key string) (types.JobResult, bool) {
	rc.lock.Lock()
	defer rc.lock.Unlock()
	entry, exists := rc.entries[key]
	if !exists {
		return types.JobResult{}, false
	}
	// If expired, remove
	if rc.maxAge > 0 && time.Since(entry.timestamp) > rc.maxAge {
		rc.order.Remove(entry.element)
		delete(rc.entries, key)
		return types.JobResult{}, false
	}
	return entry.result, true
}

func (rc *ResultCache) periodicCleanup() {
	ticker := time.NewTicker(rc.maxAge / 2)
	defer ticker.Stop()
	for range ticker.C {
		rc.cleanupExpired()
	}
}

func (rc *ResultCache) cleanupExpired() {
	rc.lock.Lock()
	defer rc.lock.Unlock()
	now := time.Now()
	for e := rc.order.Front(); e != nil; {
		next := e.Next()
		entry := e.Value.(*cacheEntry)
		if rc.maxAge > 0 && now.Sub(entry.timestamp) > rc.maxAge {
			delete(rc.entries, entry.key)
			rc.order.Remove(e)
		}
		e = next
	}
}
