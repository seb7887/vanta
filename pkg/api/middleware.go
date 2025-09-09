package api

import (
	"context"
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
	"vanta/pkg/chaos"
	"vanta/pkg/config"
	"vanta/pkg/recorder"
)

// MiddlewareFunc is the type of function for FastHTTP middleware
type MiddlewareFunc func(next fasthttp.RequestHandler) fasthttp.RequestHandler

// Stack represents a stack of middleware
type Stack struct {
	middlewares []MiddlewareFunc
	mu          sync.RWMutex
}

// NewStack creates a new middleware stack with optional initial middlewares
func NewStack(middlewares ...MiddlewareFunc) *Stack {
	stack := &Stack{
		middlewares: make([]MiddlewareFunc, len(middlewares)),
	}
	copy(stack.middlewares, middlewares)
	return stack
}

// Use adds a middleware to the stack
func (s *Stack) Use(middleware MiddlewareFunc) *Stack {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.middlewares = append(s.middlewares, middleware)
	return s
}

// Apply applies all middleware in the stack to a handler
func (s *Stack) Apply(handler fasthttp.RequestHandler) fasthttp.RequestHandler {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	// Apply middlewares in reverse order to maintain the correct execution order
	result := handler
	for i := len(s.middlewares) - 1; i >= 0; i-- {
		result = s.middlewares[i](result)
	}
	return result
}

// RequestID middleware generates and injects unique request IDs
func RequestID(enabled bool) MiddlewareFunc {
	if !enabled {
		return func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
			return next
		}
	}

	return func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
		return func(ctx *fasthttp.RequestCtx) {
			// Generate unique request ID
			requestID := uuid.New().String()
			
			// Store in user values for access by other middleware/handlers
			ctx.SetUserValue("request_id", requestID)
			
			// Add to response header
			ctx.Response.Header.Set("X-Request-ID", requestID)
			
			next(ctx)
		}
	}
}

// Logger middleware provides request/response logging with zap integration
func Logger(logger *zap.Logger, loggingCfg *config.LoggingConfig) MiddlewareFunc {
	return func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
		return func(ctx *fasthttp.RequestCtx) {
			start := time.Now()
			
			// Execute next handler
			next(ctx)
			
			// Calculate duration
			duration := time.Since(start)
			
			// Get request ID if available
			requestID := ""
			if val := ctx.UserValue("request_id"); val != nil {
				requestID = val.(string)
			}
			
			// Prepare log fields
			fields := []zap.Field{
				zap.String("method", string(ctx.Method())),
				zap.String("path", string(ctx.Path())),
				zap.Int("status", ctx.Response.StatusCode()),
				zap.Duration("duration", duration),
				zap.String("remote_addr", ctx.RemoteAddr().String()),
				zap.String("user_agent", string(ctx.UserAgent())),
				zap.Int("request_size", len(ctx.Request.Body())),
				zap.Int("response_size", len(ctx.Response.Body())),
			}
			
			// Add request ID if available
			if requestID != "" {
				fields = append(fields, zap.String("request_id", requestID))
			}
			
			// Log based on status code
			status := ctx.Response.StatusCode()
			switch {
			case status >= 500:
				logger.Error("HTTP request", fields...)
			case status >= 400:
				logger.Warn("HTTP request", fields...)
			default:
				logger.Info("HTTP request", fields...)
			}
		}
	}
}

// Recovery middleware recovers from panics and logs them
func Recovery(logger *zap.Logger, recoveryCfg *config.RecoveryConfig) MiddlewareFunc {
	if !recoveryCfg.Enabled {
		return func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
			return next
		}
	}

	return func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
		return func(ctx *fasthttp.RequestCtx) {
			defer func() {
				if r := recover(); r != nil {
					// Get request ID if available
					requestID := ""
					if val := ctx.UserValue("request_id"); val != nil {
						requestID = val.(string)
					}
					
					// Get stack trace
					stack := make([]byte, 4096)
					length := runtime.Stack(stack, false)
					stackTrace := string(stack[:length])
					
					// Prepare log fields
					fields := []zap.Field{
						zap.Any("panic", r),
						zap.String("method", string(ctx.Method())),
						zap.String("path", string(ctx.Path())),
						zap.String("remote_addr", ctx.RemoteAddr().String()),
					}
					
					if requestID != "" {
						fields = append(fields, zap.String("request_id", requestID))
					}
					
					// Add stack trace to logs if configured
					if recoveryCfg.LogStack {
						fields = append(fields, zap.String("stack_trace", stackTrace))
					}
					
					// Log the panic
					logger.Error("Panic recovered", fields...)
					
					// Print stack trace if configured
					if recoveryCfg.PrintStack {
						fmt.Printf("Panic: %v\nStack trace:\n%s\n", r, stackTrace)
					}
					
					// Set error response
					ctx.SetStatusCode(fasthttp.StatusInternalServerError)
					ctx.SetContentType("application/json")
					
					errorResponse := fmt.Sprintf(`{"error": "Internal server error", "request_id": "%s"}`, requestID)
					ctx.SetBody([]byte(errorResponse))
				}
			}()
			
			next(ctx)
		}
	}
}

// CORS middleware handles Cross-Origin Resource Sharing
func CORS(corsCfg *config.CORSConfig) MiddlewareFunc {
	if !corsCfg.Enabled {
		return func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
			return next
		}
	}

	return func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
		return func(ctx *fasthttp.RequestCtx) {
			origin := string(ctx.Request.Header.Peek("Origin"))
			
			// Check if origin is allowed
			allowedOrigin := ""
			if len(corsCfg.AllowOrigins) > 0 {
				for _, allowedOrig := range corsCfg.AllowOrigins {
					if allowedOrig == "*" || allowedOrig == origin {
						allowedOrigin = allowedOrig
						break
					}
				}
			}
			
			// Set CORS headers if origin is allowed
			if allowedOrigin != "" {
				if allowedOrigin == "*" {
					ctx.Response.Header.Set("Access-Control-Allow-Origin", "*")
				} else {
					ctx.Response.Header.Set("Access-Control-Allow-Origin", origin)
				}
				
				// Set allowed methods
				if len(corsCfg.AllowMethods) > 0 {
					methods := strings.Join(corsCfg.AllowMethods, ", ")
					ctx.Response.Header.Set("Access-Control-Allow-Methods", methods)
				}
				
				// Set allowed headers
				if len(corsCfg.AllowHeaders) > 0 {
					headers := strings.Join(corsCfg.AllowHeaders, ", ")
					ctx.Response.Header.Set("Access-Control-Allow-Headers", headers)
				}
				
				// Set credentials
				if corsCfg.AllowCredentials {
					ctx.Response.Header.Set("Access-Control-Allow-Credentials", "true")
				}
				
				// Set max age
				if corsCfg.MaxAge > 0 {
					ctx.Response.Header.Set("Access-Control-Max-Age", strconv.Itoa(corsCfg.MaxAge))
				}
			}
			
			// Handle preflight requests
			if string(ctx.Method()) == "OPTIONS" {
				ctx.SetStatusCode(fasthttp.StatusNoContent)
				return
			}
			
			next(ctx)
		}
	}
}

// TimeoutConfig holds timeout configuration for the middleware
type TimeoutConfig struct {
	Enabled  bool
	Duration time.Duration
}

// Timeout middleware enforces request timeouts
func Timeout(timeoutCfg *config.TimeoutConfig) MiddlewareFunc {
	if !timeoutCfg.Enabled {
		return func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
			return next
		}
	}

	return func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
		return func(ctx *fasthttp.RequestCtx) {
			// Create a context with timeout
			timeoutCtx, cancel := context.WithTimeout(context.Background(), timeoutCfg.Duration)
			defer cancel()
			
			// Channel to signal completion
			done := make(chan struct{})
			var timeoutReached bool
			
			// Execute handler in goroutine
			go func() {
				defer func() {
					if r := recover(); r != nil {
						// Re-panic to be caught by recovery middleware
						panic(r)
					}
					close(done)
				}()
				next(ctx)
			}()
			
			// Wait for completion or timeout
			select {
			case <-done:
				// Request completed normally
			case <-timeoutCtx.Done():
				// Timeout reached
				timeoutReached = true
				
				// Get request ID if available
				requestID := ""
				if val := ctx.UserValue("request_id"); val != nil {
					requestID = val.(string)
				}
				
				// Set timeout response
				ctx.SetStatusCode(fasthttp.StatusRequestTimeout)
				ctx.SetContentType("application/json")
				
				errorResponse := fmt.Sprintf(`{"error": "Request timeout", "timeout": "%s", "request_id": "%s"}`, 
					timeoutCfg.Duration, requestID)
				ctx.SetBody([]byte(errorResponse))
			}
			
			// If timeout was reached, we can't do anything more with the context
			// The goroutine will continue running but we've already sent the response
			_ = timeoutReached
		}
	}
}

// MetricsCollector interface for collecting HTTP metrics
type MetricsCollector interface {
	IncRequestCounter(method, path string, status int)
	ObserveLatency(method, path string, duration time.Duration)
	IncActiveConnections()
	DecActiveConnections()
}

// DefaultMetricsCollector provides a simple metrics implementation
type DefaultMetricsCollector struct {
	requestCounter    map[string]int64
	latencyHistogram  map[string][]time.Duration
	activeConnections int64
	mu                sync.RWMutex
}

// NewDefaultMetricsCollector creates a new default metrics collector
func NewDefaultMetricsCollector() *DefaultMetricsCollector {
	return &DefaultMetricsCollector{
		requestCounter:   make(map[string]int64),
		latencyHistogram: make(map[string][]time.Duration),
	}
}

// IncRequestCounter increments the request counter
func (m *DefaultMetricsCollector) IncRequestCounter(method, path string, status int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := fmt.Sprintf("%s_%s_%d", method, path, status)
	m.requestCounter[key]++
}

// ObserveLatency records request latency
func (m *DefaultMetricsCollector) ObserveLatency(method, path string, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := fmt.Sprintf("%s_%s", method, path)
	m.latencyHistogram[key] = append(m.latencyHistogram[key], duration)
}

// IncActiveConnections increments active connection count
func (m *DefaultMetricsCollector) IncActiveConnections() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.activeConnections++
}

// DecActiveConnections decrements active connection count
func (m *DefaultMetricsCollector) DecActiveConnections() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.activeConnections--
}

// GetMetrics returns current metrics (for debugging/monitoring)
func (m *DefaultMetricsCollector) GetMetrics() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	return map[string]interface{}{
		"request_counter":    m.requestCounter,
		"active_connections": m.activeConnections,
		"latency_count":      len(m.latencyHistogram),
	}
}

// Metrics middleware collects HTTP request metrics
func Metrics(metricsCfg *config.MetricsConfig, collector MetricsCollector) MiddlewareFunc {
	if !metricsCfg.Enabled || collector == nil {
		return func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
			return next
		}
	}

	return func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
		return func(ctx *fasthttp.RequestCtx) {
			start := time.Now()
			
			// Increment active connections
			collector.IncActiveConnections()
			defer collector.DecActiveConnections()
			
			// Execute next handler
			next(ctx)
			
			// Record metrics
			duration := time.Since(start)
			method := string(ctx.Method())
			path := string(ctx.Path())
			status := ctx.Response.StatusCode()
			
			collector.IncRequestCounter(method, path, status)
			collector.ObserveLatency(method, path, duration)
		}
	}
}

// Chaos returns a chaos engineering middleware that injects faults based on configuration
func Chaos(chaosEngine chaos.ChaosEngine, logger *zap.Logger) MiddlewareFunc {
	return func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
		return func(ctx *fasthttp.RequestCtx) {
			// Check if chaos engine is enabled
			if chaosEngine == nil || !chaosEngine.IsEnabled() {
				next(ctx)
				return
			}
			
			// Get the request path
			path := string(ctx.Path())
			
			// Check if chaos should be applied to this endpoint
			shouldApply, action := chaosEngine.ShouldApplyChaos(path)
			if !shouldApply {
				next(ctx)
				return
			}
			
			// Apply chaos
			if err := chaosEngine.ApplyChaos(action, ctx); err != nil {
				logger.Error("Failed to apply chaos",
					zap.String("path", path),
					zap.String("scenario", action.Scenario),
					zap.String("type", action.Type),
					zap.Error(err))
				// Continue with normal request processing even if chaos fails
				next(ctx)
				return
			}
			
			// If chaos injection was successful and it's an error injection,
			// don't continue to the next handler as the response has been set
			if action.Type == "error" {
				logger.Debug("Chaos error injection applied, skipping normal handler",
					zap.String("path", path),
					zap.String("scenario", action.Scenario),
					zap.Int("status", ctx.Response.StatusCode()))
				return
			}
			
			// For other types of chaos (like latency), continue with normal processing
			next(ctx)
		}
	}
}

// Recording returns a middleware that records HTTP requests and responses
func Recording(recordingEngine recorder.RecordingEngine, logger *zap.Logger) MiddlewareFunc {
	return func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
		return func(ctx *fasthttp.RequestCtx) {
			// Check if recording engine is enabled
			if recordingEngine == nil || !recordingEngine.IsEnabled() {
				next(ctx)
				return
			}
			
			// Record start time for duration calculation
			startTime := time.Now()
			
			// Execute next handler
			next(ctx)
			
			// Calculate request duration
			duration := time.Since(startTime)
			
			// Get response body (make a copy since fasthttp reuses buffers)
			responseBody := make([]byte, len(ctx.Response.Body()))
			copy(responseBody, ctx.Response.Body())
			
			// Record the request/response in a goroutine to avoid blocking
			go func() {
				if err := recordingEngine.Record(ctx, responseBody, duration); err != nil {
					logger.Error("Failed to record request",
						zap.Error(err),
						zap.String("method", string(ctx.Method())),
						zap.String("path", string(ctx.Path())),
						zap.Int("status", ctx.Response.StatusCode()))
				}
			}()
		}
	}
}