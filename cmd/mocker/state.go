package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"github.com/vanta/pkg/config"
	"github.com/vanta/pkg/state"
)

func newStateCommand(ctx context.Context, logger *zap.Logger) *cobra.Command {
	var configFile string

	cmd := &cobra.Command{
		Use:   "state",
		Short: "State management commands",
		Long:  "Manage mock server state including getting, setting, and exporting state data",
	}

	// Add persistent flags
	cmd.PersistentFlags().StringVarP(&configFile, "config", "c", "", "config file (default is mocker.yaml)")

	// Add subcommands
	cmd.AddCommand(newStateGetCommand(ctx, logger, &configFile))
	cmd.AddCommand(newStateSetCommand(ctx, logger, &configFile))
	cmd.AddCommand(newStateDeleteCommand(ctx, logger, &configFile))
	cmd.AddCommand(newStateClearCommand(ctx, logger, &configFile))
	cmd.AddCommand(newStateListCommand(ctx, logger, &configFile))
	cmd.AddCommand(newStateExportCommand(ctx, logger, &configFile))
	cmd.AddCommand(newStateImportCommand(ctx, logger, &configFile))

	return cmd
}

func newStateGetCommand(ctx context.Context, logger *zap.Logger, configFile *string) *cobra.Command {
	var (
		scope   string
		format  string
		output  string
	)

	cmd := &cobra.Command{
		Use:   "get <key>",
		Short: "Get a value from state",
		Long:  "Retrieve a value from the mock server's state storage",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]

			// Load configuration
			cfg, err := loadConfig(*configFile)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Create state manager
			stateManager := state.NewMemoryStateManager(cfg.State.ToStateConfig())
			if err := stateManager.Start(); err != nil {
				return fmt.Errorf("failed to start state manager: %w", err)
			}
			defer stateManager.Stop()

			// Get value
			var value interface{}
			if scope != "" {
				value, err = stateManager.GetScoped(ctx, scope, key)
			} else {
				value, err = stateManager.Get(ctx, key)
			}

			if err != nil {
				if err == state.ErrKeyNotFound {
					return fmt.Errorf("key '%s' not found", key)
				}
				return fmt.Errorf("failed to get value: %w", err)
			}

			// Format output
			var result []byte
			switch format {
			case "json":
				result, err = json.MarshalIndent(value, "", "  ")
				if err != nil {
					return fmt.Errorf("failed to marshal JSON: %w", err)
				}
			case "raw":
				result = []byte(fmt.Sprintf("%v", value))
			default:
				// Pretty format
				result, err = json.MarshalIndent(map[string]interface{}{
					"key":   key,
					"scope": scope,
					"value": value,
				}, "", "  ")
				if err != nil {
					return fmt.Errorf("failed to marshal JSON: %w", err)
				}
			}

			// Output result
			if output != "" {
				return os.WriteFile(output, result, 0644)
			}
			fmt.Print(string(result))

			return nil
		},
	}

	cmd.Flags().StringVar(&scope, "scope", "", "Scope to get value from")
	cmd.Flags().StringVar(&format, "format", "pretty", "Output format (json, raw, pretty)")
	cmd.Flags().StringVarP(&output, "output", "o", "", "Output file (default: stdout)")

	return cmd
}

func newStateSetCommand(ctx context.Context, logger *zap.Logger, configFile *string) *cobra.Command {
	var (
		scope string
		ttl   time.Duration
		file  string
	)

	cmd := &cobra.Command{
		Use:   "set <key> [value]",
		Short: "Set a value in state",
		Long:  "Store a value in the mock server's state storage",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]

			// Load configuration
			cfg, err := loadConfig(*configFile)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Create state manager
			stateManager := state.NewMemoryStateManager(cfg.State.ToStateConfig())
			if err := stateManager.Start(); err != nil {
				return fmt.Errorf("failed to start state manager: %w", err)
			}
			defer stateManager.Stop()

			// Get value to set
			var value interface{}
			if file != "" {
				data, err := os.ReadFile(file)
				if err != nil {
					return fmt.Errorf("failed to read file: %w", err)
				}

				// Try to parse as JSON first
				if err := json.Unmarshal(data, &value); err != nil {
					// If JSON parsing fails, store as string
					value = string(data)
				}
			} else if len(args) > 1 {
				valueStr := args[1]
				// Try to parse as JSON first
				if err := json.Unmarshal([]byte(valueStr), &value); err != nil {
					// If JSON parsing fails, store as string
					value = valueStr
				}
			} else {
				return fmt.Errorf("must provide either value argument or --file flag")
			}

			// Set value
			if scope != "" {
				err = stateManager.SetScoped(ctx, scope, key, value)
			} else if ttl > 0 {
				err = stateManager.SetWithTTL(ctx, key, value, ttl)
			} else {
				err = stateManager.Set(ctx, key, value)
			}

			if err != nil {
				return fmt.Errorf("failed to set value: %w", err)
			}

			fmt.Printf("Successfully set %s = %v\n", key, value)
			if scope != "" {
				fmt.Printf("in scope: %s\n", scope)
			}
			if ttl > 0 {
				fmt.Printf("with TTL: %s\n", ttl)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&scope, "scope", "", "Scope to set value in")
	cmd.Flags().DurationVar(&ttl, "ttl", 0, "Time to live for the value")
	cmd.Flags().StringVar(&file, "file", "", "Read value from file")

	return cmd
}

func newStateDeleteCommand(ctx context.Context, logger *zap.Logger, configFile *string) *cobra.Command {
	var scope string

	cmd := &cobra.Command{
		Use:   "delete <key>",
		Short: "Delete a key from state",
		Long:  "Remove a key-value pair from the mock server's state storage",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]

			// Load configuration
			cfg, err := loadConfig(*configFile)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Create state manager
			stateManager := state.NewMemoryStateManager(cfg.State.ToStateConfig())
			if err := stateManager.Start(); err != nil {
				return fmt.Errorf("failed to start state manager: %w", err)
			}
			defer stateManager.Stop()

			// Delete value
			if scope != "" {
				err = stateManager.DeleteScoped(ctx, scope, key)
			} else {
				err = stateManager.Delete(ctx, key)
			}

			if err != nil {
				return fmt.Errorf("failed to delete key: %w", err)
			}

			fmt.Printf("Successfully deleted key: %s\n", key)
			if scope != "" {
				fmt.Printf("from scope: %s\n", scope)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&scope, "scope", "", "Scope to delete key from")

	return cmd
}

func newStateClearCommand(ctx context.Context, logger *zap.Logger, configFile *string) *cobra.Command {
	var (
		scope   string
		confirm bool
	)

	cmd := &cobra.Command{
		Use:   "clear",
		Short: "Clear all state or a specific scope",
		Long:  "Remove all key-value pairs from state storage or from a specific scope",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !confirm {
				fmt.Print("This will permanently delete state data. Are you sure? (y/N): ")
				var response string
				fmt.Scanln(&response)
				if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
					fmt.Println("Operation cancelled")
					return nil
				}
			}

			// Load configuration
			cfg, err := loadConfig(*configFile)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Create state manager
			stateManager := state.NewMemoryStateManager(cfg.State.ToStateConfig())
			if err := stateManager.Start(); err != nil {
				return fmt.Errorf("failed to start state manager: %w", err)
			}
			defer stateManager.Stop()

			// Clear state
			if scope != "" {
				err = stateManager.ClearScope(ctx, scope)
				if err != nil {
					return fmt.Errorf("failed to clear scope: %w", err)
				}
				fmt.Printf("Successfully cleared scope: %s\n", scope)
			} else {
				err = stateManager.Clear(ctx)
				if err != nil {
					return fmt.Errorf("failed to clear state: %w", err)
				}
				fmt.Println("Successfully cleared all state")
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&scope, "scope", "", "Scope to clear (if not specified, clears all state)")
	cmd.Flags().BoolVar(&confirm, "yes", false, "Skip confirmation prompt")

	return cmd
}

func newStateListCommand(ctx context.Context, logger *zap.Logger, configFile *string) *cobra.Command {
	var (
		scope  string
		format string
		output string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all keys in state",
		Long:  "Show all keys currently stored in the mock server's state",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load configuration
			cfg, err := loadConfig(*configFile)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Create state manager
			stateManager := state.NewMemoryStateManager(cfg.State.ToStateConfig())
			if err := stateManager.Start(); err != nil {
				return fmt.Errorf("failed to start state manager: %w", err)
			}
			defer stateManager.Stop()

			var result interface{}

			if scope != "" {
				// List scopes
				scopes := stateManager.ListScopes(ctx)
				result = map[string]interface{}{
					"scopes": scopes,
				}
			} else {
				// List keys
				keys, err := stateManager.Keys(ctx)
				if err != nil {
					return fmt.Errorf("failed to list keys: %w", err)
				}

				size := stateManager.Size(ctx)
				result = map[string]interface{}{
					"keys":  keys,
					"count": size,
				}
			}

			// Format output
			var resultBytes []byte
			switch format {
			case "json":
				resultBytes, err = json.MarshalIndent(result, "", "  ")
				if err != nil {
					return fmt.Errorf("failed to marshal JSON: %w", err)
				}
			case "text":
				if scope != "" {
					scopes := result.(map[string]interface{})["scopes"].([]string)
					for _, s := range scopes {
						resultBytes = append(resultBytes, []byte(s+"\n")...)
					}
				} else {
					keys := result.(map[string]interface{})["keys"].([]string)
					for _, key := range keys {
						resultBytes = append(resultBytes, []byte(key+"\n")...)
					}
				}
			default:
				resultBytes, err = json.MarshalIndent(result, "", "  ")
				if err != nil {
					return fmt.Errorf("failed to marshal JSON: %w", err)
				}
			}

			// Output result
			if output != "" {
				return os.WriteFile(output, resultBytes, 0644)
			}
			fmt.Print(string(resultBytes))

			return nil
		},
	}

	cmd.Flags().StringVar(&scope, "scope", "", "List scopes instead of keys")
	cmd.Flags().StringVar(&format, "format", "json", "Output format (json, text)")
	cmd.Flags().StringVarP(&output, "output", "o", "", "Output file (default: stdout)")

	return cmd
}

func newStateExportCommand(ctx context.Context, logger *zap.Logger, configFile *string) *cobra.Command {
	var output string

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export all state to a file",
		Long:  "Export the complete state storage to a JSON file",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load configuration
			cfg, err := loadConfig(*configFile)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Create state manager
			stateManager := state.NewMemoryStateManager(cfg.State.ToStateConfig())
			if err := stateManager.Start(); err != nil {
				return fmt.Errorf("failed to start state manager: %w", err)
			}
			defer stateManager.Stop()

			// Export state
			stateData, err := stateManager.Export(ctx)
			if err != nil {
				return fmt.Errorf("failed to export state: %w", err)
			}

			// Marshal to JSON
			jsonData, err := json.MarshalIndent(stateData, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal JSON: %w", err)
			}

			// Determine output file
			outputFile := output
			if outputFile == "" {
				outputFile = fmt.Sprintf("state_export_%s.json", time.Now().Format("2006-01-02_15-04-05"))
			}

			// Ensure directory exists
			if dir := filepath.Dir(outputFile); dir != "." {
				if err := os.MkdirAll(dir, 0755); err != nil {
					return fmt.Errorf("failed to create directory: %w", err)
				}
			}

			// Write to file
			if err := os.WriteFile(outputFile, jsonData, 0644); err != nil {
				return fmt.Errorf("failed to write file: %w", err)
			}

			fmt.Printf("State exported to: %s\n", outputFile)
			fmt.Printf("Exported %d keys\n", len(stateData))

			return nil
		},
	}

	cmd.Flags().StringVarP(&output, "output", "o", "", "Output file (default: auto-generated)")

	return cmd
}

func newStateImportCommand(ctx context.Context, logger *zap.Logger, configFile *string) *cobra.Command {
	var merge bool

	cmd := &cobra.Command{
		Use:   "import <file>",
		Short: "Import state from a file",
		Long:  "Import state data from a JSON file into the state storage",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			filename := args[0]

			// Load configuration
			cfg, err := loadConfig(*configFile)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Create state manager
			stateManager := state.NewMemoryStateManager(cfg.State.ToStateConfig())
			if err := stateManager.Start(); err != nil {
				return fmt.Errorf("failed to start state manager: %w", err)
			}
			defer stateManager.Stop()

			// Read file
			data, err := os.ReadFile(filename)
			if err != nil {
				return fmt.Errorf("failed to read file: %w", err)
			}

			// Parse JSON
			var stateData map[string]*state.StateValue
			if err := json.Unmarshal(data, &stateData); err != nil {
				return fmt.Errorf("failed to parse JSON: %w", err)
			}

			// Clear existing state if not merging
			if !merge {
				if err := stateManager.Clear(ctx); err != nil {
					return fmt.Errorf("failed to clear existing state: %w", err)
				}
			}

			// Import state
			if err := stateManager.Import(ctx, stateData); err != nil {
				return fmt.Errorf("failed to import state: %w", err)
			}

			fmt.Printf("Successfully imported state from: %s\n", filename)
			fmt.Printf("Imported %d keys\n", len(stateData))
			if merge {
				fmt.Println("State was merged with existing data")
			} else {
				fmt.Println("Existing state was replaced")
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&merge, "merge", false, "Merge with existing state instead of replacing")

	return cmd
}

// Helper function to load configuration (reused from other commands)
func loadConfig(configFile string) (*config.Config, error) {
	if configFile == "" {
		configFile = "mocker.yaml"
	}

	cfg := config.DefaultConfig()
	// Add config loading logic here
	// This would typically use viper or similar to load from file

	return cfg, nil
}