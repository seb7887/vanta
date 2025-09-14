package main

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"vanta/pkg/api"
	"vanta/pkg/cli"
	"vanta/pkg/config"
	"vanta/pkg/openapi"
)

// newTUICommand creates the tui command
func newTUICommand(ctx context.Context, logger *zap.Logger) *cobra.Command {
	var (
		configPath string
		specPath   string
		readonly   bool
	)

	cmd := &cobra.Command{
		Use:   "tui",
		Short: "Launch interactive Terminal UI",
		Long: `Launch an interactive Terminal User Interface (TUI) that provides:
• Real-time metrics dashboard (RPS, latency, errors, memory usage)
• Live log viewer with filtering and search capabilities
• Interactive configuration editor with hot reload
• Server status monitoring and control`,
		Example: `  # Launch TUI with default configuration
  mocker tui

  # Launch TUI with custom config
  mocker tui --config custom-config.yaml

  # Launch TUI in read-only mode (no config editing)
  mocker tui --readonly

  # Launch TUI with specific OpenAPI spec
  mocker tui --spec petstore.yaml`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTUICommand(ctx, logger, configPath, specPath, readonly)
		},
	}

	// Add flags
	cmd.Flags().StringVarP(&configPath, "config", "c", "config.yaml", "Path to configuration file")
	cmd.Flags().StringVarP(&specPath, "spec", "s", "", "Path to OpenAPI specification file (optional)")
	cmd.Flags().BoolVar(&readonly, "readonly", false, "Launch TUI in read-only mode (no configuration editing)")

	return cmd
}

func runTUICommand(ctx context.Context, logger *zap.Logger, configPath, specPath string, readonly bool) error {
	logger.Info("Starting TUI mode",
		zap.String("config", configPath),
		zap.String("spec", specPath),
		zap.Bool("readonly", readonly))

	// Load configuration
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Load OpenAPI specification
	var spec *openapi.Specification
	if specPath != "" {
		spec, err = loadOpenAPISpec(specPath, logger)
		if err != nil {
			return fmt.Errorf("failed to load OpenAPI specification: %w", err)
		}
	} else {
		// Create a minimal spec for TUI to work
		spec = createMinimalSpec()
	}

	// Create server instance (but don't start it automatically in TUI mode)
	server, err := api.NewServer(cfg, spec, logger)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}

	// Start server if enabled in config
	if cfg.Server.Port > 0 {
		if err := server.Start(); err != nil {
			logger.Warn("Failed to start server automatically", zap.Error(err))
		} else {
			logger.Info("Server started", zap.String("address", server.GetAddr()))
		}
	}

	// Launch TUI
	logger.Info("Launching Terminal UI...")
	logger.Info("TUI Controls:",
		zap.String("navigation", "Tab/Shift+Tab to switch between panels"),
		zap.String("logs", "↑/↓ to scroll, f to filter, c to clear"),
		zap.String("config", "↑/↓ to navigate, Enter to edit, Ctrl+S to save"),
		zap.String("exit", "q or Ctrl+C to quit"))

	if err := cli.RunTUI(cfg, server, logger); err != nil {
		return fmt.Errorf("TUI failed: %w", err)
	}

	// Cleanup: Stop server if running
	if server.IsRunning() {
		logger.Info("Stopping server...")
		if err := server.Stop(); err != nil {
			logger.Warn("Failed to stop server gracefully", zap.Error(err))
		}
	}

	logger.Info("TUI session ended")
	return nil
}

func loadOpenAPISpec(specPath string, logger *zap.Logger) (*openapi.Specification, error) {
	// Get absolute path
	absPath, err := filepath.Abs(specPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	logger.Info("Loading OpenAPI specification", zap.String("path", absPath))

	// Parse the OpenAPI specification
	spec, err := openapi.LoadSpecification(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse OpenAPI spec: %w", err)
	}

	logger.Info("OpenAPI specification loaded successfully",
		zap.String("title", spec.Info.Title),
		zap.String("version", spec.Info.Version),
		zap.Int("paths", len(spec.Paths)))

	return spec, nil
}

func createMinimalSpec() *openapi.Specification {
	healthSchema := &openapi.Schema{
		Type: "object",
		Properties: map[string]*openapi.Schema{
			"status": {
				Type:    "string",
				Example: "ok",
			},
			"timestamp": {
				Type:   "string",
				Format: "date-time",
			},
		},
	}

	metricsSchema := &openapi.Schema{
		Type: "object",
		Properties: map[string]*openapi.Schema{
			"requests_total":  {Type: "number"},
			"errors_total":    {Type: "number"},
			"latency_ms":      {Type: "number"},
			"uptime_seconds":  {Type: "number"},
		},
	}

	return &openapi.Specification{
		Version: "3.0.0",
		Info: openapi.InfoObject{
			Title:       "TUI Demo API",
			Version:     "1.0.0",
			Description: "Minimal API for TUI demonstration",
		},
		Paths: map[string]openapi.PathItem{
			"/health": {
				GET: &openapi.Operation{
					Summary:     "Health check endpoint",
					Description: "Returns server health status",
					Responses: map[string]openapi.Response{
						"200": {
							Description: "Server is healthy",
							Content: map[string]openapi.MediaTypeObject{
								"application/json": {
									Schema: healthSchema,
								},
							},
						},
					},
				},
			},
			"/metrics": {
				GET: &openapi.Operation{
					Summary:     "Get server metrics",
					Description: "Returns current server metrics and statistics",
					Responses: map[string]openapi.Response{
						"200": {
							Description: "Server metrics",
							Content: map[string]openapi.MediaTypeObject{
								"application/json": {
									Schema: metricsSchema,
								},
							},
						},
					},
				},
			},
		},
	}
}