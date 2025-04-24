package stats

import (
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/masa-finance/tee-worker/api/types"
	"github.com/masa-finance/tee-worker/internal/jobs/twitter"
	"github.com/sirupsen/logrus"
)

// These are the types of statistics that we can add. The value is the JSON key that will be used for serialization.
type statType string

const (
	TwitterScrapes        statType = "twitter_scrapes"
	TwitterTweets         statType = "twitter_returned_tweets"
	TwitterProfiles       statType = "twitter_returned_profiles"
	TwitterOther          statType = "twitter_returned_other"
	TwitterErrors         statType = "twitter_errors"
	TwitterAuthErrors     statType = "twitter_auth_errors"
	TwitterRateErrors     statType = "twitter_ratelimit_errors"
	TwitterXSearchQueries statType = "twitterx_search"
	WebSuccess            statType = "web_success"
	WebErrors             statType = "web_errors"
	WebInvalid            statType = "web_invalid"
	// TODO: Should we add stats for calls to each of the Twitter job types?
)

// allStats is a list of all the stats that we support.
// Make sure to keep this in sync with the above!
var allStats []statType = []statType{
	TwitterScrapes,
	TwitterTweets,
	TwitterProfiles,
	TwitterOther,
	TwitterErrors,
	TwitterAuthErrors,
	TwitterRateErrors,
	WebSuccess,
	WebErrors,
}

// AddStat is the struct used in the rest of the tee-worker for sending statistics
type AddStat struct {
	Type statType
	Num  uint
}

// stats is the structure we use to store the statistics
type stats struct {
	BootTimeUnix         int64             `json:"boot_time"`
	LastOperationUnix    int64             `json:"last_operation_time"`
	CurrentTimeUnix      int64             `json:"current_time"`
	WorkerID             string            `json:"worker_id"`
	Stats                map[statType]uint `json:"stats"`
	ReportedCapabilities []string          `json:"reported_capabilities"`
	sync.Mutex
}

// StatsCollector is the object used to collect statistics
type StatsCollector struct {
	Stats *stats
	jc    types.JobConfiguration
	Chan  chan AddStat
}

// StartCollector starts a goroutine that listens to a channel for AddStat messages and updates the stats accordingly.
func StartCollector(bufSize uint, jc types.JobConfiguration) *StatsCollector {
	logrus.Info("Starting stats collector")

	s := stats{
		BootTimeUnix: time.Now().Unix(),
		Stats:        make(map[statType]uint),
	}
	for _, t := range allStats {
		s.Stats[t] = 0
	}

	logrus.Info("Starting stats collector")

	s = stats{
		BootTimeUnix: time.Now().Unix(),
		Stats:        make(map[statType]uint),
	}
	for _, t := range allStats {
		s.Stats[t] = 0
	}

	// Collect capabilities from config
	capabilities, isString := jc["capabilities"].(string)
	if isString {
		if strings.Contains(capabilities, ",") {
			s.ReportedCapabilities = strings.Split(capabilities, ",")
		} else {
			s.ReportedCapabilities = []string{capabilities}
		}
		logrus.Infof("Capabilities: %v", s.ReportedCapabilities)
	}

	// --- Twitter Key Type Capabilities ---
	if tamIface, ok := jc["twitter_account_manager"]; ok {
		if tam, ok := tamIface.(*twitter.TwitterAccountManager); ok {
			var hasBase, hasElevated, hasCredential bool
			for _, key := range tam.GetApiKeys() {
				switch key.Type {
				case "base":
					hasBase = true
				case "elevated":
					hasElevated = true
				case "credential":
					hasCredential = true
				}
			}
			if hasBase {
				s.ReportedCapabilities = append(s.ReportedCapabilities, "twitter_base")
			}
			if hasElevated {
				s.ReportedCapabilities = append(s.ReportedCapabilities, "twitter_elevated")
			}
			if hasCredential {
				s.ReportedCapabilities = append(s.ReportedCapabilities, "twitter_credential")
			}
		}
	}

	// --- Twitter Key Type Capabilities ---
	// Try to detect Twitter key types if TwitterAccountManager is available
	if tamIface, ok := jc["twitter_account_manager"]; ok {
		if tam, ok := tamIface.(interface {
			GetApiKeys() []*twitter.TwitterApiKey
		}); ok {
			var hasBase, hasElevated, hasCredential bool
			for _, key := range tam.GetApiKeys() {
				switch key.Type {
				case "base":
					hasBase = true
				case "elevated":
					hasElevated = true
				case "credential":
					hasCredential = true
				}
			}
			if hasBase {
				s.ReportedCapabilities = append(s.ReportedCapabilities, "twitter_base")
			}
			if hasElevated {
				s.ReportedCapabilities = append(s.ReportedCapabilities, "twitter_elevated")
			}
			if hasCredential {
				s.ReportedCapabilities = append(s.ReportedCapabilities, "twitter_credential")
			}
		}
	}

	logrus.Info("Starting stats collector")

	s = stats{
		BootTimeUnix: time.Now().Unix(),
		Stats:        make(map[statType]uint),
	}
	for _, t := range allStats {
		s.Stats[t] = 0
	}

	capabilities, isString = jc["capabilities"].(string)
	if isString {
		if strings.Contains(capabilities, ",") {
			s.ReportedCapabilities = strings.Split(capabilities, ",")
		} else {
			s.ReportedCapabilities = []string{capabilities}
		}
		logrus.Infof("Capabilities: %v", s.ReportedCapabilities)
	}

	ch := make(chan AddStat, bufSize)

	go func(s *stats, ch chan AddStat) {
		for {
			stat := <-ch
			s.Lock()
			s.LastOperationUnix = time.Now().Unix()
			if _, ok := s.Stats[stat.Type]; ok {
				s.Stats[stat.Type] += stat.Num
			} else {
				s.Stats[stat.Type] = stat.Num
			}
			s.Unlock()
			logrus.Debugf("Added %d to stat %s. Current stats: %#v", stat.Num, stat.Type, s)
		}
	}(&s, ch)

	return &StatsCollector{Stats: &s, Chan: ch}
}

// Json returns the current statistics as a JSON byte array
func (s *StatsCollector) Json() ([]byte, error) {
	s.Stats.Lock()
	defer s.Stats.Unlock()
	s.Stats.CurrentTimeUnix = time.Now().Unix()
	return json.Marshal(s.Stats)
}

// Add is a convenience method to add a number to a statistic
func (s *StatsCollector) Add(typ statType, num uint) {
	s.Chan <- AddStat{Type: typ, Num: num}
}

// SetWorkerID sets the worker ID for the stats collector
func (s *StatsCollector) SetWorkerID(workerID string) {
	s.Stats.Lock()
	defer s.Stats.Unlock()
	s.Stats.WorkerID = workerID
}
