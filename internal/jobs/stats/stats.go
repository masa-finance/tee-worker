package stats

import (
	"encoding/json"
	"sync"
	"time"

	teetypes "github.com/masa-finance/tee-types/types"
	"github.com/masa-finance/tee-worker/api/types"
	"github.com/masa-finance/tee-worker/internal/capabilities"
	"github.com/masa-finance/tee-worker/internal/versioning"
	"github.com/sirupsen/logrus"
)

// These are the types of statistics that we can add. The value is the JSON key that will be used for serialization.
type StatType string

const (
	// FIXME: Add Reddit stats!!!
	TwitterScrapes             StatType = "twitter_scrapes"
	TwitterTweets              StatType = "twitter_returned_tweets"
	TwitterProfiles            StatType = "twitter_returned_profiles"
	TwitterFollowers           StatType = "twitter_returned_followers"
	TwitterOther               StatType = "twitter_returned_other"
	TwitterErrors              StatType = "twitter_errors"
	TwitterAuthErrors          StatType = "twitter_auth_errors"
	TwitterRateErrors          StatType = "twitter_ratelimit_errors"
	TwitterXSearchQueries      StatType = "twitterx_search"
	WebSuccess                 StatType = "web_success"
	WebErrors                  StatType = "web_errors"
	WebInvalid                 StatType = "web_invalid"
	TikTokTranscriptionSuccess StatType = "tiktok_transcription_success"
	TikTokTranscriptionErrors  StatType = "tiktok_transcription_errors"
	RedditReturnedItems        StatType = "reddit_returned_items"
	RedditQueries              StatType = "reddit_queries"
	RedditErrors               StatType = "reddit_errors"
	// TODO: Should we add stats for calls to each of the Twitter capabilities to decouple business / scoring logic?
)

// AddStat is the struct used in the rest of the tee-worker for sending statistics
type AddStat struct {
	Type     StatType
	WorkerID string
	Num      uint
}

// Stats is the structure we use to store the statistics
type Stats struct {
	BootTimeUnix         int64                        `json:"boot_time"`
	LastOperationUnix    int64                        `json:"last_operation_time"`
	CurrentTimeUnix      int64                        `json:"current_time"`
	WorkerID             string                       `json:"worker_id"`
	Stats                map[string]map[StatType]uint `json:"stats"`
	ReportedCapabilities teetypes.WorkerCapabilities  `json:"reported_capabilities"`
	WorkerVersion        string                       `json:"worker_version"`
	ApplicationVersion   string                       `json:"application_version"`
	sync.Mutex
}

// StatsCollector is the object used to collect statistics
type StatsCollector struct {
	Stats            *Stats
	Chan             chan AddStat
	jobServer        capabilities.JobServerInterface
	jobConfiguration types.JobConfiguration
}

// StartCollector starts a goroutine that listens to a channel for AddStat messages and updates the stats accordingly.
func StartCollector(bufSize uint, jc types.JobConfiguration) *StatsCollector {
	logrus.Info("Starting stats collector")

	s := Stats{
		BootTimeUnix:       time.Now().Unix(),
		Stats:              make(map[string]map[StatType]uint),
		WorkerVersion:      versioning.TEEWorkerVersion,
		ApplicationVersion: versioning.ApplicationVersion,
	}

	// Initial capability detection without JobServer (basic capabilities only)
	// Full capability detection will happen when JobServer is set
	s.ReportedCapabilities = capabilities.DetectCapabilities(jc, nil)

	logrus.Infof("Initial structured capabilities: %+v", s.ReportedCapabilities)

	ch := make(chan AddStat, bufSize)

	go func(s *Stats, ch chan AddStat) {
		for {
			stat := <-ch
			s.Lock()
			s.LastOperationUnix = time.Now().Unix()
			if _, ok := s.Stats[stat.WorkerID]; !ok {
				s.Stats[stat.WorkerID] = make(map[StatType]uint)
			}
			if _, ok := s.Stats[stat.WorkerID][stat.Type]; ok {
				s.Stats[stat.WorkerID][stat.Type] += stat.Num
			} else {
				s.Stats[stat.WorkerID][stat.Type] = stat.Num
			}
			s.Unlock()
			logrus.Debugf("Added %d to stat %s. Current stats: %#v", stat.Num, stat.Type, s)
		}
	}(&s, ch)

	return &StatsCollector{Stats: &s, Chan: ch, jobConfiguration: jc}
}

// Json returns the current statistics as a JSON byte array
func (s *StatsCollector) Json() ([]byte, error) {
	s.Stats.Lock()
	defer s.Stats.Unlock()
	s.Stats.CurrentTimeUnix = time.Now().Unix()
	return json.Marshal(s.Stats)
}

// Add is a convenience method to add a number to a statistic
func (s *StatsCollector) Add(workerID string, typ StatType, num uint) {
	s.Chan <- AddStat{WorkerID: workerID, Type: typ, Num: num}
}

// SetWorkerID sets the worker ID for the stats collector
func (s *StatsCollector) SetWorkerID(workerID string) {
	s.Stats.Lock()
	defer s.Stats.Unlock()
	s.Stats.WorkerID = workerID
}

// SetJobServer sets the JobServer reference and updates capabilities with full detection
func (s *StatsCollector) SetJobServer(js capabilities.JobServerInterface) {
	s.jobServer = js

	// Now that we have the JobServer, update capabilities with full detection
	s.Stats.Lock()
	defer s.Stats.Unlock()

	// Auto-detect capabilities using the JobServer
	s.Stats.ReportedCapabilities = capabilities.DetectCapabilities(s.jobConfiguration, js)

	logrus.Infof("Updated structured capabilities with JobServer: %+v", s.Stats.ReportedCapabilities)
}
