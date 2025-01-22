package types

import (
	"encoding/json"
	"fmt"
)

type JobResponse struct {
	UID string `json:"uid"`
}

type JobArguments map[string]interface{}

type JobResult struct {
	Error      string `json:"error"`
	Data       []byte `json:"data"`
	NextCursor string `json:"next_cursor"`
}

func (jr JobResult) Success() bool {
	return jr.Error == ""
}

type Job struct {
	Type      string       `json:"type"`
	Arguments JobArguments `json:"arguments"`
	UUID      string       `json:"-"`
}

func (jr JobResult) Unmarshal(i interface{}) error {
	return json.Unmarshal(jr.Data, i)
}

func (ja JobArguments) Unmarshal(i interface{}) error {
	dat, err := json.Marshal(ja)
	if err != nil {
		return err
	}
	return json.Unmarshal(dat, i)
}

type JobConfiguration map[string]interface{}

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
