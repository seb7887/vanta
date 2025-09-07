package config

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
)

// ValidationError represents a configuration validation error
type ValidationError struct {
	Field   string
	Value   interface{}
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("validation error in field '%s': %s (value: %v)", e.Field, e.Message, e.Value)
}

// ValidationErrors represents multiple validation errors
type ValidationErrors []ValidationError

func (ve ValidationErrors) Error() string {
	if len(ve) == 0 {
		return ""
	}
	if len(ve) == 1 {
		return ve[0].Error()
	}
	
	var sb strings.Builder
	sb.WriteString(ve[0].Error())
	if len(ve) > 1 {
		sb.WriteString(fmt.Sprintf(" (and %d more errors)", len(ve)-1))
	}
	return sb.String()
}

// Validate validates the complete configuration
func Validate(cfg *Config) error {
	var errors ValidationErrors

	// Validate server configuration
	if errs := validateServer(&cfg.Server); len(errs) > 0 {
		errors = append(errors, errs...)
	}

	// Validate logging configuration
	if errs := validateLogging(&cfg.Logging); len(errs) > 0 {
		errors = append(errors, errs...)
	}

	// Validate metrics configuration
	if errs := validateMetrics(&cfg.Metrics); len(errs) > 0 {
		errors = append(errors, errs...)
	}

	// Validate chaos configuration
	if errs := validateChaos(&cfg.Chaos); len(errs) > 0 {
		errors = append(errors, errs...)
	}

	if len(errors) > 0 {
		return errors
	}

	return nil
}

func validateServer(cfg *ServerConfig) ValidationErrors {
	var errors ValidationErrors

	// Validate port
	if cfg.Port < 1 || cfg.Port > 65535 {
		errors = append(errors, ValidationError{
			Field:   "server.port",
			Value:   cfg.Port,
			Message: "must be between 1 and 65535",
		})
	}

	// Validate host
	if cfg.Host == "" {
		errors = append(errors, ValidationError{
			Field:   "server.host",
			Value:   cfg.Host,
			Message: "cannot be empty",
		})
	} else if net.ParseIP(cfg.Host) == nil && cfg.Host != "localhost" {
		// Try to resolve as hostname
		if _, err := net.LookupHost(cfg.Host); err != nil {
			errors = append(errors, ValidationError{
				Field:   "server.host",
				Value:   cfg.Host,
				Message: "must be a valid IP address or hostname",
			})
		}
	}

	// Validate timeouts
	if cfg.ReadTimeout <= 0 {
		errors = append(errors, ValidationError{
			Field:   "server.read_timeout",
			Value:   cfg.ReadTimeout,
			Message: "must be greater than 0",
		})
	}

	if cfg.WriteTimeout <= 0 {
		errors = append(errors, ValidationError{
			Field:   "server.write_timeout",
			Value:   cfg.WriteTimeout,
			Message: "must be greater than 0",
		})
	}

	// Validate max request size
	if cfg.MaxRequestSize != "" {
		if _, err := parseSize(cfg.MaxRequestSize); err != nil {
			errors = append(errors, ValidationError{
				Field:   "server.max_request_size",
				Value:   cfg.MaxRequestSize,
				Message: "invalid size format (use formats like '10MB', '1GB')",
			})
		}
	}

	// Validate concurrency
	if cfg.Concurrency < 1 {
		errors = append(errors, ValidationError{
			Field:   "server.concurrency",
			Value:   cfg.Concurrency,
			Message: "must be greater than 0",
		})
	}

	return errors
}

func validateLogging(cfg *LoggingConfig) ValidationErrors {
	var errors ValidationErrors

	// Validate log level
	validLevels := []string{"debug", "info", "warn", "error", "dpanic", "panic", "fatal"}
	levelValid := false
	for _, level := range validLevels {
		if cfg.Level == level {
			levelValid = true
			break
		}
	}
	if !levelValid {
		errors = append(errors, ValidationError{
			Field:   "logging.level",
			Value:   cfg.Level,
			Message: fmt.Sprintf("must be one of: %s", strings.Join(validLevels, ", ")),
		})
	}

	// Validate format
	if cfg.Format != "json" && cfg.Format != "console" {
		errors = append(errors, ValidationError{
			Field:   "logging.format",
			Value:   cfg.Format,
			Message: "must be either 'json' or 'console'",
		})
	}

	return errors
}

func validateMetrics(cfg *MetricsConfig) ValidationErrors {
	var errors ValidationErrors

	if cfg.Enabled {
		// Validate metrics port
		if cfg.Port < 1 || cfg.Port > 65535 {
			errors = append(errors, ValidationError{
				Field:   "metrics.port",
				Value:   cfg.Port,
				Message: "must be between 1 and 65535",
			})
		}

		// Validate metrics path
		if cfg.Path == "" || !strings.HasPrefix(cfg.Path, "/") {
			errors = append(errors, ValidationError{
				Field:   "metrics.path",
				Value:   cfg.Path,
				Message: "must start with '/'",
			})
		}
	}

	return errors
}

func validateChaos(cfg *ChaosConfig) ValidationErrors {
	var errors ValidationErrors

	if cfg.Enabled {
		for i, scenario := range cfg.Scenarios {
			// Validate scenario name
			if scenario.Name == "" {
				errors = append(errors, ValidationError{
					Field:   fmt.Sprintf("chaos.scenarios[%d].name", i),
					Value:   scenario.Name,
					Message: "cannot be empty",
				})
			}

			// Validate scenario type
			validTypes := []string{"latency", "error", "timeout"}
			typeValid := false
			for _, t := range validTypes {
				if scenario.Type == t {
					typeValid = true
					break
				}
			}
			if !typeValid {
				errors = append(errors, ValidationError{
					Field:   fmt.Sprintf("chaos.scenarios[%d].type", i),
					Value:   scenario.Type,
					Message: fmt.Sprintf("must be one of: %s", strings.Join(validTypes, ", ")),
				})
			}

			// Validate probability
			if scenario.Probability < 0 || scenario.Probability > 1 {
				errors = append(errors, ValidationError{
					Field:   fmt.Sprintf("chaos.scenarios[%d].probability", i),
					Value:   scenario.Probability,
					Message: "must be between 0 and 1",
				})
			}
		}
	}

	return errors
}

// ValidateConfig validates the complete configuration (alias for Validate)
func ValidateConfig(cfg *Config) error {
	return Validate(cfg)
}

// parseSize parses a size string like "10MB" and returns bytes
func parseSize(size string) (int64, error) {
	size = strings.TrimSpace(strings.ToUpper(size))
	
	units := map[string]int64{
		"B":  1,
		"KB": 1024,
		"MB": 1024 * 1024,
		"GB": 1024 * 1024 * 1024,
		"TB": 1024 * 1024 * 1024 * 1024,
	}

	for unit, multiplier := range units {
		if strings.HasSuffix(size, unit) {
			numStr := strings.TrimSuffix(size, unit)
			num, err := strconv.ParseFloat(numStr, 64)
			if err != nil {
				return 0, err
			}
			return int64(num * float64(multiplier)), nil
		}
	}

	// Try parsing as plain number (assume bytes)
	num, err := strconv.ParseInt(size, 10, 64)
	if err != nil {
		return 0, errors.New("invalid size format")
	}

	return num, nil
}