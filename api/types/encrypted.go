package types

type EncryptedRequest struct {
	EncryptedResult string `json:"encrypted_result"`
}

type JobError struct {
	Error string `json:"error"`
}
