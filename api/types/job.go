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
