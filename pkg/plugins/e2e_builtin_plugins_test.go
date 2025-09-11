package plugins

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"vanta/pkg/api"
	"vanta/pkg/config"
	"vanta/pkg/openapi"
)

// TestBuiltinPluginsE2E tests all built-in plugins working together in realistic scenarios
func TestBuiltinPluginsE2E(t *testing.T) {
	logger := zaptest.NewLogger(t)

	// Create a comprehensive OpenAPI spec for testing
	spec := &openapi.Specification{
		OpenAPI: "3.0.0",
		Info: openapi.InfoObject{
			Title:       "E2E Test API",
			Version:     "1.0.0",
			Description: "API for testing all built-in plugins",
		},
		Paths: map[string]openapi.PathItem{
			"/api/users": {
				Get: &openapi.Operation{
					OperationId: "getUsers",
					Summary:     "Get all users",
					Tags:        []string{"users"},
					Responses: map[string]openapi.Response{
						"200": {
							Description: "List of users",
							Content: map[string]openapi.MediaType{
								"application/json": {
									Schema: &openapi.Schema{
										Type: "array",
										Items: &openapi.Schema{
											Type: "object",
											Properties: map[string]*openapi.Schema{
												"id":    {Type: "integer"},
												"name":  {Type: "string"},
												"email": {Type: "string"},
											},
										},
									},
								},
							},
						},
						"401": {Description: "Unauthorized"},
						"429": {Description: "Rate limit exceeded"},
					},
				},
				Post: &openapi.Operation{
					OperationId: "createUser",
					Summary:     "Create a new user",
					Tags:        []string{"users"},
					RequestBody: &openapi.RequestBody{
						Required: true,
						Content: map[string]openapi.MediaType{
							"application/json": {
								Schema: &openapi.Schema{
									Type: "object",
									Properties: map[string]*openapi.Schema{
										"name":  {Type: "string"},
										"email": {Type: "string"},
									},
									Required: []string{"name", "email"},
								},
							},
						},
					},
					Responses: map[string]openapi.Response{
						"201": {Description: "User created"},
						"400": {Description: "Bad request"},
						"401": {Description: "Unauthorized"},
					},
				},
			},
			"/api/admin/users": {
				Get: &openapi.Operation{
					OperationId: "getAdminUsers",
					Summary:     "Get users (admin only)",
					Tags:        []string{"admin"},
					Security: []map[string][]string{
						{"bearerAuth": {"admin"}},
					},
					Responses: map[string]openapi.Response{
						"200": {Description: "Admin users list"},
						"401": {Description: "Unauthorized"},
						"403": {Description: "Forbidden"},
					},
				},
			},
			"/health": {
				Get: &openapi.Operation{
					OperationId: "healthCheck",
					Summary:     "Health check endpoint",
					Responses: map[string]openapi.Response{
						"200": {Description: "Service healthy"},
					},
				},
			},
			"/metrics": {
				Get: &openapi.Operation{
					OperationId: "getMetrics",
					Summary:     "Get service metrics",
					Responses: map[string]openapi.Response{
						"200": {Description: "Service metrics"},
					},
				},
			},
		},
		Components: &openapi.Components{
			SecuritySchemes: map[string]openapi.SecurityScheme{
				"bearerAuth": {
					Type:         "http",
					Scheme:       "bearer",
					BearerFormat: "JWT",
				},
				"apiKey": {
					Type: "apiKey",
					In:   "header",
					Name: "X-API-Key",
				},
			},
		},
	}

	t.Run("AuthenticationFlows", func(t *testing.T) {
		cfg := &config.Config{
			Server: config.ServerConfig{
				Host:         "127.0.0.1",
				Port:         8894,
				ReadTimeout:  30 * time.Second,
				WriteTimeout: 30 * time.Second,
				Concurrency:  256,
			},
			Plugins: []config.PluginConfig{
				{
					Name:    "auth",
					Enabled: true,
					Config: map[string]interface{}{
						"jwt_secret":       "e2e-test-secret-key-that-is-very-long-for-security-purposes",
						"jwt_method":       "HS256",
						"jwt_issuer":       "e2e-test-api",
						"jwt_audience":     "e2e-test-clients",
						"public_endpoints": []interface{}{"/health", "/metrics"},
						"api_keys": map[string]interface{}{
							"user-api-key":  "regular-user",
							"admin-api-key": "admin-user",
						},
					},
				},
				{
					Name:    "logging",
					Enabled: true,
					Config: map[string]interface{}{
						"log_level":         "info",
						"log_request_body":  true,
						"log_response_body": false,
						"include_metrics":   true,
					},
				},
			},
			Metrics: config.MetricsConfig{Enabled: true},
		}

		server, err := api.NewServer(cfg, spec, logger)
		require.NoError(t, err)

		err = server.Start()
		require.NoError(t, err)
		defer server.Stop()

		time.Sleep(100 * time.Millisecond)
		baseURL := fmt.Sprintf("http://%s", server.GetAddr())

		// Test 1: Public endpoints are accessible without authentication
		t.Run("PublicEndpoints", func(t *testing.T) {
			// Health check should work without auth
			resp, err := http.Get(baseURL + "/health")
			require.NoError(t, err)
			assert.Equal(t, http.StatusOK, resp.StatusCode)
			resp.Body.Close()

			// Metrics should work without auth
			resp, err = http.Get(baseURL + "/metrics")
			require.NoError(t, err)
			assert.Equal(t, http.StatusOK, resp.StatusCode)
			resp.Body.Close()
		})

		// Test 2: Protected endpoints require authentication
		t.Run("ProtectedEndpointsWithoutAuth", func(t *testing.T) {
			resp, err := http.Get(baseURL + "/api/users")
			require.NoError(t, err)
			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
			
			var errorResp map[string]interface{}
			json.NewDecoder(resp.Body).Decode(&errorResp)
			assert.Equal(t, "unauthorized", errorResp["error"])
			resp.Body.Close()
		})

		// Test 3: API Key authentication
		t.Run("APIKeyAuthentication", func(t *testing.T) {
			req, _ := http.NewRequest("GET", baseURL+"/api/users", nil)
			req.Header.Set("Authorization", "Bearer user-api-key")
			
			client := &http.Client{Timeout: 5 * time.Second}
			resp, err := client.Do(req)
			require.NoError(t, err)
			assert.Equal(t, http.StatusOK, resp.StatusCode)
			resp.Body.Close()

			// Test invalid API key
			req.Header.Set("Authorization", "Bearer invalid-key")
			resp, err = client.Do(req)
			require.NoError(t, err)
			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
			resp.Body.Close()
		})

		// Test 4: JWT Authentication
		t.Run("JWTAuthentication", func(t *testing.T) {
			// Create a valid JWT token
			token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
				"sub":  "test-user",
				"iss":  "e2e-test-api",
				"aud":  "e2e-test-clients",
				"exp":  time.Now().Add(time.Hour).Unix(),
				"iat":  time.Now().Unix(),
				"role": "user",
			})

			tokenString, err := token.SignedString([]byte("e2e-test-secret-key-that-is-very-long-for-security-purposes"))
			require.NoError(t, err)

			req, _ := http.NewRequest("GET", baseURL+"/api/users", nil)
			req.Header.Set("Authorization", "Bearer "+tokenString)

			client := &http.Client{Timeout: 5 * time.Second}
			resp, err := client.Do(req)
			require.NoError(t, err)
			assert.Equal(t, http.StatusOK, resp.StatusCode)
			resp.Body.Close()

			// Test expired JWT
			expiredToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
				"sub": "test-user",
				"iss": "e2e-test-api",
				"aud": "e2e-test-clients",
				"exp": time.Now().Add(-time.Hour).Unix(), // Expired
				"iat": time.Now().Add(-time.Hour * 2).Unix(),
			})

			expiredTokenString, err := expiredToken.SignedString([]byte("e2e-test-secret-key-that-is-very-long-for-security-purposes"))
			require.NoError(t, err)

			req.Header.Set("Authorization", "Bearer "+expiredTokenString)
			resp, err = client.Do(req)
			require.NoError(t, err)
			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
			resp.Body.Close()
		})
	})

	t.Run("RateLimitingBehavior", func(t *testing.T) {
		cfg := &config.Config{
			Server: config.ServerConfig{
				Host:         "127.0.0.1",
				Port:         8893,
				ReadTimeout:  30 * time.Second,
				WriteTimeout: 30 * time.Second,
				Concurrency:  256,
			},
			Plugins: []config.PluginConfig{
				{
					Name:    "rate_limit",
					Enabled: true,
					Config: map[string]interface{}{
						"ip_requests_per_second": 2.0, // Very low limit for testing
						"ip_burst":              3,
						"exempt_ips":            []interface{}{}, // No exemptions
					},
				},
				{
					Name:    "auth",
					Enabled: true,
					Config: map[string]interface{}{
						"jwt_secret":       "rate-limit-test-secret-key-that-is-very-long",
						"jwt_method":       "HS256",
						"public_endpoints": []interface{}{"/health"},
						"api_keys": map[string]interface{}{
							"rate-test-key": "rate-test-user",
						},
					},
				},
			},
			Metrics: config.MetricsConfig{Enabled: true},
		}

		server, err := api.NewServer(cfg, spec, logger)
		require.NoError(t, err)

		err = server.Start()
		require.NoError(t, err)
		defer server.Stop()

		time.Sleep(100 * time.Millisecond)
		baseURL := fmt.Sprintf("http://%s", server.GetAddr())

		// Test rate limiting
		client := &http.Client{Timeout: 5 * time.Second}

		// Make requests within the burst limit
		successCount := 0
		rateLimitedCount := 0

		for i := 0; i < 10; i++ {
			req, _ := http.NewRequest("GET", baseURL+"/api/users", nil)
			req.Header.Set("Authorization", "Bearer rate-test-key")

			resp, err := client.Do(req)
			require.NoError(t, err)

			if resp.StatusCode == http.StatusOK {
				successCount++
			} else if resp.StatusCode == http.StatusTooManyRequests {
				rateLimitedCount++
				
				var errorResp map[string]interface{}
				json.NewDecoder(resp.Body).Decode(&errorResp)
				assert.Equal(t, "rate_limit_exceeded", errorResp["error"])
			}
			resp.Body.Close()

			time.Sleep(100 * time.Millisecond) // Small delay between requests
		}

		t.Logf("Rate limiting results: %d successful, %d rate limited", successCount, rateLimitedCount)
		assert.Greater(t, rateLimitedCount, 0, "Some requests should be rate limited")
		assert.Greater(t, successCount, 0, "Some requests should succeed within limits")
	})

	t.Run("CORSHandling", func(t *testing.T) {
		cfg := &config.Config{
			Server: config.ServerConfig{
				Host:         "127.0.0.1",
				Port:         8892,
				ReadTimeout:  30 * time.Second,
				WriteTimeout: 30 * time.Second,
				Concurrency:  256,
			},
			Plugins: []config.PluginConfig{
				{
					Name:    "cors",
					Enabled: true,
					Config: map[string]interface{}{
						"allow_origins":     []interface{}{"http://localhost:3000", "https://example.com"},
						"allow_methods":     []interface{}{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
						"allow_headers":     []interface{}{"Content-Type", "Authorization", "X-API-Key"},
						"allow_credentials": true,
						"max_age":          86400,
					},
				},
				{
					Name:    "auth",
					Enabled: true,
					Config: map[string]interface{}{
						"jwt_secret":       "cors-test-secret-key-that-is-very-long-for-security",
						"jwt_method":       "HS256",
						"public_endpoints": []interface{}{"/health"},
						"api_keys": map[string]interface{}{
							"cors-test-key": "cors-test-user",
						},
					},
				},
			},
			Metrics: config.MetricsConfig{Enabled: true},
		}

		server, err := api.NewServer(cfg, spec, logger)
		require.NoError(t, err)

		err = server.Start()
		require.NoError(t, err)
		defer server.Stop()

		time.Sleep(100 * time.Millisecond)
		baseURL := fmt.Sprintf("http://%s", server.GetAddr())

		client := &http.Client{Timeout: 5 * time.Second}

		// Test CORS preflight request
		t.Run("PreflightRequest", func(t *testing.T) {
			req, _ := http.NewRequest("OPTIONS", baseURL+"/api/users", nil)
			req.Header.Set("Origin", "http://localhost:3000")
			req.Header.Set("Access-Control-Request-Method", "POST")
			req.Header.Set("Access-Control-Request-Headers", "Content-Type,Authorization")

			resp, err := client.Do(req)
			require.NoError(t, err)

			assert.Equal(t, http.StatusOK, resp.StatusCode)
			assert.Equal(t, "http://localhost:3000", resp.Header.Get("Access-Control-Allow-Origin"))
			assert.Contains(t, resp.Header.Get("Access-Control-Allow-Methods"), "POST")
			assert.Contains(t, resp.Header.Get("Access-Control-Allow-Headers"), "Authorization")
			assert.Equal(t, "true", resp.Header.Get("Access-Control-Allow-Credentials"))
			resp.Body.Close()
		})

		// Test actual CORS request
		t.Run("ActualCORSRequest", func(t *testing.T) {
			req, _ := http.NewRequest("GET", baseURL+"/api/users", nil)
			req.Header.Set("Origin", "https://example.com")
			req.Header.Set("Authorization", "Bearer cors-test-key")

			resp, err := client.Do(req)
			require.NoError(t, err)

			assert.Equal(t, http.StatusOK, resp.StatusCode)
			assert.Equal(t, "https://example.com", resp.Header.Get("Access-Control-Allow-Origin"))
			resp.Body.Close()
		})

		// Test disallowed origin
		t.Run("DisallowedOrigin", func(t *testing.T) {
			req, _ := http.NewRequest("GET", baseURL+"/api/users", nil)
			req.Header.Set("Origin", "http://malicious-site.com")
			req.Header.Set("Authorization", "Bearer cors-test-key")

			resp, err := client.Do(req)
			require.NoError(t, err)

			// Request should succeed but without CORS headers
			assert.Equal(t, http.StatusOK, resp.StatusCode)
			assert.Empty(t, resp.Header.Get("Access-Control-Allow-Origin"))
			resp.Body.Close()
		})
	})

	t.Run("LoggingFunctionality", func(t *testing.T) {
		// Create a custom logger to capture log output
		logCapture := &LogCapture{}
		logger := zaptest.NewLogger(t, zaptest.WrapOptions(logCapture.Hook()))

		cfg := &config.Config{
			Server: config.ServerConfig{
				Host:         "127.0.0.1",
				Port:         8891,
				ReadTimeout:  30 * time.Second,
				WriteTimeout: 30 * time.Second,
				Concurrency:  256,
			},
			Plugins: []config.PluginConfig{
				{
					Name:    "logging",
					Enabled: true,
					Config: map[string]interface{}{
						"log_level":         "debug",
						"log_request_body":  true,
						"log_response_body": true,
						"include_metrics":   true,
						"exclude_paths":     []interface{}{"/health"},
					},
				},
				{
					Name:    "auth",
					Enabled: true,
					Config: map[string]interface{}{
						"jwt_secret":       "logging-test-secret-key-that-is-very-long-for-security",
						"jwt_method":       "HS256",
						"public_endpoints": []interface{}{"/health"},
						"api_keys": map[string]interface{}{
							"logging-test-key": "logging-test-user",
						},
					},
				},
			},
			Metrics: config.MetricsConfig{Enabled: true},
		}

		server, err := api.NewServer(cfg, spec, logger)
		require.NoError(t, err)

		err = server.Start()
		require.NoError(t, err)
		defer server.Stop()

		time.Sleep(100 * time.Millisecond)
		baseURL := fmt.Sprintf("http://%s", server.GetAddr())

		client := &http.Client{Timeout: 5 * time.Second}

		// Test request logging
		requestBody := `{"name":"John Doe","email":"john@example.com"}`
		req, _ := http.NewRequest("POST", baseURL+"/api/users", strings.NewReader(requestBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer logging-test-key")

		resp, err := client.Do(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
		resp.Body.Close()

		// Allow time for logging
		time.Sleep(100 * time.Millisecond)

		// Verify that request was logged
		logs := logCapture.GetLogs()
		assert.Greater(t, len(logs), 0, "Should have captured log entries")

		foundRequestLog := false
		for _, logEntry := range logs {
			if strings.Contains(logEntry, "POST") && strings.Contains(logEntry, "/api/users") {
				foundRequestLog = true
				break
			}
		}
		assert.True(t, foundRequestLog, "Should have logged the request")
	})

	t.Run("AllPluginsIntegration", func(t *testing.T) {
		// Test all plugins working together
		cfg := &config.Config{
			Server: config.ServerConfig{
				Host:         "127.0.0.1",
				Port:         8890,
				ReadTimeout:  30 * time.Second,
				WriteTimeout: 30 * time.Second,
				Concurrency:  256,
			},
			Plugins: []config.PluginConfig{
				{
					Name:    "auth",
					Enabled: true,
					Config: map[string]interface{}{
						"jwt_secret":       "all-plugins-test-secret-key-that-is-very-long-for-security",
						"jwt_method":       "HS256",
						"jwt_issuer":       "all-plugins-test",
						"public_endpoints": []interface{}{"/health", "/metrics"},
						"api_keys": map[string]interface{}{
							"integration-key": "integration-user",
						},
					},
				},
				{
					Name:    "rate_limit",
					Enabled: true,
					Config: map[string]interface{}{
						"ip_requests_per_second": 50.0, // Reasonable limit
						"ip_burst":              100,
						"exempt_ips":            []interface{}{"127.0.0.1"},
					},
				},
				{
					Name:    "cors",
					Enabled: true,
					Config: map[string]interface{}{
						"allow_origins":     []interface{}{"*"},
						"allow_methods":     []interface{}{"GET", "POST", "PUT", "DELETE"},
						"allow_headers":     []interface{}{"Content-Type", "Authorization"},
						"allow_credentials": false,
					},
				},
				{
					Name:    "logging",
					Enabled: true,
					Config: map[string]interface{}{
						"log_level":         "info",
						"log_request_body":  false,
						"log_response_body": false,
						"include_metrics":   true,
					},
				},
			},
			Metrics: config.MetricsConfig{Enabled: true},
		}

		server, err := api.NewServer(cfg, spec, logger)
		require.NoError(t, err)

		err = server.Start()
		require.NoError(t, err)
		defer server.Stop()

		time.Sleep(100 * time.Millisecond)
		baseURL := fmt.Sprintf("http://%s", server.GetAddr())

		client := &http.Client{Timeout: 5 * time.Second}

		// Test complex request flow through all plugins
		req, _ := http.NewRequest("POST", baseURL+"/api/users", strings.NewReader(`{"name":"Test User","email":"test@example.com"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer integration-key")
		req.Header.Set("Origin", "http://localhost:3000")

		resp, err := client.Do(req)
		require.NoError(t, err)

		// Should succeed through all plugin layers
		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		// Should have CORS headers
		assert.NotEmpty(t, resp.Header.Get("Access-Control-Allow-Origin"))

		// Should have valid response body
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.NotEmpty(t, body)
		resp.Body.Close()

		// Verify plugin metrics
		pluginStats := server.GetPluginStats()
		assert.Len(t, pluginStats, 4, "Should have all 4 plugins loaded")

		for _, stat := range pluginStats {
			assert.Equal(t, StateEnabled, stat.State, "Plugin %s should be enabled", stat.Name)
			assert.Greater(t, stat.Metrics.RequestsProcessed, int64(0), "Plugin %s should have processed requests", stat.Name)
		}

		// Test plugin interaction order is correct
		// Rate limiting should not affect this request since IP is exempted
		// Auth should allow the request with valid API key
		// CORS should add appropriate headers
		// Logging should record the request
	})
}

// LogCapture helps capture log output for testing
type LogCapture struct {
	logs []string
	mu   sync.RWMutex
}

func (lc *LogCapture) Hook() zaptest.LoggerOption {
	return zaptest.Hook(func(e zaptest.LoggedEntry) {
		lc.mu.Lock()
		defer lc.mu.Unlock()
		lc.logs = append(lc.logs, e.Message)
	})
}

func (lc *LogCapture) GetLogs() []string {
	lc.mu.RLock()
	defer lc.mu.RUnlock()
	result := make([]string, len(lc.logs))
	copy(result, lc.logs)
	return result
}

func (lc *LogCapture) Clear() {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	lc.logs = lc.logs[:0]
}