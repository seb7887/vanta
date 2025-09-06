package config

import (
	"time"
)

// Config represents the complete configuration structure
type Config struct {
	Server  ServerConfig  `yaml:"server"`
	Mock    MockConfig    `yaml:"mock"`
	Chaos   ChaosConfig   `yaml:"chaos"`
	Plugins []PluginConfig `yaml:"plugins"`
	Logging LoggingConfig `yaml:"logging"`
	Metrics MetricsConfig `yaml:"metrics"`
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	Port            int           `yaml:"port"`
	Host            string        `yaml:"host"`
	ReadTimeout     time.Duration `yaml:"read_timeout"`
	WriteTimeout    time.Duration `yaml:"write_timeout"`
	MaxConnsPerIP   int           `yaml:"max_conns_per_ip"`
	MaxRequestSize  string        `yaml:"max_request_size"`
	Concurrency     int           `yaml:"concurrency"`
	ReusePort       bool          `yaml:"reuse_port"`
}

// MockConfig holds mock data generation configuration
type MockConfig struct {
	Seed             int64  `yaml:"seed"`               // Random seed for reproducible data generation
	Locale           string `yaml:"locale"`             // Locale for data generation (e.g., "en", "es", "fr")
	MaxDepth         int    `yaml:"max_depth"`          // Maximum depth for nested object generation
	DefaultArraySize int    `yaml:"default_array_size"` // Default size for arrays when not specified
	PreferExamples   bool   `yaml:"prefer_examples"`    // Prefer examples from OpenAPI spec when available
}

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	Level      string `yaml:"level"`
	Format     string `yaml:"format"` // json or console
	Output     string `yaml:"output"` // stdout, stderr, or file path
	Sampling   bool   `yaml:"sampling"`
	AddCaller  bool   `yaml:"add_caller"`
}

// MetricsConfig holds metrics configuration
type MetricsConfig struct {
	Enabled    bool   `yaml:"enabled"`
	Port       int    `yaml:"port"`
	Path       string `yaml:"path"`
	Prometheus bool   `yaml:"prometheus"`
}

// ChaosConfig holds chaos testing configuration
type ChaosConfig struct {
	Enabled   bool             `yaml:"enabled"`
	Scenarios []ScenarioConfig `yaml:"scenarios"`
}

// ScenarioConfig represents a single chaos scenario
type ScenarioConfig struct {
	Name        string                 `yaml:"name"`
	Type        string                 `yaml:"type"` // latency, error, timeout
	Endpoints   []string               `yaml:"endpoints"`
	Probability float64                `yaml:"probability"`
	Parameters  map[string]interface{} `yaml:"parameters"`
}

// PluginConfig holds plugin configuration
type PluginConfig struct {
	Name    string                 `yaml:"name"`
	Enabled bool                   `yaml:"enabled"`
	Config  map[string]interface{} `yaml:"config"`
}