package main

import (
	"os"

	"go.uber.org/zap"
	"github.com/seb7887/vanta/pkg/cli"
)

var (
	version   = "dev"
	commit    = "unknown"
	buildTime = "unknown"
)

func main() {
	// Create logger
	logger, err := zap.NewProduction()
	if err != nil {
		panic("Failed to create logger: " + err.Error())
	}
	defer logger.Sync()

	// Setup graceful shutdown
	ctx, cancel := cli.SetupGracefulShutdown(logger)
	defer cancel()

	// Create root command
	rootCmd := cli.NewRootCommand(ctx, logger, version, commit, buildTime)

	// Execute command
	if err := cli.ExecuteWithLogger(rootCmd, logger); err != nil {
		logger.Error("Application failed", zap.Error(err))
		os.Exit(1)
	}
}