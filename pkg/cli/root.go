package cli

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

// NewRootCommand creates the root command for the vanta CLI
func NewRootCommand(ctx context.Context, logger *zap.Logger, version, commit, buildTime string) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "vanta",
		Short: "High-performance OpenAPI mock server",
		Long: `Vanta is a high-performance CLI tool that generates realistic mock APIs 
from OpenAPI specifications. It provides advanced developer features including 
chaos testing, intelligent data generation, and seamless CI/CD integration.`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	// Add global flags
	rootCmd.PersistentFlags().StringP("log-level", "l", "info", "Set the logging level (debug, info, warn, error)")
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "Enable verbose output")

	// Add subcommands - these will be implemented in main package
	// rootCmd.AddCommand(newStartCommand(ctx, logger))
	// rootCmd.AddCommand(newConfigCommand(ctx, logger))
	// rootCmd.AddCommand(newVersionCommand(version, commit, buildTime))

	return rootCmd
}

// ExecuteWithLogger executes the root command with proper error handling
func ExecuteWithLogger(rootCmd *cobra.Command, logger *zap.Logger) error {
	if err := rootCmd.Execute(); err != nil {
		logger.Error("Command execution failed", zap.Error(err))
		return err
	}
	return nil
}

// SetupGracefulShutdown configures graceful shutdown handling
func SetupGracefulShutdown(logger *zap.Logger) (context.Context, context.CancelFunc) {
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