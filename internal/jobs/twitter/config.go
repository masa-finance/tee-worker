package twitter

import (
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	minSleepDuration  = 500 * time.Millisecond
	maxSleepDuration  = 2 * time.Second
	RateLimitDuration = 15 * time.Minute
)

var (
	rng *rand.Rand
)

type ApiConfig struct {
	APIKey   string
	Accounts []string
}

func init() {
	rng = rand.New(rand.NewSource(time.Now().UnixNano()))
}

func RandomSleep() {
	duration := minSleepDuration + time.Duration(rng.Int63n(int64(maxSleepDuration-minSleepDuration)))
	logrus.Debugf("Sleeping for %v", duration)
	time.Sleep(duration)
}

func GetRateLimitDuration() time.Duration {
	return RateLimitDuration
}

func LoadConfig() *ApiConfig {
	config := &ApiConfig{}

	// Load API key if present
	if apiKey := os.Getenv("TWITTER_API_KEY"); apiKey != "" {
		config.APIKey = apiKey
	}

	// Load accounts if present
	if accounts := os.Getenv("TWITTER_ACCOUNTS"); accounts != "" {
		config.Accounts = strings.Split(accounts, ",")
	}

	return config
}

// UseAPIKey returns true if we should use the API key for scraping
func (c *ApiConfig) UseAPIKey() bool {
	return c.APIKey != ""
}
