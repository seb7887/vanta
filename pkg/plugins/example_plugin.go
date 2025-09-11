package plugins

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

// ExampleMiddlewarePlugin demonstrates a complete middleware plugin implementation
type ExampleMiddlewarePlugin struct {
	name        string
	version     string
	description string
	logger      *zap.Logger
	config      map[string]interface{}
	
	// Plugin-specific configuration
	headerName string
	headerValue string
	priority   Priority
}

// NewExampleMiddlewarePlugin creates a new example middleware plugin
func NewExampleMiddlewarePlugin() Plugin {
	return &ExampleMiddlewarePlugin{
		name:        "example-middleware",
		version:     "1.0.0",
		description: "Example middleware plugin that adds custom headers",
		priority:    PriorityNormal,
	}
}

// Name returns the plugin name
func (p *ExampleMiddlewarePlugin) Name() string {
	return p.name
}

// Version returns the plugin version
func (p *ExampleMiddlewarePlugin) Version() string {
	return p.version
}

// Description returns the plugin description
func (p *ExampleMiddlewarePlugin) Description() string {
	return p.description
}

// Init initializes the plugin with configuration
func (p *ExampleMiddlewarePlugin) Init(ctx context.Context, config map[string]interface{}, logger *zap.Logger) error {
	p.logger = logger
	p.config = config
	
	// Parse configuration
	if headerName, ok := config["header_name"].(string); ok {
		p.headerName = headerName
	} else {
		p.headerName = "X-Example-Plugin"
	}
	
	if headerValue, ok := config["header_value"].(string); ok {
		p.headerValue = headerValue
	} else {
		p.headerValue = "processed"
	}
	
	if priority, ok := config["priority"].(int); ok {
		p.priority = Priority(priority)
	}
	
	p.logger.Info("Example middleware plugin initialized",
		zap.String("header_name", p.headerName),
		zap.String("header_value", p.headerValue),
		zap.Int("priority", int(p.priority)))
	
	return nil
}

// Cleanup performs cleanup when the plugin is unloaded
func (p *ExampleMiddlewarePlugin) Cleanup(ctx context.Context) error {
	p.logger.Info("Example middleware plugin cleaned up")
	return nil
}

// Priority returns the execution priority
func (p *ExampleMiddlewarePlugin) Priority() Priority {
	return p.priority
}

// PreProcess processes the request before the main handler
func (p *ExampleMiddlewarePlugin) PreProcess(ctx *RequestContext) (bool, error) {
	start := time.Now()
	
	// Add request processing timestamp
	ctx.SetPluginData(p.name, "start_time", start)
	
	// Add custom header to request
	ctx.RequestCtx.Request.Header.Set("X-Plugin-Processed", "true")
	
	p.logger.Debug("Request pre-processed",
		zap.String("method", ctx.Method()),
		zap.String("path", ctx.Path()),
		zap.String("request_id", ctx.RequestID))
	
	return true, nil // Continue processing
}

// PostProcess processes the response after the main handler
func (p *ExampleMiddlewarePlugin) PostProcess(ctx *ResponseContext) error {
	// Get start time from plugin data
	if startTimeVal, ok := ctx.GetPluginData(p.name, "start_time"); ok {
		if startTime, ok := startTimeVal.(time.Time); ok {
			processingTime := time.Since(startTime)
			
			// Add processing time header
			ctx.RequestCtx.Response.Header.Set("X-Processing-Time", 
				fmt.Sprintf("%.2fms", float64(processingTime.Nanoseconds())/1000000))
		}
	}
	
	// Add custom header to response
	ctx.RequestCtx.Response.Header.Set(p.headerName, p.headerValue)
	
	p.logger.Debug("Response post-processed",
		zap.String("method", ctx.Method()),
		zap.String("path", ctx.Path()),
		zap.Int("status", ctx.RequestCtx.Response.StatusCode()),
		zap.String("request_id", ctx.RequestID))
	
	return nil
}

// ShouldApply determines if this middleware should be applied to the request
func (p *ExampleMiddlewarePlugin) ShouldApply(req *fasthttp.RequestCtx) bool {
	// Apply to all requests except health checks
	path := string(req.Path())
	return !strings.HasPrefix(path, "/health")
}

// ExampleRequestProcessorPlugin demonstrates a request processor plugin
type ExampleRequestProcessorPlugin struct {
	name        string
	version     string
	description string
	logger      *zap.Logger
	config      map[string]interface{}
	
	// Plugin-specific configuration
	allowedMethods []string
	blockedPaths   []string
}

// NewExampleRequestProcessorPlugin creates a new example request processor plugin
func NewExampleRequestProcessorPlugin() Plugin {
	return &ExampleRequestProcessorPlugin{
		name:        "example-request-processor",
		version:     "1.0.0",
		description: "Example request processor plugin for method and path filtering",
	}
}

// Name returns the plugin name
func (p *ExampleRequestProcessorPlugin) Name() string {
	return p.name
}

// Version returns the plugin version
func (p *ExampleRequestProcessorPlugin) Version() string {
	return p.version
}

// Description returns the plugin description
func (p *ExampleRequestProcessorPlugin) Description() string {
	return p.description
}

// Init initializes the plugin with configuration
func (p *ExampleRequestProcessorPlugin) Init(ctx context.Context, config map[string]interface{}, logger *zap.Logger) error {
	p.logger = logger
	p.config = config
	
	// Parse allowed methods
	if methods, ok := config["allowed_methods"].([]interface{}); ok {
		p.allowedMethods = make([]string, len(methods))
		for i, method := range methods {
			if methodStr, ok := method.(string); ok {
				p.allowedMethods[i] = strings.ToUpper(methodStr)
			}
		}
	} else {
		p.allowedMethods = []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"}
	}
	
	// Parse blocked paths
	if paths, ok := config["blocked_paths"].([]interface{}); ok {
		p.blockedPaths = make([]string, len(paths))
		for i, path := range paths {
			if pathStr, ok := path.(string); ok {
				p.blockedPaths[i] = pathStr
			}
		}
	}
	
	p.logger.Info("Example request processor plugin initialized",
		zap.Strings("allowed_methods", p.allowedMethods),
		zap.Strings("blocked_paths", p.blockedPaths))
	
	return nil
}

// Cleanup performs cleanup when the plugin is unloaded
func (p *ExampleRequestProcessorPlugin) Cleanup(ctx context.Context) error {
	p.logger.Info("Example request processor plugin cleaned up")
	return nil
}

// ProcessRequest processes the incoming request
func (p *ExampleRequestProcessorPlugin) ProcessRequest(ctx *RequestContext) (*RequestResult, error) {
	result := NewRequestResult()
	
	method := ctx.Method()
	path := ctx.Path()
	
	// Check if method is allowed
	if !p.SupportsMethod(method) {
		p.logger.Warn("Method not allowed",
			zap.String("method", method),
			zap.String("path", path),
			zap.String("request_id", ctx.RequestID))
		
		return result.Stop(fasthttp.StatusMethodNotAllowed, 
			[]byte(fmt.Sprintf(`{"error": "Method %s not allowed"}`, method))), nil
	}
	
	// Check if path is blocked
	for _, blockedPath := range p.blockedPaths {
		if strings.HasPrefix(path, blockedPath) {
			p.logger.Warn("Path blocked",
				zap.String("method", method),
				zap.String("path", path),
				zap.String("blocked_path", blockedPath),
				zap.String("request_id", ctx.RequestID))
			
			return result.Stop(fasthttp.StatusForbidden,
				[]byte(fmt.Sprintf(`{"error": "Path %s is blocked"}`, path))), nil
		}
	}
	
	// Add metadata for downstream plugins
	result.SetMetadata("allowed_method", method)
	result.SetMetadata("processed_by", p.name)
	
	p.logger.Debug("Request processed successfully",
		zap.String("method", method),
		zap.String("path", path),
		zap.String("request_id", ctx.RequestID))
	
	return result, nil
}

// SupportsMethod returns true if this processor supports the given method
func (p *ExampleRequestProcessorPlugin) SupportsMethod(method string) bool {
	method = strings.ToUpper(method)
	for _, allowedMethod := range p.allowedMethods {
		if allowedMethod == method {
			return true
		}
	}
	return false
}

// SupportsPath returns true if this processor should handle requests to the given path
func (p *ExampleRequestProcessorPlugin) SupportsPath(path string) bool {
	// This processor handles all paths except those explicitly blocked
	for _, blockedPath := range p.blockedPaths {
		if strings.HasPrefix(path, blockedPath) {
			return false
		}
	}
	return true
}

// ExampleConfigurablePlugin demonstrates a plugin with runtime configuration support
type ExampleConfigurablePlugin struct {
	*ExampleMiddlewarePlugin
}

// NewExampleConfigurablePlugin creates a new configurable plugin
func NewExampleConfigurablePlugin() Plugin {
	base := NewExampleMiddlewarePlugin().(*ExampleMiddlewarePlugin)
	base.name = "example-configurable"
	base.description = "Example configurable middleware plugin"
	
	return &ExampleConfigurablePlugin{
		ExampleMiddlewarePlugin: base,
	}
}

// UpdateConfig updates the plugin configuration at runtime
func (p *ExampleConfigurablePlugin) UpdateConfig(ctx context.Context, config map[string]interface{}) error {
	p.logger.Info("Updating plugin configuration", zap.Any("new_config", config))
	
	// Validate configuration first
	if err := p.ValidateConfig(config); err != nil {
		return err
	}
	
	// Update configuration
	if headerName, ok := config["header_name"].(string); ok {
		p.headerName = headerName
	}
	
	if headerValue, ok := config["header_value"].(string); ok {
		p.headerValue = headerValue
	}
	
	if priority, ok := config["priority"].(int); ok {
		p.priority = Priority(priority)
	}
	
	p.config = config
	
	p.logger.Info("Plugin configuration updated successfully",
		zap.String("header_name", p.headerName),
		zap.String("header_value", p.headerValue),
		zap.Int("priority", int(p.priority)))
	
	return nil
}

// GetConfig returns the current plugin configuration
func (p *ExampleConfigurablePlugin) GetConfig() map[string]interface{} {
	return p.config
}

// ValidateConfig validates the provided configuration
func (p *ExampleConfigurablePlugin) ValidateConfig(config map[string]interface{}) error {
	// Validate header_name if provided
	if headerName, exists := config["header_name"]; exists {
		if _, ok := headerName.(string); !ok {
			return fmt.Errorf("header_name must be a string")
		}
	}
	
	// Validate header_value if provided
	if headerValue, exists := config["header_value"]; exists {
		if _, ok := headerValue.(string); !ok {
			return fmt.Errorf("header_value must be a string")
		}
	}
	
	// Validate priority if provided
	if priority, exists := config["priority"]; exists {
		if priorityInt, ok := priority.(int); !ok {
			return fmt.Errorf("priority must be an integer")
		} else if priorityInt < 0 || priorityInt > 1000 {
			return fmt.Errorf("priority must be between 0 and 1000")
		}
	}
	
	return nil
}

// ExampleHealthCheckPlugin demonstrates a plugin with health checking
type ExampleHealthCheckPlugin struct {
	*ExampleMiddlewarePlugin
	healthy bool
}

// NewExampleHealthCheckPlugin creates a new health check plugin
func NewExampleHealthCheckPlugin() Plugin {
	base := NewExampleMiddlewarePlugin().(*ExampleMiddlewarePlugin)
	base.name = "example-health-check"
	base.description = "Example plugin with health checking"
	
	return &ExampleHealthCheckPlugin{
		ExampleMiddlewarePlugin: base,
		healthy:                 true,
	}
}

// HealthCheck performs a health check and returns the status
func (p *ExampleHealthCheckPlugin) HealthCheck(ctx context.Context) HealthStatus {
	start := time.Now()
	
	// Simulate health check logic
	// In a real plugin, this might check database connections, external services, etc.
	
	status := HealthStatus{
		Healthy:   p.healthy,
		LastCheck: start,
		Details: map[string]interface{}{
			"plugin_name":    p.name,
			"plugin_version": p.version,
			"uptime":         time.Since(start),
		},
	}
	
	if p.healthy {
		status.Message = "Plugin is healthy"
	} else {
		status.Message = "Plugin is experiencing issues"
		status.Details["error"] = "Simulated health check failure"
	}
	
	p.logger.Debug("Health check completed",
		zap.Bool("healthy", status.Healthy),
		zap.String("message", status.Message))
	
	return status
}

// SetHealthy sets the health status (for testing purposes)
func (p *ExampleHealthCheckPlugin) SetHealthy(healthy bool) {
	p.healthy = healthy
}