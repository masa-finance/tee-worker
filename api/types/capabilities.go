package types

// JobCapability represents the capabilities of a specific job type
type JobCapability struct {
	JobType      string       `json:"job_type"`
	Capabilities []Capability `json:"capabilities"`
}

// WorkerCapabilities represents all capabilities available on a worker
type WorkerCapabilities []JobCapability
