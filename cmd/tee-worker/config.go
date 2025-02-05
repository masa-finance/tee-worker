package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
	"github.com/masa-finance/tee-worker/api/types"
	"github.com/sirupsen/logrus"
)

func readConfig() types.JobConfiguration {
	// The jobs will then unmarshal from this configuration to the specific configuration
	// that is needed for the job
	jc := types.JobConfiguration{}

	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "/home/masa"
		os.Setenv("DATA_DIR", dataDir)
	}

	jc["data_dir"] = dataDir

	// Read the env file
	if err := godotenv.Load(filepath.Join(dataDir, ".env")); err != nil {
		fmt.Println("Failed reading env file!")
		panic(err)
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
	}

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
