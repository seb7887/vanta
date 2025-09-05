package cli

import (
	"context"

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