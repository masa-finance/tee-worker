package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/masa-finance/tee-worker/api/types"
	"github.com/sirupsen/logrus"
)

// TODO: Revamp the whole config, using a Map and having multiple global functions to get the config is not nice
var dataDir = os.Getenv("DATA_DIR")

func readConfig() types.JobConfiguration {
	// The jobs will then unmarshal from this configuration to the specific configuration
	// that is needed for the job
	jc := types.JobConfiguration{}

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

	// Apify API key loading
	apifyApiKey := os.Getenv("APIFY_API_KEY")
	if apifyApiKey != "" {
		logrus.Info("Apify API key found")
		jc["apify_api_key"] = apifyApiKey
	} else {
		jc["apify_api_key"] = ""
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

	jc["stats_buf_size"] = statsBufSize()

	logLevel := os.Getenv("LOG_LEVEL")
	switch strings.ToLower(logLevel) {
	case "debug":
		logrus.SetLevel(logrus.DebugLevel)
	case "info":
		logrus.SetLevel(logrus.InfoLevel)
	case "warn":
		logrus.SetLevel(logrus.WarnLevel)
	case "error":
		logrus.SetLevel(logrus.ErrorLevel)
	default:
		logrus.SetLevel(logrus.InfoLevel)
	}

	jc["profiling_enabled"] = os.Getenv("ENABLE_PPROF") == "true"

	return jc
}

// statsBufSize returns the size of the stats channel buffer
func statsBufSize() uint {
	bufSizeStr := os.Getenv("STATS_BUF_SIZE")
	if bufSizeStr == "" {
		bufSizeStr = "128"
	}

	bufSize, err := strconv.Atoi(bufSizeStr)
	if err != nil {
		logrus.Errorf("Error parsing STATS_BUF_SIZE: %s. Setting to default.", err)
		bufSize = 128
	}
	return uint(bufSize)
}

func listenAddress() string {
	listenAddress := os.Getenv("LISTEN_ADDRESS")
	if listenAddress == "" {
		listenAddress = ":8080"
	}

	return listenAddress
}

func standaloneMode() bool {
	return os.Getenv("STANDALONE") == "true"
}
