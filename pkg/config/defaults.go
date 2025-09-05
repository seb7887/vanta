package config

import "time"

// DefaultConfig returns a configuration with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Port:            8080,
			Host:            "0.0.0.0",
			ReadTimeout:     30 * time.Second,
			WriteTimeout:    30 * time.Second,
			MaxConnsPerIP:   100,
			MaxRequestSize:  "10MB",
			Concurrency:     256000,
			ReusePort:       true,
		},
		Logging: LoggingConfig{
			Level:     "info",
			Format:    "json",
			Output:    "stdout",
			Sampling:  false,
			AddCaller: true,
		},
		Metrics: MetricsConfig{
			Enabled:    true,
			Port:       9090,
			Path:       "/metrics",
			Prometheus: true,
		},
		Chaos: ChaosConfig{
			Enabled:   false,
			Scenarios: []ScenarioConfig{},
		},
		Plugins: []PluginConfig{},
	}
}