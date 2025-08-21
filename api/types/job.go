package types

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"

	teetypes "github.com/masa-finance/tee-types/types"
	"github.com/masa-finance/tee-worker/pkg/tee"
	"golang.org/x/exp/rand"
)

type JobArguments map[string]interface{}

func (ja JobArguments) Unmarshal(i interface{}) error {
	dat, err := json.Marshal(ja)
	if err != nil {
		return err
	}
	return json.Unmarshal(dat, i)
}

type Job struct {
	Type         teetypes.JobType `json:"type"`
	Arguments    JobArguments     `json:"arguments"`
	UUID         string           `json:"-"`
	Nonce        string           `json:"quote"`
	WorkerID     string           `json:"worker_id"`
	TargetWorker string           `json:"target_worker"`
	Timeout      time.Duration    `json:"timeout"`
}

func (j Job) String() string {
	return fmt.Sprintf("UUID: %s Type: %s Arguments: %s", j.UUID, j.Type, j.Arguments)
}

var letterRunes = []rune("0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ!@#$%^&*()_+")

func randStringRunes(n int) string {
	b := make([]rune, n)
	for i := range b {
		// TODO: Move xcrypt from indexer to tee-types, and use RandomString here (although we'll need a different alpahbet)
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

// GenerateJobSignature generates a signature for the job.
func (job *Job) GenerateJobSignature() (string, error) {

	dat, err := json.Marshal(job)
	if err != nil {
		return "", err
	}

	checksum := sha256.New()
	checksum.Write(dat)

	job.Nonce = fmt.Sprintf("%s-%s", string(checksum.Sum(nil)), randStringRunes(99))

	dat, err = json.Marshal(job)
	if err != nil {
		return "", err
	}

	return tee.Seal(dat)
}

type JobResponse struct {
	UID string `json:"uid"`
}

type JobResult struct {
	Error      string `json:"error"`
	Data       []byte `json:"data"`
	Job        Job    `json:"job"`
	NextCursor string `json:"next_cursor"`
}

// Success returns true if the job was successful.
func (jr JobResult) Success() bool {
	return jr.Error == ""
}

// Seal returns the sealed job result.
func (jr JobResult) Seal() (string, error) {
	return tee.SealWithKey(jr.Job.Nonce, jr.Data)
}

// Unmarshal unmarshals the job result data.
func (jr JobResult) Unmarshal(i interface{}) error {
	return json.Unmarshal(jr.Data, i)
}

type JobRequest struct {
	EncryptedJob string `json:"encrypted_job"`
}

// DecryptJob decrypts the job request.
func (jobRequest JobRequest) DecryptJob() (*Job, error) {
	dat, err := tee.Unseal(jobRequest.EncryptedJob)
	if err != nil {
		return nil, err
	}

	job := Job{}
	if err := json.Unmarshal(dat, &job); err != nil {
		return nil, err
	}

	return &job, nil
}

type JobConfiguration map[string]interface{}

// Unmarshal unmarshals the job configuration into the supplied interface.
func (jc JobConfiguration) Unmarshal(v interface{}) error {
	data, err := json.Marshal(jc)
	if err != nil {
		return fmt.Errorf("error marshalling job configuration: %w", err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("error unmarshalling job configuration: %w", err)
	}

	return nil
}

// GetInt safely extracts an int from JobConfiguration, with a default fallback
func (jc JobConfiguration) GetInt(key string, def int) (int, error) {
	if v, ok := jc[key]; ok {
		switch val := v.(type) {
		case int:
			return val, nil
		case int64:
			return int(val), nil
		case float64:
			return int(val), nil
		case float32:
			return int(val), nil
		default:
			return def, fmt.Errorf("value %v for key %q cannot be converted to int", val, key)
		}
	}
	return def, nil
}

func (jc JobConfiguration) GetDuration(key string, defSecs int) time.Duration {
	// Go does not allow generics in methods :-(
	if v, ok := jc[key]; ok {
		if val, ok := v.(time.Duration); ok {
			return val
		}
	}
	return time.Duration(defSecs) * time.Second
}

func (jc JobConfiguration) GetString(key string, def string) string {
	if v, ok := jc[key]; ok {
		if val, ok := v.(string); ok {
			return val
		}
	}
	return def
}

// GetStringSlice safely extracts a string slice from JobConfiguration, with a default fallback
func (jc JobConfiguration) GetStringSlice(key string, def []string) []string {
	if v, ok := jc[key]; ok {
		if val, ok := v.([]string); ok {
			return val
		}
	}
	return def
}

// GetBool safely extracts a bool from JobConfiguration, with a default fallback
func (jc JobConfiguration) GetBool(key string, def bool) bool {
	if v, ok := jc[key]; ok {
		if val, ok := v.(bool); ok {
			return val
		}
	}
	return def
}

// TwitterScraperConfig represents the configuration needed for Twitter scraping
// This is defined here to avoid circular imports between api/types and internal/jobs
type TwitterScraperConfig struct {
	Accounts              []string
	ApiKeys               []string
	ApifyApiKey           string
	DataDir               string
	SkipLoginVerification bool
}

// GetTwitterConfig constructs a TwitterScraperConfig directly from the JobConfiguration
// This eliminates the need for JSON marshaling/unmarshaling
func (jc JobConfiguration) GetTwitterConfig() TwitterScraperConfig {
	return TwitterScraperConfig{
		Accounts:              jc.GetStringSlice("twitter_accounts", []string{}),
		ApiKeys:               jc.GetStringSlice("twitter_api_keys", []string{}),
		ApifyApiKey:           jc.GetString("apify_api_key", ""),
		DataDir:               jc.GetString("data_dir", ""),
		SkipLoginVerification: jc.GetBool("skip_login_verification", false),
	}
}

// RedditConfig represents the configuration needed for Reddit scraping via Apify
type RedditConfig struct {
	ApifyApiKey string
}

// GetRedditConfig constructs a RedditConfig directly from the JobConfiguration
// This eliminates the need for JSON marshaling/unmarshaling
func (jc JobConfiguration) GetRedditConfig() RedditConfig {
	return RedditConfig{
		ApifyApiKey: jc.GetString("apify_api_key", ""),
	}
}
