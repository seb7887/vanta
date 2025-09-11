package plugins

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
	"vanta/pkg/config"
)

func TestPluginManager_Basic(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	manager := NewManager(logger)
	defer manager.Shutdown()

	// Register plugin factory
	err := manager.GetRegistry().RegisterPlugin("example-middleware", NewExampleMiddlewarePlugin)
	require.NoError(t, err)

	// Test plugin loading
	config := map[string]interface{}{
		"header_name":  "X-Test-Header",
		"header_value": "test-value",
		"priority":     100,
	}

	err = manager.LoadPlugin("example-middleware", config)
	require.NoError(t, err)

	// Verify plugin is loaded but not enabled
	plugins := manager.ListPlugins()
	require.Len(t, plugins, 1)
	assert.Equal(t, "example-middleware", plugins[0].Name)
	assert.Equal(t, StateLoaded, plugins[0].State)

	// Enable plugin
	err = manager.EnablePlugin("example-middleware")
	require.NoError(t, err)

	// Verify plugin is enabled
	plugins = manager.ListPlugins()
	require.Len(t, plugins, 1)
	assert.Equal(t, StateEnabled, plugins[0].State)

	// Get plugin
	plugin, found := manager.GetPlugin("example-middleware")
	require.True(t, found)
	assert.NotNil(t, plugin)
	assert.Equal(t, "example-middleware", plugin.Name())

	// Disable plugin
	err = manager.DisablePlugin("example-middleware")
	require.NoError(t, err)

	// Verify plugin is disabled
	plugins = manager.ListPlugins()
	require.Len(t, plugins, 1)
	assert.Equal(t, StateDisabled, plugins[0].State)

	// Unload plugin
	err = manager.UnloadPlugin("example-middleware")
	require.NoError(t, err)

	// Verify plugin is unloaded
	plugins = manager.ListPlugins()
	require.Len(t, plugins, 0)
}

func TestPluginManager_LoadFromConfig(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	manager := NewManager(logger)
	defer manager.Shutdown()

	// Register plugin factories
	err := manager.GetRegistry().RegisterPlugin("example-middleware", NewExampleMiddlewarePlugin)
	require.NoError(t, err)

	err = manager.GetRegistry().RegisterPlugin("example-configurable", NewExampleConfigurablePlugin)
	require.NoError(t, err)

	// Load from config
	pluginConfigs := []config.PluginConfig{
		{
			Name:    "example-middleware",
			Enabled: true,
			Config: map[string]interface{}{
				"header_name":  "X-Config-Header",
				"header_value": "config-value",
			},
		},
		{
			Name:    "example-configurable",
			Enabled: false,
			Config: map[string]interface{}{
				"header_name": "X-Configurable-Header",
				"priority":    50,
			},
		},
	}

	err = manager.LoadFromConfig(pluginConfigs)
	require.NoError(t, err)

	// Verify plugins are loaded
	plugins := manager.ListPlugins()
	require.Len(t, plugins, 2)

	// Check enabled states
	for _, plugin := range plugins {
		if plugin.Name == "example-middleware" {
			assert.Equal(t, StateEnabled, plugin.State)
		} else if plugin.Name == "example-configurable" {
			assert.Equal(t, StateLoaded, plugin.State)
		}
	}
}

func TestPluginManager_Middleware(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	manager := NewManager(logger)
	defer manager.Shutdown()

	// Register and load middleware plugins
	err := manager.GetRegistry().RegisterPlugin("middleware1", func() Plugin {
		plugin := NewExampleMiddlewarePlugin().(*ExampleMiddlewarePlugin)
		plugin.name = "middleware1"
		plugin.priority = PriorityHigh
		return plugin
	})
	require.NoError(t, err)

	err = manager.GetRegistry().RegisterPlugin("middleware2", func() Plugin {
		plugin := NewExampleMiddlewarePlugin().(*ExampleMiddlewarePlugin)
		plugin.name = "middleware2"
		plugin.priority = PriorityLow
		return plugin
	})
	require.NoError(t, err)

	// Load and enable plugins
	err = manager.LoadPlugin("middleware1", map[string]interface{}{
		"header_name": "X-Middleware-1",
		"priority":    int(PriorityHigh),
	})
	require.NoError(t, err)
	err = manager.EnablePlugin("middleware1")
	require.NoError(t, err)

	err = manager.LoadPlugin("middleware2", map[string]interface{}{
		"header_name": "X-Middleware-2",
		"priority":    int(PriorityLow),
	})
	require.NoError(t, err)
	err = manager.EnablePlugin("middleware2")
	require.NoError(t, err)

	// Get middlewares (should be sorted by priority)
	middlewares := manager.GetMiddlewares()
	require.Len(t, middlewares, 2)
	assert.Equal(t, "middleware1", middlewares[0].Name()) // Higher priority (lower value)
	assert.Equal(t, "middleware2", middlewares[1].Name()) // Lower priority (higher value)

	// Test middleware function creation
	middlewareFunc := manager.CreateMiddlewareFunc()
	assert.NotNil(t, middlewareFunc)

	// Test with a mock handler
	handlerCalled := false
	testHandler := func(ctx *fasthttp.RequestCtx) {
		handlerCalled = true
		ctx.SetStatusCode(fasthttp.StatusOK)
		ctx.SetBody([]byte("test response"))
	}

	wrappedHandler := middlewareFunc(testHandler)

	// Create test request context
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/test")
	ctx.Request.Header.SetMethod("GET")
	ctx.SetUserValue("request_id", "test-123")

	// Execute wrapped handler
	wrappedHandler(ctx)

	// Verify handler was called
	assert.True(t, handlerCalled)
	assert.Equal(t, fasthttp.StatusOK, ctx.Response.StatusCode())

	// Verify middleware headers were added
	assert.NotEmpty(t, ctx.Response.Header.Peek("X-Processing-Time"))
}

func TestPluginManager_ReloadPlugin(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	manager := NewManager(logger)
	defer manager.Shutdown()

	// Register configurable plugin
	err := manager.GetRegistry().RegisterPlugin("example-configurable", NewExampleConfigurablePlugin)
	require.NoError(t, err)

	// Load and enable plugin
	originalConfig := map[string]interface{}{
		"header_name":  "X-Original-Header",
		"header_value": "original-value",
	}
	err = manager.LoadPlugin("example-configurable", originalConfig)
	require.NoError(t, err)
	err = manager.EnablePlugin("example-configurable")
	require.NoError(t, err)

	// Get plugin and verify original config
	plugin, found := manager.GetPlugin("example-configurable")
	require.True(t, found)
	configurablePlugin := plugin.(*ExampleConfigurablePlugin)
	assert.Equal(t, "X-Original-Header", configurablePlugin.headerName)

	// Reload with new config
	newConfig := map[string]interface{}{
		"header_name":  "X-Reloaded-Header",
		"header_value": "reloaded-value",
		"priority":     50,
	}
	err = manager.ReloadPlugin("example-configurable", newConfig)
	require.NoError(t, err)

	// Verify plugin is still enabled and config is updated
	plugins := manager.ListPlugins()
	require.Len(t, plugins, 1)
	assert.Equal(t, StateEnabled, plugins[0].State)

	// Get plugin again and verify new config
	plugin, found = manager.GetPlugin("example-configurable")
	require.True(t, found)
	configurablePlugin = plugin.(*ExampleConfigurablePlugin)
	assert.Equal(t, "X-Reloaded-Header", configurablePlugin.headerName)
	assert.Equal(t, "reloaded-value", configurablePlugin.headerValue)
}

func TestPluginManager_HealthCheck(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	manager := NewManager(logger)
	defer manager.Shutdown()

	// Disable automatic health checks for controlled testing
	manager.EnableHealthCheck(false)

	// Register health check plugin
	err := manager.GetRegistry().RegisterPlugin("example-health-check", NewExampleHealthCheckPlugin)
	require.NoError(t, err)

	// Load and enable plugin
	err = manager.LoadPlugin("example-health-check", map[string]interface{}{})
	require.NoError(t, err)
	err = manager.EnablePlugin("example-health-check")
	require.NoError(t, err)

	// Manually trigger health check
	manager.performHealthChecks()

	// Wait a bit for health check to complete
	time.Sleep(100 * time.Millisecond)

	// Verify health status
	plugins := manager.ListPlugins()
	require.Len(t, plugins, 1)
	assert.NotNil(t, plugins[0].Health)
	assert.True(t, plugins[0].Health.Healthy)
	assert.Equal(t, "Plugin is healthy", plugins[0].Health.Message)

	// Get plugin and set it unhealthy
	plugin, found := manager.GetPlugin("example-health-check")
	require.True(t, found)
	healthPlugin := plugin.(*ExampleHealthCheckPlugin)
	healthPlugin.SetHealthy(false)

	// Trigger health check again
	manager.performHealthChecks()
	time.Sleep(100 * time.Millisecond)

	// Verify unhealthy status
	plugins = manager.ListPlugins()
	require.Len(t, plugins, 1)
	assert.NotNil(t, plugins[0].Health)
	assert.False(t, plugins[0].Health.Healthy)
	assert.Contains(t, plugins[0].Health.Message, "experiencing issues")
}

func TestPluginManager_Metrics(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	manager := NewManager(logger)
	defer manager.Shutdown()

	// Register plugin
	err := manager.GetRegistry().RegisterPlugin("example-middleware", NewExampleMiddlewarePlugin)
	require.NoError(t, err)

	// Load and enable plugin
	err = manager.LoadPlugin("example-middleware", map[string]interface{}{})
	require.NoError(t, err)
	err = manager.EnablePlugin("example-middleware")
	require.NoError(t, err)

	// Get initial metrics
	metrics := manager.GetPluginMetrics()
	assert.Equal(t, 1, metrics["total_plugins"])
	assert.Equal(t, 1, metrics["enabled_plugins"])
	assert.Equal(t, int64(0), metrics["total_requests"])
	assert.Equal(t, int64(0), metrics["total_errors"])

	// Simulate some plugin activity by processing requests through middleware
	middlewareFunc := manager.CreateMiddlewareFunc()
	testHandler := func(ctx *fasthttp.RequestCtx) {
		ctx.SetStatusCode(fasthttp.StatusOK)
	}
	wrappedHandler := middlewareFunc(testHandler)

	// Process several requests
	for i := 0; i < 5; i++ {
		ctx := &fasthttp.RequestCtx{}
		ctx.Request.SetRequestURI("/test")
		ctx.Request.Header.SetMethod("GET")
		ctx.SetUserValue("request_id", "test-123")
		wrappedHandler(ctx)
	}

	// Get updated metrics
	metrics = manager.GetPluginMetrics()
	assert.Equal(t, 1, metrics["total_plugins"])
	assert.Equal(t, 1, metrics["enabled_plugins"])
	assert.True(t, metrics["total_requests"].(int64) > 0)

	// Check plugin-specific stats
	pluginStats := metrics["plugin_stats"].(map[string]PluginMetrics)
	assert.Contains(t, pluginStats, "example-middleware")
	stats := pluginStats["example-middleware"]
	assert.True(t, stats.RequestsProcessed > 0)
	assert.True(t, stats.AverageLatency > 0)
}

func TestPluginManager_ErrorHandling(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	manager := NewManager(logger)
	defer manager.Shutdown()

	// Test loading non-existent plugin
	err := manager.LoadPlugin("non-existent", map[string]interface{}{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "plugin factory not found")

	// Test enabling non-existent plugin
	err = manager.EnablePlugin("non-existent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "plugin not found")

	// Test disabling non-existent plugin
	err = manager.DisablePlugin("non-existent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "plugin not found")

	// Test unloading non-existent plugin
	err = manager.UnloadPlugin("non-existent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "plugin not found")

	// Test reloading non-existent plugin
	err = manager.ReloadPlugin("non-existent", map[string]interface{}{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "plugin not found")

	// Test duplicate plugin registration
	err = manager.GetRegistry().RegisterPlugin("test-plugin", NewExampleMiddlewarePlugin)
	require.NoError(t, err)
	err = manager.GetRegistry().RegisterPlugin("test-plugin", NewExampleMiddlewarePlugin)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "plugin already registered")

	// Test loading duplicate plugin
	err = manager.LoadPlugin("test-plugin", map[string]interface{}{})
	require.NoError(t, err)
	err = manager.LoadPlugin("test-plugin", map[string]interface{}{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "plugin already loaded")
}

func TestPluginManager_Shutdown(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	manager := NewManager(logger)

	// Register and load multiple plugins
	err := manager.GetRegistry().RegisterPlugin("plugin1", NewExampleMiddlewarePlugin)
	require.NoError(t, err)
	err = manager.GetRegistry().RegisterPlugin("plugin2", NewExampleConfigurablePlugin)
	require.NoError(t, err)

	err = manager.LoadPlugin("plugin1", map[string]interface{}{})
	require.NoError(t, err)
	err = manager.LoadPlugin("plugin2", map[string]interface{}{})
	require.NoError(t, err)

	err = manager.EnablePlugin("plugin1")
	require.NoError(t, err)
	err = manager.EnablePlugin("plugin2")
	require.NoError(t, err)

	// Verify plugins are loaded
	plugins := manager.ListPlugins()
	assert.Len(t, plugins, 2)

	// Shutdown manager
	err = manager.Shutdown()
	assert.NoError(t, err)

	// Verify all plugins are unloaded
	plugins = manager.ListPlugins()
	assert.Len(t, plugins, 0)
}

func TestPluginRegistry(t *testing.T) {
	registry := NewPluginRegistry()

	// Test registration
	err := registry.RegisterPlugin("test-plugin", NewExampleMiddlewarePlugin)
	assert.NoError(t, err)

	// Test duplicate registration
	err = registry.RegisterPlugin("test-plugin", NewExampleMiddlewarePlugin)
	assert.Error(t, err)

	// Test getting factory
	factory, exists := registry.GetFactory("test-plugin")
	assert.True(t, exists)
	assert.NotNil(t, factory)

	// Test non-existent factory
	_, exists = registry.GetFactory("non-existent")
	assert.False(t, exists)

	// Test listing factories
	factories := registry.ListFactories()
	assert.Len(t, factories, 1)
	assert.Contains(t, factories, "test-plugin")

	// Test unregistration
	err = registry.UnregisterPlugin("test-plugin")
	assert.NoError(t, err)

	// Test unregistering non-existent plugin
	err = registry.UnregisterPlugin("non-existent")
	assert.Error(t, err)

	// Verify plugin is removed
	factories = registry.ListFactories()
	assert.Len(t, factories, 0)
}

func BenchmarkPluginManager_CreateMiddlewareFunc(b *testing.B) {
	logger, _ := zap.NewDevelopment()
	manager := NewManager(logger)
	defer manager.Shutdown()

	// Register and enable multiple middleware plugins
	for i := 0; i < 5; i++ {
		pluginName := fmt.Sprintf("middleware%d", i)
		err := manager.GetRegistry().RegisterPlugin(pluginName, func() Plugin {
			plugin := NewExampleMiddlewarePlugin().(*ExampleMiddlewarePlugin)
			plugin.name = pluginName
			return plugin
		})
		require.NoError(b, err)

		err = manager.LoadPlugin(pluginName, map[string]interface{}{})
		require.NoError(b, err)
		err = manager.EnablePlugin(pluginName)
		require.NoError(b, err)
	}

	middlewareFunc := manager.CreateMiddlewareFunc()
	testHandler := func(ctx *fasthttp.RequestCtx) {
		ctx.SetStatusCode(fasthttp.StatusOK)
	}
	wrappedHandler := middlewareFunc(testHandler)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			ctx := &fasthttp.RequestCtx{}
			ctx.Request.SetRequestURI("/test")
			ctx.Request.Header.SetMethod("GET")
			ctx.SetUserValue("request_id", "bench-test")
			wrappedHandler(ctx)
		}
	})
}