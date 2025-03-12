package types

type Key struct {
	Key string `json:"key"`

	Signature string `json:"signature"`
}

type KeyResponse struct {
	Status string `json:"status"`
}
