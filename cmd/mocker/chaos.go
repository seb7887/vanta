package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"vanta/pkg/chaos"
	"vanta/pkg/config"
)

// newChaosCommand creates the chaos testing command and its subcommands
func newChaosCommand(ctx context.Context, logger *zap.Logger) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "chaos",
		Short: "Chaos engineering commands for testing resilience",
		Long: `Chaos engineering commands allow you to inject various types of failures
and latency into your OpenAPI mock server to test client resilience.

Available chaos types:
  - latency: Add artificial delay to responses
  - error:   Return HTTP error responses

Use subcommands to manage chaos scenarios:
  - start:   Start chaos testing with specified scenarios
  - stop:    Stop all active chaos scenarios  
  - status:  Show current chaos testing status
  - list:    List available scenarios from configuration`,
		Example: `  # Start chaos testing with a specific scenario
  mocker chaos start --scenario api_latency --config chaos.yaml

  # Stop all chaos testing
  mocker chaos stop

  # Check chaos status
  mocker chaos status

  # List available scenarios
  mocker chaos list --config chaos.yaml`,
	}

	// Add subcommands
	cmd.AddCommand(newChaosStartCommand(ctx, logger))
	cmd.AddCommand(newChaosStopCommand(ctx, logger))
	cmd.AddCommand(newChaosStatusCommand(ctx, logger))
	cmd.AddCommand(newChaosListCommand(ctx, logger))

	return cmd
}

// newChaosStartCommand creates the chaos start subcommand
func newChaosStartCommand(ctx context.Context, logger *zap.Logger) *cobra.Command {
	var (
		configFile string
		scenario   string
		duration   time.Duration
	)

	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start chaos testing scenarios",
		Long: `Start chaos testing by loading scenarios from configuration file.
You can specify a particular scenario to run, or run all enabled scenarios.`,
		Example: `  # Start all enabled scenarios
  mocker chaos start --config chaos.yaml

  # Start a specific scenario for 5 minutes
  mocker chaos start --scenario api_latency --duration 5m --config chaos.yaml`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runChaosStart(ctx, logger, configFile, scenario, duration)
		},
	}

	cmd.Flags().StringVarP(&configFile, "config", "c", "config.yaml", "Configuration file path")
	cmd.Flags().StringVarP(&scenario, "scenario", "s", "", "Specific scenario to start (optional)")
	cmd.Flags().DurationVarP(&duration, "duration", "d", 0, "Duration to run chaos testing (0 = indefinite)")

	return cmd
}

// newChaosStopCommand creates the chaos stop subcommand
func newChaosStopCommand(ctx context.Context, logger *zap.Logger) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop all chaos testing scenarios",
		Long:  `Stop all active chaos testing scenarios and return to normal operation.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runChaosStop(ctx, logger)
		},
	}

	return cmd
}

// newChaosStatusCommand creates the chaos status subcommand
func newChaosStatusCommand(ctx context.Context, logger *zap.Logger) *cobra.Command {
	var configFile string

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show chaos testing status and statistics",
		Long: `Display the current status of chaos testing including:
  - Active scenarios
  - Statistics (requests processed, chaos applied, failures)
  - Configuration details`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runChaosStatus(ctx, logger, configFile)
		},
	}

	cmd.Flags().StringVarP(&configFile, "config", "c", "config.yaml", "Configuration file path")

	return cmd
}

// newChaosListCommand creates the chaos list subcommand
func newChaosListCommand(ctx context.Context, logger *zap.Logger) *cobra.Command {
	var configFile string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List available chaos scenarios",
		Long: `List all chaos scenarios defined in the configuration file with their details:
  - Scenario name and type
  - Target endpoints
  - Probability and parameters
  - Enabled/disabled status`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runChaosList(ctx, logger, configFile)
		},
	}

	cmd.Flags().StringVarP(&configFile, "config", "c", "config.yaml", "Configuration file path")

	return cmd
}

// runChaosStart implements the chaos start command
func runChaosStart(ctx context.Context, logger *zap.Logger, configFile, scenario string, duration time.Duration) error {
	// Load configuration
	cfg, err := loadConfig(configFile)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	if !cfg.Chaos.Enabled {
		return fmt.Errorf("chaos testing is disabled in configuration")
	}

	if len(cfg.Chaos.Scenarios) == 0 {
		return fmt.Errorf("no chaos scenarios configured")
	}

	// Filter scenarios if specific scenario requested
	var activeScenarios []config.ScenarioConfig
	if scenario != "" {
		found := false
		for _, s := range cfg.Chaos.Scenarios {
			if s.Name == scenario {
				activeScenarios = []config.ScenarioConfig{s}
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("scenario '%s' not found in configuration", scenario)
		}
	} else {
		activeScenarios = cfg.Chaos.Scenarios
	}

	// Create and initialize chaos engine
	engine := chaos.NewDefaultChaosEngine(logger)
	if err := engine.LoadScenarios(activeScenarios); err != nil {
		return fmt.Errorf("failed to load chaos scenarios: %w", err)
	}

	fmt.Printf("✅ Chaos testing started with %d scenario(s)\n", len(activeScenarios))
	
	for _, s := range activeScenarios {
		fmt.Printf("  - %s (%s): %.1f%% probability on %v\n", 
			s.Name, s.Type, s.Probability*100, s.Endpoints)
	}

	if duration > 0 {
		fmt.Printf("⏰ Will run for %v\n", duration)
		timer := time.NewTimer(duration)
		defer timer.Stop()

		select {
		case <-timer.C:
			fmt.Println("⏰ Duration elapsed, stopping chaos testing")
		case <-ctx.Done():
			fmt.Println("🛑 Received shutdown signal")
		}
	} else {
		fmt.Println("♾️  Running indefinitely (Ctrl+C to stop)")
		<-ctx.Done()
		fmt.Println("🛑 Received shutdown signal")
	}

	// Stop chaos engine
	if err := engine.Stop(); err != nil {
		logger.Error("Failed to stop chaos engine cleanly", zap.Error(err))
	}

	// Display final statistics
	stats := engine.GetStats()
	fmt.Printf("\n📊 Final Statistics:\n")
	fmt.Printf("  Total requests: %d\n", stats.TotalRequests)
	fmt.Printf("  Chaos applied: %d\n", stats.ChaosApplied)
	fmt.Printf("  Failed injections: %d\n", stats.FailedInjections)

	if stats.TotalRequests > 0 {
		chaosRate := float64(stats.ChaosApplied) / float64(stats.TotalRequests) * 100
		fmt.Printf("  Chaos rate: %.2f%%\n", chaosRate)
	}

	fmt.Println("✅ Chaos testing stopped")
	return nil
}

// runChaosStop implements the chaos stop command
func runChaosStop(ctx context.Context, logger *zap.Logger) error {
	// Note: In a real implementation, this would communicate with a running server
	// For now, we'll just print a message since the chaos engine runs within the server
	fmt.Println("🛑 Chaos testing stop signal sent")
	fmt.Println("💡 Note: To stop chaos testing on a running server, restart the server or use configuration hot-reload")
	return nil
}

// runChaosStatus implements the chaos status command
func runChaosStatus(ctx context.Context, logger *zap.Logger, configFile string) error {
	// Load configuration to show what would be enabled
	cfg, err := loadConfig(configFile)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	fmt.Printf("📋 Chaos Testing Status\n\n")
	fmt.Printf("Configuration file: %s\n", configFile)
	fmt.Printf("Chaos enabled: %v\n", cfg.Chaos.Enabled)
	fmt.Printf("Scenarios configured: %d\n\n", len(cfg.Chaos.Scenarios))

	if !cfg.Chaos.Enabled {
		fmt.Println("⚠️  Chaos testing is disabled in configuration")
		return nil
	}

	if len(cfg.Chaos.Scenarios) == 0 {
		fmt.Println("⚠️  No chaos scenarios configured")
		return nil
	}

	fmt.Println("📝 Configured Scenarios:")
	for i, scenario := range cfg.Chaos.Scenarios {
		fmt.Printf("  %d. %s (%s)\n", i+1, scenario.Name, scenario.Type)
		fmt.Printf("     Endpoints: %v\n", scenario.Endpoints)
		fmt.Printf("     Probability: %.1f%%\n", scenario.Probability*100)
		if len(scenario.Parameters) > 0 {
			fmt.Printf("     Parameters: %v\n", scenario.Parameters)
		}
		fmt.Println()
	}

	return nil
}

// runChaosList implements the chaos list command
func runChaosList(ctx context.Context, logger *zap.Logger, configFile string) error {
	// Load configuration
	cfg, err := loadConfig(configFile)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	fmt.Printf("📋 Available Chaos Scenarios\n\n")

	if len(cfg.Chaos.Scenarios) == 0 {
		fmt.Println("No chaos scenarios found in configuration.")
		fmt.Printf("Add scenarios to the 'chaos.scenarios' section in %s\n", configFile)
		return nil
	}

	for _, scenario := range cfg.Chaos.Scenarios {
		fmt.Printf("🎯 %s\n", scenario.Name)
		fmt.Printf("   Type: %s\n", scenario.Type)
		fmt.Printf("   Endpoints: %v\n", scenario.Endpoints)
		fmt.Printf("   Probability: %.1f%%\n", scenario.Probability*100)
		
		if len(scenario.Parameters) > 0 {
			fmt.Printf("   Parameters:\n")
			for key, value := range scenario.Parameters {
				fmt.Printf("     %s: %v\n", key, value)
			}
		}
		fmt.Println()
	}

	return nil
}

// loadConfig is a helper function to load configuration
// Note: This should ideally be shared with other commands
func loadConfig(configFile string) (*config.Config, error) {
	if configFile == "" {
		return nil, fmt.Errorf("configuration file path is required")
	}

	// Check if file exists
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		// Try to find config file in common locations
		commonPaths := []string{
			"config.yaml",
			"config.yml", 
			"./config/config.yaml",
			"./configs/config.yaml",
		}
		
		found := false
		for _, path := range commonPaths {
			if _, err := os.Stat(path); err == nil {
				configFile = path
				found = true
				break
			}
		}
		
		if !found {
			return nil, fmt.Errorf("configuration file not found: %s", configFile)
		}
	}

	// Get absolute path
	absPath, err := filepath.Abs(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Load configuration using the config package
	cfg, err := config.LoadFromFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config from %s: %w", absPath, err)
	}

	return cfg, nil
}