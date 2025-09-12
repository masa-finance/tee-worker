package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
)

var (
	// MinersWhiteList is set by the build process
	// it contains a comma separated list of miners that are allowed to send jobs to the worker
	// If empty, anyone is allowed to send jobs to the worker
	MinersWhiteList = ""
)

const defaultDataDir = "/home/masa"
const defaultListenAddress = ":8080"

// TODO: Revamp this whole thing, a map[string]any is not really maintainable
type JobConfiguration map[string]any

func ReadConfig() JobConfiguration {
	// The jobs will then unmarshal from this configuration to the specific configuration
	// that is needed for the job
	jc := JobConfiguration{}

	logLevel := os.Getenv("LOG_LEVEL")
	level := ParseLogLevel(logLevel)
	jc["log_level"] = level.String()
	SetLogLevel(level)

	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "/home/masa"
		err := os.Setenv("DATA_DIR", dataDir)
		if err != nil {
			logrus.Fatalf("Failed to set DATA_DIR: %v", err)
		}
	}
	jc["data_dir"] = dataDir

	// Read the env file
	if err := godotenv.Load(filepath.Join(dataDir, ".env")); err != nil {
		if os.Getenv("OE_SIMULATION") == "" {
			fmt.Println("Failed reading env file!")
			panic(err)
		}
		fmt.Println("Failed reading env file. Running in simulation mode, reading from environment variables")
	}

	bufSizeStr := os.Getenv("STATS_BUF_SIZE")
	if bufSizeStr == "" {
		bufSizeStr = "128"
	}
	bufSize, err := strconv.Atoi(bufSizeStr)
	if err != nil {
		logrus.Errorf("Error parsing STATS_BUF_SIZE: %s. Setting to default.", err)
		bufSize = 128
	}
	jc["stats_buf_size"] = uint(bufSize)

	maxJobsStr := os.Getenv("STATS_BUF_SIZE")
	if maxJobsStr == "" {
		maxJobsStr = "10"
	}
	maxJobs, err := strconv.Atoi(maxJobsStr)
	if err != nil {
		logrus.Errorf("Error parsing MAX_JOBS %s. Setting to default.", err)
		bufSize = 10
	}
	jc["max_jobs"] = uint(maxJobs)

	listenAddress := os.Getenv("LISTEN_ADDRESS")
	if listenAddress == "" {
		listenAddress = defaultListenAddress
	}
	jc["listen_address"] = listenAddress

	jc["standalone_mode"] = os.Getenv("STANDALONE") == "true"

	// Result cache config
	resultCacheMaxSize := 1000
	if s := os.Getenv("RESULT_CACHE_MAX_SIZE"); s != "" {
		if v, err := strconv.Atoi(s); err == nil && v > 0 {
			resultCacheMaxSize = v
		}
	}
	jc["result_cache_max_size"] = resultCacheMaxSize

	resultCacheMaxAge := 600
	if s := os.Getenv("RESULT_CACHE_MAX_AGE_SECONDS"); s != "" {
		if v, err := strconv.Atoi(s); err == nil && v > 0 {
			resultCacheMaxAge = v
		}
	}
	jc["result_cache_max_age_seconds"] = time.Duration(resultCacheMaxAge) * time.Second

	jobTimeout := 300
	if s := os.Getenv("JOB_TIMEOUT_SECONDS"); s != "" {
		if v, err := strconv.Atoi(s); err == nil && v > 0 {
			jobTimeout = v
		}
	}
	jc["job_timeout_seconds"] = time.Duration(jobTimeout) * time.Second

	// API Key for authentication
	apiKey := os.Getenv("API_KEY")
	if apiKey != "" {
		jc["api_key"] = apiKey
	}

	webScraperBlacklist := os.Getenv("WEBSCRAPER_BLACKLIST")
	if webScraperBlacklist != "" {
		blacklistURLs := strings.Split(webScraperBlacklist, ",")
		for i, u := range blacklistURLs {
			blacklistURLs[i] = strings.TrimSpace(u)
		}
		jc["webscraper_blacklist"] = blacklistURLs
	}

	twitterAccount := os.Getenv("TWITTER_ACCOUNTS")
	if twitterAccount != "" {
		twitterAccounts := strings.Split(twitterAccount, ",")
		for i, u := range twitterAccounts {
			twitterAccounts[i] = strings.TrimSpace(u)
		}
		jc["twitter_accounts"] = twitterAccounts
	} else {
		jc["twitter_accounts"] = []string{}
	}

	twitterApiKeys := os.Getenv("TWITTER_API_KEYS")
	if twitterApiKeys != "" {
		logrus.Info("Twitter API keys found")
		apiKeys := strings.Split(twitterApiKeys, ",")
		for i, u := range apiKeys {
			apiKeys[i] = strings.TrimSpace(u)
		}
		jc["twitter_api_keys"] = apiKeys
	} else {
		jc["twitter_api_keys"] = []string{}
	}

	jc["twitter_skip_login_verification"] = os.Getenv("TWITTER_SKIP_LOGIN_VERIFICATION") == "true"

	// Apify API key loading
	apifyApiKey := os.Getenv("APIFY_API_KEY")
	if apifyApiKey != "" {
		logrus.Info("Apify API key found")
		jc["apify_api_key"] = apifyApiKey
	} else {
		jc["apify_api_key"] = ""
	}

	geminiApiKey := os.Getenv("GEMINI_API_KEY")
	if geminiApiKey != "" {
		logrus.Info("Gemini API key found")
		jc["gemini_api_key"] = geminiApiKey
	} else {
		jc["gemini_api_key"] = ""
	}

	tikTokLang := os.Getenv("TIKTOK_DEFAULT_LANGUAGE")
	if tikTokLang == "" {
		tikTokLang = "eng-US"
		logrus.Info("TIKTOK_DEFAULT_LANGUAGE not set, using default: ", tikTokLang)
	}
	jc["tiktok_default_language"] = tikTokLang

	// TikTok API Origin and Referer now use hardcoded defaults in NewTikTokTranscriber

	if userAgent := os.Getenv("TIKTOK_API_USER_AGENT"); userAgent != "" {
		jc["tiktok_api_user_agent"] = userAgent
	} // Default for userAgent is set in NewTikTokTranscriber

	jc["profiling_enabled"] = os.Getenv("ENABLE_PPROF") == "true"

	return jc
}

// Unmarshal unmarshals the job configuration into the supplied interface.
func (jc JobConfiguration) Unmarshal(v any) error {
	data, err := json.Marshal(jc)
	if err != nil {
		return fmt.Errorf("error marshalling job configuration: %w", err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("error unmarshalling job configuration: %w", err)
	}

	return nil
}

func (jc JobConfiguration) DataDir() string {
	return jc.GetString("data_dir", defaultDataDir)
}

func (jc JobConfiguration) ListenAddress() string {
	return jc.GetString("listen_address", defaultListenAddress)
}

func (jc JobConfiguration) IsStandaloneMode() bool {
	return jc.GetBool("standalone_mode", false)
}

// GetInt safely extracts an int from JobConfiguration, with a default fallback
func (jc JobConfiguration) GetInt(key string, def int) (int, error) {
	if v, ok := jc[key]; ok {
		switch val := v.(type) {
		case int:
			return val, nil
		case int64:
			return int(val), nil
		case float64:
			return int(val), nil
		case float32:
			return int(val), nil
		default:
			return def, fmt.Errorf("value %v for key %q cannot be converted to int", val, key)
		}
	}
	return def, nil
}

func (jc JobConfiguration) GetDuration(key string, defSecs int) time.Duration {
	// Go does not allow generics in methods :-(
	if v, ok := jc[key]; ok {
		if val, ok := v.(time.Duration); ok {
			return val
		}
	}
	return time.Duration(defSecs) * time.Second
}

func (jc JobConfiguration) GetString(key string, def string) string {
	if v, ok := jc[key]; ok {
		if val, ok := v.(string); ok {
			return val
		}
	}
	return def
}

// GetStringSlice safely extracts a string slice from JobConfiguration, with a default fallback
func (jc JobConfiguration) GetStringSlice(key string, def []string) []string {
	if v, ok := jc[key]; ok {
		if val, ok := v.([]string); ok {
			return val
		}
	}
	return def
}

// GetBool safely extracts a bool from JobConfiguration, with a default fallback
func (jc JobConfiguration) GetBool(key string, def bool) bool {
	if v, ok := jc[key]; ok {
		if val, ok := v.(bool); ok {
			return val
		}
	}
	return def
}

// TwitterScraperConfig represents the configuration needed for Twitter scraping
// This is defined here to avoid circular imports between api/types and internal/jobs
type TwitterScraperConfig struct {
	Accounts              []string
	ApiKeys               []string
	ApifyApiKey           string
	DataDir               string
	SkipLoginVerification bool
}

// GetTwitterConfig constructs a TwitterScraperConfig directly from the JobConfiguration
// This eliminates the need for JSON marshaling/unmarshaling
func (jc JobConfiguration) GetTwitterConfig() TwitterScraperConfig {
	return TwitterScraperConfig{
		Accounts:              jc.GetStringSlice("twitter_accounts", []string{}),
		ApiKeys:               jc.GetStringSlice("twitter_api_keys", []string{}),
		ApifyApiKey:           jc.GetString("apify_api_key", ""),
		DataDir:               jc.GetString("data_dir", ""),
		SkipLoginVerification: jc.GetBool("skip_login_verification", false),
	}
}

// RedditConfig represents the configuration needed for Reddit scraping via Apify
type RedditConfig struct {
	ApifyApiKey string
}

// GetRedditConfig constructs a RedditConfig directly from the JobConfiguration
// This eliminates the need for JSON marshaling/unmarshaling
func (jc JobConfiguration) GetRedditConfig() RedditConfig {
	return RedditConfig{
		ApifyApiKey: jc.GetString("apify_api_key", ""),
	}
}

// LlmApiKey represents an LLM API key with validation capabilities
type LlmApiKey string

// IsValid checks if the LLM API key is valid
func (k LlmApiKey) IsValid() bool {
	if k == "" {
		return false
	}
	
	// TODO: Add actual Gemini API key validation with a handler
	// For now, just check if it's not empty
	return true
}

type LlmConfig struct {
	GeminiApiKey LlmApiKey
}

// WebConfig represents the configuration needed for Web scraping via Apify
type WebConfig struct {
	LlmConfig
	ApifyApiKey string
}

// GetWebConfig constructs a WebConfig directly from the JobConfiguration
// This eliminates the need for JSON marshaling/unmarshaling
func (jc JobConfiguration) GetWebConfig() WebConfig {
	return WebConfig{
		LlmConfig: LlmConfig{
			GeminiApiKey: LlmApiKey(jc.GetString("gemini_api_key", "")),
		},
		ApifyApiKey: jc.GetString("apify_api_key", ""),
	}
}

// ParseLogLevel parses a string and returns the corresponding logrus.Level.
func ParseLogLevel(logLevel string) logrus.Level {
	switch strings.ToLower(logLevel) {
	case "debug":
		return logrus.DebugLevel
	case "info":
		return logrus.InfoLevel
	case "warn":
		return logrus.WarnLevel
	case "error":
		return logrus.ErrorLevel
	default:
		logrus.Error("Invalid log level", "level", logLevel, "setting_to", logrus.InfoLevel.String())
		return logrus.InfoLevel
	}
}

// SetLogLevel sets the log level for the application.
func SetLogLevel(level logrus.Level) {
	logrus.SetLevel(level)
}
