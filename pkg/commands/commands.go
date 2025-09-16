package commands

import (
	"context"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

// RegisterCommands adds all available commands to the root command
func RegisterCommands(rootCmd *cobra.Command, ctx context.Context, logger *zap.Logger, version, commit, buildTime string) {
	// Add main commands
	rootCmd.AddCommand(NewStartCommand(ctx, logger))
	rootCmd.AddCommand(NewTUICommand(ctx, logger))
	rootCmd.AddCommand(NewConfigCommand(ctx, logger))
	rootCmd.AddCommand(NewValidationCommand(ctx, logger))
	rootCmd.AddCommand(NewRecordCommand(ctx, logger))
	rootCmd.AddCommand(NewChaosCommand(ctx, logger))
	rootCmd.AddCommand(NewStateCommand(ctx, logger))
	rootCmd.AddCommand(NewVersionCommand(version, commit, buildTime))
}