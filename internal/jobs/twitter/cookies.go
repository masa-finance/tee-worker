package twitter

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	twitterscraper "github.com/imperatrona/twitter-scraper"
	"github.com/masa-finance/tee-worker/api/types"
	"github.com/sirupsen/logrus"
)

func SaveCookies(scraper *twitterscraper.Scraper, account *types.TwitterAccount, baseDir string) error {
	logrus.Debugf("Saving cookies for user %s", account.Username)
	cookieFile := filepath.Join(baseDir, fmt.Sprintf("%s_twitter_cookies.json", account.Username))
	cookies := scraper.GetCookies()
	logrus.Debugf("Got %d cookies to save", len(cookies))

	data, err := json.Marshal(cookies)
	if err != nil {
		return fmt.Errorf("error marshaling cookies: %v", err)
	}

	logrus.Debugf("Writing cookies to file: %s", cookieFile)
	if err = os.WriteFile(cookieFile, data, 0644); err != nil {
		return fmt.Errorf("error saving cookies: %v", err)
	}
	logrus.Debug("Successfully saved cookies")
	return nil
}

func LoadCookies(scraper *twitterscraper.Scraper, account *types.TwitterAccount, baseDir string) error {
	cookieFile := filepath.Join(baseDir, fmt.Sprintf("%s_twitter_cookies.json", account.Username))
	logrus.Debugf("Loading cookies from file: %s", cookieFile)
	data, err := os.ReadFile(cookieFile)
	if err != nil {
		return fmt.Errorf("error reading cookies file: %v", err)
	}

	var cookies []*http.Cookie
	if err = json.Unmarshal(data, &cookies); err != nil {
		return fmt.Errorf("error unmarshaling cookies: %v", err)
	}
	logrus.Debugf("Loaded %d cookies", len(cookies))
	scraper.SetCookies(cookies)
	return nil
}
