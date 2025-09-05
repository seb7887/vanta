package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
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

	// Unmarshal into config struct
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
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

// setDefaults sets default values in viper
func setDefaults(v *viper.Viper) {
	// Server defaults - use time.Duration values for proper parsing
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.read_timeout", time.Duration(30*time.Second))
	v.SetDefault("server.write_timeout", time.Duration(30*time.Second))
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
}