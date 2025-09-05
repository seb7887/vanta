package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"vanta/pkg/config"
)

func newConfigCommand(ctx context.Context, logger *zap.Logger) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Configuration management commands",
		Long:  `Commands for managing vanta configuration files.`,
	}

	cmd.AddCommand(newConfigInitCommand(logger))
	cmd.AddCommand(newConfigValidateCommand(logger))
	cmd.AddCommand(newConfigEditCommand(logger))

	return cmd
}

func newConfigInitCommand(logger *zap.Logger) *cobra.Command {
	var outputFile string

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a new configuration file",
		Long:  `Create a new configuration file with default values.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if outputFile == "" {
				outputFile = "vanta.yaml"
			}

			// Check if file already exists
			if _, err := os.Stat(outputFile); err == nil {
				return fmt.Errorf("configuration file already exists: %s", outputFile)
			}

			logger.Info("Creating configuration file", zap.String("file", outputFile))

			// Generate default configuration
			cfg := config.DefaultConfig()

			// Write to file
			if err := config.WriteToFile(cfg, outputFile); err != nil {
				return fmt.Errorf("failed to write configuration file: %w", err)
			}

			logger.Info("Configuration file created successfully", zap.String("file", outputFile))
			fmt.Printf("Configuration file created: %s\n", outputFile)

			return nil
		},
	}

	cmd.Flags().StringVarP(&outputFile, "output", "o", "vanta.yaml", "Output configuration file")

	return cmd
}

func newConfigValidateCommand(logger *zap.Logger) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate [config-file]",
		Short: "Validate a configuration file",
		Long:  `Validate the syntax and content of a configuration file.`,
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			configFile := "vanta.yaml"
			if len(args) > 0 {
				configFile = args[0]
			}

			logger.Info("Validating configuration file", zap.String("file", configFile))

			// Check if file exists
			if _, err := os.Stat(configFile); os.IsNotExist(err) {
				return fmt.Errorf("configuration file not found: %s", configFile)
			}

			// Load and validate configuration
			cfg, err := config.LoadFromFile(configFile)
			if err != nil {
				return fmt.Errorf("failed to load configuration: %w", err)
			}

			if err := config.Validate(cfg); err != nil {
				return fmt.Errorf("configuration validation failed: %w", err)
			}

			logger.Info("Configuration file is valid", zap.String("file", configFile))
			fmt.Printf("Configuration file is valid: %s\n", configFile)

			return nil
		},
	}

	return cmd
}

func newConfigEditCommand(logger *zap.Logger) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "edit [config-file]",
		Short: "Edit a configuration file interactively",
		Long:  `Open a configuration file in an interactive editor.`,
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			configFile := "vanta.yaml"
			if len(args) > 0 {
				configFile = args[0]
			}

			// Get absolute path
			absPath, err := filepath.Abs(configFile)
			if err != nil {
				return fmt.Errorf("failed to get absolute path: %w", err)
			}

			// Check if file exists, create if it doesn't
			if _, err := os.Stat(absPath); os.IsNotExist(err) {
				logger.Info("Configuration file does not exist, creating default", zap.String("file", absPath))
				
				cfg := config.DefaultConfig()
				if err := config.WriteToFile(cfg, absPath); err != nil {
					return fmt.Errorf("failed to create configuration file: %w", err)
				}
			}

			// Get editor from environment or use default
			editor := os.Getenv("EDITOR")
			if editor == "" {
				editor = "vi" // fallback to vi
			}

			logger.Info("Opening configuration file in editor", 
				zap.String("file", absPath),
				zap.String("editor", editor),
			)

			fmt.Printf("Opening %s in %s...\n", absPath, editor)
			fmt.Println("Note: Interactive editing is not yet implemented in this version.")
			fmt.Printf("Please manually edit the file: %s\n", absPath)

			return nil
		},
	}

	return cmd
}