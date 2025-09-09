package recorder

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestNewReplayer(t *testing.T) {
	logger := zaptest.NewLogger(t)
	storage := NewMemoryStorage()

	replayer := NewReplayer(storage, logger)

	assert.NotNil(t, replayer)
	assert.NotNil(t, replayer.client)
	assert.NotNil(t, replayer.stats)
}

func TestReplayer_LoadRecordings(t *testing.T) {
	logger := zaptest.NewLogger(t)
	storage := NewMemoryStorage()

	// Add test recordings to storage
	recordings := []*Recording{
		{
			ID:        "test-1",
			Timestamp: time.Now(),
			Request: RecordedRequest{
				Method: "GET",
				URI:    "http://example.com/api/test1",
			},
			Response: RecordedResponse{
				StatusCode: 200,
			},
		},
		{
			ID:        "test-2",
			Timestamp: time.Now(),
			Request: RecordedRequest{
				Method: "POST",
				URI:    "http://example.com/api/test2",
			},
			Response: RecordedResponse{
				StatusCode: 201,
			},
		},
	}

	for _, recording := range recordings {
		err := storage.Save(recording)
		require.NoError(t, err)
	}

	replayer := NewReplayer(storage, logger)

	// Test LoadRecordings
	err := replayer.LoadRecordings(ListFilter{})
	require.NoError(t, err)

	loadedRecordings := replayer.GetLoadedRecordings()
	assert.Len(t, loadedRecordings, 2)
}

func TestReplayer_LoadRecordingsByIDs(t *testing.T) {
	logger := zaptest.NewLogger(t)
	storage := NewMemoryStorage()

	// Add test recordings to storage
	recordings := []*Recording{
		{
			ID:        "test-1",
			Timestamp: time.Now(),
			Request: RecordedRequest{
				Method: "GET",
				URI:    "http://example.com/api/test1",
			},
			Response: RecordedResponse{
				StatusCode: 200,
			},
		},
		{
			ID:        "test-2",
			Timestamp: time.Now(),
			Request: RecordedRequest{
				Method: "POST",
				URI:    "http://example.com/api/test2",
			},
			Response: RecordedResponse{
				StatusCode: 201,
			},
		},
	}

	for _, recording := range recordings {
		err := storage.Save(recording)
		require.NoError(t, err)
	}

	replayer := NewReplayer(storage, logger)

	// Test LoadRecordingsByIDs
	err := replayer.LoadRecordingsByIDs([]string{"test-1"})
	require.NoError(t, err)

	loadedRecordings := replayer.GetLoadedRecordings()
	assert.Len(t, loadedRecordings, 1)
	assert.Equal(t, "test-1", loadedRecordings[0].ID)
}

func TestReplayer_ReplayTraffic(t *testing.T) {
	// Create a test HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Echo back some info about the request
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/test1" && r.Method == "GET" {
			w.WriteHeader(200)
			w.Write([]byte(`{"replayed": true, "path": "/api/test1"}`))
		} else if r.URL.Path == "/api/test2" && r.Method == "POST" {
			w.WriteHeader(201)
			w.Write([]byte(`{"replayed": true, "path": "/api/test2"}`))
		} else {
			w.WriteHeader(404)
			w.Write([]byte(`{"error": "not found"}`))
		}
	}))
	defer server.Close()

	logger := zaptest.NewLogger(t)
	storage := NewMemoryStorage()

	// Add test recordings to storage
	recordings := []*Recording{
		{
			ID:        "test-1",
			Timestamp: time.Now(),
			Request: RecordedRequest{
				Method: "GET",
				URI:    "http://original.com/api/test1",
				Headers: map[string]string{
					"User-Agent": "test-agent",
				},
			},
			Response: RecordedResponse{
				StatusCode: 200,
			},
		},
		{
			ID:        "test-2",
			Timestamp: time.Now(),
			Request: RecordedRequest{
				Method: "POST",
				URI:    "http://original.com/api/test2",
				Headers: map[string]string{
					"Content-Type": "application/json",
				},
				Body: []byte(`{"test": true}`),
			},
			Response: RecordedResponse{
				StatusCode: 201,
			},
		},
	}

	for _, recording := range recordings {
		err := storage.Save(recording)
		require.NoError(t, err)
	}

	replayer := NewReplayer(storage, logger)

	// Load recordings
	err := replayer.LoadRecordings(ListFilter{})
	require.NoError(t, err)

	// Configure replay
	config := &ReplayConfig{
		TargetURL:    server.URL,
		Concurrency:  1,
		DelayBetween: 10 * time.Millisecond,
		Timeout:      5 * time.Second,
		ReplaceHost:  true,
	}

	// Replay traffic
	err = replayer.ReplayTraffic(config)
	require.NoError(t, err)

	// Check stats
	stats := replayer.GetStats()
	assert.Equal(t, int64(2), stats.TotalRequests)
	assert.Equal(t, int64(2), stats.SuccessRequests)
	assert.Equal(t, int64(0), stats.FailedRequests)
	assert.True(t, stats.AverageLatency > 0)
}

func TestReplayConfig_HeaderHandling(t *testing.T) {
	logger := zaptest.NewLogger(t)
	storage := NewMemoryStorage()

	replayer := NewReplayer(storage, logger)

	// Test shouldIncludeHeader with default settings
	config := &ReplayConfig{
		PreserveHeaders: []string{},
		OverrideHeaders: map[string]string{},
	}
	replayer.config = config

	// Default behavior - should include most headers except special ones
	assert.True(t, replayer.shouldIncludeHeader("User-Agent"))
	assert.True(t, replayer.shouldIncludeHeader("Content-Type"))
	assert.False(t, replayer.shouldIncludeHeader("Host"))
	assert.False(t, replayer.shouldIncludeHeader("Content-Length"))
	assert.False(t, replayer.shouldIncludeHeader("Connection"))

	// Test with preserve headers list
	config.PreserveHeaders = []string{"User-Agent", "Authorization"}
	assert.True(t, replayer.shouldIncludeHeader("User-Agent"))
	assert.True(t, replayer.shouldIncludeHeader("Authorization"))
	assert.False(t, replayer.shouldIncludeHeader("Content-Type"))
}

func TestReplayManager(t *testing.T) {
	logger := zaptest.NewLogger(t)
	storage := NewMemoryStorage()

	manager := NewReplayManager(storage, logger)
	assert.NotNil(t, manager)
	assert.NotNil(t, manager.active)

	// Test ListActiveReplays with no active replays
	active := manager.ListActiveReplays()
	assert.Empty(t, active)

	// Test GetReplayStats for non-existent replay
	_, err := manager.GetReplayStats("non-existent")
	assert.Error(t, err)
}

func TestReplayer_BuildReplayURI(t *testing.T) {
	logger := zaptest.NewLogger(t)
	storage := NewMemoryStorage()
	replayer := NewReplayer(storage, logger)

	tests := []struct {
		name        string
		originalURI string
		targetURL   string
		replaceHost bool
		expected    string
	}{
		{
			name:        "replace host enabled",
			originalURI: "http://original.com/api/test?param=value",
			targetURL:   "http://localhost:8080",
			replaceHost: true,
			expected:    "http://localhost:8080/api/test?param=value",
		},
		{
			name:        "replace host disabled",
			originalURI: "http://original.com/api/test?param=value",
			targetURL:   "http://localhost:8080",
			replaceHost: false,
			expected:    "http://original.com/api/test?param=value",
		},
		{
			name:        "replace host with path only",
			originalURI: "/api/test?param=value",
			targetURL:   "http://localhost:8080",
			replaceHost: true,
			expected:    "http://localhost:8080/api/test?param=value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &ReplayConfig{ReplaceHost: tt.replaceHost}
			replayer.config = config

			// Parse target URL
			targetURL, err := url.Parse(tt.targetURL)
			require.NoError(t, err)

			result := replayer.buildReplayURI(tt.originalURI, targetURL)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestReplayStats(t *testing.T) {
	logger := zaptest.NewLogger(t)
	storage := NewMemoryStorage()
	replayer := NewReplayer(storage, logger)

	// Test initial stats
	stats := replayer.GetStats()
	assert.Equal(t, int64(0), stats.TotalRequests)
	assert.Equal(t, int64(0), stats.SuccessRequests)
	assert.Equal(t, int64(0), stats.FailedRequests)
	assert.Equal(t, time.Duration(0), stats.AverageLatency)

	// Test stat updates
	replayer.incrementTotalRequests()
	replayer.incrementSuccessRequests()
	replayer.updateAverageLatency(100 * time.Millisecond)

	stats = replayer.GetStats()
	assert.Equal(t, int64(1), stats.TotalRequests)
	assert.Equal(t, int64(1), stats.SuccessRequests)
	assert.Equal(t, int64(0), stats.FailedRequests)
	assert.Equal(t, 100*time.Millisecond, stats.AverageLatency)

	// Test multiple latency updates for average calculation
	replayer.incrementTotalRequests()
	replayer.incrementSuccessRequests()
	replayer.updateAverageLatency(200 * time.Millisecond)

	stats = replayer.GetStats()
	assert.Equal(t, int64(2), stats.TotalRequests)
	assert.Equal(t, int64(2), stats.SuccessRequests)
	// Average should be (100 + 200) / 2 = 150ms
	assert.Equal(t, 150*time.Millisecond, stats.AverageLatency)
}