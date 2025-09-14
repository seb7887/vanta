package validation

import (
	"time"
)

// Config holds validation configuration
type Config struct {
	// General validation settings
	Enabled              bool `json:"enabled" yaml:"enabled"`
	StrictMode           bool `json:"strict_mode" yaml:"strict_mode"`
	FailOnInvalid        bool `json:"fail_on_invalid" yaml:"fail_on_invalid"`

	// Request validation settings
	ValidateHeaders      bool `json:"validate_headers" yaml:"validate_headers"`
	ValidateQuery        bool `json:"validate_query" yaml:"validate_query"`
	ValidatePath         bool `json:"validate_path" yaml:"validate_path"`
	ValidateBody         bool `json:"validate_body" yaml:"validate_body"`

	// Response validation settings
	ValidateStatusCodes  bool `json:"validate_status_codes" yaml:"validate_status_codes"`

	// Schema validation settings
	AllowExtraFields     bool `json:"allow_extra_fields" yaml:"allow_extra_fields"`
	ValidateFormats      bool `json:"validate_formats" yaml:"validate_formats"`

	// Coverage and reporting settings
	CoverageReporting    bool              `json:"coverage_reporting" yaml:"coverage_reporting"`
	ReportFormat         []string          `json:"report_format" yaml:"report_format"`
	ReportPath           string            `json:"report_path" yaml:"report_path"`
	ReportInterval       time.Duration     `json:"report_interval" yaml:"report_interval"`

	// Performance settings
	MaxConcurrentValidations int `json:"max_concurrent_validations" yaml:"max_concurrent_validations"`
	ValidationTimeout        time.Duration `json:"validation_timeout" yaml:"validation_timeout"`

	// Custom validators
	CustomValidators map[string]CustomValidatorConfig `json:"custom_validators" yaml:"custom_validators"`
}

// CustomValidatorConfig defines configuration for custom validators
type CustomValidatorConfig struct {
	Type   string                 `json:"type" yaml:"type"`
	Config map[string]interface{} `json:"config" yaml:"config"`
}

// DefaultValidationConfig returns a default validation configuration
func DefaultValidationConfig() *Config {
	return &Config{
		Enabled:                  true,
		StrictMode:               false,
		FailOnInvalid:           false,
		ValidateHeaders:         true,
		ValidateQuery:           true,
		ValidatePath:            true,
		ValidateBody:            true,
		ValidateStatusCodes:     true,
		AllowExtraFields:        true,
		ValidateFormats:         true,
		CoverageReporting:       true,
		ReportFormat:            []string{"json", "html"},
		ReportPath:              "./validation-reports",
		ReportInterval:          5 * time.Minute,
		MaxConcurrentValidations: 100,
		ValidationTimeout:       30 * time.Second,
		CustomValidators:        make(map[string]CustomValidatorConfig),
	}
}

// Merge merges another config into this one, with the other config taking precedence
func (c *Config) Merge(other *Config) {
	if other == nil {
		return
	}

	if other.Enabled != c.Enabled {
		c.Enabled = other.Enabled
	}

	if other.StrictMode != c.StrictMode {
		c.StrictMode = other.StrictMode
	}

	if other.FailOnInvalid != c.FailOnInvalid {
		c.FailOnInvalid = other.FailOnInvalid
	}

	if other.ValidateHeaders != c.ValidateHeaders {
		c.ValidateHeaders = other.ValidateHeaders
	}

	if other.ValidateQuery != c.ValidateQuery {
		c.ValidateQuery = other.ValidateQuery
	}

	if other.ValidatePath != c.ValidatePath {
		c.ValidatePath = other.ValidatePath
	}

	if other.ValidateBody != c.ValidateBody {
		c.ValidateBody = other.ValidateBody
	}

	if other.ValidateStatusCodes != c.ValidateStatusCodes {
		c.ValidateStatusCodes = other.ValidateStatusCodes
	}

	if other.AllowExtraFields != c.AllowExtraFields {
		c.AllowExtraFields = other.AllowExtraFields
	}

	if other.ValidateFormats != c.ValidateFormats {
		c.ValidateFormats = other.ValidateFormats
	}

	if other.CoverageReporting != c.CoverageReporting {
		c.CoverageReporting = other.CoverageReporting
	}

	if len(other.ReportFormat) > 0 {
		c.ReportFormat = other.ReportFormat
	}

	if other.ReportPath != "" {
		c.ReportPath = other.ReportPath
	}

	if other.ReportInterval != 0 {
		c.ReportInterval = other.ReportInterval
	}

	if other.MaxConcurrentValidations != 0 {
		c.MaxConcurrentValidations = other.MaxConcurrentValidations
	}

	if other.ValidationTimeout != 0 {
		c.ValidationTimeout = other.ValidationTimeout
	}

	if len(other.CustomValidators) > 0 {
		if c.CustomValidators == nil {
			c.CustomValidators = make(map[string]CustomValidatorConfig)
		}
		for k, v := range other.CustomValidators {
			c.CustomValidators[k] = v
		}
	}
}

// ValidationManager manages request and response validators
type ValidationManager struct {
	requestValidator  *RequestValidator
	responseValidator *ResponseValidator
	reporter          *Reporter
	config            *Config
}

// NewValidationManager creates a new validation manager
func NewValidationManager(spec *openapi.Specification, config *Config) *ValidationManager {
	if config == nil {
		config = DefaultValidationConfig()
	}

	requestValidator := NewRequestValidator(spec, config)
	responseValidator := NewResponseValidator(spec, config)
	reporter := NewReporter()

	return &ValidationManager{
		requestValidator:  requestValidator,
		responseValidator: responseValidator,
		reporter:          reporter,
		config:            config,
	}
}

// GetRequestValidator returns the request validator
func (vm *ValidationManager) GetRequestValidator() *RequestValidator {
	return vm.requestValidator
}

// GetResponseValidator returns the response validator
func (vm *ValidationManager) GetResponseValidator() *ResponseValidator {
	return vm.responseValidator
}

// GetReporter returns the reporter
func (vm *ValidationManager) GetReporter() *Reporter {
	return vm.reporter
}

// GetConfig returns the validation configuration
func (vm *ValidationManager) GetConfig() *Config {
	return vm.config
}

// UpdateConfig updates the validation configuration
func (vm *ValidationManager) UpdateConfig(config *Config) {
	if config != nil {
		vm.config = config
		// TODO: Update validators with new config
	}
}