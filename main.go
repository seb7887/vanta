package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"
	"github.com/seb7887/vanta/pkg/cli"
	"github.com/seb7887/vanta/pkg/config"

	// Import command packages
	"github.com/seb7887/vanta/pkg/commands"
)

var (
	version   = "dev"
	commit    = "unknown"
	buildTime = "unknown"
)

func main() {
	// Initialize logger - try to load from config file if available
	logger := initializeLogger()
	defer logger.Sync()

	// Setup graceful shutdown
	ctx, cancel := setupGracefulShutdown(logger)
	defer cancel()

	// Create root command with context and logger
	rootCmd := cli.NewRootCommand(ctx, logger, version, commit, buildTime)

	// Register all commands
	commands.RegisterCommands(rootCmd, ctx, logger, version, commit, buildTime)

	// Execute the root command
	if err := rootCmd.Execute(); err != nil {
		logger.Error("Command execution failed", zap.Error(err))
		os.Exit(1)
	}
}

// initializeLogger creates a logger, preferring configuration from vanta.yaml if available
func initializeLogger() *zap.Logger {
	// Try to load config from default location first
	configPaths := []string{"vanta.yaml", "vanta.yml", "config.yaml", "config.yml"}

	for _, configPath := range configPaths {
		if _, err := os.Stat(configPath); err == nil {
			// Config file exists, try to load it
			cfg, err := config.LoadFromFile(configPath)
			if err != nil {
				// Failed to load config file, fall back to default logger
				logger, _ := config.NewLoggerWithDefaults()
				logger.Warn("Failed to load config file, using default logging configuration",
					zap.String("config_file", configPath),
					zap.Error(err))
				return logger
			}

			// Create logger from config
			logger, err := config.NewLoggerFromConfig(cfg.Logging)
			if err != nil {
				// Failed to create logger from config, fall back to default
				defaultLogger, _ := config.NewLoggerWithDefaults()
				defaultLogger.Warn("Failed to create logger from config, using defaults",
					zap.String("config_file", configPath),
					zap.Error(err))
				return defaultLogger
			}

			logger.Info("Logger initialized from configuration",
				zap.String("config_file", configPath),
				zap.String("level", cfg.Logging.Level),
				zap.String("format", cfg.Logging.Format))
			return logger
		}
	}

	// No config file found, use defaults
	logger, err := config.NewLoggerWithDefaults()
	if err != nil {
		// Last resort: use zap's production logger
		logger, _ := zap.NewProduction()
		logger.Warn("Failed to create default logger, using production defaults")
		return logger
	}

	logger.Info("Logger initialized with default configuration")
	return logger
}

func setupGracefulShutdown(logger *zap.Logger) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		sig := <-c
		logger.Info("Received shutdown signal", zap.String("signal", sig.String()))
		logger.Info("Initiating graceful shutdown...")

		// Give commands time to shut down gracefully
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutdownCancel()

		// Cancel the main context to signal shutdown
		cancel()

		// Wait for shutdown timeout
		<-shutdownCtx.Done()
		if shutdownCtx.Err() == context.DeadlineExceeded {
			logger.Warn("Graceful shutdown timeout exceeded, forcing exit")
		}

		logger.Info("Shutdown complete")
		os.Exit(0)
	}()

	return ctx, cancel
}