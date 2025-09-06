package api

import (
	"fmt"
	"time"

	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
	"vanta/pkg/config"
	"vanta/pkg/openapi"
)

// Server represents the HTTP server
type Server struct {
	config    *config.ServerConfig
	router    *Router
	server    *fasthttp.Server
	logger    *zap.Logger
	spec      *openapi.Specification
	generator openapi.DataGenerator
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

	// Create FastHTTP server with configuration
	server := &fasthttp.Server{
		Handler:               router.Handler,
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
		config:    &cfg.Server,
		router:    router,
		server:    server,
		logger:    logger,
		spec:      spec,
		generator: generator,
	}, nil
}

// Start starts the HTTP server
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
	
	s.logger.Info("Starting HTTP server",
		zap.String("address", addr),
		zap.Int("concurrency", s.config.Concurrency),
		zap.Duration("read_timeout", s.config.ReadTimeout),
		zap.Duration("write_timeout", s.config.WriteTimeout),
	)

	if s.config.ReusePort {
		return s.server.ListenAndServe(addr)
	}

	return s.server.ListenAndServe(addr)
}

// Stop stops the HTTP server gracefully
func (s *Server) Stop() error {
	s.logger.Info("Stopping HTTP server...")
	
	if err := s.server.Shutdown(); err != nil {
		return fmt.Errorf("failed to shutdown server: %w", err)
	}

	s.logger.Info("HTTP server stopped successfully")
	return nil
}

// GetAddr returns the server address
func (s *Server) GetAddr() string {
	return fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
}

// GetStats returns server statistics
func (s *Server) GetStats() ServerStats {
	return ServerStats{
		Addr:        s.GetAddr(),
		Endpoints:   len(s.spec.Paths),
		StartTime:   time.Now(), // TODO: Store actual start time
		IsRunning:   true,       // TODO: Track actual state
	}
}

// ServerStats represents server statistics
type ServerStats struct {
	Addr        string    `json:"addr"`
	Endpoints   int       `json:"endpoints"`
	StartTime   time.Time `json:"start_time"`
	IsRunning   bool      `json:"is_running"`
}