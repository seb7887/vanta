package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"vanta/pkg/api"
	"vanta/pkg/config"
	"vanta/pkg/openapi"
)

func newStartCommand(ctx context.Context, logger *zap.Logger) *cobra.Command {
	var (
		specFile   string
		port       int
		host       string
		configFile string
	)

	cmd := &cobra.Command{
		Use:   "start [OpenAPI spec file]",
		Short: "Start the mock server",
		Long:  `Start the mock server using the provided OpenAPI specification file.`,
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Determine spec file from args or flag
			if len(args) > 0 {
				specFile = args[0]
			}

			if specFile == "" {
				return fmt.Errorf("OpenAPI specification file is required")
			}

			logger.Info("Starting vanta server",
				zap.String("spec", specFile),
				zap.Int("port", port),
				zap.String("host", host),
			)

			// Validate spec file exists
			if _, err := os.Stat(specFile); os.IsNotExist(err) {
				return fmt.Errorf("OpenAPI spec file not found: %s", specFile)
			}

			// Load configuration
			cfg, err := loadConfiguration(configFile, port, host, logger)
			if err != nil {
				return fmt.Errorf("failed to load configuration: %w", err)
			}

			// Parse OpenAPI specification
			spec, err := parseOpenAPISpec(specFile, logger)
			if err != nil {
				return fmt.Errorf("failed to parse OpenAPI spec: %w", err)
			}

			// Create and start server
			server, err := api.NewServer(cfg, spec, logger)
			if err != nil {
				return fmt.Errorf("failed to create server: %w", err)
			}

			logger.Info("Server created successfully, starting...")
			
			// Start server in a goroutine
			serverErrCh := make(chan error, 1)
			go func() {
				if err := server.Start(); err != nil {
					serverErrCh <- err
				}
			}()

			// Wait for context cancellation or server error
			select {
			case <-ctx.Done():
				logger.Info("Shutdown signal received, stopping server...")
				if err := server.Stop(); err != nil {
					logger.Error("Error stopping server", zap.Error(err))
					return err
				}
				logger.Info("Server stopped successfully")
				return nil
			case err := <-serverErrCh:
				return fmt.Errorf("server error: %w", err)
			}
		},
	}

	// Add flags
	cmd.Flags().StringVarP(&specFile, "spec", "s", "", "Path to OpenAPI specification file")
	cmd.Flags().IntVarP(&port, "port", "p", 8080, "Server port")
	cmd.Flags().StringVarP(&host, "host", "H", "0.0.0.0", "Server host")
	cmd.Flags().StringVarP(&configFile, "config", "c", "", "Path to configuration file")

	return cmd
}

func loadConfiguration(configFile string, port int, host string, logger *zap.Logger) (*config.Config, error) {
	var cfg *config.Config
	var err error

	if configFile != "" {
		logger.Info("Loading configuration from file", zap.String("file", configFile))
		cfg, err = config.LoadFromFile(configFile)
		if err != nil {
			return nil, err
		}
	} else {
		logger.Info("Using default configuration")
		cfg = config.DefaultConfig()
	}

	// Override with command line flags
	if port != 8080 {
		cfg.Server.Port = port
	}
	if host != "0.0.0.0" {
		cfg.Server.Host = host
	}

	// Validate configuration
	if err := config.Validate(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func parseOpenAPISpec(specFile string, logger *zap.Logger) (*openapi.Specification, error) {
	logger.Info("Parsing OpenAPI specification", zap.String("file", specFile))

	// Get absolute path
	absPath, err := filepath.Abs(specFile)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Read spec file
	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read spec file: %w", err)
	}

	// Parse specification
	parser := openapi.NewParser()
	spec, err := parser.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse OpenAPI spec: %w", err)
	}

	// Validate specification
	if err := parser.Validate(spec); err != nil {
		return nil, fmt.Errorf("invalid OpenAPI spec: %w", err)
	}

	logger.Info("OpenAPI specification parsed successfully",
		zap.String("version", spec.Version),
		zap.String("title", spec.Info.Title),
		zap.Int("endpoints", len(spec.Paths)),
	)

	return spec, nil
}