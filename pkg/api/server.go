package api

import (
	"fmt"
	"sync"
	"time"

	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
	"vanta/pkg/config"
	"vanta/pkg/openapi"
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

	// Create and configure middleware stack
	stack := NewStack()

	// Add middleware based on configuration
	if cfg.Middleware.RequestID {
		stack.Use(RequestID(true))
	}

	// Logger middleware is always added
	stack.Use(Logger(logger, &cfg.Logging))

	if cfg.Middleware.Recovery.Enabled {
		stack.Use(Recovery(logger, &cfg.Middleware.Recovery))
	}

	if cfg.Middleware.CORS.Enabled {
		stack.Use(CORS(&cfg.Middleware.CORS))
	}

	if cfg.Middleware.Timeout.Enabled {
		stack.Use(Timeout(&cfg.Middleware.Timeout))
	}

	if cfg.Metrics.Enabled && metricsCollector != nil {
		stack.Use(Metrics(&cfg.Metrics, metricsCollector))
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

// ServerStats represents server statistics
type ServerStats struct {
	Addr        string                 `json:"addr"`
	Endpoints   int                    `json:"endpoints"`
	StartTime   time.Time              `json:"start_time"`
	IsRunning   bool                   `json:"is_running"`
	Metrics     map[string]interface{} `json:"metrics,omitempty"`
}