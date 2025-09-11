package api

import (
	"fmt"
	"sync"
	"time"

	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
	"vanta/pkg/chaos"
	"vanta/pkg/config"
	"vanta/pkg/openapi"
	"vanta/pkg/plugins"
	"vanta/pkg/recorder"
)

// Server represents the HTTP server
type Server struct {
	config           *config.ServerConfig
	fullConfig       *config.Config  // Added for hot reload
	router           *Router
	server           *fasthttp.Server
	logger           *zap.Logger
	spec             *openapi.Specification
	generator        openapi.DataGenerator
	metricsCollector *DefaultMetricsCollector
	chaosEngine      chaos.ChaosEngine
	recordingEngine  recorder.RecordingEngine
	pluginsManager   *plugins.Manager
	
	// Hot reload support
	mu       sync.RWMutex
	running  bool
	startTime time.Time
}

// NewServer creates a new HTTP server instance
func NewServer(cfg *config.Config, spec *openapi.Specification, logger *zap.Logger) (*Server, error) {
	if cfg == nil {
		return nil, fmt.Errorf("configuration cannot be nil")
	}
	if spec == nil {
		return nil, fmt.Errorf("specification cannot be nil")
	}
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}

	// Initialize data generator with mock configuration
	var generator openapi.DataGenerator
	if cfg.Mock.Seed != 0 {
		generator = openapi.NewDefaultDataGeneratorWithSeed(cfg.Mock.Seed)
	} else {
		generator = openapi.NewDefaultDataGenerator()
	}
	
	// Configure generator
	if cfg.Mock.Locale != "" {
		generator.SetLocale(cfg.Mock.Locale)
	}

	// Create router with generator
	router, err := NewRouterWithGenerator(spec, generator, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create router: %w", err)
	}

	// Create metrics collector if enabled
	var metricsCollector *DefaultMetricsCollector
	if cfg.Metrics.Enabled {
		metricsCollector = NewDefaultMetricsCollector()
	}

	// Create chaos engine if enabled
	var chaosEngine chaos.ChaosEngine
	if cfg.Chaos.Enabled && len(cfg.Chaos.Scenarios) > 0 {
		chaosEngine = chaos.NewDefaultChaosEngine(logger)
		if err := chaosEngine.LoadScenarios(cfg.Chaos.Scenarios); err != nil {
			logger.Warn("Failed to load chaos scenarios", zap.Error(err))
			chaosEngine = nil
		}
	}

	// Create recording engine if enabled
	var recordingEngine recorder.RecordingEngine
	if cfg.Recording.Enabled {
		storage, err := recorder.NewFileStorage(&cfg.Recording.Storage, logger)
		if err != nil {
			logger.Warn("Failed to create recording storage", zap.Error(err))
		} else {
			recordingEngine = recorder.NewDefaultRecordingEngine(storage, logger)
			if err := recordingEngine.Start(&cfg.Recording); err != nil {
				logger.Warn("Failed to start recording engine", zap.Error(err))
				recordingEngine = nil
			}
		}
	}

	// Create and configure plugin manager
	pluginsManager := plugins.NewManager(logger)
	
	// Set metrics collector for plugins if available
	if metricsCollector != nil {
		// Create a plugin metrics adapter that wraps the existing metrics collector
		pluginMetricsCollector := NewPluginMetricsAdapter(metricsCollector, logger)
		pluginsManager.SetMetricsCollector(pluginMetricsCollector)
	}
	
	// Register built-in plugins
	if err := plugins.RegisterBuiltinPlugins(pluginsManager.GetRegistry()); err != nil {
		return nil, fmt.Errorf("failed to register built-in plugins: %w", err)
	}
	
	// Load plugins from configuration
	if len(cfg.Plugins) > 0 {
		if err := pluginsManager.LoadFromConfig(cfg.Plugins); err != nil {
			logger.Warn("Failed to load plugins from configuration", zap.Error(err))
			// Continue with server creation even if some plugins fail to load
		}
	}

	// Create and configure middleware stack
	stack := NewStack()

	// Add middleware in proper order:
	// Request ID → Auth → Rate Limit → CORS → Logger → Recovery → Chaos → Metrics → Recording → Logging
	
	// 1. Request ID middleware (highest priority)
	if cfg.Middleware.RequestID {
		stack.Use(RequestID(true))
	}

	// 2. Plugin middleware (Auth, Rate Limit, CORS plugins with priority ordering)
	if pluginsManager != nil {
		stack.Use(pluginsManager.CreateMiddlewareFunc())
	}

	// 3. Legacy middleware (if not replaced by plugins)
	if !hasPluginEnabled(cfg.Plugins, "auth") {
		// No auth plugin, continue without authentication
	}
	
	if !hasPluginEnabled(cfg.Plugins, "cors") && cfg.Middleware.CORS.Enabled {
		stack.Use(CORS(&cfg.Middleware.CORS))
	}

	// 4. Logger middleware (always added for request logging)
	stack.Use(Logger(logger, &cfg.Logging))

	// 5. Recovery middleware
	if cfg.Middleware.Recovery.Enabled {
		stack.Use(Recovery(logger, &cfg.Middleware.Recovery))
	}

	// 6. Timeout middleware
	if cfg.Middleware.Timeout.Enabled {
		stack.Use(Timeout(&cfg.Middleware.Timeout))
	}

	// 7. Chaos middleware (before metrics to capture chaos effects)
	if chaosEngine != nil {
		stack.Use(Chaos(chaosEngine, logger))
	}

	// 8. Metrics middleware
	if cfg.Metrics.Enabled && metricsCollector != nil {
		stack.Use(Metrics(&cfg.Metrics, metricsCollector))
	}

	// 9. Recording middleware (after metrics to capture complete response)
	if recordingEngine != nil {
		stack.Use(Recording(recordingEngine, logger))
	}

	// Apply middleware stack to router
	finalHandler := stack.Apply(router.Handler)

	// Create FastHTTP server with configuration
	server := &fasthttp.Server{
		Handler:               finalHandler,
		ReadTimeout:           cfg.Server.ReadTimeout,
		WriteTimeout:          cfg.Server.WriteTimeout,
		MaxConnsPerIP:         cfg.Server.MaxConnsPerIP,
		Concurrency:          cfg.Server.Concurrency,
		DisableKeepalive:     false,
		DisablePreParseMultipartForm: false,
		LogAllErrors:         false,
		ErrorHandler: func(ctx *fasthttp.RequestCtx, err error) {
			logger.Error("FastHTTP error", 
				zap.Error(err),
				zap.String("path", string(ctx.Path())),
				zap.String("method", string(ctx.Method())),
			)
		},
	}

	return &Server{
		config:           &cfg.Server,
		fullConfig:       cfg,  // Store full config for hot reload
		router:           router,
		server:           server,
		logger:           logger,
		spec:             spec,
		generator:        generator,
		metricsCollector: metricsCollector,
		chaosEngine:      chaosEngine,
		recordingEngine:  recordingEngine,
		pluginsManager:   pluginsManager,
	}, nil
}

// Start starts the HTTP server
func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.running {
		return fmt.Errorf("server is already running")
	}
	
	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
	
	s.logger.Info("Starting HTTP server",
		zap.String("address", addr),
		zap.Int("concurrency", s.config.Concurrency),
		zap.Duration("read_timeout", s.config.ReadTimeout),
		zap.Duration("write_timeout", s.config.WriteTimeout),
	)

	s.running = true
	s.startTime = time.Now()
	
	// Start server in goroutine to allow non-blocking start
	go func() {
		defer func() {
			s.mu.Lock()
			s.running = false
			s.mu.Unlock()
		}()
		
		if err := s.server.ListenAndServe(addr); err != nil {
			s.logger.Error("Server stopped with error", zap.Error(err))
		}
	}()
	
	// Give server a moment to start
	time.Sleep(100 * time.Millisecond)
	return nil
}

// Stop stops the HTTP server gracefully
func (s *Server) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if !s.running {
		s.logger.Debug("Server is not running")
		return nil
	}
	
	s.logger.Info("Stopping HTTP server...")
	
	// Shutdown plugins first to stop processing new requests
	if s.pluginsManager != nil {
		if err := s.pluginsManager.Shutdown(); err != nil {
			s.logger.Warn("Failed to shutdown plugins gracefully", zap.Error(err))
		}
	}
	
	if err := s.server.Shutdown(); err != nil {
		return fmt.Errorf("failed to shutdown server: %w", err)
	}

	s.running = false
	s.logger.Info("HTTP server stopped successfully")
	return nil
}

// GetAddr returns the server address
func (s *Server) GetAddr() string {
	return fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
}

// Restart restarts the server with new configuration and/or specification
func (s *Server) Restart(newConfig *config.Config, newSpec *openapi.Specification) error {
	s.logger.Info("Restarting server with new configuration/specification")
	
	// Stop current server
	if err := s.Stop(); err != nil {
		return fmt.Errorf("failed to stop server: %w", err)
	}
	
	// Wait a moment for shutdown to complete
	time.Sleep(200 * time.Millisecond)
	
	// Create new server instance with new config and spec
	newServer, err := NewServer(newConfig, newSpec, s.logger)
	if err != nil {
		// On failure, try to restart with old config
		s.logger.Error("Failed to create new server, attempting to restore old configuration", zap.Error(err))
		if restoreErr := s.Start(); restoreErr != nil {
			s.logger.Error("Failed to restore server", zap.Error(restoreErr))
			return fmt.Errorf("failed to create new server and restore old server: %w", err)
		}
		return fmt.Errorf("failed to create new server: %w", err)
	}
	
	// Update current server with new configuration
	s.mu.Lock()
	s.config = newServer.config
	s.fullConfig = newServer.fullConfig
	s.router = newServer.router
	s.server = newServer.server
	s.spec = newServer.spec
	s.generator = newServer.generator
	s.metricsCollector = newServer.metricsCollector
	s.chaosEngine = newServer.chaosEngine
	s.recordingEngine = newServer.recordingEngine
	s.pluginsManager = newServer.pluginsManager
	s.mu.Unlock()
	
	// Start with new configuration
	if err := s.Start(); err != nil {
		return fmt.Errorf("failed to start server with new configuration: %w", err)
	}
	
	s.logger.Info("Server restarted successfully")
	return nil
}

// GetStats returns server statistics
func (s *Server) GetStats() ServerStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	stats := ServerStats{
		Addr:        s.GetAddr(),
		Endpoints:   len(s.spec.Paths),
		StartTime:   s.startTime,
		IsRunning:   s.running,
	}
	
	// Add metrics if available
	if s.metricsCollector != nil {
		stats.Metrics = s.metricsCollector.GetMetrics()
	}
	
	// Add plugin metrics and statistics
	if s.pluginsManager != nil {
		pluginStats := s.pluginsManager.GetPluginMetrics()
		stats.PluginMetrics = pluginStats
		
		// Count loaded and enabled plugins
		pluginInfos := s.pluginsManager.ListPlugins()
		stats.PluginsLoaded = len(pluginInfos)
		for _, info := range pluginInfos {
			if info.State == plugins.StateEnabled {
				stats.PluginsEnabled++
			}
		}
		
		// Plugin metrics are already included in pluginStats above
	}
	
	return stats
}

// IsRunning returns true if the server is currently running
func (s *Server) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// GetMetrics returns current metrics if collector is available
func (s *Server) GetMetrics() map[string]interface{} {
	if s.metricsCollector == nil {
		return map[string]interface{}{"error": "metrics not enabled"}
	}
	return s.metricsCollector.GetMetrics()
}

// GetRecordingEngine returns the recording engine if available
func (s *Server) GetRecordingEngine() recorder.RecordingEngine {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.recordingEngine
}

// GetPluginsManager returns the plugins manager if available
func (s *Server) GetPluginsManager() *plugins.Manager {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.pluginsManager
}

// GetPluginStats returns detailed plugin statistics and status
func (s *Server) GetPluginStats() []plugins.PluginInfo {
	if s.pluginsManager == nil {
		return []plugins.PluginInfo{}
	}
	return s.pluginsManager.ListPlugins()
}

// ReloadPlugin reloads a specific plugin with new configuration
func (s *Server) ReloadPlugin(pluginName string, config map[string]interface{}) error {
	if s.pluginsManager == nil {
		return fmt.Errorf("plugin manager not initialized")
	}
	
	s.logger.Info("Reloading plugin",
		zap.String("plugin", pluginName))
	
	return s.pluginsManager.ReloadPlugin(pluginName, config)
}

// EnablePlugin enables a loaded plugin
func (s *Server) EnablePlugin(pluginName string) error {
	if s.pluginsManager == nil {
		return fmt.Errorf("plugin manager not initialized")
	}
	
	s.logger.Info("Enabling plugin",
		zap.String("plugin", pluginName))
	
	return s.pluginsManager.EnablePlugin(pluginName)
}

// DisablePlugin disables an enabled plugin
func (s *Server) DisablePlugin(pluginName string) error {
	if s.pluginsManager == nil {
		return fmt.Errorf("plugin manager not initialized")
	}
	
	s.logger.Info("Disabling plugin",
		zap.String("plugin", pluginName))
	
	return s.pluginsManager.DisablePlugin(pluginName)
}

// ServerStats represents server statistics
type ServerStats struct {
	Addr           string                 `json:"addr"`
	Endpoints      int                    `json:"endpoints"`
	StartTime      time.Time              `json:"start_time"`
	IsRunning      bool                   `json:"is_running"`
	Metrics        map[string]interface{} `json:"metrics,omitempty"`
	PluginMetrics  map[string]interface{} `json:"plugin_metrics,omitempty"`
	PluginsLoaded  int                    `json:"plugins_loaded"`
	PluginsEnabled int                    `json:"plugins_enabled"`
}

// hasPluginEnabled checks if a specific plugin is enabled in the configuration
func hasPluginEnabled(pluginConfigs []config.PluginConfig, pluginName string) bool {
	for _, pluginConfig := range pluginConfigs {
		if pluginConfig.Name == pluginName && pluginConfig.Enabled {
			return true
		}
	}
	return false
}

// PluginMetricsAdapter adapts the existing metrics collector to work with plugins
type PluginMetricsAdapter struct {
	metricsCollector *DefaultMetricsCollector
	logger           *zap.Logger
	counters         map[string]int64
	latencies        map[string][]time.Duration
	gauges           map[string]float64
	mu               sync.RWMutex
}

// NewPluginMetricsAdapter creates a new plugin metrics adapter
func NewPluginMetricsAdapter(metricsCollector *DefaultMetricsCollector, logger *zap.Logger) *PluginMetricsAdapter {
	return &PluginMetricsAdapter{
		metricsCollector: metricsCollector,
		logger:           logger,
		counters:         make(map[string]int64),
		latencies:        make(map[string][]time.Duration),
		gauges:           make(map[string]float64),
	}
}

// IncPluginOperation implements plugins.MetricsCollector
func (p *PluginMetricsAdapter) IncPluginOperation(pluginName, operation string, success bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	// Track in our internal counters for plugin-specific metrics
	key := fmt.Sprintf("plugin_%s_%s_total", pluginName, operation)
	p.counters[key]++
	
	if !success {
		errorKey := fmt.Sprintf("plugin_%s_%s_errors_total", pluginName, operation)
		p.counters[errorKey]++
	}
	
	// Map to existing metrics collector using generic path
	if p.metricsCollector != nil {
		status := 200
		if !success {
			status = 500
		}
		p.metricsCollector.IncRequestCounter("PLUGIN", fmt.Sprintf("/plugin/%s/%s", pluginName, operation), status)
	}
}

// ObservePluginLatency implements plugins.MetricsCollector
func (p *PluginMetricsAdapter) ObservePluginLatency(pluginName, operation string, duration time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	// Track in our internal latencies for plugin-specific metrics
	key := fmt.Sprintf("plugin_%s_%s_duration", pluginName, operation)
	p.latencies[key] = append(p.latencies[key], duration)
	
	// Map to existing metrics collector using generic path
	if p.metricsCollector != nil {
		p.metricsCollector.ObserveLatency("PLUGIN", fmt.Sprintf("/plugin/%s/%s", pluginName, operation), duration)
	}
}

// SetPluginState implements plugins.MetricsCollector
func (p *PluginMetricsAdapter) SetPluginState(pluginName string, state string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	var stateValue float64
	switch state {
	case "enabled":
		stateValue = 1
	case "disabled":
		stateValue = 0
	case "error":
		stateValue = -1
	default:
		stateValue = 0
	}
	
	key := fmt.Sprintf("plugin_%s_state", pluginName)
	p.gauges[key] = stateValue
}

// IncPluginError implements plugins.MetricsCollector
func (p *PluginMetricsAdapter) IncPluginError(pluginName string, errorType string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	key := fmt.Sprintf("plugin_%s_errors_%s_total", pluginName, errorType)
	p.counters[key]++
	
	// Map to existing metrics collector as an error
	if p.metricsCollector != nil {
		p.metricsCollector.IncRequestCounter("PLUGIN", fmt.Sprintf("/plugin/%s/error", pluginName), 500)
	}
	
	p.logger.Warn("Plugin error occurred",
		zap.String("plugin", pluginName),
		zap.String("error_type", errorType))
}

// GetPluginMetrics returns plugin-specific metrics
func (p *PluginMetricsAdapter) GetPluginMetrics() map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()
	
	return map[string]interface{}{
		"counters":  p.counters,
		"latencies": p.latencies,
		"gauges":    p.gauges,
	}
}