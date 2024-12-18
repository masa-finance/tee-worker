package twitter

import (
	"sync"
	"time"
)

type TwitterAccount struct {
	Username         string
	Password         string
	TwoFACode        string
	RateLimitedUntil time.Time
	LastScraped      time.Time
	LoginStatus      string
}

type TwitterAccountManager struct {
	accounts []*TwitterAccount
	index    int
	mutex    sync.Mutex
}

func NewTwitterAccountManager(accounts []*TwitterAccount) *TwitterAccountManager {
	return &TwitterAccountManager{
		accounts: accounts,
		index:    0,
	}
}

func (manager *TwitterAccountManager) GetNextAccount() *TwitterAccount {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()
	for i := 0; i < len(manager.accounts); i++ {
		account := manager.accounts[manager.index]
		manager.index = (manager.index + 1) % len(manager.accounts)
		if time.Now().After(account.RateLimitedUntil) {
			return account
		}
	}
	return nil
}

func (manager *TwitterAccountManager) MarkAccountRateLimited(account *TwitterAccount) {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()
	account.RateLimitedUntil = time.Now().Add(GetRateLimitDuration())
}

// AccountState holds the state of a Twitter account
type AccountState struct {
	Username         string
	IsRateLimited    bool
	RateLimitedUntil time.Time
	LastScraped      time.Time
	LoginStatus      string // e.g., "Successful", "Please verify", "Failed - [Reason]"
}

// GetAccountStates returns the state of all Twitter accounts
func (manager *TwitterAccountManager) GetAccountStates() []AccountState {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()

	states := make([]AccountState, len(manager.accounts))
	for i, account := range manager.accounts {
		state := AccountState{
			Username:         account.Username,
			IsRateLimited:    time.Now().Before(account.RateLimitedUntil),
			RateLimitedUntil: account.RateLimitedUntil,
			LastScraped:      account.LastScraped,
			LoginStatus:      account.LoginStatus,
		}
		if state.LastScraped.IsZero() {
			state.LoginStatus = "Not initialized"
		}
		states[i] = state
	}
	return states
}

func (manager *TwitterAccountManager) GetAccountByUsername(username string) *TwitterAccount {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()
	for _, account := range manager.accounts {
		if account.Username == username {
			return account
		}
	}
	return nil
}

//TODO: this seems inappropriate. Are we sure we want this kind of feature here?
//func AttemptLoginForUsername(username string) error {
//	account := accountManager.GetAccountByUsername(username)
//	if account == nil {
//		return fmt.Errorf("account with username %s not found", username)
//	}
//
//	scraper := NewScraper(account, "path to config here?")
//	if scraper == nil {
//		err := fmt.Errorf("twitter authentication failed for %s", account.Username)
//		return err
//	}
//	logrus.Infof("Login successful for %s", username)
//	return nil
//}
