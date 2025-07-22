package types

// ScraperCapability represents the capabilities of a specific scraper type
type ScraperCapability struct {
	JobType      string       `json:"job_type"`
	Capabilities []Capability `json:"capabilities"`
}

// WorkerCapabilities represents all capabilities available on a worker
type WorkerCapabilities []ScraperCapability
