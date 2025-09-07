package api

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"vanta/pkg/config"
)

// Test helpers
func createTestRequestCtx(method, path string, body []byte) *fasthttp.RequestCtx {
	var ctx fasthttp.RequestCtx
	var req fasthttp.Request
	req.SetRequestURI(path)
	req.Header.SetMethod(method)
	if body != nil {
		req.SetBody(body)
	}
	ctx.Init(&req, nil, nil)
	return &ctx
}

func createTestLogger() (*zap.Logger, *observer.ObservedLogs) {
	observedZapCore, observedLogs := observer.New(zap.DebugLevel)
	observedLogger := zap.New(observedZapCore)
	return observedLogger, observedLogs
}

func createTestConfig() *config.Config {
	return &config.Config{
		Logging: config.LoggingConfig{
			Level:     "info",
			Format:    "json",
			Output:    "stdout",
			AddCaller: true,
		},
		Metrics: config.MetricsConfig{
			Enabled:    true,
			Port:       9090,
			Path:       "/metrics",
			Prometheus: false,
		},
		Middleware: config.MiddlewareConfig{
			CORS: config.CORSConfig{
				Enabled:          true,
				AllowOrigins:     []string{"*"},
				AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
				AllowHeaders:     []string{"Content-Type", "Authorization"},
				AllowCredentials: false,
				MaxAge:           3600,
			},
			Timeout: config.TimeoutConfig{
				Enabled:  true,
				Duration: 5 * time.Second,
			},
			Recovery: config.RecoveryConfig{
				Enabled:    true,
				PrintStack: false,
				LogStack:   true,
			},
			RequestID: true,
		},
	}
}

// Test handler that can simulate different behaviors
type testHandler struct {
	statusCode int
	response   []byte
	delay      time.Duration
	panic      bool
	panicMsg   interface{}
}

func (h *testHandler) handle(ctx *fasthttp.RequestCtx) {
	if h.delay > 0 {
		time.Sleep(h.delay)
	}

	if h.panic {
		panic(h.panicMsg)
	}

	if h.statusCode > 0 {
		ctx.SetStatusCode(h.statusCode)
	} else {
		ctx.SetStatusCode(fasthttp.StatusOK)
	}

	if h.response != nil {
		ctx.SetBody(h.response)
	}
}

// Stack Tests
func TestNewStack(t *testing.T) {
	t.Run("empty stack", func(t *testing.T) {
		stack := NewStack()
		require.NotNil(t, stack)
		assert.Empty(t, stack.middlewares)
	})

	t.Run("stack with initial middleware", func(t *testing.T) {
		middleware1 := func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
			return func(ctx *fasthttp.RequestCtx) {
				ctx.Response.Header.Set("X-Test-1", "applied")
				next(ctx)
			}
		}
		middleware2 := func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
			return func(ctx *fasthttp.RequestCtx) {
				ctx.Response.Header.Set("X-Test-2", "applied")
				next(ctx)
			}
		}

		stack := NewStack(middleware1, middleware2)
		require.NotNil(t, stack)
		assert.Len(t, stack.middlewares, 2)

		// Test that the middlewares were actually copied and work
		handler := &testHandler{statusCode: fasthttp.StatusOK}
		wrappedHandler := stack.Apply(handler.handle)
		ctx := createTestRequestCtx("GET", "/test", nil)

		wrappedHandler(ctx)

		assert.Equal(t, "applied", string(ctx.Response.Header.Peek("X-Test-1")))
		assert.Equal(t, "applied", string(ctx.Response.Header.Peek("X-Test-2")))
	})
}

func TestStack_Use(t *testing.T) {
	stack := NewStack()
	
	middleware1 := func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
		return next
	}
	middleware2 := func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
		return next
	}

	// Test adding first middleware
	result := stack.Use(middleware1)
	assert.Equal(t, stack, result) // Should return self for chaining
	assert.Len(t, stack.middlewares, 1)

	// Test adding second middleware
	stack.Use(middleware2)
	assert.Len(t, stack.middlewares, 2)
}

func TestStack_Apply_EmptyStack(t *testing.T) {
	stack := NewStack()
	handler := &testHandler{statusCode: fasthttp.StatusOK, response: []byte("test")}

	wrappedHandler := stack.Apply(handler.handle)
	ctx := createTestRequestCtx("GET", "/test", nil)

	wrappedHandler(ctx)

	assert.Equal(t, fasthttp.StatusOK, ctx.Response.StatusCode())
	assert.Equal(t, "test", string(ctx.Response.Body()))
}

func TestStack_Apply_MultipleMiddleware(t *testing.T) {
	stack := NewStack()

	// Middleware that adds a header
	middleware1 := func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
		return func(ctx *fasthttp.RequestCtx) {
			ctx.Response.Header.Set("X-Middleware-1", "applied")
			next(ctx)
		}
	}

	// Middleware that adds another header
	middleware2 := func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
		return func(ctx *fasthttp.RequestCtx) {
			ctx.Response.Header.Set("X-Middleware-2", "applied")
			next(ctx)
		}
	}

	stack.Use(middleware1).Use(middleware2)
	
	handler := &testHandler{statusCode: fasthttp.StatusOK, response: []byte("test")}
	wrappedHandler := stack.Apply(handler.handle)
	ctx := createTestRequestCtx("GET", "/test", nil)

	wrappedHandler(ctx)

	assert.Equal(t, fasthttp.StatusOK, ctx.Response.StatusCode())
	assert.Equal(t, "applied", string(ctx.Response.Header.Peek("X-Middleware-1")))
	assert.Equal(t, "applied", string(ctx.Response.Header.Peek("X-Middleware-2")))
}

// RequestID Middleware Tests
func TestRequestID(t *testing.T) {
	t.Run("enabled", func(t *testing.T) {
		middleware := RequestID(true)
		handler := &testHandler{statusCode: fasthttp.StatusOK}
		wrappedHandler := middleware(handler.handle)
		ctx := createTestRequestCtx("GET", "/test", nil)

		wrappedHandler(ctx)

		// Check request ID header
		requestIDHeader := string(ctx.Response.Header.Peek("X-Request-ID"))
		assert.NotEmpty(t, requestIDHeader)

		// Validate UUID format
		_, err := uuid.Parse(requestIDHeader)
		assert.NoError(t, err)

		// Check user value
		requestIDValue := ctx.UserValue("request_id")
		require.NotNil(t, requestIDValue)
		assert.Equal(t, requestIDHeader, requestIDValue.(string))
	})

	t.Run("disabled", func(t *testing.T) {
		middleware := RequestID(false)
		handler := &testHandler{statusCode: fasthttp.StatusOK}
		wrappedHandler := middleware(handler.handle)
		ctx := createTestRequestCtx("GET", "/test", nil)

		wrappedHandler(ctx)

		// Check no request ID header
		requestIDHeader := string(ctx.Response.Header.Peek("X-Request-ID"))
		assert.Empty(t, requestIDHeader)

		// Check no user value
		requestIDValue := ctx.UserValue("request_id")
		assert.Nil(t, requestIDValue)
	})

	t.Run("unique request IDs", func(t *testing.T) {
		middleware := RequestID(true)
		handler := &testHandler{statusCode: fasthttp.StatusOK}
		wrappedHandler := middleware(handler.handle)

		requestIDs := make(map[string]bool)
		for i := 0; i < 100; i++ {
			ctx := createTestRequestCtx("GET", "/test", nil)
			wrappedHandler(ctx)

			requestID := string(ctx.Response.Header.Peek("X-Request-ID"))
			assert.NotEmpty(t, requestID)
			assert.False(t, requestIDs[requestID], "Request ID should be unique")
			requestIDs[requestID] = true
		}
	})
}

// Logger Middleware Tests
func TestLogger(t *testing.T) {
	t.Run("basic logging", func(t *testing.T) {
		logger, logs := createTestLogger()
		cfg := &config.LoggingConfig{Level: "info"}
		middleware := Logger(logger, cfg)
		handler := &testHandler{statusCode: fasthttp.StatusOK, response: []byte("test response")}
		wrappedHandler := middleware(handler.handle)
		ctx := createTestRequestCtx("POST", "/api/test", []byte("test body"))

		wrappedHandler(ctx)

		assert.Equal(t, 1, logs.Len())
		logEntry := logs.All()[0]
		assert.Equal(t, zap.InfoLevel, logEntry.Level)
		assert.Equal(t, "HTTP request", logEntry.Message)

		// Check log fields
		fields := logEntry.ContextMap()
		assert.Equal(t, "POST", fields["method"])
		assert.Equal(t, "/api/test", fields["path"])
		assert.Equal(t, int64(200), fields["status"])
		assert.Contains(t, fields, "duration")
		assert.Contains(t, fields, "remote_addr")
		assert.Equal(t, int64(9), fields["request_size"])  // len("test body")
		assert.Equal(t, int64(13), fields["response_size"]) // len("test response")
	})

	t.Run("with request ID", func(t *testing.T) {
		logger, logs := createTestLogger()
		cfg := &config.LoggingConfig{Level: "info"}
		middleware := Logger(logger, cfg)
		handler := &testHandler{statusCode: fasthttp.StatusOK}
		wrappedHandler := middleware(handler.handle)
		ctx := createTestRequestCtx("GET", "/test", nil)
		ctx.SetUserValue("request_id", "test-request-id")

		wrappedHandler(ctx)

		assert.Equal(t, 1, logs.Len())
		logEntry := logs.All()[0]
		fields := logEntry.ContextMap()
		assert.Equal(t, "test-request-id", fields["request_id"])
	})

	t.Run("different status code levels", func(t *testing.T) {
		testCases := []struct {
			statusCode    int
			expectedLevel zapcore.Level
		}{
			{200, zap.InfoLevel},
			{300, zap.InfoLevel},
			{400, zap.WarnLevel},
			{404, zap.WarnLevel},
			{500, zap.ErrorLevel},
			{503, zap.ErrorLevel},
		}

		for _, tc := range testCases {
			t.Run(fmt.Sprintf("status_%d", tc.statusCode), func(t *testing.T) {
				logger, logs := createTestLogger()
				cfg := &config.LoggingConfig{Level: "info"}
				middleware := Logger(logger, cfg)
				handler := &testHandler{statusCode: tc.statusCode}
				wrappedHandler := middleware(handler.handle)
				ctx := createTestRequestCtx("GET", "/test", nil)

				wrappedHandler(ctx)

				assert.Equal(t, 1, logs.Len())
				logEntry := logs.All()[0]
				assert.Equal(t, tc.expectedLevel, logEntry.Level)
			})
		}
	})

	t.Run("timing measurement", func(t *testing.T) {
		logger, logs := createTestLogger()
		cfg := &config.LoggingConfig{Level: "info"}
		middleware := Logger(logger, cfg)
		handler := &testHandler{statusCode: fasthttp.StatusOK, delay: 50 * time.Millisecond}
		wrappedHandler := middleware(handler.handle)
		ctx := createTestRequestCtx("GET", "/test", nil)

		wrappedHandler(ctx)

		assert.Equal(t, 1, logs.Len())
		logEntry := logs.All()[0]
		fields := logEntry.ContextMap()
		duration, ok := fields["duration"]
		require.True(t, ok)
		assert.Greater(t, duration.(time.Duration), 40*time.Millisecond)
	})
}

// Recovery Middleware Tests
func TestRecovery_NoPanic(t *testing.T) {
	logger, logs := createTestLogger()
	cfg := &config.RecoveryConfig{Enabled: true, PrintStack: false, LogStack: true}
	middleware := Recovery(logger, cfg)
	handler := &testHandler{statusCode: fasthttp.StatusOK, response: []byte("normal response")}
	wrappedHandler := middleware(handler.handle)
	ctx := createTestRequestCtx("GET", "/test", nil)

	wrappedHandler(ctx)

	assert.Equal(t, fasthttp.StatusOK, ctx.Response.StatusCode())
	assert.Equal(t, "normal response", string(ctx.Response.Body()))
	assert.Equal(t, 0, logs.Len()) // No panic logs
}

func TestRecovery_WithPanic(t *testing.T) {
	logger, logs := createTestLogger()
	cfg := &config.RecoveryConfig{Enabled: true, PrintStack: false, LogStack: true}
	middleware := Recovery(logger, cfg)
	handler := &testHandler{panic: true, panicMsg: "test panic"}
	wrappedHandler := middleware(handler.handle)
	ctx := createTestRequestCtx("GET", "/test", nil)

	// Should not panic, should recover
	assert.NotPanics(t, func() {
		wrappedHandler(ctx)
	})

	// Check response
	assert.Equal(t, fasthttp.StatusInternalServerError, ctx.Response.StatusCode())
	assert.Equal(t, "application/json", string(ctx.Response.Header.ContentType()))
	
	var errorResponse map[string]string
	err := json.Unmarshal(ctx.Response.Body(), &errorResponse)
	require.NoError(t, err)
	assert.Equal(t, "Internal server error", errorResponse["error"])

	// Check logs
	assert.Equal(t, 1, logs.Len())
	logEntry := logs.All()[0]
	assert.Equal(t, zap.ErrorLevel, logEntry.Level)
	assert.Equal(t, "Panic recovered", logEntry.Message)

	fields := logEntry.ContextMap()
	assert.Equal(t, "test panic", fields["panic"])
	assert.Equal(t, "GET", fields["method"])
	assert.Equal(t, "/test", fields["path"])
	assert.Contains(t, fields, "stack_trace")
}

func TestRecovery_WithRequestID(t *testing.T) {
	logger, logs := createTestLogger()
	cfg := &config.RecoveryConfig{Enabled: true, PrintStack: false, LogStack: true}
	middleware := Recovery(logger, cfg)
	handler := &testHandler{panic: true, panicMsg: "test panic"}
	wrappedHandler := middleware(handler.handle)
	ctx := createTestRequestCtx("GET", "/test", nil)
	ctx.SetUserValue("request_id", "test-request-id")

	wrappedHandler(ctx)

	// Check response includes request ID
	var errorResponse map[string]string
	err := json.Unmarshal(ctx.Response.Body(), &errorResponse)
	require.NoError(t, err)
	assert.Equal(t, "test-request-id", errorResponse["request_id"])

	// Check log includes request ID
	logEntry := logs.All()[0]
	fields := logEntry.ContextMap()
	assert.Equal(t, "test-request-id", fields["request_id"])
}

func TestRecovery_Disabled(t *testing.T) {
	logger, _ := createTestLogger()
	cfg := &config.RecoveryConfig{Enabled: false}
	middleware := Recovery(logger, cfg)
	handler := &testHandler{panic: true, panicMsg: "test panic"}
	wrappedHandler := middleware(handler.handle)
	ctx := createTestRequestCtx("GET", "/test", nil)

	// Should panic since recovery is disabled
	assert.Panics(t, func() {
		wrappedHandler(ctx)
	})
}

func TestRecovery_Configuration(t *testing.T) {
	t.Run("log stack enabled", func(t *testing.T) {
		logger, logs := createTestLogger()
		cfg := &config.RecoveryConfig{Enabled: true, PrintStack: false, LogStack: true}
		middleware := Recovery(logger, cfg)
		handler := &testHandler{panic: true, panicMsg: "test panic"}
		wrappedHandler := middleware(handler.handle)
		ctx := createTestRequestCtx("GET", "/test", nil)

		wrappedHandler(ctx)

		logEntry := logs.All()[0]
		fields := logEntry.ContextMap()
		assert.Contains(t, fields, "stack_trace")
	})

	t.Run("log stack disabled", func(t *testing.T) {
		logger, logs := createTestLogger()
		cfg := &config.RecoveryConfig{Enabled: true, PrintStack: false, LogStack: false}
		middleware := Recovery(logger, cfg)
		handler := &testHandler{panic: true, panicMsg: "test panic"}
		wrappedHandler := middleware(handler.handle)
		ctx := createTestRequestCtx("GET", "/test", nil)

		wrappedHandler(ctx)

		logEntry := logs.All()[0]
		fields := logEntry.ContextMap()
		assert.NotContains(t, fields, "stack_trace")
	})
}

// CORS Middleware Tests
func TestCORS_Disabled(t *testing.T) {
	cfg := &config.CORSConfig{Enabled: false}
	middleware := CORS(cfg)
	handler := &testHandler{statusCode: fasthttp.StatusOK}
	wrappedHandler := middleware(handler.handle)
	ctx := createTestRequestCtx("GET", "/test", nil)
	ctx.Request.Header.Set("Origin", "https://example.com")

	wrappedHandler(ctx)

	assert.Equal(t, fasthttp.StatusOK, ctx.Response.StatusCode())
	assert.Empty(t, string(ctx.Response.Header.Peek("Access-Control-Allow-Origin")))
}

func TestCORS_AllowOrigins(t *testing.T) {
	t.Run("allowed origin", func(t *testing.T) {
		cfg := &config.CORSConfig{
			Enabled:      true,
			AllowOrigins: []string{"https://example.com", "https://test.com"},
		}
		middleware := CORS(cfg)
		handler := &testHandler{statusCode: fasthttp.StatusOK}
		wrappedHandler := middleware(handler.handle)
		ctx := createTestRequestCtx("GET", "/test", nil)
		ctx.Request.Header.Set("Origin", "https://example.com")

		wrappedHandler(ctx)

		assert.Equal(t, "https://example.com", string(ctx.Response.Header.Peek("Access-Control-Allow-Origin")))
	})

	t.Run("disallowed origin", func(t *testing.T) {
		cfg := &config.CORSConfig{
			Enabled:      true,
			AllowOrigins: []string{"https://example.com"},
		}
		middleware := CORS(cfg)
		handler := &testHandler{statusCode: fasthttp.StatusOK}
		wrappedHandler := middleware(handler.handle)
		ctx := createTestRequestCtx("GET", "/test", nil)
		ctx.Request.Header.Set("Origin", "https://malicious.com")

		wrappedHandler(ctx)

		assert.Empty(t, string(ctx.Response.Header.Peek("Access-Control-Allow-Origin")))
	})
}

func TestCORS_WildcardOrigin(t *testing.T) {
	cfg := &config.CORSConfig{
		Enabled:      true,
		AllowOrigins: []string{"*"},
	}
	middleware := CORS(cfg)
	handler := &testHandler{statusCode: fasthttp.StatusOK}
	wrappedHandler := middleware(handler.handle)
	ctx := createTestRequestCtx("GET", "/test", nil)
	ctx.Request.Header.Set("Origin", "https://any-domain.com")

	wrappedHandler(ctx)

	assert.Equal(t, "*", string(ctx.Response.Header.Peek("Access-Control-Allow-Origin")))
}

func TestCORS_PreflightRequest(t *testing.T) {
	cfg := &config.CORSConfig{
		Enabled:          true,
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE"},
		AllowHeaders:     []string{"Content-Type", "Authorization"},
		AllowCredentials: true,
		MaxAge:           3600,
	}
	middleware := CORS(cfg)
	handler := &testHandler{statusCode: fasthttp.StatusOK}
	wrappedHandler := middleware(handler.handle)
	ctx := createTestRequestCtx("OPTIONS", "/test", nil)
	ctx.Request.Header.Set("Origin", "https://example.com")

	wrappedHandler(ctx)

	assert.Equal(t, fasthttp.StatusNoContent, ctx.Response.StatusCode())
	assert.Equal(t, "*", string(ctx.Response.Header.Peek("Access-Control-Allow-Origin")))
	assert.Equal(t, "GET, POST, PUT, DELETE", string(ctx.Response.Header.Peek("Access-Control-Allow-Methods")))
	assert.Equal(t, "Content-Type, Authorization", string(ctx.Response.Header.Peek("Access-Control-Allow-Headers")))
	assert.Equal(t, "true", string(ctx.Response.Header.Peek("Access-Control-Allow-Credentials")))
	assert.Equal(t, "3600", string(ctx.Response.Header.Peek("Access-Control-Max-Age")))
}

func TestCORS_Headers(t *testing.T) {
	cfg := &config.CORSConfig{
		Enabled:          true,
		AllowOrigins:     []string{"https://example.com"},
		AllowMethods:     []string{"GET", "POST"},
		AllowHeaders:     []string{"Content-Type"},
		AllowCredentials: false,
		MaxAge:           1800,
	}
	middleware := CORS(cfg)
	handler := &testHandler{statusCode: fasthttp.StatusOK}
	wrappedHandler := middleware(handler.handle)
	ctx := createTestRequestCtx("GET", "/test", nil)
	ctx.Request.Header.Set("Origin", "https://example.com")

	wrappedHandler(ctx)

	assert.Equal(t, "https://example.com", string(ctx.Response.Header.Peek("Access-Control-Allow-Origin")))
	assert.Equal(t, "GET, POST", string(ctx.Response.Header.Peek("Access-Control-Allow-Methods")))
	assert.Equal(t, "Content-Type", string(ctx.Response.Header.Peek("Access-Control-Allow-Headers")))
	assert.Empty(t, string(ctx.Response.Header.Peek("Access-Control-Allow-Credentials")))
	assert.Equal(t, "1800", string(ctx.Response.Header.Peek("Access-Control-Max-Age")))
}

// Timeout Middleware Tests
func TestTimeout_Disabled(t *testing.T) {
	cfg := &config.TimeoutConfig{Enabled: false}
	middleware := Timeout(cfg)
	handler := &testHandler{statusCode: fasthttp.StatusOK, delay: 100 * time.Millisecond}
	wrappedHandler := middleware(handler.handle)
	ctx := createTestRequestCtx("GET", "/test", nil)

	start := time.Now()
	wrappedHandler(ctx)
	duration := time.Since(start)

	assert.Equal(t, fasthttp.StatusOK, ctx.Response.StatusCode())
	assert.Greater(t, duration, 90*time.Millisecond) // Should complete normally
}

func TestTimeout_WithinLimit(t *testing.T) {
	cfg := &config.TimeoutConfig{Enabled: true, Duration: 200 * time.Millisecond}
	middleware := Timeout(cfg)
	handler := &testHandler{statusCode: fasthttp.StatusOK, delay: 50 * time.Millisecond}
	wrappedHandler := middleware(handler.handle)
	ctx := createTestRequestCtx("GET", "/test", nil)

	wrappedHandler(ctx)

	assert.Equal(t, fasthttp.StatusOK, ctx.Response.StatusCode())
}

func TestTimeout_Exceeded(t *testing.T) {
	cfg := &config.TimeoutConfig{Enabled: true, Duration: 50 * time.Millisecond}
	middleware := Timeout(cfg)
	handler := &testHandler{statusCode: fasthttp.StatusOK, delay: 200 * time.Millisecond}
	wrappedHandler := middleware(handler.handle)
	ctx := createTestRequestCtx("GET", "/test", nil)

	start := time.Now()
	wrappedHandler(ctx)
	duration := time.Since(start)

	assert.Equal(t, fasthttp.StatusRequestTimeout, ctx.Response.StatusCode())
	assert.Equal(t, "application/json", string(ctx.Response.Header.ContentType()))
	assert.Less(t, duration, 100*time.Millisecond) // Should timeout quickly

	var errorResponse map[string]string
	err := json.Unmarshal(ctx.Response.Body(), &errorResponse)
	require.NoError(t, err)
	assert.Equal(t, "Request timeout", errorResponse["error"])
	assert.Equal(t, "50ms", errorResponse["timeout"])
}

func TestTimeout_WithRequestID(t *testing.T) {
	cfg := &config.TimeoutConfig{Enabled: true, Duration: 50 * time.Millisecond}
	middleware := Timeout(cfg)
	handler := &testHandler{statusCode: fasthttp.StatusOK, delay: 200 * time.Millisecond}
	wrappedHandler := middleware(handler.handle)
	ctx := createTestRequestCtx("GET", "/test", nil)
	ctx.SetUserValue("request_id", "test-timeout-id")

	wrappedHandler(ctx)

	assert.Equal(t, fasthttp.StatusRequestTimeout, ctx.Response.StatusCode())

	var errorResponse map[string]string
	err := json.Unmarshal(ctx.Response.Body(), &errorResponse)
	require.NoError(t, err)
	assert.Equal(t, "test-timeout-id", errorResponse["request_id"])
}

// Metrics Middleware Tests
func TestMetrics_Disabled(t *testing.T) {
	cfg := &config.MetricsConfig{Enabled: false}
	collector := NewDefaultMetricsCollector()
	middleware := Metrics(cfg, collector)
	handler := &testHandler{statusCode: fasthttp.StatusOK}
	wrappedHandler := middleware(handler.handle)
	ctx := createTestRequestCtx("GET", "/test", nil)

	wrappedHandler(ctx)

	assert.Equal(t, fasthttp.StatusOK, ctx.Response.StatusCode())
	metrics := collector.GetMetrics()
	assert.Equal(t, int64(0), metrics["active_connections"])
	assert.Equal(t, 0, len(collector.requestCounter))
}

func TestMetrics_RequestCounting(t *testing.T) {
	cfg := &config.MetricsConfig{Enabled: true}
	collector := NewDefaultMetricsCollector()
	middleware := Metrics(cfg, collector)
	handler := &testHandler{statusCode: fasthttp.StatusOK}
	wrappedHandler := middleware(handler.handle)

	// Make multiple requests
	for i := 0; i < 3; i++ {
		ctx := createTestRequestCtx("GET", "/test", nil)
		wrappedHandler(ctx)
	}

	// Check metrics
	assert.Equal(t, int64(3), collector.requestCounter["GET_/test_200"])
	metrics := collector.GetMetrics()
	assert.Equal(t, int64(0), metrics["active_connections"]) // Should be 0 after completion
}

func TestMetrics_LatencyMeasurement(t *testing.T) {
	cfg := &config.MetricsConfig{Enabled: true}
	collector := NewDefaultMetricsCollector()
	middleware := Metrics(cfg, collector)
	handler := &testHandler{statusCode: fasthttp.StatusOK, delay: 50 * time.Millisecond}
	wrappedHandler := middleware(handler.handle)
	ctx := createTestRequestCtx("GET", "/test", nil)

	wrappedHandler(ctx)

	latencies := collector.latencyHistogram["GET_/test"]
	require.Len(t, latencies, 1)
	assert.Greater(t, latencies[0], 40*time.Millisecond)
}

func TestMetrics_StatusCodeTracking(t *testing.T) {
	cfg := &config.MetricsConfig{Enabled: true}
	collector := NewDefaultMetricsCollector()
	middleware := Metrics(cfg, collector)

	testCases := []int{200, 404, 500}
	for _, status := range testCases {
		handler := &testHandler{statusCode: status}
		wrappedHandler := middleware(handler.handle)
		ctx := createTestRequestCtx("GET", "/test", nil)
		wrappedHandler(ctx)
	}

	assert.Equal(t, int64(1), collector.requestCounter["GET_/test_200"])
	assert.Equal(t, int64(1), collector.requestCounter["GET_/test_404"])
	assert.Equal(t, int64(1), collector.requestCounter["GET_/test_500"])
}

func TestMetrics_NilCollector(t *testing.T) {
	cfg := &config.MetricsConfig{Enabled: true}
	middleware := Metrics(cfg, nil)
	handler := &testHandler{statusCode: fasthttp.StatusOK}
	wrappedHandler := middleware(handler.handle)
	ctx := createTestRequestCtx("GET", "/test", nil)

	// Should not panic with nil collector
	assert.NotPanics(t, func() {
		wrappedHandler(ctx)
	})
	assert.Equal(t, fasthttp.StatusOK, ctx.Response.StatusCode())
}

// DefaultMetricsCollector Tests
func TestDefaultMetricsCollector_IncrementRequests(t *testing.T) {
	collector := NewDefaultMetricsCollector()

	collector.IncRequestCounter("GET", "/api/users", 200)
	collector.IncRequestCounter("GET", "/api/users", 200)
	collector.IncRequestCounter("POST", "/api/users", 201)

	assert.Equal(t, int64(2), collector.requestCounter["GET_/api/users_200"])
	assert.Equal(t, int64(1), collector.requestCounter["POST_/api/users_201"])
}

func TestDefaultMetricsCollector_RecordLatency(t *testing.T) {
	collector := NewDefaultMetricsCollector()

	collector.ObserveLatency("GET", "/api/users", 100*time.Millisecond)
	collector.ObserveLatency("GET", "/api/users", 150*time.Millisecond)
	collector.ObserveLatency("POST", "/api/users", 200*time.Millisecond)

	getLatencies := collector.latencyHistogram["GET_/api/users"]
	assert.Len(t, getLatencies, 2)
	assert.Contains(t, getLatencies, 100*time.Millisecond)
	assert.Contains(t, getLatencies, 150*time.Millisecond)

	postLatencies := collector.latencyHistogram["POST_/api/users"]
	assert.Len(t, postLatencies, 1)
	assert.Contains(t, postLatencies, 200*time.Millisecond)
}

func TestDefaultMetricsCollector_ActiveConnections(t *testing.T) {
	collector := NewDefaultMetricsCollector()

	assert.Equal(t, int64(0), collector.activeConnections)

	collector.IncActiveConnections()
	collector.IncActiveConnections()
	assert.Equal(t, int64(2), collector.activeConnections)

	collector.DecActiveConnections()
	assert.Equal(t, int64(1), collector.activeConnections)
}

func TestDefaultMetricsCollector_GetMetrics(t *testing.T) {
	collector := NewDefaultMetricsCollector()

	collector.IncRequestCounter("GET", "/test", 200)
	collector.ObserveLatency("GET", "/test", 100*time.Millisecond)
	collector.IncActiveConnections()

	metrics := collector.GetMetrics()
	require.Contains(t, metrics, "request_counter")
	require.Contains(t, metrics, "active_connections")
	require.Contains(t, metrics, "latency_count")

	assert.Equal(t, int64(1), metrics["active_connections"])
	assert.Equal(t, 1, metrics["latency_count"])

	requestCounter := metrics["request_counter"].(map[string]int64)
	assert.Equal(t, int64(1), requestCounter["GET_/test_200"])
}

func TestDefaultMetricsCollector_ThreadSafety(t *testing.T) {
	collector := NewDefaultMetricsCollector()
	
	var wg sync.WaitGroup
	numGoroutines := 100
	numOperations := 100

	// Test concurrent request counting
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				collector.IncRequestCounter("GET", "/test", 200)
				collector.ObserveLatency("GET", "/test", time.Duration(j)*time.Millisecond)
				collector.IncActiveConnections()
				collector.DecActiveConnections()
			}
		}()
	}
	wg.Wait()

	// Verify results
	expectedRequests := int64(numGoroutines * numOperations)
	assert.Equal(t, expectedRequests, collector.requestCounter["GET_/test_200"])
	
	latencies := collector.latencyHistogram["GET_/test"]
	assert.Len(t, latencies, int(expectedRequests))
	
	assert.Equal(t, int64(0), collector.activeConnections)
}

// Integration Tests
func TestMiddlewareStack_Integration(t *testing.T) {
	// Create a complete middleware stack
	logger, logs := createTestLogger()
	cfg := createTestConfig()
	collector := NewDefaultMetricsCollector()

	stack := NewStack()
	stack.Use(RequestID(cfg.Middleware.RequestID))
	stack.Use(Logger(logger, &cfg.Logging))
	stack.Use(Recovery(logger, &cfg.Middleware.Recovery))
	stack.Use(CORS(&cfg.Middleware.CORS))
	stack.Use(Metrics(&cfg.Metrics, collector))

	handler := &testHandler{statusCode: fasthttp.StatusOK, response: []byte("integration test")}
	wrappedHandler := stack.Apply(handler.handle)
	ctx := createTestRequestCtx("POST", "/api/integration", []byte("test data"))
	ctx.Request.Header.Set("Origin", "https://example.com")

	wrappedHandler(ctx)

	// Verify response
	assert.Equal(t, fasthttp.StatusOK, ctx.Response.StatusCode())
	assert.Equal(t, "integration test", string(ctx.Response.Body()))

	// Verify RequestID
	requestID := string(ctx.Response.Header.Peek("X-Request-ID"))
	assert.NotEmpty(t, requestID)
	
	// Verify CORS
	assert.NotEmpty(t, string(ctx.Response.Header.Peek("Access-Control-Allow-Origin")))

	// Verify Logging
	assert.Equal(t, 1, logs.Len())
	logEntry := logs.All()[0]
	fields := logEntry.ContextMap()
	assert.Equal(t, requestID, fields["request_id"])

	// Verify Metrics
	assert.Equal(t, int64(1), collector.requestCounter["POST_/api/integration_200"])
	assert.Len(t, collector.latencyHistogram["POST_/api/integration"], 1)
}

func TestMiddlewareStack_PanicRecovery(t *testing.T) {
	logger, logs := createTestLogger()
	cfg := createTestConfig()

	stack := NewStack()
	stack.Use(RequestID(cfg.Middleware.RequestID))
	stack.Use(Logger(logger, &cfg.Logging))
	stack.Use(Recovery(logger, &cfg.Middleware.Recovery))

	handler := &testHandler{panic: true, panicMsg: "integration panic test"}
	wrappedHandler := stack.Apply(handler.handle)
	ctx := createTestRequestCtx("GET", "/panic", nil)

	assert.NotPanics(t, func() {
		wrappedHandler(ctx)
	})

	// Should have both recovery log and request log
	// The middleware stack execution order: Logger -> Recovery -> Handler (panic)
	// Recovery catches panic and sets 500 status, then returns to Logger which logs the 500 status
	assert.Equal(t, 2, logs.Len())
	
	// First log should be the panic recovery
	panicLog := logs.All()[0]
	assert.Equal(t, zap.ErrorLevel, panicLog.Level)
	assert.Equal(t, "Panic recovered", panicLog.Message)

	// Second log should be the request log (with 500 status)
	requestLog := logs.All()[1]
	assert.Equal(t, zap.ErrorLevel, requestLog.Level)
	assert.Equal(t, "HTTP request", requestLog.Message)
	fields := requestLog.ContextMap()
	assert.Equal(t, int64(500), fields["status"])
}

// Benchmark Tests
func BenchmarkLogger(b *testing.B) {
	logger := zap.NewNop() // No-op logger for benchmarking
	cfg := &config.LoggingConfig{}
	middleware := Logger(logger, cfg)
	handler := &testHandler{statusCode: fasthttp.StatusOK}
	wrappedHandler := middleware(handler.handle)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx := createTestRequestCtx("GET", "/benchmark", nil)
		wrappedHandler(ctx)
	}
}

func BenchmarkMetrics(b *testing.B) {
	cfg := &config.MetricsConfig{Enabled: true}
	collector := NewDefaultMetricsCollector()
	middleware := Metrics(cfg, collector)
	handler := &testHandler{statusCode: fasthttp.StatusOK}
	wrappedHandler := middleware(handler.handle)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx := createTestRequestCtx("GET", "/benchmark", nil)
		wrappedHandler(ctx)
	}
}

func BenchmarkFullStack(b *testing.B) {
	logger := zap.NewNop()
	cfg := createTestConfig()
	collector := NewDefaultMetricsCollector()

	stack := NewStack()
	stack.Use(RequestID(cfg.Middleware.RequestID))
	stack.Use(Logger(logger, &cfg.Logging))
	stack.Use(Recovery(logger, &cfg.Middleware.Recovery))
	stack.Use(CORS(&cfg.Middleware.CORS))
	stack.Use(Metrics(&cfg.Metrics, collector))

	handler := &testHandler{statusCode: fasthttp.StatusOK}
	wrappedHandler := stack.Apply(handler.handle)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx := createTestRequestCtx("GET", "/benchmark", nil)
		wrappedHandler(ctx)
	}
}