package types

import "encoding/json"

type JobResponse struct {
	UID string `json:"uid"`
}

type JobArguments map[string]interface{}

type JobResult struct {
	Error string      `json:"error"`
	Data  interface{} `json:"data"`
}

func (jr JobResult) Success() bool {
	return jr.Error == ""
}

type Job struct {
	Type      string       `json:"type"`
	Arguments JobArguments `json:"arguments"`
	UUID      string       `json:"-"`
}

func (ja JobArguments) Unmarshal(i interface{}) error {
	dat, err := json.Marshal(ja)
	if err != nil {
		return err
	}
	return json.Unmarshal(dat, i)
}
