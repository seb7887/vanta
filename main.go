package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"github.com/seb7887/vanta/pkg/cli"
)

var (
	version   = "dev"
	commit    = "unknown"
	buildTime = "unknown"
)

func main() {
	// Initialize structured logger
	logger, err := zap.NewProduction()
	if err != nil {
		panic(err)
	}
	defer logger.Sync()

	// Setup graceful shutdown
	ctx, cancel := setupGracefulShutdown(logger)
	defer cancel()

	// Create root command with context and logger - minimal version without subcommands
	rootCmd := cli.NewRootCommand(ctx, logger, version, commit, buildTime)

	// For now, just show a message pointing to the full version
	rootCmd.Short = "Vanta mocker - install from cmd/mocker for full functionality"
	rootCmd.Long = `This is a convenience entry point for Vanta.

For the full vanta application with all commands, install using:
  go install github.com/seb7887/vanta/cmd/mocker@latest

Or if you have the source code:
  go install ./cmd/mocker`

	rootCmd.RunE = func(cmd *cobra.Command, args []string) error {
		logger.Info("For full functionality, install the complete version",
			zap.String("command", "go install github.com/seb7887/vanta/cmd/mocker@latest"))
		return cmd.Help()
	}

	// Execute the root command
	if err := rootCmd.Execute(); err != nil {
		logger.Error("Command execution failed", zap.Error(err))
		os.Exit(1)
	}
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