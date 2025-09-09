package recorder

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap/zaptest"
	"vanta/pkg/config"
)

func TestNewDefaultRecordingEngine(t *testing.T) {
	logger := zaptest.NewLogger(t)
	storage := NewMemoryStorage()

	engine := NewDefaultRecordingEngine(storage, logger)

	assert.NotNil(t, engine)
	assert.False(t, engine.IsEnabled())
	assert.NotNil(t, engine.GetStats())
}

func TestRecordingEngine_StartStop(t *testing.T) {
	logger := zaptest.NewLogger(t)
	storage := NewMemoryStorage()
	engine := NewDefaultRecordingEngine(storage, logger)

	config := &config.RecordingConfig{
		Enabled:       true,
		MaxRecordings: 100,
		MaxBodySize:   1024 * 1024,
		Storage: config.StorageConfig{
			Type:      "memory",
			Directory: "/tmp/test",
			Format:    "json",
		},
		Filters:        []config.RecordingFilter{},
		IncludeHeaders: []string{},
		ExcludeHeaders: []string{"authorization"},
	}

	// Test Start
	err := engine.Start(config)
	require.NoError(t, err)
	assert.True(t, engine.IsEnabled())

	// Test Stop
	err = engine.Stop()
	require.NoError(t, err)
	assert.False(t, engine.IsEnabled())
}

func TestRecordingEngine_Record(t *testing.T) {
	logger := zaptest.NewLogger(t)
	storage := NewMemoryStorage()
	engine := NewDefaultRecordingEngine(storage, logger)

	config := &config.RecordingConfig{
		Enabled:       true,
		MaxRecordings: 100,
		MaxBodySize:   1024 * 1024,
		Storage: config.StorageConfig{
			Type:   "memory",
			Format: "json",
		},
		Filters:        []config.RecordingFilter{},
		IncludeHeaders: []string{},
		ExcludeHeaders: []string{},
	}

	err := engine.Start(config)
	require.NoError(t, err)

	// Create a mock FastHTTP context
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("http://example.com/api/test")
	ctx.Request.Header.SetMethod("GET")
	ctx.Request.Header.Set("User-Agent", "test-agent")
	ctx.Response.SetStatusCode(200)
	ctx.Response.SetBody([]byte(`{"message": "test"}`))

	// Set user value for request ID
	ctx.SetUserValue("request_id", "test-request-id")

	responseBody := []byte(`{"message": "test"}`)
	duration := 100 * time.Millisecond

	// Record the request
	err = engine.Record(ctx, responseBody, duration)
	require.NoError(t, err)

	// Check stats
	stats := engine.GetStats()
	assert.Equal(t, int64(1), stats.TotalRequests)
	assert.Equal(t, int64(1), stats.RecordedRequests)
	assert.Equal(t, int64(0), stats.FilteredRequests)
	assert.Equal(t, int64(0), stats.Errors)
}

func TestRecordingEngine_RecordWithFilters(t *testing.T) {
	logger := zaptest.NewLogger(t)
	storage := NewMemoryStorage()
	engine := NewDefaultRecordingEngine(storage, logger)

	// Configure with method filter (only GET requests)
	config := &config.RecordingConfig{
		Enabled:       true,
		MaxRecordings: 100,
		MaxBodySize:   1024 * 1024,
		Storage: config.StorageConfig{
			Type:   "memory",
			Format: "json",
		},
		Filters: []config.RecordingFilter{
			{
				Type:   "method",
				Values: []string{"GET"},
				Negate: false,
			},
		},
		IncludeHeaders: []string{},
		ExcludeHeaders: []string{},
	}

	err := engine.Start(config)
	require.NoError(t, err)

	// Test GET request (should be recorded)
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("http://example.com/api/test")
	ctx.Request.Header.SetMethod("GET")
	ctx.Response.SetStatusCode(200)
	ctx.Response.SetBody([]byte(`{"message": "test"}`))

	err = engine.Record(ctx, []byte(`{"message": "test"}`), 100*time.Millisecond)
	require.NoError(t, err)

	// Test POST request (should be filtered out)
	ctx2 := &fasthttp.RequestCtx{}
	ctx2.Request.SetRequestURI("http://example.com/api/test")
	ctx2.Request.Header.SetMethod("POST")
	ctx2.Response.SetStatusCode(201)
	ctx2.Response.SetBody([]byte(`{"created": true}`))

	err = engine.Record(ctx2, []byte(`{"created": true}`), 150*time.Millisecond)
	require.NoError(t, err)

	// Check stats
	stats := engine.GetStats()
	assert.Equal(t, int64(2), stats.TotalRequests)
	assert.Equal(t, int64(1), stats.RecordedRequests)  // Only GET recorded
	assert.Equal(t, int64(1), stats.FilteredRequests)  // POST filtered out
	assert.Equal(t, int64(0), stats.Errors)
}

func TestRecordingEngine_RecordWithBodySizeLimit(t *testing.T) {
	logger := zaptest.NewLogger(t)
	storage := NewMemoryStorage()
	engine := NewDefaultRecordingEngine(storage, logger)

	// Configure with small body size limit
	config := &config.RecordingConfig{
		Enabled:       true,
		MaxRecordings: 100,
		MaxBodySize:   10, // Very small limit
		Storage: config.StorageConfig{
			Type:   "memory",
			Format: "json",
		},
		Filters:        []config.RecordingFilter{},
		IncludeHeaders: []string{},
		ExcludeHeaders: []string{},
	}

	err := engine.Start(config)
	require.NoError(t, err)

	// Create request with large body
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("http://example.com/api/test")
	ctx.Request.Header.SetMethod("POST")
	ctx.Request.SetBody([]byte("This is a very long request body that exceeds the limit"))
	ctx.Response.SetStatusCode(200)
	ctx.Response.SetBody([]byte(`{"message": "test"}`))

	responseBody := []byte(`{"message": "test"}`)
	duration := 100 * time.Millisecond

	// Record the request (should be filtered due to body size)
	err = engine.Record(ctx, responseBody, duration)
	require.NoError(t, err)

	// Check stats
	stats := engine.GetStats()
	assert.Equal(t, int64(1), stats.TotalRequests)
	assert.Equal(t, int64(0), stats.RecordedRequests)  // Filtered due to body size
	assert.Equal(t, int64(1), stats.FilteredRequests)
	assert.Equal(t, int64(0), stats.Errors)
}

func TestFilters(t *testing.T) {
	tests := []struct {
		name     string
		filter   Filter
		recording *Recording
		expected bool
	}{
		{
			name: "method filter - match",
			filter: NewMethodFilter([]string{"GET", "POST"}, false),
			recording: &Recording{
				Request: RecordedRequest{Method: "GET"},
			},
			expected: true,
		},
		{
			name: "method filter - no match",
			filter: NewMethodFilter([]string{"GET", "POST"}, false),
			recording: &Recording{
				Request: RecordedRequest{Method: "PUT"},
			},
			expected: false,
		},
		{
			name: "method filter - negate match",
			filter: NewMethodFilter([]string{"GET", "POST"}, true),
			recording: &Recording{
				Request: RecordedRequest{Method: "PUT"},
			},
			expected: true,
		},
		{
			name: "endpoint filter - match",
			filter: NewEndpointFilter([]string{"/api/users"}, false),
			recording: &Recording{
				Request: RecordedRequest{URI: "http://example.com/api/users"},
			},
			expected: true,
		},
		{
			name: "endpoint filter - wildcard match",
			filter: NewEndpointFilter([]string{"/api/*"}, false),
			recording: &Recording{
				Request: RecordedRequest{URI: "http://example.com/api/users/123"},
			},
			expected: true,
		},
		{
			name: "status filter - match",
			filter: NewStatusFilter([]int{200, 201}, false),
			recording: &Recording{
				Response: RecordedResponse{StatusCode: 200},
			},
			expected: true,
		},
		{
			name: "status filter - no match",
			filter: NewStatusFilter([]int{200, 201}, false),
			recording: &Recording{
				Response: RecordedResponse{StatusCode: 404},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.filter.Apply(tt.recording)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNewFilter(t *testing.T) {
	tests := []struct {
		name    string
		config  config.RecordingFilter
		wantErr bool
	}{
		{
			name: "method filter",
			config: config.RecordingFilter{
				Type:   "method",
				Values: []string{"GET", "POST"},
				Negate: false,
			},
			wantErr: false,
		},
		{
			name: "endpoint filter",
			config: config.RecordingFilter{
				Type:   "endpoint",
				Values: []string{"/api/users"},
				Negate: false,
			},
			wantErr: false,
		},
		{
			name: "status filter",
			config: config.RecordingFilter{
				Type:   "status",
				Values: []string{"200", "201"},
				Negate: false,
			},
			wantErr: false,
		},
		{
			name: "status filter - invalid status code",
			config: config.RecordingFilter{
				Type:   "status",
				Values: []string{"invalid"},
				Negate: false,
			},
			wantErr: true,
		},
		{
			name: "unknown filter type",
			config: config.RecordingFilter{
				Type:   "unknown",
				Values: []string{"value"},
				Negate: false,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter, err := NewFilter(tt.config)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, filter)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, filter)
			}
		})
	}
}

func TestRecordingEngine_HeaderFiltering(t *testing.T) {
	logger := zaptest.NewLogger(t)
	storage := NewMemoryStorage()
	engine := NewDefaultRecordingEngine(storage, logger)

	config := &config.RecordingConfig{
		Enabled:       true,
		MaxRecordings: 100,
		MaxBodySize:   1024 * 1024,
		Storage: config.StorageConfig{
			Type:   "memory",
			Format: "json",
		},
		Filters:        []config.RecordingFilter{},
		IncludeHeaders: []string{"content-type", "user-agent"},
		ExcludeHeaders: []string{"authorization"},
	}

	err := engine.Start(config)
	require.NoError(t, err)

	// Create mock context with various headers
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("http://example.com/api/test")
	ctx.Request.Header.SetMethod("GET")
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Request.Header.Set("User-Agent", "test-agent")
	ctx.Request.Header.Set("Authorization", "Bearer token")
	ctx.Request.Header.Set("X-Custom-Header", "custom-value")
	ctx.Response.SetStatusCode(200)

	err = engine.Record(ctx, []byte(`{}`), 100*time.Millisecond)
	require.NoError(t, err)

	// Verify recording was created and headers were filtered
	recordings, err := storage.List(ListFilter{})
	require.NoError(t, err)
	require.Len(t, recordings, 1)

	recording := recordings[0]
	
	// Should include these headers
	assert.Contains(t, recording.Request.Headers, "content-type")
	assert.Contains(t, recording.Request.Headers, "user-agent")
	
	// Should not include excluded header
	assert.NotContains(t, recording.Request.Headers, "authorization")
	
	// Should not include non-specified header when include list is provided
	assert.NotContains(t, recording.Request.Headers, "x-custom-header")
}