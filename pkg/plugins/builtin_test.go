package plugins

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap/zaptest"
)

func TestBuiltinPluginRegistration(t *testing.T) {
	registry := NewPluginRegistry()
	err := RegisterBuiltinPlugins(registry)
	require.NoError(t, err)

	expectedPlugins := []string{"auth", "cors", "logging", "rate_limit"}
	registeredPlugins := registry.ListFactories()

	assert.ElementsMatch(t, expectedPlugins, registeredPlugins)
}

func TestAuthPlugin_Init(t *testing.T) {
	logger := zaptest.NewLogger(t)
	plugin := NewAuthPlugin()

	config := map[string]interface{}{
		"jwt_secret":       "test-secret",
		"jwt_method":       "HS256",
		"api_keys":         map[string]string{"test-key": "user123"},
		"public_endpoints": []string{"/health", "/status"},
	}

	err := plugin.Init(context.Background(), config, logger)
	require.NoError(t, err)

	assert.Equal(t, "auth", plugin.Name())
	assert.Equal(t, BuiltinVersion, plugin.Version())
}

func TestAuthPlugin_PreProcess_PublicEndpoint(t *testing.T) {
	logger := zaptest.NewLogger(t)
	plugin := NewAuthPlugin().(*AuthPlugin)

	config := map[string]interface{}{
		"public_endpoints": []string{"/health"},
	}

	err := plugin.Init(context.Background(), config, logger)
	require.NoError(t, err)

	// Create mock request context
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/health")
	ctx.Request.Header.SetMethod("GET")

	requestCtx := &RequestContext{
		RequestCtx: ctx,
		StartTime:  time.Now(),
		Logger:     logger,
		Context:    context.Background(),
	}

	shouldContinue, err := plugin.PreProcess(requestCtx)
	require.NoError(t, err)
	assert.True(t, shouldContinue)
}

func TestAuthPlugin_PreProcess_APIKey(t *testing.T) {
	logger := zaptest.NewLogger(t)
	plugin := NewAuthPlugin().(*AuthPlugin)

	config := map[string]interface{}{
		"api_keys": map[string]string{"test-key": "user123"},
	}

	err := plugin.Init(context.Background(), config, logger)
	require.NoError(t, err)

	// Create mock request context with API key
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/protected")
	ctx.Request.Header.SetMethod("GET")
	ctx.Request.Header.Set("Authorization", "test-key")

	requestCtx := &RequestContext{
		RequestCtx: ctx,
		StartTime:  time.Now(),
		Logger:     logger,
		Context:    context.Background(),
		UserValues: make(map[string]interface{}),
	}

	shouldContinue, err := plugin.PreProcess(requestCtx)
	require.NoError(t, err)
	assert.True(t, shouldContinue)

	// Check that user_id was set
	userID, exists := requestCtx.GetUserValue("user_id")
	assert.True(t, exists)
	assert.Equal(t, "user123", userID)
}

func TestAuthPlugin_PreProcess_Unauthorized(t *testing.T) {
	logger := zaptest.NewLogger(t)
	plugin := NewAuthPlugin().(*AuthPlugin)

	config := map[string]interface{}{}
	err := plugin.Init(context.Background(), config, logger)
	require.NoError(t, err)

	// Create mock request context without auth
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/protected")
	ctx.Request.Header.SetMethod("GET")

	requestCtx := &RequestContext{
		RequestCtx: ctx,
		StartTime:  time.Now(),
		Logger:     logger,
		Context:    context.Background(),
		UserValues: make(map[string]interface{}),
	}

	shouldContinue, err := plugin.PreProcess(requestCtx)
	require.NoError(t, err)
	assert.False(t, shouldContinue)
	assert.Equal(t, fasthttp.StatusUnauthorized, ctx.Response.StatusCode())
}

func TestRateLimitPlugin_Init(t *testing.T) {
	logger := zaptest.NewLogger(t)
	plugin := NewRateLimitPlugin()

	config := map[string]interface{}{
		"global_requests_per_second": 100.0,
		"global_burst":               200,
		"ip_requests_per_second":     10.0,
		"ip_burst":                   20,
		"exempt_ips":                 []string{"127.0.0.1"},
	}

	err := plugin.Init(context.Background(), config, logger)
	require.NoError(t, err)

	assert.Equal(t, "rate_limit", plugin.Name())
	assert.Equal(t, BuiltinVersion, plugin.Version())
}

func TestCORSPlugin_Init(t *testing.T) {
	logger := zaptest.NewLogger(t)
	plugin := NewCORSPlugin()

	config := map[string]interface{}{
		"allow_origins":     []string{"http://localhost:3000", "https://example.com"},
		"allow_methods":     []string{"GET", "POST", "PUT", "DELETE"},
		"allow_credentials": true,
		"max_age":           3600,
	}

	err := plugin.Init(context.Background(), config, logger)
	require.NoError(t, err)

	assert.Equal(t, "cors", plugin.Name())
	assert.Equal(t, BuiltinVersion, plugin.Version())
}

func TestCORSPlugin_PreProcess_Preflight(t *testing.T) {
	logger := zaptest.NewLogger(t)
	plugin := NewCORSPlugin().(*CORSPlugin)

	config := map[string]interface{}{
		"allow_origins": []string{"http://localhost:3000"},
		"allow_methods": []string{"GET", "POST", "PUT", "DELETE"},
	}

	err := plugin.Init(context.Background(), config, logger)
	require.NoError(t, err)

	// Create mock preflight request
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/api/test")
	ctx.Request.Header.SetMethod("OPTIONS")
	ctx.Request.Header.Set("Origin", "http://localhost:3000")
	ctx.Request.Header.Set("Access-Control-Request-Method", "POST")

	requestCtx := &RequestContext{
		RequestCtx: ctx,
		StartTime:  time.Now(),
		Logger:     logger,
		Context:    context.Background(),
	}

	shouldContinue, err := plugin.PreProcess(requestCtx)
	require.NoError(t, err)
	assert.False(t, shouldContinue) // Preflight should stop processing
	assert.Equal(t, fasthttp.StatusNoContent, ctx.Response.StatusCode())

	// Check CORS headers
	assert.Equal(t, "http://localhost:3000", string(ctx.Response.Header.Peek("Access-Control-Allow-Origin")))
	assert.Contains(t, string(ctx.Response.Header.Peek("Access-Control-Allow-Methods")), "POST")
}

func TestLoggingPlugin_Init(t *testing.T) {
	logger := zaptest.NewLogger(t)
	plugin := NewLoggingPlugin()

	config := map[string]interface{}{
		"log_level":          "debug",
		"log_request_body":   true,
		"log_response_body":  false,
		"max_body_size":      2048,
		"sensitive_headers":  []string{"x-secret"},
		"include_metrics":    true,
	}

	err := plugin.Init(context.Background(), config, logger)
	require.NoError(t, err)

	assert.Equal(t, "logging", plugin.Name())
	assert.Equal(t, BuiltinVersion, plugin.Version())
}

func TestGetBuiltinPluginFactories(t *testing.T) {
	factories := GetBuiltinPluginFactories()
	
	expectedPlugins := []string{"auth", "cors", "logging", "rate_limit"}
	
	assert.Len(t, factories, len(expectedPlugins))
	
	for _, name := range expectedPlugins {
		factory, exists := factories[name]
		assert.True(t, exists, "Factory for plugin %s should exist", name)
		
		plugin := factory()
		assert.NotNil(t, plugin)
		assert.Equal(t, name, plugin.Name())
		assert.Equal(t, BuiltinVersion, plugin.Version())
	}
}

func TestCreateBuiltinPlugin(t *testing.T) {
	tests := []struct {
		name        string
		pluginName  string
		expectError bool
	}{
		{"Valid auth plugin", "auth", false},
		{"Valid rate_limit plugin", "rate_limit", false},
		{"Valid cors plugin", "cors", false},
		{"Valid logging plugin", "logging", false},
		{"Invalid plugin", "nonexistent", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plugin, err := CreateBuiltinPlugin(tt.pluginName)
			
			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, plugin)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, plugin)
				assert.Equal(t, tt.pluginName, plugin.Name())
			}
		})
	}
}

func TestGetBuiltinPluginNames(t *testing.T) {
	names := GetBuiltinPluginNames()
	
	expectedNames := []string{"auth", "cors", "logging", "rate_limit"}
	assert.ElementsMatch(t, expectedNames, names)
	
	// Check that names are sorted
	assert.Equal(t, []string{"auth", "cors", "logging", "rate_limit"}, names)
}

func TestMapToStruct(t *testing.T) {
	type TestStruct struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
		Flag  bool   `json:"flag"`
	}

	input := map[string]interface{}{
		"name":  "test",
		"value": 42,
		"flag":  true,
	}

	var result TestStruct
	err := mapToStruct(input, &result)
	
	require.NoError(t, err)
	assert.Equal(t, "test", result.Name)
	assert.Equal(t, 42, result.Value)
	assert.True(t, result.Flag)
}

func TestPriorityOrdering(t *testing.T) {
	authPlugin := NewAuthPlugin().(Middleware)
	rateLimitPlugin := NewRateLimitPlugin().(Middleware)
	corsPlugin := NewCORSPlugin().(Middleware)
	loggingPlugin := NewLoggingPlugin().(Middleware)

	// Auth should have highest priority (lowest value)
	assert.Equal(t, PriorityHigh, authPlugin.Priority())
	
	// Rate limit and CORS should have normal priority
	assert.Equal(t, PriorityNormal, rateLimitPlugin.Priority())
	assert.Equal(t, PriorityNormal, corsPlugin.Priority())
	
	// Logging should have lowest priority (highest value)
	assert.Equal(t, PriorityLow, loggingPlugin.Priority())
}