package types

import (
	"encoding/json"
	"fmt"

	"github.com/masa-finance/tee-worker/pkg/tee"
)

type EncryptedRequest struct {
	EncryptedResult  string `json:"encrypted_result"`
	EncryptedRequest string `json:"encrypted_request"`
}

func (payload EncryptedRequest) Unseal() (string, error) {
	jobRequest, err := tee.Unseal(payload.EncryptedRequest)
	if err != nil {
		return "", fmt.Errorf("error while unsealing the encrypted request: %w", err)
	}

	job := Job{}
	if err := json.Unmarshal(jobRequest, &job); err != nil {
		return "", fmt.Errorf("error while unmarshalling the job request: %w", err)
	}

	dat, err := tee.UnsealWithKey(job.Nonce, payload.EncryptedResult)
	if err != nil {
		return "", fmt.Errorf("error while unsealing the job result: %w", err)
	}

	return string(dat), nil
}

type JobError struct {
	Error string `json:"error"`
}
