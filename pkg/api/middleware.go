package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
	"github.com/vanta/pkg/chaos"
	"github.com/vanta/pkg/config"
	"github.com/vanta/pkg/recorder"
	"github.com/vanta/pkg/state"
	"github.com/vanta/pkg/validation"
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

// StateManagement returns a middleware that manages state contexts
func StateManagement(stateManager state.StateManager, contextManager *state.ContextManager, logger *zap.Logger) MiddlewareFunc {
	return func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
		return func(ctx *fasthttp.RequestCtx) {
			// Check if state management is enabled
			if stateManager == nil || !stateManager.IsEnabled() {
				next(ctx)
				return
			}

			// Extract session ID from header or cookie
			sessionID := string(ctx.Request.Header.Peek("X-Session-ID"))
			if sessionID == "" {
				// Try to get from cookie
				sessionID = string(ctx.Request.Header.Cookie("session_id"))
			}
			if sessionID == "" {
				// Generate new session ID
				sessionID = uuid.New().String()
			}

			// Get request ID from context
			requestID := ""
			if val := ctx.UserValue("request_id"); val != nil {
				requestID = val.(string)
			}

			// Extract user ID from JWT token or header
			userID := string(ctx.Request.Header.Peek("X-User-ID"))

			// Create state context
			endpointPath := string(ctx.Path())
			stateContext := contextManager.CreateContext(sessionID, endpointPath, requestID, userID)

			// Store state context in fasthttp context
			ctx.SetUserValue("state_context", stateContext)

			// Create scoped state manager for this request
			scopedStateManager := state.NewScopedStateManager(stateManager, contextManager)
			ctx.SetUserValue("state_manager", scopedStateManager)

			// Set session ID in response header for client
			ctx.Response.Header.Set("X-Session-ID", sessionID)

			// Execute next handler
			next(ctx)

			// Cleanup context after request (optional, for short-lived contexts)
			// contextManager.DeleteContext(sessionID, endpointPath, requestID)
		}
	}
}

// RequestValidation returns a middleware that validates incoming requests
func RequestValidation(validationManager *validation.ValidationManager, logger *zap.Logger) MiddlewareFunc {
	return func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
		return func(ctx *fasthttp.RequestCtx) {
			validator := validationManager.GetRequestValidator()
			config := validationManager.GetConfig()

			// Check if validation is enabled
			if !config.Enabled {
				next(ctx)
				return
			}

			// Convert fasthttp request to http.Request for validation
			httpReq, err := fasthttpToHTTPRequest(ctx)
			if err != nil {
				logger.Error("Failed to convert request for validation", zap.Error(err))
				if config.FailOnInvalid {
					ctx.SetStatusCode(fasthttp.StatusBadRequest)
					ctx.SetContentType("application/json")
					ctx.SetBodyString(`{"error": "Invalid request format"}`)
					return
				}
				next(ctx)
				return
			}

			// Validate the request
			goCtx := context.Background()
			if requestID := ctx.UserValue("request_id"); requestID != nil {
				goCtx = context.WithValue(goCtx, "request_id", requestID)
			}

			result, err := validator.ValidateRequest(goCtx, httpReq)
			if err != nil {
				logger.Error("Request validation failed", zap.Error(err))
				if config.FailOnInvalid {
					ctx.SetStatusCode(fasthttp.StatusInternalServerError)
					ctx.SetContentType("application/json")
					ctx.SetBodyString(`{"error": "Validation error"}`)
					return
				}
				next(ctx)
				return
			}

			// Store validation result for use by other middleware/handlers
			ctx.SetUserValue("request_validation_result", result)

			// Handle invalid requests based on configuration
			if !result.Valid && config.FailOnInvalid {
				ctx.SetStatusCode(fasthttp.StatusBadRequest)
				ctx.SetContentType("application/json")

				// Create detailed error response
				errorResponse := map[string]interface{}{
					"error":   "Request validation failed",
					"valid":   result.Valid,
					"errors":  result.Errors,
					"warnings": result.Warnings,
				}

				if result.RequestID != "" {
					errorResponse["request_id"] = result.RequestID
				}

				// Convert to JSON and send response
				jsonBytes, _ := json.Marshal(errorResponse)
				ctx.SetBody(jsonBytes)
				return
			}

			// Log validation warnings if any
			if len(result.Warnings) > 0 {
				logger.Warn("Request validation warnings",
					zap.String("path", string(ctx.Path())),
					zap.String("method", string(ctx.Method())),
					zap.Any("warnings", result.Warnings))
			}

			next(ctx)
		}
	}
}

// ResponseValidation returns a middleware that validates outgoing responses
func ResponseValidation(validationManager *validation.ValidationManager, logger *zap.Logger) MiddlewareFunc {
	return func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
		return func(ctx *fasthttp.RequestCtx) {
			validator := validationManager.GetResponseValidator()
			config := validationManager.GetConfig()

			// Check if validation is enabled
			if !config.Enabled {
				next(ctx)
				return
			}

			// Execute next handler first to get the response
			next(ctx)

			// Convert fasthttp request/response to http types for validation
			httpReq, err := fasthttpToHTTPRequest(ctx)
			if err != nil {
				logger.Error("Failed to convert request for response validation", zap.Error(err))
				return
			}

			httpResp := fasthttpToHTTPResponse(ctx)

			// Validate the response
			goCtx := context.Background()
			if requestID := ctx.UserValue("request_id"); requestID != nil {
				goCtx = context.WithValue(goCtx, "request_id", requestID)
			}

			result, err := validator.ValidateResponse(goCtx, httpReq, httpResp)
			if err != nil {
				logger.Error("Response validation failed", zap.Error(err))
				return
			}

			// Store validation result
			ctx.SetUserValue("response_validation_result", result)

			// Log validation issues
			if !result.Valid {
				logger.Warn("Response validation failed",
					zap.String("path", string(ctx.Path())),
					zap.String("method", string(ctx.Method())),
					zap.Int("status", result.StatusCode),
					zap.Any("errors", result.Errors))
			}

			if len(result.Warnings) > 0 {
				logger.Warn("Response validation warnings",
					zap.String("path", string(ctx.Path())),
					zap.String("method", string(ctx.Method())),
					zap.Int("status", result.StatusCode),
					zap.Any("warnings", result.Warnings))
			}
		}
	}
}

// Helper functions to convert between fasthttp and net/http types

func fasthttpToHTTPRequest(ctx *fasthttp.RequestCtx) (*http.Request, error) {
	// Create a new HTTP request
	method := string(ctx.Method())
	uri := string(ctx.RequestURI())

	var bodyReader io.Reader
	if len(ctx.Request.Body()) > 0 {
		bodyReader = bytes.NewReader(ctx.Request.Body())
	}

	req, err := http.NewRequest(method, uri, bodyReader)
	if err != nil {
		return nil, err
	}

	// Copy headers
	ctx.Request.Header.VisitAll(func(key, value []byte) {
		req.Header.Add(string(key), string(value))
	})

	// Set host
	req.Host = string(ctx.Host())

	// Set remote address
	req.RemoteAddr = ctx.RemoteAddr().String()

	return req, nil
}

func fasthttpToHTTPResponse(ctx *fasthttp.RequestCtx) *http.Response {
	resp := &http.Response{
		StatusCode: ctx.Response.StatusCode(),
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewReader(ctx.Response.Body())),
	}

	// Copy headers
	ctx.Response.Header.VisitAll(func(key, value []byte) {
		resp.Header.Add(string(key), string(value))
	})

	return resp
}