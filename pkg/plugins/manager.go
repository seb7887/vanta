package plugins

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
	"vanta/pkg/config"
)

// PluginState represents the current state of a plugin
type PluginState string

const (
	StateUnloaded PluginState = "unloaded"
	StateLoaded   PluginState = "loaded"
	StateEnabled  PluginState = "enabled"
	StateDisabled PluginState = "disabled"
	StateError    PluginState = "error"
)

// PluginInfo contains metadata and status information about a plugin
type PluginInfo struct {
	Name         string                 `json:"name"`
	Version      string                 `json:"version"`
	Description  string                 `json:"description"`
	State        PluginState            `json:"state"`
	Config       map[string]interface{} `json:"config"`
	LoadedAt     time.Time              `json:"loaded_at"`
	LastError    string                 `json:"last_error,omitempty"`
	Metrics      PluginMetrics          `json:"metrics"`
	Health       *HealthStatus          `json:"health,omitempty"`
	Dependencies []string               `json:"dependencies"`
}

// PluginMetrics contains performance and usage metrics for a plugin
type PluginMetrics struct {
	RequestsProcessed int64         `json:"requests_processed"`
	ErrorCount        int64         `json:"error_count"`
	AverageLatency    time.Duration `json:"average_latency"`
	LastUsed          time.Time     `json:"last_used"`
	TotalLatency      time.Duration `json:"total_latency"`
}

// PluginFactory is a function that creates a new plugin instance
type PluginFactory func() Plugin

// PluginRegistry maintains a registry of available plugin factories
type PluginRegistry struct {
	factories map[string]PluginFactory
	mu        sync.RWMutex
}

// NewPluginRegistry creates a new plugin registry
func NewPluginRegistry() *PluginRegistry {
	return &PluginRegistry{
		factories: make(map[string]PluginFactory),
	}
}

// RegisterPlugin registers a plugin factory with the registry
func (r *PluginRegistry) RegisterPlugin(name string, factory PluginFactory) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	if _, exists := r.factories[name]; exists {
		return NewPluginError(name, "register", "plugin already registered", ErrPluginAlreadyExists)
	}
	
	r.factories[name] = factory
	return nil
}

// UnregisterPlugin removes a plugin factory from the registry
func (r *PluginRegistry) UnregisterPlugin(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	if _, exists := r.factories[name]; !exists {
		return NewPluginError(name, "unregister", "plugin not found", ErrPluginNotFound)
	}
	
	delete(r.factories, name)
	return nil
}

// GetFactory returns a plugin factory by name
func (r *PluginRegistry) GetFactory(name string) (PluginFactory, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	factory, exists := r.factories[name]
	return factory, exists
}

// ListFactories returns a list of all registered plugin names
func (r *PluginRegistry) ListFactories() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	names := make([]string, 0, len(r.factories))
	for name := range r.factories {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// pluginEntry represents an active plugin instance with its metadata
type pluginEntry struct {
	plugin      Plugin
	state       PluginState
	config      map[string]interface{}
	loadedAt    time.Time
	lastError   string
	metrics     PluginMetrics
	health      *HealthStatus
	healthTimer *time.Timer
	dependencies []string
	mu          sync.RWMutex
}

// Manager manages the lifecycle of plugins and provides thread-safe operations
type Manager struct {
	plugins      map[string]*pluginEntry
	registry     *PluginRegistry
	logger       *zap.Logger
	shutdownCtx  context.Context
	shutdownFunc context.CancelFunc
	healthCheck  struct {
		interval time.Duration
		enabled  bool
	}
	mu               sync.RWMutex
	metricsCollector MetricsCollector
}

// MetricsCollector interface for collecting plugin operation metrics
type MetricsCollector interface {
	IncPluginOperation(pluginName, operation string, success bool)
	ObservePluginLatency(pluginName, operation string, duration time.Duration)
	SetPluginState(pluginName string, state string)
	IncPluginError(pluginName string, errorType string)
}

// DefaultMetricsCollector provides a basic metrics implementation
type DefaultMetricsCollector struct {
	operations map[string]int64
	latencies  map[string][]time.Duration
	states     map[string]string
	errors     map[string]int64
	mu         sync.RWMutex
}

// NewDefaultMetricsCollector creates a new default metrics collector
func NewDefaultMetricsCollector() *DefaultMetricsCollector {
	return &DefaultMetricsCollector{
		operations: make(map[string]int64),
		latencies:  make(map[string][]time.Duration),
		states:     make(map[string]string),
		errors:     make(map[string]int64),
	}
}

func (m *DefaultMetricsCollector) IncPluginOperation(pluginName, operation string, success bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := fmt.Sprintf("%s_%s_%t", pluginName, operation, success)
	m.operations[key]++
}

func (m *DefaultMetricsCollector) ObservePluginLatency(pluginName, operation string, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := fmt.Sprintf("%s_%s", pluginName, operation)
	m.latencies[key] = append(m.latencies[key], duration)
}

func (m *DefaultMetricsCollector) SetPluginState(pluginName string, state string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.states[pluginName] = state
}

func (m *DefaultMetricsCollector) IncPluginError(pluginName string, errorType string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := fmt.Sprintf("%s_%s", pluginName, errorType)
	m.errors[key]++
}

// NewManager creates a new plugin manager with the specified logger
func NewManager(logger *zap.Logger) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	
	manager := &Manager{
		plugins:      make(map[string]*pluginEntry),
		registry:     NewPluginRegistry(),
		logger:       logger,
		shutdownCtx:  ctx,
		shutdownFunc: cancel,
		metricsCollector: NewDefaultMetricsCollector(),
	}
	
	// Configure health checking
	manager.healthCheck.interval = 30 * time.Second
	manager.healthCheck.enabled = true
	
	// Start background health checking
	go manager.healthCheckLoop()
	
	return manager
}

// SetMetricsCollector sets a custom metrics collector
func (m *Manager) SetMetricsCollector(collector MetricsCollector) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.metricsCollector = collector
}

// SetHealthCheckInterval configures the health check interval
func (m *Manager) SetHealthCheckInterval(interval time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.healthCheck.interval = interval
}

// EnableHealthCheck enables or disables automatic health checking
func (m *Manager) EnableHealthCheck(enabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.healthCheck.enabled = enabled
}

// GetRegistry returns the plugin registry for registering plugin factories
func (m *Manager) GetRegistry() *PluginRegistry {
	return m.registry
}

// LoadPlugin loads and initializes a plugin with the given configuration
func (m *Manager) LoadPlugin(name string, config map[string]interface{}) error {
	start := time.Now()
	defer func() {
		if m.metricsCollector != nil {
			m.metricsCollector.ObservePluginLatency(name, "load", time.Since(start))
		}
	}()
	
	// Check if plugin is already loaded
	m.mu.RLock()
	if _, exists := m.plugins[name]; exists {
		m.mu.RUnlock()
		if m.metricsCollector != nil {
			m.metricsCollector.IncPluginOperation(name, "load", false)
		}
		return NewPluginError(name, "load", "plugin already loaded", ErrPluginAlreadyExists)
	}
	m.mu.RUnlock()
	
	// Get plugin factory
	factory, exists := m.registry.GetFactory(name)
	if !exists {
		if m.metricsCollector != nil {
			m.metricsCollector.IncPluginOperation(name, "load", false)
		}
		return NewPluginError(name, "load", "plugin factory not found", ErrPluginNotFound)
	}
	
	// Create plugin instance
	plugin := factory()
	if plugin == nil {
		if m.metricsCollector != nil {
			m.metricsCollector.IncPluginOperation(name, "load", false)
		}
		return NewPluginError(name, "load", "factory returned nil plugin", ErrPluginInitFailed)
	}
	
	// Validate plugin name matches
	if plugin.Name() != name {
		if m.metricsCollector != nil {
			m.metricsCollector.IncPluginOperation(name, "load", false)
		}
		return NewPluginError(name, "load", 
			fmt.Sprintf("plugin name mismatch: expected %s, got %s", name, plugin.Name()), 
			ErrPluginConfigInvalid)
	}
	
	// Create plugin context
	pluginCtx, cancel := context.WithTimeout(m.shutdownCtx, 30*time.Second)
	defer cancel()
	
	// Initialize plugin
	pluginLogger := m.logger.With(zap.String("plugin", name), zap.String("version", plugin.Version()))
	if err := plugin.Init(pluginCtx, config, pluginLogger); err != nil {
		if m.metricsCollector != nil {
			m.metricsCollector.IncPluginOperation(name, "load", false)
			m.metricsCollector.IncPluginError(name, "init_failed")
		}
		return NewPluginError(name, "load", "plugin initialization failed", err)
	}
	
	// Resolve dependencies if plugin provides them
	var dependencies []string
	if metadataProvider, ok := plugin.(interface{ GetDependencies() []string }); ok {
		dependencies = metadataProvider.GetDependencies()
		if err := m.validateDependencies(dependencies); err != nil {
			// Cleanup plugin
			plugin.Cleanup(pluginCtx)
			if m.metricsCollector != nil {
				m.metricsCollector.IncPluginOperation(name, "load", false)
			}
			return NewPluginError(name, "load", "dependency validation failed", err)
		}
	}
	
	// Create plugin entry
	entry := &pluginEntry{
		plugin:       plugin,
		state:        StateLoaded,
		config:       config,
		loadedAt:     time.Now(),
		dependencies: dependencies,
		metrics: PluginMetrics{
			RequestsProcessed: 0,
			ErrorCount:        0,
			AverageLatency:    0,
			LastUsed:          time.Now(),
		},
	}
	
	// Add to plugins map
	m.mu.Lock()
	m.plugins[name] = entry
	m.mu.Unlock()
	
	// Update metrics
	if m.metricsCollector != nil {
		m.metricsCollector.IncPluginOperation(name, "load", true)
		m.metricsCollector.SetPluginState(name, string(StateLoaded))
	}
	
	m.logger.Info("Plugin loaded successfully",
		zap.String("plugin", name),
		zap.String("version", plugin.Version()),
		zap.Duration("load_time", time.Since(start)))
	
	return nil
}

// UnloadPlugin gracefully shuts down and removes a plugin
func (m *Manager) UnloadPlugin(name string) error {
	start := time.Now()
	defer func() {
		if m.metricsCollector != nil {
			m.metricsCollector.ObservePluginLatency(name, "unload", time.Since(start))
		}
	}()
	
	m.mu.Lock()
	entry, exists := m.plugins[name]
	if !exists {
		m.mu.Unlock()
		if m.metricsCollector != nil {
			m.metricsCollector.IncPluginOperation(name, "unload", false)
		}
		return NewPluginError(name, "unload", "plugin not found", ErrPluginNotFound)
	}
	
	// Remove from map first to prevent new requests
	delete(m.plugins, name)
	m.mu.Unlock()
	
	// Cancel health check timer if exists
	entry.mu.Lock()
	if entry.healthTimer != nil {
		entry.healthTimer.Stop()
	}
	entry.mu.Unlock()
	
	// Cleanup plugin
	cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	if err := entry.plugin.Cleanup(cleanupCtx); err != nil {
		m.logger.Warn("Plugin cleanup failed",
			zap.String("plugin", name),
			zap.Error(err))
		if m.metricsCollector != nil {
			m.metricsCollector.IncPluginError(name, "cleanup_failed")
		}
		// Don't return error as plugin is already removed from active set
	}
	
	// Update metrics
	if m.metricsCollector != nil {
		m.metricsCollector.IncPluginOperation(name, "unload", true)
		m.metricsCollector.SetPluginState(name, string(StateUnloaded))
	}
	
	m.logger.Info("Plugin unloaded successfully",
		zap.String("plugin", name),
		zap.Duration("unload_time", time.Since(start)))
	
	return nil
}

// GetPlugin retrieves a plugin by name
func (m *Manager) GetPlugin(name string) (Plugin, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	entry, exists := m.plugins[name]
	if !exists || entry.state != StateEnabled {
		return nil, false
	}
	
	// Update last used time
	entry.mu.Lock()
	entry.metrics.LastUsed = time.Now()
	entry.mu.Unlock()
	
	return entry.plugin, true
}

// ListPlugins returns information about all plugins
func (m *Manager) ListPlugins() []PluginInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	infos := make([]PluginInfo, 0, len(m.plugins))
	for name, entry := range m.plugins {
		entry.mu.RLock()
		info := PluginInfo{
			Name:         name,
			Version:      entry.plugin.Version(),
			Description:  entry.plugin.Description(),
			State:        entry.state,
			Config:       entry.config,
			LoadedAt:     entry.loadedAt,
			LastError:    entry.lastError,
			Metrics:      entry.metrics,
			Health:       entry.health,
			Dependencies: entry.dependencies,
		}
		entry.mu.RUnlock()
		infos = append(infos, info)
	}
	
	// Sort by name for consistent output
	sort.Slice(infos, func(i, j int) bool {
		return infos[i].Name < infos[j].Name
	})
	
	return infos
}

// EnablePlugin enables a loaded plugin
func (m *Manager) EnablePlugin(name string) error {
	start := time.Now()
	defer func() {
		if m.metricsCollector != nil {
			m.metricsCollector.ObservePluginLatency(name, "enable", time.Since(start))
		}
	}()
	
	m.mu.RLock()
	entry, exists := m.plugins[name]
	m.mu.RUnlock()
	
	if !exists {
		if m.metricsCollector != nil {
			m.metricsCollector.IncPluginOperation(name, "enable", false)
		}
		return NewPluginError(name, "enable", "plugin not found", ErrPluginNotFound)
	}
	
	entry.mu.Lock()
	defer entry.mu.Unlock()
	
	if entry.state == StateEnabled {
		if m.metricsCollector != nil {
			m.metricsCollector.IncPluginOperation(name, "enable", true)
		}
		return nil // Already enabled
	}
	
	if entry.state != StateLoaded && entry.state != StateDisabled {
		if m.metricsCollector != nil {
			m.metricsCollector.IncPluginOperation(name, "enable", false)
		}
		return NewPluginError(name, "enable", 
			fmt.Sprintf("cannot enable plugin in state %s", entry.state), 
			ErrPluginNotEnabled)
	}
	
	// Check dependencies
	if err := m.validateEnabledDependencies(entry.dependencies); err != nil {
		if m.metricsCollector != nil {
			m.metricsCollector.IncPluginOperation(name, "enable", false)
		}
		return NewPluginError(name, "enable", "dependency check failed", err)
	}
	
	entry.state = StateEnabled
	entry.lastError = ""
	
	// Update metrics
	if m.metricsCollector != nil {
		m.metricsCollector.IncPluginOperation(name, "enable", true)
		m.metricsCollector.SetPluginState(name, string(StateEnabled))
	}
	
	m.logger.Info("Plugin enabled",
		zap.String("plugin", name),
		zap.Duration("enable_time", time.Since(start)))
	
	return nil
}

// DisablePlugin disables an enabled plugin without unloading it
func (m *Manager) DisablePlugin(name string) error {
	start := time.Now()
	defer func() {
		if m.metricsCollector != nil {
			m.metricsCollector.ObservePluginLatency(name, "disable", time.Since(start))
		}
	}()
	
	m.mu.RLock()
	entry, exists := m.plugins[name]
	m.mu.RUnlock()
	
	if !exists {
		if m.metricsCollector != nil {
			m.metricsCollector.IncPluginOperation(name, "disable", false)
		}
		return NewPluginError(name, "disable", "plugin not found", ErrPluginNotFound)
	}
	
	entry.mu.Lock()
	defer entry.mu.Unlock()
	
	if entry.state == StateDisabled {
		if m.metricsCollector != nil {
			m.metricsCollector.IncPluginOperation(name, "disable", true)
		}
		return nil // Already disabled
	}
	
	entry.state = StateDisabled
	
	// Update metrics
	if m.metricsCollector != nil {
		m.metricsCollector.IncPluginOperation(name, "disable", true)
		m.metricsCollector.SetPluginState(name, string(StateDisabled))
	}
	
	m.logger.Info("Plugin disabled",
		zap.String("plugin", name),
		zap.Duration("disable_time", time.Since(start)))
	
	return nil
}

// ReloadPlugin reloads a plugin's configuration
func (m *Manager) ReloadPlugin(name string, config map[string]interface{}) error {
	start := time.Now()
	defer func() {
		if m.metricsCollector != nil {
			m.metricsCollector.ObservePluginLatency(name, "reload", time.Since(start))
		}
	}()
	
	m.mu.RLock()
	entry, exists := m.plugins[name]
	m.mu.RUnlock()
	
	if !exists {
		if m.metricsCollector != nil {
			m.metricsCollector.IncPluginOperation(name, "reload", false)
		}
		return NewPluginError(name, "reload", "plugin not found", ErrPluginNotFound)
	}
	
	// Check if plugin supports hot reloading
	if hotReloadable, ok := entry.plugin.(HotReloadable); ok {
		if !hotReloadable.CanReload() {
			if m.metricsCollector != nil {
				m.metricsCollector.IncPluginOperation(name, "reload", false)
			}
			return NewPluginError(name, "reload", "plugin cannot be reloaded at this time", nil)
		}
		
		reloadCtx, cancel := context.WithTimeout(m.shutdownCtx, 30*time.Second)
		defer cancel()
		
		if err := hotReloadable.Reload(reloadCtx, config); err != nil {
			entry.mu.Lock()
			entry.lastError = err.Error()
			atomic.AddInt64(&entry.metrics.ErrorCount, 1)
			entry.mu.Unlock()
			
			if m.metricsCollector != nil {
				m.metricsCollector.IncPluginOperation(name, "reload", false)
				m.metricsCollector.IncPluginError(name, "reload_failed")
			}
			return NewPluginError(name, "reload", "hot reload failed", err)
		}
		
		entry.mu.Lock()
		entry.config = config
		entry.lastError = ""
		entry.mu.Unlock()
		
		if m.metricsCollector != nil {
			m.metricsCollector.IncPluginOperation(name, "reload", true)
		}
		
		m.logger.Info("Plugin hot reloaded",
			zap.String("plugin", name),
			zap.Duration("reload_time", time.Since(start)))
		
		return nil
	}
	
	// Fallback to unload/load cycle
	originalState := entry.state
	
	// Disable first
	if err := m.DisablePlugin(name); err != nil {
		if m.metricsCollector != nil {
			m.metricsCollector.IncPluginOperation(name, "reload", false)
		}
		return NewPluginError(name, "reload", "failed to disable for reload", err)
	}
	
	// Unload
	if err := m.UnloadPlugin(name); err != nil {
		if m.metricsCollector != nil {
			m.metricsCollector.IncPluginOperation(name, "reload", false)
		}
		return NewPluginError(name, "reload", "failed to unload for reload", err)
	}
	
	// Reload
	if err := m.LoadPlugin(name, config); err != nil {
		if m.metricsCollector != nil {
			m.metricsCollector.IncPluginOperation(name, "reload", false)
		}
		return NewPluginError(name, "reload", "failed to load after reload", err)
	}
	
	// Restore original state if it was enabled
	if originalState == StateEnabled {
		if err := m.EnablePlugin(name); err != nil {
			m.logger.Warn("Failed to re-enable plugin after reload",
				zap.String("plugin", name),
				zap.Error(err))
		}
	}
	
	if m.metricsCollector != nil {
		m.metricsCollector.IncPluginOperation(name, "reload", true)
	}
	
	m.logger.Info("Plugin reloaded via unload/load cycle",
		zap.String("plugin", name),
		zap.Duration("reload_time", time.Since(start)))
	
	return nil
}

// GetMiddlewares returns all enabled middleware plugins sorted by priority
func (m *Manager) GetMiddlewares() []Middleware {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	var middlewares []Middleware
	for _, entry := range m.plugins {
		if entry.state == StateEnabled {
			if middleware, ok := entry.plugin.(Middleware); ok {
				middlewares = append(middlewares, middleware)
			}
		}
	}
	
	// Sort by priority (lower values = higher priority)
	sort.Slice(middlewares, func(i, j int) bool {
		return middlewares[i].Priority() < middlewares[j].Priority()
	})
	
	return middlewares
}

// CreateMiddlewareFunc creates a FastHTTP middleware function from plugin middlewares
func (m *Manager) CreateMiddlewareFunc() func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
	return func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
		return func(ctx *fasthttp.RequestCtx) {
			middlewares := m.GetMiddlewares()
			
			// Create request context
			requestCtx := &RequestContext{
				RequestCtx: ctx,
				RequestID:  m.getRequestID(ctx),
				StartTime:  time.Now(),
				UserValues: make(map[string]interface{}),
				PluginData: make(map[string]interface{}),
				Logger:     m.logger.With(zap.String("request_id", m.getRequestID(ctx))),
				Context:    context.Background(),
			}
			
			// Process middleware chain
			m.processMiddlewareChain(middlewares, requestCtx, next)
		}
	}
}

// processMiddlewareChain processes the middleware chain with proper error handling
func (m *Manager) processMiddlewareChain(middlewares []Middleware, requestCtx *RequestContext, handler fasthttp.RequestHandler) {
	// Pre-process phase
	for _, middleware := range middlewares {
		if !middleware.ShouldApply(requestCtx.RequestCtx) {
			continue
		}
		
		start := time.Now()
		shouldContinue, err := m.safePreProcess(middleware, requestCtx)
		
		// Update plugin metrics
		pluginName := middleware.Name()
		m.updatePluginMetrics(pluginName, time.Since(start), err)
		
		if err != nil {
			m.logger.Error("Middleware pre-processing failed",
				zap.String("plugin", pluginName),
				zap.Error(err))
			requestCtx.RequestCtx.SetStatusCode(fasthttp.StatusInternalServerError)
			return
		}
		
		if !shouldContinue {
			return // Middleware short-circuited the request
		}
	}
	
	// Execute main handler
	handler(requestCtx.RequestCtx)
	
	// Create response context
	responseCtx := &ResponseContext{
		RequestContext:  requestCtx,
		ProcessingTime:  time.Since(requestCtx.StartTime),
		ResponseBody:    requestCtx.RequestCtx.Response.Body(),
		ProcessingError: nil,
	}
	
	// Post-process phase (reverse order)
	for i := len(middlewares) - 1; i >= 0; i-- {
		middleware := middlewares[i]
		if !middleware.ShouldApply(requestCtx.RequestCtx) {
			continue
		}
		
		start := time.Now()
		err := m.safePostProcess(middleware, responseCtx)
		
		// Update plugin metrics
		pluginName := middleware.Name()
		m.updatePluginMetrics(pluginName, time.Since(start), err)
		
		if err != nil {
			m.logger.Error("Middleware post-processing failed",
				zap.String("plugin", pluginName),
				zap.Error(err))
			// Don't modify response on post-process errors
		}
	}
}

// safePreProcess safely executes middleware pre-processing with panic recovery
func (m *Manager) safePreProcess(middleware Middleware, ctx *RequestContext) (shouldContinue bool, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic in middleware pre-process: %v", r)
			shouldContinue = false
			
			if m.metricsCollector != nil {
				m.metricsCollector.IncPluginError(middleware.Name(), "panic")
			}
		}
	}()
	
	return middleware.PreProcess(ctx)
}

// safePostProcess safely executes middleware post-processing with panic recovery
func (m *Manager) safePostProcess(middleware Middleware, ctx *ResponseContext) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic in middleware post-process: %v", r)
			
			if m.metricsCollector != nil {
				m.metricsCollector.IncPluginError(middleware.Name(), "panic")
			}
		}
	}()
	
	return middleware.PostProcess(ctx)
}

// updatePluginMetrics updates metrics for a plugin operation
func (m *Manager) updatePluginMetrics(pluginName string, duration time.Duration, err error) {
	m.mu.RLock()
	entry, exists := m.plugins[pluginName]
	m.mu.RUnlock()
	
	if !exists {
		return
	}
	
	entry.mu.Lock()
	defer entry.mu.Unlock()
	
	atomic.AddInt64(&entry.metrics.RequestsProcessed, 1)
	entry.metrics.TotalLatency += duration
	entry.metrics.AverageLatency = time.Duration(int64(entry.metrics.TotalLatency) / entry.metrics.RequestsProcessed)
	entry.metrics.LastUsed = time.Now()
	
	if err != nil {
		atomic.AddInt64(&entry.metrics.ErrorCount, 1)
		entry.lastError = err.Error()
	}
}

// getRequestID extracts or generates a request ID
func (m *Manager) getRequestID(ctx *fasthttp.RequestCtx) string {
	if val := ctx.UserValue("request_id"); val != nil {
		return val.(string)
	}
	return "unknown"
}

// LoadFromConfig loads plugins from configuration
func (m *Manager) LoadFromConfig(pluginConfigs []config.PluginConfig) error {
	var loadErrors []error
	
	for _, pluginConfig := range pluginConfigs {
		if err := m.LoadPlugin(pluginConfig.Name, pluginConfig.Config); err != nil {
			loadErrors = append(loadErrors, err)
			continue
		}
		
		if pluginConfig.Enabled {
			if err := m.EnablePlugin(pluginConfig.Name); err != nil {
				loadErrors = append(loadErrors, err)
			}
		}
	}
	
	if len(loadErrors) > 0 {
		return fmt.Errorf("failed to load %d plugins: %v", len(loadErrors), loadErrors)
	}
	
	return nil
}

// validateDependencies validates that all required dependencies exist and are registered
func (m *Manager) validateDependencies(dependencies []string) error {
	for _, dep := range dependencies {
		if _, exists := m.registry.GetFactory(dep); !exists {
			return fmt.Errorf("dependency not found: %s", dep)
		}
	}
	return nil
}

// validateEnabledDependencies validates that all dependencies are enabled
func (m *Manager) validateEnabledDependencies(dependencies []string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	for _, dep := range dependencies {
		entry, exists := m.plugins[dep]
		if !exists {
			return fmt.Errorf("dependency not loaded: %s", dep)
		}
		if entry.state != StateEnabled {
			return fmt.Errorf("dependency not enabled: %s (state: %s)", dep, entry.state)
		}
	}
	return nil
}

// healthCheckLoop runs periodic health checks on plugins
func (m *Manager) healthCheckLoop() {
	ticker := time.NewTicker(m.healthCheck.interval)
	defer ticker.Stop()
	
	for {
		select {
		case <-m.shutdownCtx.Done():
			return
		case <-ticker.C:
			if m.healthCheck.enabled {
				m.performHealthChecks()
			}
		}
	}
}

// performHealthChecks performs health checks on all enabled plugins that support it
func (m *Manager) performHealthChecks() {
	m.mu.RLock()
	plugins := make([]*pluginEntry, 0, len(m.plugins))
	for _, entry := range m.plugins {
		if entry.state == StateEnabled {
			plugins = append(plugins, entry)
		}
	}
	m.mu.RUnlock()
	
	for _, entry := range plugins {
		if checker, ok := entry.plugin.(HealthChecker); ok {
			go m.performPluginHealthCheck(entry, checker)
		}
	}
}

// performPluginHealthCheck performs a health check on a single plugin
func (m *Manager) performPluginHealthCheck(entry *pluginEntry, checker HealthChecker) {
	ctx, cancel := context.WithTimeout(m.shutdownCtx, 10*time.Second)
	defer cancel()
	
	health := checker.HealthCheck(ctx)
	
	entry.mu.Lock()
	entry.health = &health
	if !health.Healthy {
		entry.lastError = health.Message
		if entry.state == StateEnabled {
			entry.state = StateError
		}
		if m.metricsCollector != nil {
			m.metricsCollector.IncPluginError(entry.plugin.Name(), "health_check_failed")
			m.metricsCollector.SetPluginState(entry.plugin.Name(), string(StateError))
		}
	}
	entry.mu.Unlock()
	
	if !health.Healthy {
		m.logger.Warn("Plugin health check failed",
			zap.String("plugin", entry.plugin.Name()),
			zap.String("message", health.Message))
	}
}

// Shutdown gracefully shuts down all plugins
func (m *Manager) Shutdown() error {
	m.logger.Info("Shutting down plugin manager")
	
	// Cancel shutdown context to stop health checks
	m.shutdownFunc()
	
	// Get all plugins
	m.mu.RLock()
	pluginNames := make([]string, 0, len(m.plugins))
	for name := range m.plugins {
		pluginNames = append(pluginNames, name)
	}
	m.mu.RUnlock()
	
	// Unload all plugins
	var errors []error
	for _, name := range pluginNames {
		if err := m.UnloadPlugin(name); err != nil {
			errors = append(errors, err)
		}
	}
	
	if len(errors) > 0 {
		return fmt.Errorf("shutdown completed with %d errors: %v", len(errors), errors)
	}
	
	m.logger.Info("Plugin manager shutdown completed")
	return nil
}

// GetPluginMetrics returns aggregated metrics for all plugins
func (m *Manager) GetPluginMetrics() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	totalPlugins := len(m.plugins)
	enabledPlugins := 0
	totalRequests := int64(0)
	totalErrors := int64(0)
	
	pluginStats := make(map[string]PluginMetrics)
	
	for name, entry := range m.plugins {
		entry.mu.RLock()
		if entry.state == StateEnabled {
			enabledPlugins++
		}
		totalRequests += entry.metrics.RequestsProcessed
		totalErrors += entry.metrics.ErrorCount
		pluginStats[name] = entry.metrics
		entry.mu.RUnlock()
	}
	
	return map[string]interface{}{
		"total_plugins":   totalPlugins,
		"enabled_plugins": enabledPlugins,
		"total_requests":  totalRequests,
		"total_errors":    totalErrors,
		"plugin_stats":    pluginStats,
	}
}