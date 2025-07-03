package stats

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/masa-finance/tee-worker/api/types"
	"github.com/masa-finance/tee-worker/internal/capabilities"
	"github.com/masa-finance/tee-worker/internal/versioning"
	"github.com/sirupsen/logrus"
)

// These are the types of statistics that we can add. The value is the JSON key that will be used for serialization.
type statType string

const (
	TwitterScrapes             statType = "twitter_scrapes"
	TwitterTweets              statType = "twitter_returned_tweets"
	TwitterProfiles            statType = "twitter_returned_profiles"
	TwitterOther               statType = "twitter_returned_other"
	TwitterErrors              statType = "twitter_errors"
	TwitterAuthErrors          statType = "twitter_auth_errors"
	TwitterRateErrors          statType = "twitter_ratelimit_errors"
	TwitterXSearchQueries      statType = "twitterx_search"
	WebSuccess                 statType = "web_success"
	WebErrors                  statType = "web_errors"
	WebInvalid                 statType = "web_invalid"
	TikTokTranscriptionSuccess statType = "tiktok_transcription_success"
	TikTokTranscriptionErrors  statType = "tiktok_transcription_errors"
	LinkedInScrapes            statType = "linkedin_scrapes"
	LinkedInProfiles           statType = "linkedin_returned_profiles"
	LinkedInErrors             statType = "linkedin_errors"
	LinkedInAuthErrors         statType = "linkedin_auth_errors"
	LinkedInRateErrors         statType = "linkedin_ratelimit_errors"
	// TODO: Should we add stats for calls to each of the Twitter job types?
)

// AddStat is the struct used in the rest of the tee-worker for sending statistics
type AddStat struct {
	Type     statType
	WorkerID string
	Num      uint
}

// stats is the structure we use to store the statistics
type stats struct {
	BootTimeUnix         int64                        `json:"boot_time"`
	LastOperationUnix    int64                        `json:"last_operation_time"`
	CurrentTimeUnix      int64                        `json:"current_time"`
	WorkerID             string                       `json:"worker_id"`
	Stats                map[string]map[statType]uint `json:"stats"`
	ReportedCapabilities []string                     `json:"reported_capabilities"`
	WorkerVersion        string                       `json:"worker_version"`
	ApplicationVersion   string                       `json:"application_version"`
	sync.Mutex
}

// StatsCollector is the object used to collect statistics
type StatsCollector struct {
	Stats            *stats
	Chan             chan AddStat
	jobServer        capabilities.JobServerInterface
	jobConfiguration types.JobConfiguration
}

// StartCollector starts a goroutine that listens to a channel for AddStat messages and updates the stats accordingly.
func StartCollector(bufSize uint, jc types.JobConfiguration) *StatsCollector {
	logrus.Info("Starting stats collector")

	s := stats{
		BootTimeUnix:         time.Now().Unix(),
		Stats:                make(map[string]map[statType]uint),
		WorkerVersion:        versioning.TEEWorkerVersion,
		ApplicationVersion:   versioning.ApplicationVersion,
		ReportedCapabilities: []string{},
	}

	// Initial capability detection without JobServer (basic capabilities only)
	// Full capability detection will happen when JobServer is set
	detectedCapabilities := capabilities.DetectCapabilities(jc, nil)

	// Convert ScraperCapabilities to []string for tee-indexer compatibility
	s.ReportedCapabilities = make([]string, len(detectedCapabilities))
	for i, cap := range detectedCapabilities {
		s.ReportedCapabilities[i] = cap.Scraper
	}

	logrus.Infof("Initial capabilities: %v", s.ReportedCapabilities)

	ch := make(chan AddStat, bufSize)

	go func(s *stats, ch chan AddStat) {
		for {
			stat := <-ch
			s.Lock()
			s.LastOperationUnix = time.Now().Unix()
			if _, ok := s.Stats[stat.WorkerID]; !ok {
				s.Stats[stat.WorkerID] = make(map[statType]uint)
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
func (s *StatsCollector) Add(workerID string, typ statType, num uint) {
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

	// Update capabilities with full detection
	detectedCapabilities := capabilities.DetectCapabilities(s.jobConfiguration, js)

	// Convert ScraperCapabilities to []string for tee-indexer compatibility
	s.Stats.ReportedCapabilities = make([]string, len(detectedCapabilities))
	for i, cap := range detectedCapabilities {
		s.Stats.ReportedCapabilities[i] = cap.Scraper
	}

	logrus.Infof("Updated capabilities with full detection: %v", s.Stats.ReportedCapabilities)
}
