package types

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"

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
	Type      string        `json:"type"`
	Arguments JobArguments  `json:"arguments"`
	UUID      string        `json:"-"`
	Nonce     string        `json:"quote"`
	WorkerID  string        `json:"worker_id"`
	Timeout   time.Duration `json:"-"`
}

var letterRunes = []rune("0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ!@#$%^&*()_+")

func randStringRunes(n int) string {
	b := make([]rune, n)
	for i := range b {
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

// getInt safely extracts an int from JobConfiguration, with a default fallback
func (jc JobConfiguration) GetInt(key string, def int) int {
	if v, ok := jc[key]; ok {
		switch val := v.(type) {
		case int:
			return val
		case int64:
			return int(val)
		case float64:
			return int(val)
		case float32:
			return int(val)
		}
	}
	return def
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
