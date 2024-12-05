package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
	"github.com/masa-finance/tee-worker/api/types"
)

func readConfig() types.JobConfiguration {
	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "/home/masa"
		os.Setenv("DATA_DIR", dataDir)
	}

	// Read the env file
	if err := godotenv.Load(filepath.Join(dataDir, ".env")); err != nil {
		fmt.Println("Failed reading env file!")
		panic(err)
	}

	webScraperBlacklist := os.Getenv("WEBSCRAPER_BLACKLIST")

	blacklistURLs := strings.Split(webScraperBlacklist, ",")
	for i, u := range blacklistURLs {
		blacklistURLs[i] = strings.TrimSpace(u)
	}

	twitterAccount := os.Getenv("TWITTER_ACCOUNTS")

	twitterAccounts := strings.Split(twitterAccount, ",")
	for i, u := range twitterAccounts {
		twitterAccounts[i] = strings.TrimSpace(u)
	}

	// Read the .env file and set the global configuration for all the jobs
	// The jobs will then unmarshal from this configuration to the specific configuration
	// that is needed for the job
	jc := types.JobConfiguration{}
	jc["webscraper_blacklist"] = blacklistURLs
	jc["twitter_accounts"] = twitterAccounts
	jc["data_dir"] = dataDir

	return jc
}

func listenAddress() string {
	listenAddress := os.Getenv("LISTEN_ADDRESS")
	if listenAddress == "" {
		listenAddress = ":8080"
	}

	return listenAddress
}
