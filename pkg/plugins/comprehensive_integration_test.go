package plugins

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"vanta/pkg/api"
	"vanta/pkg/config"
	"vanta/pkg/openapi"
)

// TestPluginSystemServerIntegration tests the complete plugin system integration with the server
func TestPluginSystemServerIntegration(t *testing.T) {
	logger := zaptest.NewLogger(t, zaptest.Level(zap.DebugLevel))

	// Create a minimal OpenAPI spec for testing
	spec := &openapi.Specification{
		OpenAPI: "3.0.0",
		Info: openapi.InfoObject{
			Title:   "Test API",
			Version: "1.0.0",
		},
		Paths: map[string]openapi.PathItem{
			"/api/users": {
				Get: &openapi.Operation{
					OperationId: "getUsers",
					Responses: map[string]openapi.Response{
						"200": {
							Description: "Success",
							Content: map[string]openapi.MediaType{
								"application/json": {
									Schema: &openapi.Schema{
										Type: "array",
										Items: &openapi.Schema{
											Type: "object",
											Properties: map[string]*openapi.Schema{
												"id":   {Type: "integer"},
												"name": {Type: "string"},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			"/health": {
				Get: &openapi.Operation{
					OperationId: "health",
					Responses: map[string]openapi.Response{
						"200": {
							Description: "Health check",
							Content: map[string]openapi.MediaType{
								"application/json": {
									Schema: &openapi.Schema{
										Type: "object",
										Properties: map[string]*openapi.Schema{
											"status": {Type: "string"},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	t.Run("PluginSystemWithServerLifecycle", func(t *testing.T) {
		// Create server configuration with all plugins enabled
		cfg := &config.Config{
			Server: config.ServerConfig{
				Host:         "127.0.0.1",
				Port:         8899, // Use a different port for testing
				ReadTimeout:  30 * time.Second,
				WriteTimeout: 30 * time.Second,
				Concurrency:  256,
			},
			Plugins: []config.PluginConfig{
				{
					Name:    "auth",
					Enabled: true,
					Config: map[string]interface{}{
						"jwt_secret":       "test-secret-key-that-is-very-long-for-security-purposes",
						"jwt_method":       "HS256",
						"jwt_issuer":       "test-server",
						"public_endpoints": []interface{}{"/health"},
						"api_keys": map[string]interface{}{
							"test-api-key": "test-user",
						},
					},
				},
				{
					Name:    "rate_limit",
					Enabled: true,
					Config: map[string]interface{}{
						"ip_requests_per_second": 10.0,
						"ip_burst":              20,
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
						"log_request_body":  true,
						"log_response_body": true,
						"include_metrics":   true,
					},
				},
			},
			Metrics: config.MetricsConfig{
				Enabled: true,
			},
			Logging: config.LoggingConfig{
				Level:  "debug",
				Format: "console",
			},
		}

		// Create and start server
		server, err := api.NewServer(cfg, spec, logger)
		require.NoError(t, err, "Should create server successfully")

		err = server.Start()
		require.NoError(t, err, "Should start server successfully")
		defer func() {
			err := server.Stop()
			assert.NoError(t, err, "Should stop server gracefully")
		}()

		// Wait for server to be ready
		time.Sleep(100 * time.Millisecond)

		baseURL := fmt.Sprintf("http://%s", server.GetAddr())

		// Test 1: Verify plugins are loaded and enabled
		pluginStats := server.GetPluginStats()
		assert.Len(t, pluginStats, 4, "Should have 4 plugins loaded")

		enabledCount := 0
		for _, stat := range pluginStats {
			if stat.State == StateEnabled {
				enabledCount++
			}
		}
		assert.Equal(t, 4, enabledCount, "All 4 plugins should be enabled")

		// Test 2: Verify health endpoint is accessible (public endpoint)
		resp, err := http.Get(baseURL + "/health")
		require.NoError(t, err, "Health check should succeed")
		assert.Equal(t, http.StatusOK, resp.StatusCode, "Health check should return 200")
		resp.Body.Close()

		// Test 3: Verify protected endpoint requires authentication
		resp, err = http.Get(baseURL + "/api/users")
		require.NoError(t, err, "Request should succeed")
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should require authentication")
		resp.Body.Close()

		// Test 4: Verify API key authentication works
		req, _ := http.NewRequest("GET", baseURL+"/api/users", nil)
		req.Header.Set("Authorization", "Bearer test-api-key")
		client := &http.Client{Timeout: 5 * time.Second}
		resp, err = client.Do(req)
		require.NoError(t, err, "API key request should succeed")
		assert.Equal(t, http.StatusOK, resp.StatusCode, "API key should authenticate successfully")
		resp.Body.Close()

		// Test 5: Verify CORS headers are added
		req, _ = http.NewRequest("OPTIONS", baseURL+"/api/users", nil)
		req.Header.Set("Origin", "http://localhost:3000")
		resp, err = client.Do(req)
		require.NoError(t, err, "CORS preflight should succeed")
		assert.Contains(t, resp.Header.Get("Access-Control-Allow-Origin"), "*", "CORS headers should be present")
		resp.Body.Close()

		// Test 6: Verify plugin metrics are collected
		metrics := server.GetMetrics()
		assert.NotEmpty(t, metrics, "Metrics should be collected")

		pluginMetrics := server.GetPluginStats()
		foundMetrics := false
		for _, stat := range pluginMetrics {
			if stat.Metrics.RequestsProcessed > 0 {
				foundMetrics = true
				break
			}
		}
		assert.True(t, foundMetrics, "Plugin metrics should show processed requests")
	})

	t.Run("PluginHotReloadWithServer", func(t *testing.T) {
		// Create server with minimal configuration
		cfg := &config.Config{
			Server: config.ServerConfig{
				Host:         "127.0.0.1",
				Port:         8898, // Different port
				ReadTimeout:  30 * time.Second,
				WriteTimeout: 30 * time.Second,
				Concurrency:  256,
			},
			Plugins: []config.PluginConfig{
				{
					Name:    "auth",
					Enabled: true,
					Config: map[string]interface{}{
						"jwt_secret":       "initial-secret",
						"jwt_method":       "HS256",
						"public_endpoints": []interface{}{"/health"},
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

		// Test initial state
		pluginStats := server.GetPluginStats()
		require.Len(t, pluginStats, 1)
		assert.Equal(t, "auth", pluginStats[0].Name)
		assert.Equal(t, StateEnabled, pluginStats[0].State)

		// Test plugin hot reload
		newConfig := map[string]interface{}{
			"jwt_secret":       "updated-secret-key-that-is-very-long",
			"jwt_method":       "HS256",
			"jwt_issuer":       "updated-issuer",
			"public_endpoints": []interface{}{"/health", "/status"},
		}

		err = server.ReloadPlugin("auth", newConfig)
		assert.NoError(t, err, "Plugin reload should succeed")

		// Verify plugin is still enabled after reload
		pluginStats = server.GetPluginStats()
		require.Len(t, pluginStats, 1)
		assert.Equal(t, StateEnabled, pluginStats[0].State)

		// Test enable/disable operations
		err = server.DisablePlugin("auth")
		assert.NoError(t, err, "Plugin disable should succeed")

		pluginStats = server.GetPluginStats()
		assert.Equal(t, StateDisabled, pluginStats[0].State)

		err = server.EnablePlugin("auth")
		assert.NoError(t, err, "Plugin enable should succeed")

		pluginStats = server.GetPluginStats()
		assert.Equal(t, StateEnabled, pluginStats[0].State)
	})

	t.Run("PluginMiddlewareChaining", func(t *testing.T) {
		// Test plugin middleware execution order and chaining
		cfg := &config.Config{
			Server: config.ServerConfig{
				Host:         "127.0.0.1",
				Port:         8897,
				ReadTimeout:  30 * time.Second,
				WriteTimeout: 30 * time.Second,
				Concurrency:  256,
			},
			Plugins: []config.PluginConfig{
				{
					Name:    "cors",
					Enabled: true,
					Config: map[string]interface{}{
						"allow_origins": []interface{}{"*"},
						"allow_methods": []interface{}{"GET", "POST"},
					},
				},
				{
					Name:    "auth",
					Enabled: true,
					Config: map[string]interface{}{
						"jwt_secret":       "middleware-test-secret-key-that-is-very-long",
						"jwt_method":       "HS256",
						"public_endpoints": []interface{}{"/health"},
						"api_keys": map[string]interface{}{
							"chain-test-key": "chain-user",
						},
					},
				},
				{
					Name:    "logging",
					Enabled: true,
					Config: map[string]interface{}{
						"log_level":        "debug",
						"log_request_body": true,
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

		// Test that middleware chain processes requests correctly
		req, _ := http.NewRequest("GET", baseURL+"/api/users", nil)
		req.Header.Set("Authorization", "Bearer chain-test-key")
		req.Header.Set("Origin", "http://example.com")

		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Do(req)
		require.NoError(t, err)

		// Should succeed with authentication and CORS headers
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.NotEmpty(t, resp.Header.Get("Access-Control-Allow-Origin"))
		resp.Body.Close()

		// Verify all plugins processed the request
		pluginStats := server.GetPluginStats()
		processedCount := 0
		for _, stat := range pluginStats {
			if stat.Metrics.RequestsProcessed > 0 {
				processedCount++
			}
		}
		assert.Greater(t, processedCount, 0, "At least one plugin should have processed requests")
	})
}

// TestPluginSystemConcurrency tests the plugin system under concurrent load
func TestPluginSystemConcurrency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrency test in short mode")
	}

	logger := zaptest.NewLogger(t, zaptest.Level(zap.WarnLevel))

	spec := &openapi.Specification{
		OpenAPI: "3.0.0",
		Info: openapi.InfoObject{
			Title:   "Concurrency Test API",
			Version: "1.0.0",
		},
		Paths: map[string]openapi.PathItem{
			"/api/test": {
				Get: &openapi.Operation{
					OperationId: "test",
					Responses: map[string]openapi.Response{
						"200": {Description: "Success"},
					},
				},
			},
		},
	}

	cfg := &config.Config{
		Server: config.ServerConfig{
			Host:         "127.0.0.1",
			Port:         8896,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
			Concurrency:  1000,
		},
		Plugins: []config.PluginConfig{
			{
				Name:    "auth",
				Enabled: true,
				Config: map[string]interface{}{
					"jwt_secret": "concurrency-test-secret-key-that-is-very-long",
					"jwt_method": "HS256",
					"api_keys": map[string]interface{}{
						"concurrent-key": "concurrent-user",
					},
				},
			},
			{
				Name:    "rate_limit",
				Enabled: true,
				Config: map[string]interface{}{
					"ip_requests_per_second": 1000.0, // High limit for concurrency test
					"ip_burst":              2000,
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

	t.Run("ConcurrentRequests", func(t *testing.T) {
		const numGoroutines = 50
		const requestsPerGoroutine = 10

		var wg sync.WaitGroup
		var successCount int64
		var errorCount int64

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				client := &http.Client{Timeout: 10 * time.Second}

				for j := 0; j < requestsPerGoroutine; j++ {
					req, _ := http.NewRequest("GET", baseURL+"/api/test", nil)
					req.Header.Set("Authorization", "Bearer concurrent-key")

					resp, err := client.Do(req)
					if err != nil {
						t.Logf("Worker %d request %d failed: %v", workerID, j, err)
						errorCount++
						continue
					}

					if resp.StatusCode == http.StatusOK {
						successCount++
					} else {
						errorCount++
					}
					resp.Body.Close()
				}
			}(i)
		}

		wg.Wait()

		totalRequests := int64(numGoroutines * requestsPerGoroutine)
		successRate := float64(successCount) / float64(totalRequests) * 100

		t.Logf("Concurrent test results: %d/%d successful (%.2f%%)", 
			successCount, totalRequests, successRate)

		// Expect at least 90% success rate under concurrent load
		assert.Greater(t, successRate, 90.0, "Success rate should be > 90%% under concurrent load")

		// Verify plugin metrics reflect the load
		pluginStats := server.GetPluginStats()
		totalProcessed := int64(0)
		for _, stat := range pluginStats {
			totalProcessed += stat.Metrics.RequestsProcessed
		}

		assert.Greater(t, totalProcessed, int64(0), "Plugins should have processed requests")
		t.Logf("Total requests processed by plugins: %d", totalProcessed)
	})
}

// TestPluginSystemErrorRecovery tests plugin error handling and recovery
func TestPluginSystemErrorRecovery(t *testing.T) {
	logger := zaptest.NewLogger(t, zaptest.Level(zap.ErrorLevel))

	// Create a faulty plugin for testing error recovery
	faultyPlugin := &FaultyTestPlugin{
		name:        "faulty",
		version:     "1.0.0",
		description: "Plugin that simulates failures",
		errorRate:   0.3, // 30% error rate
	}

	spec := &openapi.Specification{
		OpenAPI: "3.0.0",
		Info: openapi.InfoObject{
			Title:   "Error Recovery Test API",
			Version: "1.0.0",
		},
		Paths: map[string]openapi.PathItem{
			"/api/test": {
				Get: &openapi.Operation{
					OperationId: "test",
					Responses: map[string]openapi.Response{
						"200": {Description: "Success"},
					},
				},
			},
		},
	}

	cfg := &config.Config{
		Server: config.ServerConfig{
			Host:         "127.0.0.1",
			Port:         8895,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
			Concurrency:  256,
		},
		Plugins: []config.PluginConfig{
			{
				Name:    "auth",
				Enabled: true,
				Config: map[string]interface{}{
					"jwt_secret": "error-recovery-test-secret-key-that-is-very-long",
					"jwt_method": "HS256",
					"api_keys": map[string]interface{}{
						"recovery-key": "recovery-user",
					},
				},
			},
		},
		Metrics: config.MetricsConfig{Enabled: true},
	}

	server, err := api.NewServer(cfg, spec, logger)
	require.NoError(t, err)

	// Register the faulty plugin for testing
	pluginManager := server.GetPluginsManager()
	require.NotNil(t, pluginManager)

	registry := pluginManager.GetRegistry()
	err = registry.RegisterPlugin("faulty", func() Plugin { return faultyPlugin })
	require.NoError(t, err)

	err = server.Start()
	require.NoError(t, err)
	defer server.Stop()

	time.Sleep(100 * time.Millisecond)

	t.Run("PluginLoadFailureRecovery", func(t *testing.T) {
		// Try to load the faulty plugin with invalid config
		err := pluginManager.LoadPlugin("faulty", map[string]interface{}{
			"cause_init_failure": true,
		})
		assert.Error(t, err, "Faulty plugin load should fail")

		// Verify the plugin is not in the loaded plugins list
		pluginStats := server.GetPluginStats()
		for _, stat := range pluginStats {
			assert.NotEqual(t, "faulty", stat.Name, "Faulty plugin should not be loaded")
		}

		// Server should still be functional
		baseURL := fmt.Sprintf("http://%s", server.GetAddr())
		req, _ := http.NewRequest("GET", baseURL+"/api/test", nil)
		req.Header.Set("Authorization", "Bearer recovery-key")

		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Do(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode, "Server should remain functional after plugin load failure")
		resp.Body.Close()
	})

	t.Run("PluginRuntimeErrorHandling", func(t *testing.T) {
		// Load the faulty plugin with valid config but runtime errors
		err := pluginManager.LoadPlugin("faulty", map[string]interface{}{
			"error_rate": 0.5, // 50% error rate
		})
		require.NoError(t, err, "Should load faulty plugin with valid config")

		err = pluginManager.EnablePlugin("faulty")
		require.NoError(t, err, "Should enable faulty plugin")

		baseURL := fmt.Sprintf("http://%s", server.GetAddr())

		// Make multiple requests to trigger plugin errors
		successCount := 0
		errorCount := 0
		for i := 0; i < 20; i++ {
			req, _ := http.NewRequest("GET", baseURL+"/api/test", nil)
			req.Header.Set("Authorization", "Bearer recovery-key")

			client := &http.Client{Timeout: 5 * time.Second}
			resp, err := client.Do(req)
			if err != nil {
				errorCount++
				continue
			}

			if resp.StatusCode == http.StatusOK {
				successCount++
			} else {
				errorCount++
			}
			resp.Body.Close()
		}

		// Server should handle plugin errors gracefully and continue serving requests
		assert.Greater(t, successCount, 0, "Some requests should succeed despite plugin errors")
		t.Logf("Error recovery test: %d successes, %d errors", successCount, errorCount)

		// Check plugin metrics for error tracking
		pluginStats := server.GetPluginStats()
		var faultyStats *PluginInfo
		for _, stat := range pluginStats {
			if stat.Name == "faulty" {
				faultyStats = &stat
				break
			}
		}

		if faultyStats != nil {
			assert.Greater(t, faultyStats.Metrics.ErrorCount, int64(0), "Faulty plugin should have recorded errors")
		}
	})
}

// FaultyTestPlugin is a test plugin that simulates failures
type FaultyTestPlugin struct {
	name        string
	version     string
	description string
	errorRate   float64
	initialized bool
	mu          sync.RWMutex
}

func (p *FaultyTestPlugin) Name() string        { return p.name }
func (p *FaultyTestPlugin) Version() string     { return p.version }
func (p *FaultyTestPlugin) Description() string { return p.description }

func (p *FaultyTestPlugin) Init(ctx context.Context, config map[string]interface{}, logger *zap.Logger) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if config["cause_init_failure"] == true {
		return fmt.Errorf("simulated initialization failure")
	}

	if errorRate, ok := config["error_rate"].(float64); ok {
		p.errorRate = errorRate
	}

	p.initialized = true
	return nil
}

func (p *FaultyTestPlugin) Cleanup(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.initialized = false
	return nil
}

func (p *FaultyTestPlugin) Priority() Priority {
	return PriorityNormal
}

func (p *FaultyTestPlugin) PreProcess(ctx *RequestContext) (bool, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.initialized {
		return false, fmt.Errorf("plugin not initialized")
	}

	// Simulate random errors based on error rate
	if p.shouldSimulateError() {
		return false, fmt.Errorf("simulated pre-process error")
	}

	return true, nil
}

func (p *FaultyTestPlugin) PostProcess(ctx *ResponseContext) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.initialized {
		return fmt.Errorf("plugin not initialized")
	}

	if p.shouldSimulateError() {
		return fmt.Errorf("simulated post-process error")
	}

	return nil
}

func (p *FaultyTestPlugin) ShouldApply(req *fasthttp.RequestCtx) bool {
	return true // Apply to all requests
}

func (p *FaultyTestPlugin) shouldSimulateError() bool {
	// Simple pseudo-random error simulation
	return (time.Now().UnixNano()%100) < int64(p.errorRate*100)
}