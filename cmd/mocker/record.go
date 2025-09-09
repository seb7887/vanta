package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"vanta/pkg/config"
	"vanta/pkg/recorder"
)

// newRecordCommand creates the record command with subcommands
func newRecordCommand(ctx context.Context, logger *zap.Logger) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "record",
		Short: "Recording and replay commands for traffic capture",
		Long: `Recording commands allow you to capture live API traffic and replay it later.

Available subcommands:
  - start:     Start recording API traffic
  - stop:      Stop active recording
  - list:      List available recordings
  - show:      Show details of a specific recording
  - delete:    Delete recordings
  - replay:    Replay recorded traffic
  - export:    Export recordings to different formats`,
		Example: `  # Start recording with default settings
  mocker record start

  # List all recordings
  mocker record list

  # Show details of a specific recording
  mocker record show <recording-id>

  # Replay recordings to a target URL
  mocker record replay --target http://localhost:8080`,
	}

	// Add subcommands
	cmd.AddCommand(newRecordStartCommand(ctx, logger))
	cmd.AddCommand(newRecordStopCommand(ctx, logger))
	cmd.AddCommand(newRecordListCommand(ctx, logger))
	cmd.AddCommand(newRecordShowCommand(ctx, logger))
	cmd.AddCommand(newRecordDeleteCommand(ctx, logger))
	cmd.AddCommand(newRecordReplayCommand(ctx, logger))
	cmd.AddCommand(newRecordExportCommand(ctx, logger))

	return cmd
}

// newRecordStartCommand creates the record start subcommand
func newRecordStartCommand(ctx context.Context, logger *zap.Logger) *cobra.Command {
	var configPath string
	var filters []string
	var outputDir string
	var maxRecordings int
	var maxBodySize string

	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start recording API traffic",
		Long:  `Start recording incoming API requests and responses to files.`,
		Example: `  # Start recording with default settings
  mocker record start

  # Start recording with custom configuration
  mocker record start --config recording.yaml --output ./my-recordings

  # Start recording with filters
  mocker record start --filter "method:GET" --filter "endpoint:/api/users"

  # Start recording with limits
  mocker record start --max-recordings 500 --max-body-size 2MB`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRecordStart(ctx, logger, configPath, filters, outputDir, maxRecordings, maxBodySize)
		},
	}

	cmd.Flags().StringVarP(&configPath, "config", "c", "", "Configuration file path")
	cmd.Flags().StringSliceVarP(&filters, "filter", "f", nil, "Recording filters (method:GET, endpoint:/api/users, status:200)")
	cmd.Flags().StringVarP(&outputDir, "output", "o", "", "Output directory for recordings")
	cmd.Flags().IntVar(&maxRecordings, "max-recordings", 0, "Maximum number of recordings to keep")
	cmd.Flags().StringVar(&maxBodySize, "max-body-size", "", "Maximum body size to record (e.g., 1MB, 2KB)")

	return cmd
}

// newRecordStopCommand creates the record stop subcommand
func newRecordStopCommand(ctx context.Context, logger *zap.Logger) *cobra.Command {
	var configPath string

	cmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop active recording",
		Long:  `Stop the currently active recording session.`,
		Example: `  # Stop recording
  mocker record stop

  # Stop recording with custom config
  mocker record stop --config recording.yaml`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRecordStop(ctx, logger, configPath)
		},
	}

	cmd.Flags().StringVarP(&configPath, "config", "c", "", "Configuration file path")

	return cmd
}

// newRecordListCommand creates the record list subcommand
func newRecordListCommand(ctx context.Context, logger *zap.Logger) *cobra.Command {
	var configPath string
	var limit int
	var method string
	var status string
	var since string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List available recordings",
		Long:  `List all available recordings with optional filtering.`,
		Example: `  # List all recordings
  mocker record list

  # List last 10 recordings
  mocker record list --limit 10

  # List GET requests only
  mocker record list --method GET

  # List recordings from the last hour
  mocker record list --since 1h`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRecordList(ctx, logger, configPath, limit, method, status, since)
		},
	}

	cmd.Flags().StringVarP(&configPath, "config", "c", "", "Configuration file path")
	cmd.Flags().IntVarP(&limit, "limit", "l", 50, "Maximum number of recordings to list")
	cmd.Flags().StringVarP(&method, "method", "m", "", "Filter by HTTP method")
	cmd.Flags().StringVarP(&status, "status", "s", "", "Filter by status code")
	cmd.Flags().StringVar(&since, "since", "", "Filter by time (e.g., 1h, 30m, 24h)")

	return cmd
}

// newRecordShowCommand creates the record show subcommand
func newRecordShowCommand(ctx context.Context, logger *zap.Logger) *cobra.Command {
	var configPath string
	var format string

	cmd := &cobra.Command{
		Use:   "show <recording-id>",
		Short: "Show details of a specific recording",
		Long:  `Display detailed information about a specific recording.`,
		Args:  cobra.ExactArgs(1),
		Example: `  # Show recording details
  mocker record show abc123def

  # Show recording in JSON format
  mocker record show abc123def --format json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRecordShow(ctx, logger, configPath, args[0], format)
		},
	}

	cmd.Flags().StringVarP(&configPath, "config", "c", "", "Configuration file path")
	cmd.Flags().StringVar(&format, "format", "table", "Output format (table, json, yaml)")

	return cmd
}

// newRecordDeleteCommand creates the record delete subcommand
func newRecordDeleteCommand(ctx context.Context, logger *zap.Logger) *cobra.Command {
	var configPath string
	var all bool
	var force bool

	cmd := &cobra.Command{
		Use:   "delete [recording-id...]",
		Short: "Delete recordings",
		Long:  `Delete one or more recordings by ID, or delete all recordings.`,
		Example: `  # Delete specific recordings
  mocker record delete abc123def xyz789abc

  # Delete all recordings (with confirmation)
  mocker record delete --all

  # Force delete all recordings without confirmation
  mocker record delete --all --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRecordDelete(ctx, logger, configPath, args, all, force)
		},
	}

	cmd.Flags().StringVarP(&configPath, "config", "c", "", "Configuration file path")
	cmd.Flags().BoolVar(&all, "all", false, "Delete all recordings")
	cmd.Flags().BoolVar(&force, "force", false, "Force deletion without confirmation")

	return cmd
}

// newRecordReplayCommand creates the record replay subcommand
func newRecordReplayCommand(ctx context.Context, logger *zap.Logger) *cobra.Command {
	var configPath string
	var targetURL string
	var concurrency int
	var delay string
	var recordingIDs []string
	var since string
	var limit int

	cmd := &cobra.Command{
		Use:   "replay",
		Short: "Replay recorded traffic",
		Long:  `Replay previously recorded traffic against a target URL.`,
		Example: `  # Replay all recordings to localhost
  mocker record replay --target http://localhost:8080

  # Replay specific recordings
  mocker record replay --target http://localhost:8080 --ids abc123,def456

  # Replay with concurrency and delay
  mocker record replay --target http://localhost:8080 --concurrency 5 --delay 100ms

  # Replay recent recordings
  mocker record replay --target http://localhost:8080 --since 1h --limit 10`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRecordReplay(ctx, logger, configPath, targetURL, concurrency, delay, recordingIDs, since, limit)
		},
	}

	cmd.Flags().StringVarP(&configPath, "config", "c", "", "Configuration file path")
	cmd.Flags().StringVarP(&targetURL, "target", "t", "", "Target URL for replay (required)")
	cmd.Flags().IntVar(&concurrency, "concurrency", 1, "Number of concurrent requests")
	cmd.Flags().StringVar(&delay, "delay", "100ms", "Delay between requests")
	cmd.Flags().StringSliceVar(&recordingIDs, "ids", nil, "Specific recording IDs to replay")
	cmd.Flags().StringVar(&since, "since", "", "Replay recordings from specific time (e.g., 1h, 30m)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum number of recordings to replay")

	cmd.MarkFlagRequired("target")

	return cmd
}

// newRecordExportCommand creates the record export subcommand
func newRecordExportCommand(ctx context.Context, logger *zap.Logger) *cobra.Command {
	var configPath string
	var format string
	var output string
	var recordingIDs []string

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export recordings to different formats",
		Long:  `Export recordings to various formats like HAR, Postman, or cURL commands.`,
		Example: `  # Export all recordings to HAR format
  mocker record export --format har --output recordings.har

  # Export specific recordings to Postman collection
  mocker record export --format postman --output collection.json --ids abc123,def456

  # Export recordings as cURL commands
  mocker record export --format curl --output commands.sh`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRecordExport(ctx, logger, configPath, format, output, recordingIDs)
		},
	}

	cmd.Flags().StringVarP(&configPath, "config", "c", "", "Configuration file path")
	cmd.Flags().StringVar(&format, "format", "json", "Export format (json, har, postman, curl)")
	cmd.Flags().StringVarP(&output, "output", "o", "", "Output file (default: stdout)")
	cmd.Flags().StringSliceVar(&recordingIDs, "ids", nil, "Specific recording IDs to export")

	return cmd
}

// Implementation functions

func runRecordStart(ctx context.Context, logger *zap.Logger, configPath string, filters []string, outputDir string, maxRecordings int, maxBodySize string) error {
	fmt.Println("🎬 Starting recording...")

	// Load configuration
	cfg, err := loadConfigForRecording(configPath)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Override configuration with command line parameters
	if outputDir != "" {
		cfg.Recording.Storage.Directory = outputDir
	}
	if maxRecordings > 0 {
		cfg.Recording.MaxRecordings = maxRecordings
	}
	if maxBodySize != "" {
		size, err := parseSize(maxBodySize)
		if err != nil {
			return fmt.Errorf("invalid max body size: %w", err)
		}
		cfg.Recording.MaxBodySize = size
	}

	// Parse command line filters
	if len(filters) > 0 {
		parsedFilters, err := parseFilters(filters)
		if err != nil {
			return fmt.Errorf("invalid filters: %w", err)
		}
		cfg.Recording.Filters = append(cfg.Recording.Filters, parsedFilters...)
	}

	// Enable recording
	cfg.Recording.Enabled = true

	fmt.Printf("✅ Recording enabled\n")
	fmt.Printf("📁 Storage directory: %s\n", cfg.Recording.Storage.Directory)
	fmt.Printf("📊 Max recordings: %d\n", cfg.Recording.MaxRecordings)
	fmt.Printf("📏 Max body size: %d bytes\n", cfg.Recording.MaxBodySize)
	fmt.Printf("🔍 Filters: %d configured\n", len(cfg.Recording.Filters))

	return nil
}

func runRecordStop(ctx context.Context, logger *zap.Logger, configPath string) error {
	fmt.Println("⏹️  Stopping recording...")

	// Load configuration
	cfg, err := loadConfigForRecording(configPath)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Disable recording
	cfg.Recording.Enabled = false

	fmt.Println("✅ Recording stopped")
	return nil
}

func runRecordList(ctx context.Context, logger *zap.Logger, configPath string, limit int, method, status, since string) error {
	// Load storage configuration
	cfg, err := loadConfigForRecording(configPath)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Create storage instance
	storage, err := recorder.NewFileStorage(&cfg.Recording.Storage, logger)
	if err != nil {
		return fmt.Errorf("failed to create storage: %w", err)
	}
	defer storage.Close()

	// Build filter
	filter := recorder.ListFilter{
		Limit: limit,
	}

	if method != "" {
		filter.Methods = []string{method}
	}

	if status != "" {
		statusCode, err := strconv.Atoi(status)
		if err != nil {
			return fmt.Errorf("invalid status code: %s", status)
		}
		filter.StatusCodes = []int{statusCode}
	}

	if since != "" {
		duration, err := time.ParseDuration(since)
		if err != nil {
			return fmt.Errorf("invalid duration: %s", since)
		}
		filter.StartTime = time.Now().Add(-duration)
	}

	// List recordings
	recordings, err := storage.List(filter)
	if err != nil {
		return fmt.Errorf("failed to list recordings: %w", err)
	}

	// Display results
	fmt.Printf("📋 Found %d recordings:\n\n", len(recordings))
	fmt.Printf("%-40s %-8s %-50s %-6s %-20s\n", "ID", "METHOD", "URI", "STATUS", "TIMESTAMP")
	fmt.Printf("%s\n", strings.Repeat("-", 130))

	for _, recording := range recordings {
		fmt.Printf("%-40s %-8s %-50s %-6d %-20s\n",
			recording.ID[:40],
			recording.Request.Method,
			truncateString(recording.Request.URI, 50),
			recording.Response.StatusCode,
			recording.Timestamp.Format("2006-01-02 15:04:05"))
	}

	return nil
}

func runRecordShow(ctx context.Context, logger *zap.Logger, configPath, recordingID, format string) error {
	// Load storage configuration
	cfg, err := loadConfigForRecording(configPath)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Create storage instance
	storage, err := recorder.NewFileStorage(&cfg.Recording.Storage, logger)
	if err != nil {
		return fmt.Errorf("failed to create storage: %w", err)
	}
	defer storage.Close()

	// Load recording
	recording, err := storage.Load(recordingID)
	if err != nil {
		return fmt.Errorf("failed to load recording: %w", err)
	}

	// Display recording based on format
	switch format {
	case "json":
		return displayRecordingJSON(recording)
	case "yaml":
		return displayRecordingYAML(recording)
	default:
		return displayRecordingTable(recording)
	}
}

func runRecordDelete(ctx context.Context, logger *zap.Logger, configPath string, recordingIDs []string, all bool, force bool) error {
	// Load storage configuration
	cfg, err := loadConfigForRecording(configPath)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Create storage instance
	storage, err := recorder.NewFileStorage(&cfg.Recording.Storage, logger)
	if err != nil {
		return fmt.Errorf("failed to create storage: %w", err)
	}
	defer storage.Close()

	if all {
		if !force {
			fmt.Print("⚠️  Are you sure you want to delete ALL recordings? (y/N): ")
			var response string
			fmt.Scanln(&response)
			if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
				fmt.Println("❌ Deletion cancelled")
				return nil
			}
		}

		if err := storage.DeleteAll(); err != nil {
			return fmt.Errorf("failed to delete all recordings: %w", err)
		}

		fmt.Println("✅ All recordings deleted")
		return nil
	}

	if len(recordingIDs) == 0 {
		return fmt.Errorf("no recording IDs specified")
	}

	// Delete specific recordings
	for _, id := range recordingIDs {
		if err := storage.Delete(id); err != nil {
			logger.Error("Failed to delete recording", zap.String("id", id), zap.Error(err))
			fmt.Printf("❌ Failed to delete recording %s: %v\n", id, err)
		} else {
			fmt.Printf("✅ Deleted recording %s\n", id)
		}
	}

	return nil
}

func runRecordReplay(ctx context.Context, logger *zap.Logger, configPath, targetURL string, concurrency int, delay string, recordingIDs []string, since string, limit int) error {
	fmt.Printf("🔄 Starting replay to %s...\n", targetURL)

	// Load storage configuration
	cfg, err := loadConfigForRecording(configPath)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Create storage instance
	storage, err := recorder.NewFileStorage(&cfg.Recording.Storage, logger)
	if err != nil {
		return fmt.Errorf("failed to create storage: %w", err)
	}
	defer storage.Close()

	// Create replayer
	replayer := recorder.NewReplayer(storage, logger)

	// Parse delay
	delayDuration, err := time.ParseDuration(delay)
	if err != nil {
		return fmt.Errorf("invalid delay duration: %w", err)
	}

	// Create replay configuration
	replayConfig := &recorder.ReplayConfig{
		TargetURL:    targetURL,
		Concurrency:  concurrency,
		DelayBetween: delayDuration,
		Timeout:      30 * time.Second,
		ReplaceHost:  true,
	}

	// Load recordings
	if len(recordingIDs) > 0 {
		if err := replayer.LoadRecordingsByIDs(recordingIDs); err != nil {
			return fmt.Errorf("failed to load recordings by IDs: %w", err)
		}
	} else {
		// Build filter for loading recordings
		filter := recorder.ListFilter{
			Limit: limit,
		}

		if since != "" {
			duration, err := time.ParseDuration(since)
			if err != nil {
				return fmt.Errorf("invalid duration: %s", since)
			}
			filter.StartTime = time.Now().Add(-duration)
		}

		if err := replayer.LoadRecordings(filter); err != nil {
			return fmt.Errorf("failed to load recordings: %w", err)
		}
	}

	// Start replay
	if err := replayer.ReplayTraffic(replayConfig); err != nil {
		return fmt.Errorf("replay failed: %w", err)
	}

	// Show stats
	stats := replayer.GetStats()
	fmt.Printf("\n📊 Replay completed:\n")
	fmt.Printf("   Total requests: %d\n", stats.TotalRequests)
	fmt.Printf("   Successful: %d\n", stats.SuccessRequests)
	fmt.Printf("   Failed: %d\n", stats.FailedRequests)
	fmt.Printf("   Average latency: %v\n", stats.AverageLatency)
	fmt.Printf("   Duration: %v\n", stats.EndTime.Sub(stats.StartTime))

	return nil
}

func runRecordExport(ctx context.Context, logger *zap.Logger, configPath, format, output string, recordingIDs []string) error {
	fmt.Printf("📤 Exporting recordings in %s format...\n", format)

	// Load storage configuration
	cfg, err := loadConfigForRecording(configPath)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Create storage instance
	storage, err := recorder.NewFileStorage(&cfg.Recording.Storage, logger)
	if err != nil {
		return fmt.Errorf("failed to create storage: %w", err)
	}
	defer storage.Close()

	// Load recordings
	var recordings []*recorder.Recording
	if len(recordingIDs) > 0 {
		for _, id := range recordingIDs {
			recording, err := storage.Load(id)
			if err != nil {
				logger.Warn("Failed to load recording", zap.String("id", id), zap.Error(err))
				continue
			}
			recordings = append(recordings, recording)
		}
	} else {
		// Load all recordings
		allRecordings, err := storage.List(recorder.ListFilter{})
		if err != nil {
			return fmt.Errorf("failed to list recordings: %w", err)
		}
		recordings = allRecordings
	}

	fmt.Printf("📋 Loaded %d recordings for export\n", len(recordings))

	// Export based on format
	switch format {
	case "har":
		return exportHAR(recordings, output)
	case "postman":
		return exportPostman(recordings, output)
	case "curl":
		return exportCurl(recordings, output)
	default:
		return exportJSON(recordings, output)
	}
}

// Helper functions

func loadConfigForRecording(configPath string) (*config.Config, error) {
	// If no config path specified, use defaults
	if configPath == "" {
		return config.DefaultConfig(), nil
	}

	// Load configuration from file
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return nil, err
	}

	return cfg, nil
}

func parseFilters(filterStrings []string) ([]config.RecordingFilter, error) {
	var filters []config.RecordingFilter

	for _, filterStr := range filterStrings {
		parts := strings.SplitN(filterStr, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid filter format: %s (expected type:value)", filterStr)
		}

		filterType := parts[0]
		value := parts[1]

		filter := config.RecordingFilter{
			Type:   filterType,
			Values: []string{value},
			Negate: false,
		}

		filters = append(filters, filter)
	}

	return filters, nil
}

func parseSize(sizeStr string) (int64, error) {
	sizeStr = strings.ToUpper(strings.TrimSpace(sizeStr))
	
	var multiplier int64 = 1
	var numberStr string

	if strings.HasSuffix(sizeStr, "KB") {
		multiplier = 1024
		numberStr = strings.TrimSuffix(sizeStr, "KB")
	} else if strings.HasSuffix(sizeStr, "MB") {
		multiplier = 1024 * 1024
		numberStr = strings.TrimSuffix(sizeStr, "MB")
	} else if strings.HasSuffix(sizeStr, "GB") {
		multiplier = 1024 * 1024 * 1024
		numberStr = strings.TrimSuffix(sizeStr, "GB")
	} else if strings.HasSuffix(sizeStr, "B") {
		numberStr = strings.TrimSuffix(sizeStr, "B")
	} else {
		numberStr = sizeStr
	}

	number, err := strconv.ParseInt(numberStr, 10, 64)
	if err != nil {
		return 0, err
	}

	return number * multiplier, nil
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func displayRecordingTable(recording *recorder.Recording) error {
	fmt.Printf("🎬 Recording Details\n\n")
	fmt.Printf("ID:        %s\n", recording.ID)
	fmt.Printf("Timestamp: %s\n", recording.Timestamp.Format("2006-01-02 15:04:05"))
	fmt.Printf("Duration:  %v\n", recording.Duration)
	fmt.Printf("\n📨 Request:\n")
	fmt.Printf("  Method: %s\n", recording.Request.Method)
	fmt.Printf("  URI:    %s\n", recording.Request.URI)
	fmt.Printf("  Headers (%d):\n", len(recording.Request.Headers))
	for key, value := range recording.Request.Headers {
		fmt.Printf("    %s: %s\n", key, value)
	}
	fmt.Printf("  Body:   %d bytes\n", len(recording.Request.Body))

	fmt.Printf("\n📤 Response:\n")
	fmt.Printf("  Status: %d\n", recording.Response.StatusCode)
	fmt.Printf("  Headers (%d):\n", len(recording.Response.Headers))
	for key, value := range recording.Response.Headers {
		fmt.Printf("    %s: %s\n", key, value)
	}
	fmt.Printf("  Body:   %d bytes\n", len(recording.Response.Body))

	fmt.Printf("\n🏷️  Metadata:\n")
	fmt.Printf("  Source:    %s\n", recording.Metadata.Source)
	fmt.Printf("  Client IP: %s\n", recording.Metadata.ClientIP)
	if recording.Metadata.UserAgent != "" {
		fmt.Printf("  User Agent: %s\n", recording.Metadata.UserAgent)
	}
	if recording.Metadata.RequestID != "" {
		fmt.Printf("  Request ID: %s\n", recording.Metadata.RequestID)
	}

	return nil
}

func displayRecordingJSON(recording *recorder.Recording) error {
	// Implementation would use json.MarshalIndent
	fmt.Printf("JSON format not yet implemented\n")
	return nil
}

func displayRecordingYAML(recording *recorder.Recording) error {
	// Implementation would use yaml.Marshal
	fmt.Printf("YAML format not yet implemented\n")
	return nil
}

func exportHAR(recordings []*recorder.Recording, output string) error {
	fmt.Printf("HAR export not yet implemented\n")
	return nil
}

func exportPostman(recordings []*recorder.Recording, output string) error {
	fmt.Printf("Postman export not yet implemented\n")
	return nil
}

func exportCurl(recordings []*recorder.Recording, output string) error {
	fmt.Printf("cURL export not yet implemented\n")
	return nil
}

func exportJSON(recordings []*recorder.Recording, output string) error {
	fmt.Printf("JSON export not yet implemented\n")
	return nil
}