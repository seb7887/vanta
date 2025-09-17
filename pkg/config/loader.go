package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mitchellh/mapstructure"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/yaml.v3"
)

// LoadFromFile loads configuration from a YAML file
func LoadFromFile(configPath string) (*Config, error) {
	v := viper.New()
	
	// Set config file path
	v.SetConfigFile(configPath)
	
	// Set environment variable prefix
	v.SetEnvPrefix("VANTA")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Set defaults
	setDefaults(v)

	// Read config file
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Get all settings as a map
	settings := v.AllSettings()

	// Unmarshal into config struct with custom decode hooks for duration parsing
	var cfg Config
	decoderConfig := &mapstructure.DecoderConfig{
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			mapstructure.StringToTimeDurationHookFunc(),
		),
		Result:           &cfg,
		TagName:          "yaml",
		WeaklyTypedInput: true,
	}

	decoder, err := mapstructure.NewDecoder(decoderConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create decoder: %w", err)
	}

	if err := decoder.Decode(settings); err != nil {
		return nil, fmt.Errorf("failed to decode config: %w", err)
	}

	return &cfg, nil
}

// WriteToFile writes configuration to a YAML file
func WriteToFile(cfg *Config, filePath string) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// LoadConfig loads configuration from a file path (alias for LoadFromFile)
func LoadConfig(configPath string) (*Config, error) {
	return LoadFromFile(configPath)
}

// setDefaults sets default values in viper
func setDefaults(v *viper.Viper) {
	// Server defaults - use time.Duration values for proper parsing
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.read_timeout", 30*time.Second)
	v.SetDefault("server.write_timeout", 30*time.Second)
	v.SetDefault("server.max_conns_per_ip", 100)
	v.SetDefault("server.max_request_size", "10MB")
	v.SetDefault("server.concurrency", 256000)
	v.SetDefault("server.reuse_port", true)

	// Logging defaults
	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.format", "json")
	v.SetDefault("logging.output", "stdout")
	v.SetDefault("logging.sampling", false)
	v.SetDefault("logging.add_caller", true)

	// Metrics defaults
	v.SetDefault("metrics.enabled", true)
	v.SetDefault("metrics.port", 9090)
	v.SetDefault("metrics.path", "/metrics")
	v.SetDefault("metrics.prometheus", true)

	// Chaos defaults
	v.SetDefault("chaos.enabled", false)

	// Hot reload defaults
	v.SetDefault("hotreload.enabled", false)
	v.SetDefault("hotreload.watch_config", true)
	v.SetDefault("hotreload.watch_spec", true)
	v.SetDefault("hotreload.debounce_delay", 500*time.Millisecond)
}

// NewLoggerFromConfig creates a zap logger from the logging configuration
func NewLoggerFromConfig(logConfig LoggingConfig) (*zap.Logger, error) {
	// Parse log level
	level, err := zapcore.ParseLevel(logConfig.Level)
	if err != nil {
		return nil, fmt.Errorf("invalid log level '%s': %w", logConfig.Level, err)
	}

	// Configure encoder
	var config zap.Config
	if logConfig.Format == "console" {
		config = zap.NewDevelopmentConfig()
		config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	} else {
		config = zap.NewProductionConfig()
	}

	// Set log level
	config.Level = zap.NewAtomicLevelAt(level)

	// Configure sampling
	if !logConfig.Sampling {
		config.Sampling = nil
	}

	// Configure caller information
	config.DisableCaller = !logConfig.AddCaller

	// Configure output paths
	switch logConfig.Output {
	case "stdout", "":
		config.OutputPaths = []string{"stdout"}
	case "stderr":
		config.OutputPaths = []string{"stderr"}
	default:
		// If it's not stdout/stderr, treat it as a file path
		config.OutputPaths = []string{logConfig.Output}
	}

	// Build the logger
	logger, err := config.Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build logger: %w", err)
	}

	return logger, nil
}

// NewLoggerWithDefaults creates a logger with sensible defaults if no config is provided
func NewLoggerWithDefaults() (*zap.Logger, error) {
	defaultLogConfig := LoggingConfig{
		Level:     "info",
		Format:    "json",
		Output:    "stdout",
		Sampling:  false,
		AddCaller: true,
	}
	return NewLoggerFromConfig(defaultLogConfig)
}