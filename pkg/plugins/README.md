# Plugin Manager

A comprehensive, production-ready plugin manager for the Vanta project that provides thread-safe plugin lifecycle management, hot reload support, health checking, and metrics collection.

## Features

- **Thread-safe plugin registry and management** using sync.RWMutex
- **Plugin lifecycle management**: Load, Unload, Enable, Disable, Reload
- **Hot reload support** for plugins that implement the `HotReloadable` interface
- **Health checking** with automatic monitoring for plugins implementing `HealthChecker`
- **Metrics collection** for plugin operations and performance monitoring
- **Dependency resolution** between plugins
- **FastHTTP middleware integration** with priority-based execution
- **Graceful error handling** with structured logging
- **Configuration validation** for plugins supporting runtime configuration changes

## Core Components

### Manager

The main `Manager` struct provides all plugin management functionality:

```go
manager := plugins.NewManager(logger)
defer manager.Shutdown()
```

### Plugin Registry

Manages plugin factories for creating plugin instances:

```go
registry := manager.GetRegistry()
err := registry.RegisterPlugin("my-plugin", MyPluginFactory)
```

### Plugin Interfaces

The system supports several plugin types:

- **Plugin**: Base interface for all plugins
- **Middleware**: HTTP middleware plugins with pre/post processing
- **RequestProcessor**: Request processing plugins
- **ResponseProcessor**: Response processing plugins
- **ConfigurablePlugin**: Plugins supporting runtime configuration changes
- **HealthChecker**: Plugins providing health check functionality
- **HotReloadable**: Plugins supporting hot reloading

## Usage Examples

### Basic Plugin Management

```go
// Create manager
logger, _ := zap.NewDevelopment()
manager := plugins.NewManager(logger)
defer manager.Shutdown()

// Register plugin factory
err := manager.GetRegistry().RegisterPlugin("example-middleware", NewExampleMiddlewarePlugin)
if err != nil {
    log.Fatal(err)
}

// Load plugin with configuration
config := map[string]interface{}{
    "header_name":  "X-Custom-Header",
    "header_value": "custom-value",
    "priority":     100,
}
err = manager.LoadPlugin("example-middleware", config)
if err != nil {
    log.Fatal(err)
}

// Enable plugin
err = manager.EnablePlugin("example-middleware")
if err != nil {
    log.Fatal(err)
}

// Get plugin information
plugins := manager.ListPlugins()
for _, plugin := range plugins {
    fmt.Printf("Plugin: %s, State: %s, Version: %s\n", 
        plugin.Name, plugin.State, plugin.Version)
}
```

### Loading from Configuration

```go
// Load plugins from configuration file
pluginConfigs := []config.PluginConfig{
    {
        Name:    "auth-middleware",
        Enabled: true,
        Config: map[string]interface{}{
            "secret_key": "my-secret",
            "algorithm":  "HS256",
        },
    },
    {
        Name:    "rate-limiter",
        Enabled: true,
        Config: map[string]interface{}{
            "requests_per_minute": 100,
            "burst_size":         10,
        },
    },
}

err := manager.LoadFromConfig(pluginConfigs)
if err != nil {
    log.Fatal(err)
}
```

### FastHTTP Middleware Integration

```go
// Create FastHTTP middleware function from enabled plugins
middlewareFunc := manager.CreateMiddlewareFunc()

// Use with FastHTTP server
server := &fasthttp.Server{
    Handler: middlewareFunc(yourMainHandler),
}
```

### Hot Reloading

```go
// Reload plugin with new configuration
newConfig := map[string]interface{}{
    "header_name":  "X-Updated-Header",
    "header_value": "updated-value",
    "priority":     50,
}

err := manager.ReloadPlugin("example-middleware", newConfig)
if err != nil {
    log.Fatal(err)
}
```

### Health Monitoring

```go
// Configure health checking
manager.SetHealthCheckInterval(30 * time.Second)
manager.EnableHealthCheck(true)

// Check plugin health status
plugins := manager.ListPlugins()
for _, plugin := range plugins {
    if plugin.Health != nil {
        fmt.Printf("Plugin %s health: %v (%s)\n", 
            plugin.Name, plugin.Health.Healthy, plugin.Health.Message)
    }
}
```

### Metrics Collection

```go
// Get aggregated plugin metrics
metrics := manager.GetPluginMetrics()
fmt.Printf("Total plugins: %d\n", metrics["total_plugins"])
fmt.Printf("Enabled plugins: %d\n", metrics["enabled_plugins"])
fmt.Printf("Total requests: %d\n", metrics["total_requests"])

// Get per-plugin statistics
pluginStats := metrics["plugin_stats"].(map[string]plugins.PluginMetrics)
for name, stats := range pluginStats {
    fmt.Printf("Plugin %s: %d requests, %d errors, avg latency: %v\n",
        name, stats.RequestsProcessed, stats.ErrorCount, stats.AverageLatency)
}
```

## Creating Custom Plugins

### Basic Plugin

```go
type MyPlugin struct {
    name        string
    version     string
    description string
    logger      *zap.Logger
    config      map[string]interface{}
}

func NewMyPlugin() plugins.Plugin {
    return &MyPlugin{
        name:        "my-plugin",
        version:     "1.0.0",
        description: "My custom plugin",
    }
}

func (p *MyPlugin) Name() string { return p.name }
func (p *MyPlugin) Version() string { return p.version }
func (p *MyPlugin) Description() string { return p.description }

func (p *MyPlugin) Init(ctx context.Context, config map[string]interface{}, logger *zap.Logger) error {
    p.logger = logger
    p.config = config
    
    // Initialize plugin-specific resources
    p.logger.Info("Plugin initialized", zap.Any("config", config))
    return nil
}

func (p *MyPlugin) Cleanup(ctx context.Context) error {
    // Cleanup plugin-specific resources
    p.logger.Info("Plugin cleaned up")
    return nil
}
```

### Middleware Plugin

```go
type MyMiddleware struct {
    *MyPlugin
    priority plugins.Priority
}

func NewMyMiddleware() plugins.Plugin {
    return &MyMiddleware{
        MyPlugin: &MyPlugin{
            name:        "my-middleware",
            version:     "1.0.0",
            description: "My custom middleware",
        },
        priority: plugins.PriorityNormal,
    }
}

func (m *MyMiddleware) Priority() plugins.Priority {
    return m.priority
}

func (m *MyMiddleware) PreProcess(ctx *plugins.RequestContext) (bool, error) {
    // Process request before main handler
    ctx.Logger.Debug("Processing request", 
        zap.String("method", ctx.Method()),
        zap.String("path", ctx.Path()))
    
    // Add custom header
    ctx.RequestCtx.Request.Header.Set("X-My-Middleware", "processed")
    
    return true, nil // Continue processing
}

func (m *MyMiddleware) PostProcess(ctx *plugins.ResponseContext) error {
    // Process response after main handler
    ctx.RequestCtx.Response.Header.Set("X-Response-Time", 
        fmt.Sprintf("%.2fms", float64(ctx.ProcessingTime.Nanoseconds())/1000000))
    
    return nil
}

func (m *MyMiddleware) ShouldApply(req *fasthttp.RequestCtx) bool {
    // Apply to all requests except health checks
    path := string(req.Path())
    return !strings.HasPrefix(path, "/health")
}
```

### Configurable Plugin

```go
type MyConfigurablePlugin struct {
    *MyPlugin
    setting1 string
    setting2 int
}

func (p *MyConfigurablePlugin) UpdateConfig(ctx context.Context, config map[string]interface{}) error {
    if err := p.ValidateConfig(config); err != nil {
        return err
    }
    
    if val, ok := config["setting1"].(string); ok {
        p.setting1 = val
    }
    
    if val, ok := config["setting2"].(int); ok {
        p.setting2 = val
    }
    
    p.config = config
    p.logger.Info("Configuration updated", zap.Any("new_config", config))
    return nil
}

func (p *MyConfigurablePlugin) GetConfig() map[string]interface{} {
    return p.config
}

func (p *MyConfigurablePlugin) ValidateConfig(config map[string]interface{}) error {
    if setting1, exists := config["setting1"]; exists {
        if _, ok := setting1.(string); !ok {
            return fmt.Errorf("setting1 must be a string")
        }
    }
    
    if setting2, exists := config["setting2"]; exists {
        if val, ok := setting2.(int); !ok {
            return fmt.Errorf("setting2 must be an integer")
        } else if val < 0 {
            return fmt.Errorf("setting2 must be non-negative")
        }
    }
    
    return nil
}
```

### Health Check Plugin

```go
type MyHealthCheckPlugin struct {
    *MyPlugin
    healthy bool
}

func (p *MyHealthCheckPlugin) HealthCheck(ctx context.Context) plugins.HealthStatus {
    // Perform health check logic
    // This might check database connections, external services, etc.
    
    status := plugins.HealthStatus{
        Healthy:   p.healthy,
        LastCheck: time.Now(),
        Details: map[string]interface{}{
            "plugin_name": p.name,
            "uptime":     time.Since(p.startTime),
        },
    }
    
    if p.healthy {
        status.Message = "Plugin is healthy"
    } else {
        status.Message = "Plugin is experiencing issues"
        status.Details["error"] = "Database connection failed"
    }
    
    return status
}
```

## Configuration

The plugin manager integrates with the existing configuration system:

```yaml
plugins:
  - name: "auth-middleware"
    enabled: true
    config:
      secret_key: "your-secret-key"
      algorithm: "HS256"
      
  - name: "rate-limiter"
    enabled: true
    config:
      requests_per_minute: 100
      burst_size: 10
      
  - name: "cors-middleware"
    enabled: false
    config:
      allowed_origins: ["*"]
      allowed_methods: ["GET", "POST", "PUT", "DELETE"]
```

## Error Handling

The plugin manager provides comprehensive error handling:

```go
// All operations return detailed plugin errors
err := manager.LoadPlugin("my-plugin", config)
if err != nil {
    var pluginErr *plugins.PluginError
    if errors.As(err, &pluginErr) {
        fmt.Printf("Plugin error: %s, Operation: %s, Message: %s\n",
            pluginErr.PluginName, pluginErr.Operation, pluginErr.Message)
    }
}

// Common error types
- ErrPluginNotFound
- ErrPluginAlreadyExists
- ErrPluginInitFailed
- ErrPluginConfigInvalid
- ErrPluginNotEnabled
- ErrPluginTimeout
```

## Thread Safety

The plugin manager is fully thread-safe:

- All operations use appropriate locking mechanisms
- Plugin state changes are atomic
- Metrics updates are performed safely
- Health checks run in separate goroutines

## Performance Considerations

- Plugin operations are optimized for high-concurrency scenarios
- Middleware processing includes panic recovery
- Health checks have configurable intervals
- Metrics collection is lightweight and non-blocking
- Plugin lookup operations use read-write mutexes for optimal read performance

## Testing

The plugin manager includes comprehensive tests covering:

- Basic plugin lifecycle operations
- Middleware integration and execution order
- Configuration management and hot reloading
- Health checking functionality
- Error handling scenarios
- Concurrent access patterns
- Performance benchmarks

Run tests with:
```bash
go test ./pkg/plugins/ -v
```

## Integration with FastHTTP

The plugin manager seamlessly integrates with the existing FastHTTP-based system:

```go
// Create middleware stack
middlewareFunc := manager.CreateMiddlewareFunc()

// Apply to FastHTTP server
server := &fasthttp.Server{
    Handler: middlewareFunc(yourMainHandler),
}
```

Plugins process requests in priority order during the pre-processing phase and in reverse order during post-processing, ensuring proper middleware behavior.