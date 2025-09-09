package recorder

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
	"vanta/pkg/config"
)

// RecordingEngine defines the interface for the recording system
type RecordingEngine interface {
	Start(config *config.RecordingConfig) error
	Stop() error
	Record(ctx *fasthttp.RequestCtx, responseBody []byte, duration time.Duration) error
	IsEnabled() bool
	GetStats() *RecordingStats
	GetStorage() Storage
}

// DefaultRecordingEngine implements the RecordingEngine interface
type DefaultRecordingEngine struct {
	storage Storage
	config  *config.RecordingConfig
	filters []Filter
	enabled bool
	logger  *zap.Logger
	stats   *RecordingStats
	mu      sync.RWMutex
}

// NewDefaultRecordingEngine creates a new recording engine instance
func NewDefaultRecordingEngine(storage Storage, logger *zap.Logger) *DefaultRecordingEngine {
	return &DefaultRecordingEngine{
		storage: storage,
		logger:  logger,
		stats: &RecordingStats{
			StartTime: time.Now(),
		},
	}
}

// Start starts the recording engine with the given configuration
func (r *DefaultRecordingEngine) Start(config *config.RecordingConfig) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if config == nil {
		return fmt.Errorf("recording configuration cannot be nil")
	}

	r.config = config
	r.enabled = config.Enabled

	// Create filters from configuration
	filters, err := r.createFilters(config.Filters)
	if err != nil {
		return fmt.Errorf("failed to create filters: %w", err)
	}
	r.filters = filters

	// Reset stats
	r.stats = &RecordingStats{
		StartTime: time.Now(),
	}

	if r.enabled {
		r.logger.Info("Recording engine started",
			zap.Int("filters", len(r.filters)),
			zap.Int("max_recordings", config.MaxRecordings),
			zap.Int64("max_body_size", config.MaxBodySize))
	}

	return nil
}

// Stop stops the recording engine
func (r *DefaultRecordingEngine) Stop() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.enabled = false
	r.logger.Info("Recording engine stopped",
		zap.Int64("total_requests", r.stats.TotalRequests),
		zap.Int64("recorded_requests", r.stats.RecordedRequests),
		zap.Int64("filtered_requests", r.stats.FilteredRequests))

	return nil
}

// Record captures a request/response pair
func (r *DefaultRecordingEngine) Record(ctx *fasthttp.RequestCtx, responseBody []byte, duration time.Duration) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Increment total request counter
	r.stats.TotalRequests++

	if !r.enabled {
		return nil
	}

	// Create recording
	recording, err := r.createRecording(ctx, responseBody, duration)
	if err != nil {
		r.stats.Errors++
		return fmt.Errorf("failed to create recording: %w", err)
	}

	// Apply filters
	if !r.shouldRecord(recording) {
		r.stats.FilteredRequests++
		return nil
	}

	// Check body size limit
	if r.config.MaxBodySize > 0 {
		requestBodySize := int64(len(recording.Request.Body))
		responseBodySize := int64(len(recording.Response.Body))
		if requestBodySize > r.config.MaxBodySize || responseBodySize > r.config.MaxBodySize {
			r.logger.Debug("Recording skipped due to body size limit",
				zap.String("id", recording.ID),
				zap.Int64("request_body_size", requestBodySize),
				zap.Int64("response_body_size", responseBodySize),
				zap.Int64("max_body_size", r.config.MaxBodySize))
			r.stats.FilteredRequests++
			return nil
		}
	}

	// Save recording
	if err := r.storage.Save(recording); err != nil {
		r.stats.Errors++
		return fmt.Errorf("failed to save recording: %w", err)
	}

	// Update stats
	r.stats.RecordedRequests++
	r.stats.LastRecording = recording.Timestamp

	r.logger.Debug("Request recorded",
		zap.String("id", recording.ID),
		zap.String("method", recording.Request.Method),
		zap.String("uri", recording.Request.URI),
		zap.Int("status", recording.Response.StatusCode),
		zap.Duration("duration", duration))

	return nil
}

// IsEnabled returns true if recording is enabled
func (r *DefaultRecordingEngine) IsEnabled() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.enabled
}

// GetStats returns recording statistics
func (r *DefaultRecordingEngine) GetStats() *RecordingStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Create a copy to avoid race conditions
	return &RecordingStats{
		TotalRequests:    r.stats.TotalRequests,
		RecordedRequests: r.stats.RecordedRequests,
		FilteredRequests: r.stats.FilteredRequests,
		Errors:           r.stats.Errors,
		StartTime:        r.stats.StartTime,
		LastRecording:    r.stats.LastRecording,
	}
}

// GetStorage returns the storage backend
func (r *DefaultRecordingEngine) GetStorage() Storage {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.storage
}

// createRecording creates a Recording from FastHTTP context
func (r *DefaultRecordingEngine) createRecording(ctx *fasthttp.RequestCtx, responseBody []byte, duration time.Duration) (*Recording, error) {
	// Generate unique ID
	recordingID := uuid.New().String()

	// Extract request information
	request := RecordedRequest{
		Method:      string(ctx.Method()),
		URI:         string(ctx.RequestURI()),
		Headers:     r.extractHeaders(&ctx.Request.Header, r.config.IncludeHeaders, r.config.ExcludeHeaders),
		Body:        ctx.Request.Body(),
		QueryParams: r.extractQueryParams(ctx),
		ContentType: string(ctx.Request.Header.ContentType()),
	}

	// Extract response information
	response := RecordedResponse{
		StatusCode:  ctx.Response.StatusCode(),
		Headers:     r.extractResponseHeaders(&ctx.Response.Header, r.config.IncludeHeaders, r.config.ExcludeHeaders),
		Body:        responseBody,
		ContentType: string(ctx.Response.Header.ContentType()),
	}

	// Extract metadata
	metadata := RecordingMetadata{
		Source:    "live",
		ClientIP:  ctx.RemoteIP().String(),
		UserAgent: string(ctx.Request.Header.UserAgent()),
	}

	// Get request ID if available
	if requestID := ctx.UserValue("request_id"); requestID != nil {
		if id, ok := requestID.(string); ok {
			metadata.RequestID = id
		}
	}

	// Check if chaos was applied
	if chaosApplied := ctx.UserValue("chaos_applied"); chaosApplied != nil {
		if applied, ok := chaosApplied.(bool); ok {
			metadata.ChaosApplied = applied
		}
	}

	recording := &Recording{
		ID:        recordingID,
		Timestamp: time.Now(),
		Request:   request,
		Response:  response,
		Metadata:  metadata,
		Duration:  duration,
	}

	return recording, nil
}

// extractHeaders extracts headers based on include/exclude lists
func (r *DefaultRecordingEngine) extractHeaders(header *fasthttp.RequestHeader, include, exclude []string) map[string]string {
	headers := make(map[string]string)

	header.VisitAll(func(key, value []byte) {
		keyStr := string(key)
		valueStr := string(value)

		// Check exclude list first
		if len(exclude) > 0 {
			for _, excludeHeader := range exclude {
				if strings.EqualFold(keyStr, excludeHeader) {
					return
				}
			}
		}

		// Check include list
		if len(include) > 0 {
			for _, includeHeader := range include {
				if strings.EqualFold(keyStr, includeHeader) {
					headers[keyStr] = valueStr
					return
				}
			}
		} else {
			// If no include list, include all (except excluded)
			headers[keyStr] = valueStr
		}
	})

	return headers
}

// extractResponseHeaders extracts response headers based on include/exclude lists
func (r *DefaultRecordingEngine) extractResponseHeaders(header *fasthttp.ResponseHeader, include, exclude []string) map[string]string {
	headers := make(map[string]string)

	header.VisitAll(func(key, value []byte) {
		keyStr := string(key)
		valueStr := string(value)

		// Check exclude list first
		if len(exclude) > 0 {
			for _, excludeHeader := range exclude {
				if strings.EqualFold(keyStr, excludeHeader) {
					return
				}
			}
		}

		// Check include list
		if len(include) > 0 {
			for _, includeHeader := range include {
				if strings.EqualFold(keyStr, includeHeader) {
					headers[keyStr] = valueStr
					return
				}
			}
		} else {
			// If no include list, include all (except excluded)
			headers[keyStr] = valueStr
		}
	})

	return headers
}

// extractQueryParams extracts query parameters from the request
func (r *DefaultRecordingEngine) extractQueryParams(ctx *fasthttp.RequestCtx) map[string]string {
	params := make(map[string]string)

	ctx.QueryArgs().VisitAll(func(key, value []byte) {
		params[string(key)] = string(value)
	})

	return params
}

// shouldRecord determines if a recording should be saved based on filters
func (r *DefaultRecordingEngine) shouldRecord(recording *Recording) bool {
	if len(r.filters) == 0 {
		return true
	}

	for _, filter := range r.filters {
		if !filter.Apply(recording) {
			return false
		}
	}

	return true
}

// createFilters creates filter instances from configuration
func (r *DefaultRecordingEngine) createFilters(filterConfigs []config.RecordingFilter) ([]Filter, error) {
	var filters []Filter

	for _, config := range filterConfigs {
		filter, err := NewFilter(config)
		if err != nil {
			return nil, fmt.Errorf("failed to create filter %s: %w", config.Type, err)
		}
		filters = append(filters, filter)
	}

	return filters, nil
}

// MethodFilter filters recordings based on HTTP method
type MethodFilter struct {
	methods []string
	negate  bool
}

// NewMethodFilter creates a new method filter
func NewMethodFilter(methods []string, negate bool) *MethodFilter {
	return &MethodFilter{
		methods: methods,
		negate:  negate,
	}
}

// Apply applies the method filter to a recording
func (f *MethodFilter) Apply(recording *Recording) bool {
	found := false
	for _, method := range f.methods {
		if strings.EqualFold(recording.Request.Method, method) {
			found = true
			break
		}
	}

	if f.negate {
		return !found
	}
	return found
}

// String returns a string representation of the filter
func (f *MethodFilter) String() string {
	operation := "include"
	if f.negate {
		operation = "exclude"
	}
	return fmt.Sprintf("method:%s:%v", operation, f.methods)
}

// EndpointFilter filters recordings based on endpoint pattern
type EndpointFilter struct {
	patterns []string
	negate   bool
}

// NewEndpointFilter creates a new endpoint filter
func NewEndpointFilter(patterns []string, negate bool) *EndpointFilter {
	return &EndpointFilter{
		patterns: patterns,
		negate:   negate,
	}
}

// Apply applies the endpoint filter to a recording
func (f *EndpointFilter) Apply(recording *Recording) bool {
	found := false
	for _, pattern := range f.patterns {
		if f.matchesPattern(recording.Request.URI, pattern) {
			found = true
			break
		}
	}

	if f.negate {
		return !found
	}
	return found
}

// matchesPattern checks if a URI matches a pattern (supports * wildcards)
func (f *EndpointFilter) matchesPattern(uri, pattern string) bool {
	if pattern == "*" {
		return true
	}
	if strings.Contains(pattern, "*") {
		// Simple wildcard matching
		parts := strings.Split(pattern, "*")
		if len(parts) == 2 {
			prefix, suffix := parts[0], parts[1]
			return strings.HasPrefix(uri, prefix) && strings.HasSuffix(uri, suffix)
		}
	}
	return strings.Contains(uri, pattern)
}

// String returns a string representation of the filter
func (f *EndpointFilter) String() string {
	operation := "include"
	if f.negate {
		operation = "exclude"
	}
	return fmt.Sprintf("endpoint:%s:%v", operation, f.patterns)
}

// StatusFilter filters recordings based on HTTP status code
type StatusFilter struct {
	statusCodes []int
	negate      bool
}

// NewStatusFilter creates a new status filter
func NewStatusFilter(statusCodes []int, negate bool) *StatusFilter {
	return &StatusFilter{
		statusCodes: statusCodes,
		negate:      negate,
	}
}

// Apply applies the status filter to a recording
func (f *StatusFilter) Apply(recording *Recording) bool {
	found := false
	for _, status := range f.statusCodes {
		if recording.Response.StatusCode == status {
			found = true
			break
		}
	}

	if f.negate {
		return !found
	}
	return found
}

// String returns a string representation of the filter
func (f *StatusFilter) String() string {
	operation := "include"
	if f.negate {
		operation = "exclude"
	}
	return fmt.Sprintf("status:%s:%v", operation, f.statusCodes)
}

// NewFilter creates a filter based on configuration
func NewFilter(config config.RecordingFilter) (Filter, error) {
	switch strings.ToLower(config.Type) {
	case "method":
		return NewMethodFilter(config.Values, config.Negate), nil

	case "endpoint":
		return NewEndpointFilter(config.Values, config.Negate), nil

	case "status":
		var statusCodes []int
		for _, value := range config.Values {
			var status int
			if _, err := fmt.Sscanf(value, "%d", &status); err != nil {
				return nil, fmt.Errorf("invalid status code: %s", value)
			}
			statusCodes = append(statusCodes, status)
		}
		return NewStatusFilter(statusCodes, config.Negate), nil

	default:
		return nil, fmt.Errorf("unknown filter type: %s", config.Type)
	}
}