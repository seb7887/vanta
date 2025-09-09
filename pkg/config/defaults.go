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
		Mock: MockConfig{
			Seed:             0,     // 0 means use current timestamp
			Locale:           "en",  // English by default
			MaxDepth:         5,     // Reasonable depth to prevent infinite recursion
			DefaultArraySize: 2,     // Small default array size
			PreferExamples:   true,  // Prefer OpenAPI examples when available
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
		Middleware: MiddlewareConfig{
			RequestID: true, // Enable request ID by default for traceability
			CORS: CORSConfig{
				Enabled:          false, // Disabled by default for security
				AllowOrigins:     []string{"*"},
				AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
				AllowHeaders:     []string{"Content-Type", "Authorization", "X-Request-ID"},
				AllowCredentials: false,
				MaxAge:           3600,
			},
			Timeout: TimeoutConfig{
				Enabled:  false, // Disabled by default to avoid breaking existing functionality
				Duration: 30 * time.Second,
			},
			Recovery: RecoveryConfig{
				Enabled:    true,  // Enabled by default for stability
				PrintStack: false, // Don't print to stdout by default
				LogStack:   true,  // Log stack traces for debugging
			},
		},
		HotReload: HotReloadConfig{
			Enabled:       false, // Disabled by default
			WatchConfig:   true,  // Watch config file when enabled
			WatchSpec:     true,  // Watch OpenAPI spec file when enabled
			DebounceDelay: 500 * time.Millisecond, // Default debounce delay
		},
		Recording: RecordingConfig{
			Enabled:       false, // Disabled by default
			MaxRecordings: 1000,  // Default max recordings
			MaxBodySize:   1024 * 1024, // 1MB default max body size
			Storage: StorageConfig{
				Type:      "file",      // File storage by default
				Directory: "./recordings", // Default directory
				Format:    "jsonlines", // JSON Lines format by default
			},
			Filters:        []RecordingFilter{}, // No filters by default
			IncludeHeaders: []string{}, // Include all headers by default
			ExcludeHeaders: []string{ // Exclude sensitive headers by default
				"cookie",
				"set-cookie",
				"authorization", 
				"x-api-key",
			},
		},
	}
}