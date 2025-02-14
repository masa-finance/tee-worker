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

	// API-based auth
	APIKey string
}

func NewScraper(config AuthConfig) *Scraper {
	// Try API auth first if key is provided
	if config.APIKey != "" {
		logrus.Debug("Using API key authentication")
		return &Scraper{
			apiClient: NewClient(config.APIKey),
		}
	}

	// Fall back to account-based auth
	if config.Account == nil {
		logrus.Error("No authentication method provided")
		return nil
	}

	scraper := &Scraper{Scraper: newTwitterScraper()}

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
	// Skip login if using API auth
	if scraper.apiClient != nil {
		return nil
	}

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

func (scraper *Scraper) Logout() error {
	// No logout needed for API auth
	if scraper.apiClient != nil {
		return nil
	}

	if err := scraper.Scraper.Logout(); err != nil {
		return fmt.Errorf("logout failed: %v", err)
	}
	return nil
}

func (scraper *Scraper) IsLoggedIn() bool {
	if scraper.apiClient != nil {
		return scraper.apiClient.apiKey != ""
	}
	return scraper.Scraper != nil && scraper.Scraper.IsLoggedIn()
}
