package recorder

import (
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

// Replayer handles traffic replay functionality
type Replayer struct {
	recordings []*Recording
	storage    Storage
	client     *fasthttp.Client
	logger     *zap.Logger
	config     *ReplayConfig
	stats      *ReplayStats
	mu         sync.RWMutex
}

// NewReplayer creates a new replayer instance
func NewReplayer(storage Storage, logger *zap.Logger) *Replayer {
	client := &fasthttp.Client{
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	return &Replayer{
		storage: storage,
		client:  client,
		logger:  logger,
		stats: &ReplayStats{
			StartTime: time.Now(),
		},
	}
}

// LoadRecordings loads recordings from storage for replay
func (r *Replayer) LoadRecordings(filter ListFilter) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	recordings, err := r.storage.List(filter)
	if err != nil {
		return fmt.Errorf("failed to load recordings: %w", err)
	}

	r.recordings = recordings
	r.logger.Info("Loaded recordings for replay", zap.Int("count", len(recordings)))

	return nil
}

// LoadRecordingsByIDs loads specific recordings by their IDs
func (r *Replayer) LoadRecordingsByIDs(ids []string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var recordings []*Recording
	for _, id := range ids {
		recording, err := r.storage.Load(id)
		if err != nil {
			r.logger.Warn("Failed to load recording", zap.String("id", id), zap.Error(err))
			continue
		}
		recordings = append(recordings, recording)
	}

	r.recordings = recordings
	r.logger.Info("Loaded recordings for replay", zap.Int("count", len(recordings)))

	return nil
}

// ReplayTraffic replays the loaded recordings against a target URL
func (r *Replayer) ReplayTraffic(config *ReplayConfig) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if config == nil {
		return fmt.Errorf("replay configuration cannot be nil")
	}

	if len(r.recordings) == 0 {
		return fmt.Errorf("no recordings loaded for replay")
	}

	r.config = config
	r.stats = &ReplayStats{
		StartTime: time.Now(),
	}

	// Configure client based on replay config
	r.client.ReadTimeout = config.Timeout
	r.client.WriteTimeout = config.Timeout

	if config.SkipTLSVerify {
		// Note: FastHTTP doesn't have built-in TLS skip verification
		// This would require custom TLS config
		r.logger.Warn("TLS verification skip not implemented for FastHTTP client")
	}

	r.logger.Info("Starting traffic replay",
		zap.String("target", config.TargetURL),
		zap.Int("recordings", len(r.recordings)),
		zap.Int("concurrency", config.Concurrency))

	// Parse target URL
	targetURL, err := url.Parse(config.TargetURL)
	if err != nil {
		return fmt.Errorf("invalid target URL: %w", err)
	}

	// Create semaphore for concurrency control
	sem := make(chan struct{}, config.Concurrency)
	var wg sync.WaitGroup

	// Calculate delay based on configuration
	delay := config.DelayBetween
	if delay == 0 {
		delay = 100 * time.Millisecond // Default delay
	}

	// Replay recordings
	for i, recording := range r.recordings {
		sem <- struct{}{} // Acquire semaphore
		wg.Add(1)

		go func(idx int, rec *Recording) {
			defer func() {
				<-sem // Release semaphore
				wg.Done()
			}()

			if err := r.replayRecording(rec, targetURL); err != nil {
				r.logger.Error("Failed to replay recording",
					zap.String("id", rec.ID),
					zap.Int("index", idx),
					zap.Error(err))
				r.incrementFailedRequests()
			} else {
				r.incrementSuccessRequests()
			}
		}(i, recording)

		// Add delay between requests if specified
		if delay > 0 && i < len(r.recordings)-1 {
			time.Sleep(delay)
		}
	}

	// Wait for all replays to complete
	wg.Wait()

	r.stats.EndTime = time.Now()
	r.logger.Info("Traffic replay completed",
		zap.Int64("total", r.stats.TotalRequests),
		zap.Int64("success", r.stats.SuccessRequests),
		zap.Int64("failed", r.stats.FailedRequests),
		zap.Duration("duration", r.stats.EndTime.Sub(r.stats.StartTime)))

	return nil
}

// replayRecording replays a single recording
func (r *Replayer) replayRecording(recording *Recording, targetURL *url.URL) error {
	r.incrementTotalRequests()

	// Create request
	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)

	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(resp)

	// Set method
	req.Header.SetMethod(recording.Request.Method)

	// Set URI
	uri := r.buildReplayURI(recording.Request.URI, targetURL)
	req.SetRequestURI(uri)

	// Set headers
	for key, value := range recording.Request.Headers {
		if r.shouldIncludeHeader(key) {
			req.Header.Set(key, value)
		}
	}

	// Apply header overrides
	for key, value := range r.config.OverrideHeaders {
		req.Header.Set(key, value)
	}

	// Set body
	if len(recording.Request.Body) > 0 {
		req.SetBody(recording.Request.Body)
	}

	// Record start time for latency calculation
	startTime := time.Now()

	// Make request
	err := r.client.Do(req, resp)
	latency := time.Since(startTime)

	// Update average latency
	r.updateAverageLatency(latency)

	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}

	r.logger.Debug("Request replayed",
		zap.String("id", recording.ID),
		zap.String("method", recording.Request.Method),
		zap.String("uri", uri),
		zap.Int("original_status", recording.Response.StatusCode),
		zap.Int("replay_status", resp.StatusCode()),
		zap.Duration("latency", latency))

	return nil
}

// buildReplayURI constructs the target URI for replay
func (r *Replayer) buildReplayURI(originalURI string, targetURL *url.URL) string {
	if r.config.ReplaceHost {
		// Parse original URI to extract path and query
		origURL, err := url.Parse(originalURI)
		if err != nil {
			// If parsing fails, use original URI as path
			return targetURL.Scheme + "://" + targetURL.Host + originalURI
		}

		// Construct new URL with target host
		newURL := *targetURL
		newURL.Path = origURL.Path
		newURL.RawQuery = origURL.RawQuery
		return newURL.String()
	}

	return originalURI
}

// shouldIncludeHeader determines if a header should be included in replay
func (r *Replayer) shouldIncludeHeader(headerName string) bool {
	lowerName := strings.ToLower(headerName)

	// Skip certain headers that shouldn't be replayed
	skipHeaders := []string{
		"host",           // Will be set by target URL
		"content-length", // Will be set automatically
		"connection",     // FastHTTP manages this
		"transfer-encoding",
	}

	for _, skip := range skipHeaders {
		if lowerName == skip {
			return false
		}
	}

	// Check preserve headers list
	if len(r.config.PreserveHeaders) > 0 {
		for _, preserve := range r.config.PreserveHeaders {
			if strings.EqualFold(headerName, preserve) {
				return true
			}
		}
		return false
	}

	return true
}

// GetStats returns replay statistics
func (r *Replayer) GetStats() *ReplayStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Return a copy to avoid race conditions
	return &ReplayStats{
		TotalRequests:   r.stats.TotalRequests,
		SuccessRequests: r.stats.SuccessRequests,
		FailedRequests:  r.stats.FailedRequests,
		AverageLatency:  r.stats.AverageLatency,
		StartTime:       r.stats.StartTime,
		EndTime:         r.stats.EndTime,
	}
}

// GetLoadedRecordings returns the currently loaded recordings
func (r *Replayer) GetLoadedRecordings() []*Recording {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Return a copy to avoid race conditions
	recordings := make([]*Recording, len(r.recordings))
	copy(recordings, r.recordings)
	return recordings
}

// incrementTotalRequests atomically increments the total request counter
func (r *Replayer) incrementTotalRequests() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stats.TotalRequests++
}

// incrementSuccessRequests atomically increments the success request counter
func (r *Replayer) incrementSuccessRequests() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stats.SuccessRequests++
}

// incrementFailedRequests atomically increments the failed request counter
func (r *Replayer) incrementFailedRequests() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stats.FailedRequests++
}

// updateAverageLatency updates the average latency calculation
func (r *Replayer) updateAverageLatency(latency time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()

	totalRequests := r.stats.TotalRequests
	if totalRequests == 1 {
		r.stats.AverageLatency = latency
	} else {
		// Calculate running average
		currentAvg := r.stats.AverageLatency
		r.stats.AverageLatency = time.Duration(int64(currentAvg) + (int64(latency)-int64(currentAvg))/totalRequests)
	}
}

// ReplayManager manages multiple replay operations
type ReplayManager struct {
	storage Storage
	logger  *zap.Logger
	active  map[string]*Replayer
	mu      sync.RWMutex
}

// NewReplayManager creates a new replay manager
func NewReplayManager(storage Storage, logger *zap.Logger) *ReplayManager {
	return &ReplayManager{
		storage: storage,
		logger:  logger,
		active:  make(map[string]*Replayer),
	}
}

// StartReplay starts a new replay operation
func (rm *ReplayManager) StartReplay(id string, config *ReplayConfig, filter ListFilter) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if _, exists := rm.active[id]; exists {
		return fmt.Errorf("replay with ID %s already active", id)
	}

	replayer := NewReplayer(rm.storage, rm.logger.With(zap.String("replay_id", id)))
	
	if err := replayer.LoadRecordings(filter); err != nil {
		return fmt.Errorf("failed to load recordings: %w", err)
	}

	rm.active[id] = replayer

	// Start replay in background
	go func() {
		defer func() {
			rm.mu.Lock()
			delete(rm.active, id)
			rm.mu.Unlock()
		}()

		if err := replayer.ReplayTraffic(config); err != nil {
			rm.logger.Error("Replay failed", zap.String("id", id), zap.Error(err))
		}
	}()

	return nil
}

// GetReplayStats returns statistics for an active replay
func (rm *ReplayManager) GetReplayStats(id string) (*ReplayStats, error) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	replayer, exists := rm.active[id]
	if !exists {
		return nil, fmt.Errorf("replay with ID %s not found", id)
	}

	return replayer.GetStats(), nil
}

// ListActiveReplays returns a list of active replay IDs
func (rm *ReplayManager) ListActiveReplays() []string {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	var ids []string
	for id := range rm.active {
		ids = append(ids, id)
	}

	return ids
}

// StopReplay stops an active replay (not implemented as replays run to completion)
func (rm *ReplayManager) StopReplay(id string) error {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	if _, exists := rm.active[id]; !exists {
		return fmt.Errorf("replay with ID %s not found", id)
	}

	// Note: Current implementation doesn't support stopping replays mid-execution
	// This would require adding context cancellation to the replay process
	return fmt.Errorf("stopping active replays is not currently supported")
}