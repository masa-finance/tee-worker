package twitter

import (
	"fmt"
	"github.com/sirupsen/logrus"
)

// AuthConfig holds authentication configuration
type AuthConfig struct {
	// Account-based auth
	Account *TwitterAccount
	BaseDir string
	// SkipLoginVerification when true, skips the IsLoggedIn check after loading cookies
	// This can help avoid rate limiting on Twitter's verify_credentials endpoint
	// Default is false (verification enabled)
	SkipLoginVerification bool
}

func NewScraper(config AuthConfig) *Scraper {

	// Fall back to account-based auth
	if config.Account == nil {
		logrus.Error("No authentication method provided")
		return nil
	}

	scraper := &Scraper{Scraper: newTwitterScraper()}

	// Configure whether to skip login verification
	scraper.SetSkipLoginVerification(config.SkipLoginVerification)

	// Try loading cookies
	if err := LoadCookies(scraper.Scraper, config.Account, config.BaseDir); err == nil {
		logrus.Debugf("Cookies loaded for user %s.", config.Account.Username)
		if scraper.IsLoggedIn() {
			logrus.Debugf("Already logged in as %s.", config.Account.Username)
			return scraper
		}
	}

	RandomSleep()

	if err := scraper.Login(config.Account.Username, config.Account.Password, config.Account.TwoFACode); err != nil {
		logrus.WithError(err).Warnf("Login failed for %s", config.Account.Username)
		return nil
	}

	RandomSleep()

	if err := SaveCookies(scraper.Scraper, config.Account, config.BaseDir); err != nil {
		logrus.WithError(err).Errorf("Failed to save cookies for %s", config.Account.Username)
	}

	logrus.Debugf("Login successful for %s", config.Account.Username)
	return scraper
}

func (scraper *Scraper) Login(username, password string, twoFACode ...string) error {

	var err error
	if len(twoFACode) > 0 {
		err = scraper.Scraper.Login(username, password, twoFACode[0])
	} else {
		err = scraper.Scraper.Login(username, password)
	}
	if err != nil {
		return fmt.Errorf("login failed: %v", err)
	}
	return nil
}
