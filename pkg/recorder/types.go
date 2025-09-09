package recorder

import (
	"time"
)

// Recording represents a captured HTTP request/response pair
type Recording struct {
	ID        string            `json:"id"`
	Timestamp time.Time         `json:"timestamp"`
	Request   RecordedRequest   `json:"request"`
	Response  RecordedResponse  `json:"response"`
	Metadata  RecordingMetadata `json:"metadata"`
	Duration  time.Duration     `json:"duration"` // Request processing time
}

// RecordedRequest captures the essential parts of an HTTP request
type RecordedRequest struct {
	Method      string            `json:"method"`
	URI         string            `json:"uri"`
	Headers     map[string]string `json:"headers"`
	Body        []byte            `json:"body,omitempty"`
	QueryParams map[string]string `json:"query_params"`
	ContentType string            `json:"content_type"`
}

// RecordedResponse captures the essential parts of an HTTP response
type RecordedResponse struct {
	StatusCode  int               `json:"status_code"`
	Headers     map[string]string `json:"headers"`
	Body        []byte            `json:"body,omitempty"`
	ContentType string            `json:"content_type"`
}

// RecordingMetadata contains additional context about the recording
type RecordingMetadata struct {
	Source       string   `json:"source"`        // "live" or "generated"
	Endpoint     string   `json:"endpoint"`      // OpenAPI operation ID
	ClientIP     string   `json:"client_ip"`
	UserAgent    string   `json:"user_agent"`
	RequestID    string   `json:"request_id"`
	ChaosApplied bool     `json:"chaos_applied,omitempty"`
	Tags         []string `json:"tags,omitempty"`
}


// ListFilter defines filtering options for listing recordings
type ListFilter struct {
	Limit       int       `json:"limit,omitempty"`
	Offset      int       `json:"offset,omitempty"`
	StartTime   time.Time `json:"start_time,omitempty"`
	EndTime     time.Time `json:"end_time,omitempty"`
	Methods     []string  `json:"methods,omitempty"`
	Endpoints   []string  `json:"endpoints,omitempty"`
	StatusCodes []int     `json:"status_codes,omitempty"`
}

// StorageStats provides statistics about storage usage
type StorageStats struct {
	TotalRecordings int64     `json:"total_recordings"`
	TotalSize       int64     `json:"total_size_bytes"`
	OldestRecording time.Time `json:"oldest_recording,omitempty"`
	NewestRecording time.Time `json:"newest_recording,omitempty"`
}

// RecordingStats tracks recording system statistics
type RecordingStats struct {
	TotalRequests    int64     `json:"total_requests"`
	RecordedRequests int64     `json:"recorded_requests"`
	FilteredRequests int64     `json:"filtered_requests"`
	Errors           int64     `json:"errors"`
	StartTime        time.Time `json:"start_time"`
	LastRecording    time.Time `json:"last_recording"`
}

// ReplayConfig defines configuration for traffic replay
type ReplayConfig struct {
	TargetURL       string            `yaml:"target_url"`
	Concurrency     int               `yaml:"concurrency"`
	DelayBetween    time.Duration     `yaml:"delay_between"`
	FollowRedirects bool              `yaml:"follow_redirects"`
	Timeout         time.Duration     `yaml:"timeout"`
	SkipTLSVerify   bool              `yaml:"skip_tls_verify"`
	ReplaceHost     bool              `yaml:"replace_host"`
	PreserveHeaders []string          `yaml:"preserve_headers"`
	OverrideHeaders map[string]string `yaml:"override_headers"`
}

// ReplayStats tracks replay operation statistics
type ReplayStats struct {
	TotalRequests   int64         `json:"total_requests"`
	SuccessRequests int64         `json:"success_requests"`
	FailedRequests  int64         `json:"failed_requests"`
	AverageLatency  time.Duration `json:"average_latency"`
	StartTime       time.Time     `json:"start_time"`
	EndTime         time.Time     `json:"end_time"`
}

// RecordingIndex represents metadata for efficient recording lookup
type RecordingIndex struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Method    string    `json:"method"`
	URI       string    `json:"uri"`
	Status    int       `json:"status"`
	Filename  string    `json:"filename"`
}

// Filter represents a recording filter function
type Filter interface {
	Apply(recording *Recording) bool
	String() string
}