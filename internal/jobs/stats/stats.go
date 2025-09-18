package stats

import (
	"encoding/json"
	"sync"
	"time"

	teetypes "github.com/masa-finance/tee-types/types"
	"github.com/masa-finance/tee-worker/internal/config"
	"github.com/masa-finance/tee-worker/internal/versioning"
	"github.com/sirupsen/logrus"
)

// WorkerCapabilitiesProvider abstracts capability retrieval to avoid import cycles
type WorkerCapabilitiesProvider interface {
	GetWorkerCapabilities() teetypes.WorkerCapabilities
}

// These are the types of statistics that we can add. The value is the JSON key that will be used for serialization.
type StatType string

const (
	TwitterScrapes             StatType = "twitter_scrapes"
	TwitterTweets              StatType = "twitter_returned_tweets"
	TwitterProfiles            StatType = "twitter_returned_profiles"
	TwitterFollowers           StatType = "twitter_returned_followers"
	TwitterOther               StatType = "twitter_returned_other"
	TwitterErrors              StatType = "twitter_errors"
	TwitterAuthErrors          StatType = "twitter_auth_errors"
	TwitterRateErrors          StatType = "twitter_ratelimit_errors"
	TwitterXSearchQueries      StatType = "twitterx_search" // TODO: investigate if this is needed or used...
	WebQueries                 StatType = "web_queries"
	WebScrapedPages            StatType = "web_scraped_pages"
	WebProcessedPages          StatType = "web_processed_pages"
	WebErrors                  StatType = "web_errors"
	LLMQueries                 StatType = "llm_queries"
	LLMProcessedItems          StatType = "llm_processed_items"
	LLMErrors                  StatType = "llm_errors"
	TikTokTranscriptionSuccess StatType = "tiktok_transcription_success"
	TikTokTranscriptionErrors  StatType = "tiktok_transcription_errors"
	TikTokVideos               StatType = "tiktok_returned_videos"
	TikTokQueries              StatType = "tiktok_queries"
	TikTokErrors               StatType = "tiktok_errors"
	TikTokAuthErrors           StatType = "tiktok_auth_errors"
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
	jobServer        WorkerCapabilitiesProvider
	jobConfiguration config.JobConfiguration
}

// StartCollector starts a goroutine that listens to a channel for AddStat messages and updates the stats accordingly.
func StartCollector(bufSize uint, jc config.JobConfiguration) *StatsCollector {
	logrus.Info("Starting stats collector")

	s := Stats{
		BootTimeUnix:       time.Now().Unix(),
		Stats:              make(map[string]map[StatType]uint),
		WorkerVersion:      versioning.TEEWorkerVersion,
		ApplicationVersion: versioning.ApplicationVersion,
	}

	ch := make(chan AddStat, bufSize)

	go func(s *Stats, ch chan AddStat) {
		for {
			stat := <-ch
			s.Lock()
			s.LastOperationUnix = time.Now().Unix()
			if _, ok := s.Stats[stat.WorkerID]; !ok {
				s.Stats[stat.WorkerID] = make(map[StatType]uint)
			}
			s.Stats[stat.WorkerID][stat.Type] += stat.Num
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

// SetJobServer sets the JobServer reference and updates capabilities
func (s *StatsCollector) SetJobServer(js WorkerCapabilitiesProvider) {
	s.jobServer = js

	// Now that we have the JobServer, update capabilities
	s.Stats.Lock()
	defer s.Stats.Unlock()

	// Get capabilities from the JobServer directly
	s.Stats.ReportedCapabilities = js.GetWorkerCapabilities()

	logrus.Infof("Updated structured capabilities with JobServer: %+v", s.Stats.ReportedCapabilities)
}
