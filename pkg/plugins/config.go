package plugins

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"vanta/pkg/config"
)

// ConfigVersion represents the version of the plugin configuration schema
type ConfigVersion string

const (
	ConfigVersionV1 ConfigVersion = "v1"
	CurrentVersion  ConfigVersion = ConfigVersionV1
)

// PluginLoadResult contains the result of loading a plugin from configuration
type PluginLoadResult struct {
	Name    string                 `json:"name"`
	Plugin  Plugin                 `json:"-"`
	Config  map[string]interface{} `json:"config"`
	Enabled bool                   `json:"enabled"`
	Error   error                  `json:"error,omitempty"`
	LoadedAt time.Time             `json:"loaded_at"`
}

// ConfigValidationError represents a configuration validation error with field path
type ConfigValidationError struct {
	Field   string `json:"field"`
	Value   interface{} `json:"value"`
	Message string `json:"message"`
	Rule    string `json:"rule"`
}

func (e *ConfigValidationError) Error() string {
	return fmt.Sprintf("validation failed for field '%s': %s (value: %v)", e.Field, e.Message, e.Value)
}

// ConfigValidationResult contains the result of configuration validation
type ConfigValidationResult struct {
	Valid  bool                     `json:"valid"`
	Errors []ConfigValidationError  `json:"errors"`
}

// ConfigMigration represents a configuration migration from one version to another
type ConfigMigration struct {
	FromVersion ConfigVersion                                      `json:"from_version"`
	ToVersion   ConfigVersion                                      `json:"to_version"`
	Migrate     func(config map[string]interface{}) (map[string]interface{}, error) `json:"-"`
}

// JSONSchemaProperty represents a JSON schema property definition
type JSONSchemaProperty struct {
	Type        string                         `json:"type,omitempty"`
	Description string                         `json:"description,omitempty"`
	Default     interface{}                    `json:"default,omitempty"`
	Required    bool                           `json:"required,omitempty"`
	Enum        []interface{}                  `json:"enum,omitempty"`
	Minimum     *float64                       `json:"minimum,omitempty"`
	Maximum     *float64                       `json:"maximum,omitempty"`
	MinLength   *int                           `json:"minLength,omitempty"`
	MaxLength   *int                           `json:"maxLength,omitempty"`
	Pattern     string                         `json:"pattern,omitempty"`
	Properties  map[string]JSONSchemaProperty  `json:"properties,omitempty"`
	Items       *JSONSchemaProperty            `json:"items,omitempty"`
}

// JSONSchema represents a JSON schema for plugin configuration
type JSONSchema struct {
	Schema     string                        `json:"$schema"`
	Type       string                        `json:"type"`
	Title      string                        `json:"title"`
	Properties map[string]JSONSchemaProperty `json:"properties"`
	Required   []string                      `json:"required"`
	Version    ConfigVersion                 `json:"version"`
}

// PluginConfigRegistry manages plugin configuration schemas and validation
type PluginConfigRegistry struct {
	schemas    map[string]*JSONSchema
	migrations map[string][]ConfigMigration
	validators map[string]func(config map[string]interface{}) []ConfigValidationError
	mu         sync.RWMutex
}

// NewPluginConfigRegistry creates a new plugin configuration registry
func NewPluginConfigRegistry() *PluginConfigRegistry {
	registry := &PluginConfigRegistry{
		schemas:    make(map[string]*JSONSchema),
		migrations: make(map[string][]ConfigMigration),
		validators: make(map[string]func(config map[string]interface{}) []ConfigValidationError),
	}
	
	// Register built-in plugin schemas
	registry.registerBuiltinSchemas()
	
	return registry
}

// RegisterSchema registers a JSON schema for a plugin
func (r *PluginConfigRegistry) RegisterSchema(pluginName string, schema *JSONSchema) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	if schema.Version == "" {
		schema.Version = CurrentVersion
	}
	
	r.schemas[pluginName] = schema
	return nil
}

// RegisterMigration registers a configuration migration for a plugin
func (r *PluginConfigRegistry) RegisterMigration(pluginName string, migration ConfigMigration) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	r.migrations[pluginName] = append(r.migrations[pluginName], migration)
	return nil
}

// RegisterValidator registers a custom validator function for a plugin
func (r *PluginConfigRegistry) RegisterValidator(pluginName string, validator func(config map[string]interface{}) []ConfigValidationError) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	r.validators[pluginName] = validator
	return nil
}

// GetSchema returns the JSON schema for a plugin
func (r *PluginConfigRegistry) GetSchema(pluginName string) (*JSONSchema, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	schema, exists := r.schemas[pluginName]
	return schema, exists
}

// ValidateConfig validates a plugin configuration against its schema
func (r *PluginConfigRegistry) ValidateConfig(pluginName string, config map[string]interface{}) ConfigValidationResult {
	r.mu.RLock()
	schema, hasSchema := r.schemas[pluginName]
	validator, hasValidator := r.validators[pluginName]
	r.mu.RUnlock()
	
	var errors []ConfigValidationError
	
	// Apply JSON schema validation if available
	if hasSchema {
		errors = append(errors, r.validateAgainstSchema(schema, config, "")...)
	}
	
	// Apply custom validation if available
	if hasValidator {
		errors = append(errors, validator(config)...)
	}
	
	return ConfigValidationResult{
		Valid:  len(errors) == 0,
		Errors: errors,
	}
}

// MigrateConfig migrates a configuration to the current version
func (r *PluginConfigRegistry) MigrateConfig(pluginName string, config map[string]interface{}, currentVersion ConfigVersion) (map[string]interface{}, error) {
	r.mu.RLock()
	migrations, hasMigrations := r.migrations[pluginName]
	r.mu.RUnlock()
	
	if !hasMigrations {
		return config, nil
	}
	
	// Apply migrations in sequence
	result := config
	for _, migration := range migrations {
		if migration.FromVersion == currentVersion {
			migrated, err := migration.Migrate(result)
			if err != nil {
				return nil, fmt.Errorf("migration from %s to %s failed: %w", migration.FromVersion, migration.ToVersion, err)
			}
			result = migrated
			currentVersion = migration.ToVersion
		}
	}
	
	return result, nil
}

// validateAgainstSchema validates configuration against a JSON schema
func (r *PluginConfigRegistry) validateAgainstSchema(schema *JSONSchema, config map[string]interface{}, fieldPath string) []ConfigValidationError {
	var errors []ConfigValidationError
	
	// Check required fields
	for _, required := range schema.Required {
		if _, exists := config[required]; !exists {
			errors = append(errors, ConfigValidationError{
				Field:   r.buildFieldPath(fieldPath, required),
				Message: "required field is missing",
				Rule:    "required",
			})
		}
	}
	
	// Validate each property
	for key, value := range config {
		property, hasProperty := schema.Properties[key]
		if !hasProperty {
			continue // Allow additional properties for flexibility
		}
		
		fieldPathKey := r.buildFieldPath(fieldPath, key)
		errors = append(errors, r.validateProperty(property, value, fieldPathKey)...)
	}
	
	return errors
}

// validateProperty validates a single property against its schema
func (r *PluginConfigRegistry) validateProperty(property JSONSchemaProperty, value interface{}, fieldPath string) []ConfigValidationError {
	var errors []ConfigValidationError
	
	// Type validation
	if property.Type != "" && !r.isValidType(value, property.Type) {
		errors = append(errors, ConfigValidationError{
			Field:   fieldPath,
			Value:   value,
			Message: fmt.Sprintf("expected type %s", property.Type),
			Rule:    "type",
		})
		return errors // Stop validation if type is wrong
	}
	
	// Enum validation
	if len(property.Enum) > 0 && !r.isInEnum(value, property.Enum) {
		errors = append(errors, ConfigValidationError{
			Field:   fieldPath,
			Value:   value,
			Message: fmt.Sprintf("value must be one of: %v", property.Enum),
			Rule:    "enum",
		})
	}
	
	// Numeric validations
	if num, ok := r.toFloat64(value); ok {
		if property.Minimum != nil && num < *property.Minimum {
			errors = append(errors, ConfigValidationError{
				Field:   fieldPath,
				Value:   value,
				Message: fmt.Sprintf("value must be >= %v", *property.Minimum),
				Rule:    "minimum",
			})
		}
		if property.Maximum != nil && num > *property.Maximum {
			errors = append(errors, ConfigValidationError{
				Field:   fieldPath,
				Value:   value,
				Message: fmt.Sprintf("value must be <= %v", *property.Maximum),
				Rule:    "maximum",
			})
		}
	}
	
	// String validations
	if str, ok := value.(string); ok {
		if property.MinLength != nil && len(str) < *property.MinLength {
			errors = append(errors, ConfigValidationError{
				Field:   fieldPath,
				Value:   value,
				Message: fmt.Sprintf("string length must be >= %d", *property.MinLength),
				Rule:    "minLength",
			})
		}
		if property.MaxLength != nil && len(str) > *property.MaxLength {
			errors = append(errors, ConfigValidationError{
				Field:   fieldPath,
				Value:   value,
				Message: fmt.Sprintf("string length must be <= %d", *property.MaxLength),
				Rule:    "maxLength",
			})
		}
		if property.Pattern != "" {
			if matched, _ := regexp.MatchString(property.Pattern, str); !matched {
				errors = append(errors, ConfigValidationError{
					Field:   fieldPath,
					Value:   value,
					Message: fmt.Sprintf("string must match pattern: %s", property.Pattern),
					Rule:    "pattern",
				})
			}
		}
	}
	
	// Object validation
	if property.Type == "object" && len(property.Properties) > 0 {
		if obj, ok := value.(map[string]interface{}); ok {
			schema := &JSONSchema{
				Properties: property.Properties,
			}
			errors = append(errors, r.validateAgainstSchema(schema, obj, fieldPath)...)
		}
	}
	
	// Array validation
	if property.Type == "array" && property.Items != nil {
		if arr, ok := value.([]interface{}); ok {
			for i, item := range arr {
				itemPath := fmt.Sprintf("%s[%d]", fieldPath, i)
				errors = append(errors, r.validateProperty(*property.Items, item, itemPath)...)
			}
		}
	}
	
	return errors
}

// isValidType checks if a value matches the expected JSON schema type
func (r *PluginConfigRegistry) isValidType(value interface{}, expectedType string) bool {
	switch expectedType {
	case "string":
		_, ok := value.(string)
		return ok
	case "number":
		return r.isNumeric(value)
	case "integer":
		return r.isInteger(value)
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "array":
		_, ok := value.([]interface{})
		return ok
	case "object":
		_, ok := value.(map[string]interface{})
		return ok
	case "null":
		return value == nil
	default:
		return true // Unknown types pass validation
	}
}

// isNumeric checks if a value is numeric
func (r *PluginConfigRegistry) isNumeric(value interface{}) bool {
	switch value.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return true
	default:
		return false
	}
}

// isInteger checks if a value is an integer
func (r *PluginConfigRegistry) isInteger(value interface{}) bool {
	switch value.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return true
	default:
		return false
	}
}

// toFloat64 converts a numeric value to float64
func (r *PluginConfigRegistry) toFloat64(value interface{}) (float64, bool) {
	switch v := value.(type) {
	case int:
		return float64(v), true
	case int8:
		return float64(v), true
	case int16:
		return float64(v), true
	case int32:
		return float64(v), true
	case int64:
		return float64(v), true
	case uint:
		return float64(v), true
	case uint8:
		return float64(v), true
	case uint16:
		return float64(v), true
	case uint32:
		return float64(v), true
	case uint64:
		return float64(v), true
	case float32:
		return float64(v), true
	case float64:
		return v, true
	default:
		return 0, false
	}
}

// isInEnum checks if a value is in the enum list
func (r *PluginConfigRegistry) isInEnum(value interface{}, enum []interface{}) bool {
	for _, item := range enum {
		if value == item {
			return true
		}
	}
	return false
}

// buildFieldPath builds a nested field path for error reporting
func (r *PluginConfigRegistry) buildFieldPath(parent, field string) string {
	if parent == "" {
		return field
	}
	return fmt.Sprintf("%s.%s", parent, field)
}

// substituteEnvironmentVariables performs environment variable substitution in configuration
func (r *PluginConfigRegistry) substituteEnvironmentVariables(config map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	
	for key, value := range config {
		result[key] = r.substituteValue(value)
	}
	
	return result
}

// substituteValue performs environment variable substitution on a single value
func (r *PluginConfigRegistry) substituteValue(value interface{}) interface{} {
	switch v := value.(type) {
	case string:
		return r.expandEnvironmentVariables(v)
	case map[string]interface{}:
		return r.substituteEnvironmentVariables(v)
	case []interface{}:
		result := make([]interface{}, len(v))
		for i, item := range v {
			result[i] = r.substituteValue(item)
		}
		return result
	default:
		return value
	}
}

// expandEnvironmentVariables expands environment variables in a string
func (r *PluginConfigRegistry) expandEnvironmentVariables(s string) string {
	// Support ${VAR} and ${VAR:default} syntax
	re := regexp.MustCompile(`\$\{([^}:]+)(?::([^}]*))?\}`)
	
	return re.ReplaceAllStringFunc(s, func(match string) string {
		parts := re.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}
		
		varName := parts[1]
		defaultValue := ""
		if len(parts) > 2 {
			defaultValue = parts[2]
		}
		
		if envValue := os.Getenv(varName); envValue != "" {
			return envValue
		}
		return defaultValue
	})
}

// Global configuration registry instance
var globalConfigRegistry = NewPluginConfigRegistry()

// Factory Functions for Plugin Configuration Management

// CreatePluginFromConfig creates a plugin instance from configuration
func CreatePluginFromConfig(name string, config map[string]interface{}) (Plugin, error) {
	// Substitute environment variables
	config = globalConfigRegistry.substituteEnvironmentVariables(config)
	
	// Validate configuration
	validationResult := globalConfigRegistry.ValidateConfig(name, config)
	if !validationResult.Valid {
		var errorMessages []string
		for _, err := range validationResult.Errors {
			errorMessages = append(errorMessages, err.Error())
		}
		return nil, fmt.Errorf("configuration validation failed: %s", strings.Join(errorMessages, "; "))
	}
	
	// Create plugin using built-in factory or registry
	if factory, exists := GetBuiltinPluginFactories()[name]; exists {
		plugin := factory()
		
		// Initialize plugin with validated config
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		
		logger := zap.NewNop() // Default logger, should be replaced by manager
		if err := plugin.Init(ctx, config, logger); err != nil {
			return nil, fmt.Errorf("plugin initialization failed: %w", err)
		}
		
		return plugin, nil
	}
	
	return nil, fmt.Errorf("plugin factory not found for: %s", name)
}

// ValidatePluginConfig validates a plugin configuration without creating the plugin
func ValidatePluginConfig(name string, config map[string]interface{}) error {
	// Substitute environment variables
	config = globalConfigRegistry.substituteEnvironmentVariables(config)
	
	// Validate configuration
	validationResult := globalConfigRegistry.ValidateConfig(name, config)
	if !validationResult.Valid {
		var errorMessages []string
		for _, err := range validationResult.Errors {
			errorMessages = append(errorMessages, err.Error())
		}
		return fmt.Errorf("configuration validation failed: %s", strings.Join(errorMessages, "; "))
	}
	
	return nil
}

// GetDefaultConfig returns the default configuration for a plugin
func GetDefaultConfig(name string) map[string]interface{} {
	schema, exists := globalConfigRegistry.GetSchema(name)
	if !exists {
		return make(map[string]interface{})
	}
	
	return extractDefaultsFromSchema(schema.Properties)
}

// extractDefaultsFromSchema extracts default values from schema properties
func extractDefaultsFromSchema(properties map[string]JSONSchemaProperty) map[string]interface{} {
	defaults := make(map[string]interface{})
	
	for key, property := range properties {
		if property.Default != nil {
			defaults[key] = property.Default
		} else if property.Type == "object" && len(property.Properties) > 0 {
			defaults[key] = extractDefaultsFromSchema(property.Properties)
		}
	}
	
	return defaults
}

// LoadPluginsFromConfig loads multiple plugins from configuration
func LoadPluginsFromConfig(configs []config.PluginConfig) ([]*PluginLoadResult, error) {
	results := make([]*PluginLoadResult, 0, len(configs))
	
	for _, pluginConfig := range configs {
		result := &PluginLoadResult{
			Name:     pluginConfig.Name,
			Config:   pluginConfig.Config,
			Enabled:  pluginConfig.Enabled,
			LoadedAt: time.Now(),
		}
		
		plugin, err := CreatePluginFromConfig(pluginConfig.Name, pluginConfig.Config)
		if err != nil {
			result.Error = err
		} else {
			result.Plugin = plugin
		}
		
		results = append(results, result)
	}
	
	return results, nil
}

// LoadPluginsIntoManager loads multiple plugins directly into a plugin manager
func LoadPluginsIntoManager(manager *Manager, configs []config.PluginConfig) error {
	var loadErrors []error
	
	for _, pluginConfig := range configs {
		// Validate configuration first
		if err := ValidatePluginConfig(pluginConfig.Name, pluginConfig.Config); err != nil {
			loadErrors = append(loadErrors, fmt.Errorf("validation failed for plugin %s: %w", pluginConfig.Name, err))
			continue
		}
		
		// Load plugin into manager
		if err := manager.LoadPlugin(pluginConfig.Name, pluginConfig.Config); err != nil {
			loadErrors = append(loadErrors, fmt.Errorf("failed to load plugin %s: %w", pluginConfig.Name, err))
			continue
		}
		
		// Enable if requested
		if pluginConfig.Enabled {
			if err := manager.EnablePlugin(pluginConfig.Name); err != nil {
				loadErrors = append(loadErrors, fmt.Errorf("failed to enable plugin %s: %w", pluginConfig.Name, err))
			}
		}
	}
	
	if len(loadErrors) > 0 {
		var errorMessages []string
		for _, err := range loadErrors {
			errorMessages = append(errorMessages, err.Error())
		}
		return fmt.Errorf("failed to load %d plugins: %s", len(loadErrors), strings.Join(errorMessages, "; "))
	}
	
	return nil
}

// Hot-reload support functions

// ValidateConfigForHotReload validates if a configuration can be hot-reloaded
func ValidateConfigForHotReload(pluginName string, oldConfig, newConfig map[string]interface{}) error {
	// Basic validation
	if err := ValidatePluginConfig(pluginName, newConfig); err != nil {
		return fmt.Errorf("new configuration is invalid: %w", err)
	}
	
	// Check for non-hot-reloadable changes (plugin-specific logic)
	if !canHotReload(pluginName, oldConfig, newConfig) {
		return fmt.Errorf("configuration changes require plugin restart")
	}
	
	return nil
}

// canHotReload determines if configuration changes can be applied via hot reload
func canHotReload(pluginName string, oldConfig, newConfig map[string]interface{}) bool {
	// For built-in plugins, most configuration changes can be hot-reloaded
	// except for fundamental changes like signing methods in auth plugin
	
	switch pluginName {
	case "auth":
		// Check if JWT method changed (requires restart)
		oldMethod, _ := oldConfig["jwt_method"].(string)
		newMethod, _ := newConfig["jwt_method"].(string)
		if oldMethod != newMethod {
			return false
		}
	case "rate_limit":
		// All rate limit changes can be hot-reloaded
		return true
	case "cors":
		// All CORS changes can be hot-reloaded
		return true
	case "logging":
		// Most logging changes can be hot-reloaded
		return true
	}
	
	return true
}

// Configuration type conversion helpers

// ConvertToTypedConfig converts a generic config map to a typed configuration struct
func ConvertToTypedConfig(config map[string]interface{}, target interface{}) error {
	// Use JSON marshaling for type conversion
	data, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	
	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("failed to unmarshal config to target type: %w", err)
	}
	
	return nil
}

// ConvertFromTypedConfig converts a typed configuration struct to a generic config map
func ConvertFromTypedConfig(typedConfig interface{}) (map[string]interface{}, error) {
	data, err := json.Marshal(typedConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal typed config: %w", err)
	}
	
	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal to config map: %w", err)
	}
	
	return config, nil
}

// Helper functions for plugin manager integration

// GetConfigRegistry returns the global configuration registry
func GetConfigRegistry() *PluginConfigRegistry {
	return globalConfigRegistry
}

// SetConfigRegistry sets a custom configuration registry (for testing)
func SetConfigRegistry(registry *PluginConfigRegistry) {
	globalConfigRegistry = registry
}

// resetConfigRegistry resets the global registry to default (for testing)
func resetConfigRegistry() {
	globalConfigRegistry = NewPluginConfigRegistry()
}

// Backwards compatibility helpers

// LegacyConfigConverter provides conversion from legacy configuration formats
type LegacyConfigConverter struct {
	converters map[string]func(map[string]interface{}) (map[string]interface{}, error)
	mu         sync.RWMutex
}

// NewLegacyConfigConverter creates a new legacy config converter
func NewLegacyConfigConverter() *LegacyConfigConverter {
	return &LegacyConfigConverter{
		converters: make(map[string]func(map[string]interface{}) (map[string]interface{}, error)),
	}
}

// RegisterConverter registers a legacy configuration converter for a plugin
func (c *LegacyConfigConverter) RegisterConverter(pluginName string, converter func(map[string]interface{}) (map[string]interface{}, error)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.converters[pluginName] = converter
}

// Convert converts a legacy configuration to the current format
func (c *LegacyConfigConverter) Convert(pluginName string, legacyConfig map[string]interface{}) (map[string]interface{}, error) {
	c.mu.RLock()
	converter, exists := c.converters[pluginName]
	c.mu.RUnlock()
	
	if !exists {
		return legacyConfig, nil // No conversion needed
	}
	
	return converter(legacyConfig)
}

var globalLegacyConverter = NewLegacyConfigConverter()

// ConvertLegacyConfig converts legacy configuration using the global converter
func ConvertLegacyConfig(pluginName string, legacyConfig map[string]interface{}) (map[string]interface{}, error) {
	return globalLegacyConverter.Convert(pluginName, legacyConfig)
}

// registerBuiltinSchemas registers JSON schemas for all built-in plugins
func (r *PluginConfigRegistry) registerBuiltinSchemas() {
	// Auth Plugin Schema
	authSchema := &JSONSchema{
		Schema:  "http://json-schema.org/draft-07/schema#",
		Type:    "object",
		Title:   "Auth Plugin Configuration",
		Version: CurrentVersion,
		Properties: map[string]JSONSchemaProperty{
			"jwt_secret": {
				Type:        "string",
				Description: "Secret key for HMAC-based JWT signing",
				MinLength:   intPtr(32),
			},
			"jwt_public_key": {
				Type:        "string",
				Description: "Public key for RSA-based JWT verification",
			},
			"jwt_method": {
				Type:        "string",
				Description: "JWT signing method",
				Enum:        []interface{}{"HS256", "HS384", "HS512", "RS256", "RS384", "RS512"},
				Default:     "HS256",
			},
			"jwt_issuer": {
				Type:        "string",
				Description: "Expected JWT issuer",
			},
			"jwt_audience": {
				Type:        "string",
				Description: "Expected JWT audience",
			},
			"api_keys": {
				Type:        "object",
				Description: "Map of API keys to user IDs",
				Default:     map[string]interface{}{},
			},
			"auth_header": {
				Type:        "string",
				Description: "Header name for API key authentication",
				Default:     "Authorization",
			},
			"auth_query": {
				Type:        "string",
				Description: "Query parameter name for API key authentication",
				Default:     "api_key",
			},
			"auth_cookie": {
				Type:        "string",
				Description: "Cookie name for authentication",
				Default:     "auth_token",
			},
			"public_endpoints": {
				Type:        "array",
				Description: "List of endpoints that don't require authentication",
				Items: &JSONSchemaProperty{
					Type: "string",
				},
				Default: []interface{}{},
			},
		},
	}
	r.RegisterSchema("auth", authSchema)

	// Rate Limit Plugin Schema
	rateLimitSchema := &JSONSchema{
		Schema:  "http://json-schema.org/draft-07/schema#",
		Type:    "object",
		Title:   "Rate Limit Plugin Configuration",
		Version: CurrentVersion,
		Properties: map[string]JSONSchemaProperty{
			"global_requests_per_second": {
				Type:        "number",
				Description: "Global requests per second limit",
				Minimum:     float64Ptr(0),
				Default:     0.0,
			},
			"global_burst": {
				Type:        "integer",
				Description: "Global burst size",
				Minimum:     float64Ptr(0),
				Default:     0,
			},
			"ip_requests_per_second": {
				Type:        "number",
				Description: "Per-IP requests per second limit",
				Minimum:     float64Ptr(0),
				Default:     10.0,
			},
			"ip_burst": {
				Type:        "integer",
				Description: "Per-IP burst size",
				Minimum:     float64Ptr(1),
				Default:     20,
			},
			"user_requests_per_second": {
				Type:        "number",
				Description: "Per-user requests per second limit",
				Minimum:     float64Ptr(0),
				Default:     0.0,
			},
			"user_burst": {
				Type:        "integer",
				Description: "Per-user burst size",
				Minimum:     float64Ptr(0),
				Default:     0,
			},
			"exempt_ips": {
				Type:        "array",
				Description: "List of IP addresses exempt from rate limiting",
				Items: &JSONSchemaProperty{
					Type:    "string",
					Pattern: `^(?:[0-9]{1,3}\.){3}[0-9]{1,3}$|^([0-9a-fA-F]{1,4}:){7}[0-9a-fA-F]{1,4}$`,
				},
				Default: []interface{}{},
			},
			"cleanup_interval_seconds": {
				Type:        "integer",
				Description: "Cleanup interval in seconds",
				Minimum:     float64Ptr(1),
				Default:     300,
			},
			"entry_ttl_seconds": {
				Type:        "integer",
				Description: "Entry TTL in seconds",
				Minimum:     float64Ptr(1),
				Default:     1800,
			},
		},
	}
	r.RegisterSchema("rate_limit", rateLimitSchema)

	// CORS Plugin Schema
	corsSchema := &JSONSchema{
		Schema:  "http://json-schema.org/draft-07/schema#",
		Type:    "object",
		Title:   "CORS Plugin Configuration",
		Version: CurrentVersion,
		Properties: map[string]JSONSchemaProperty{
			"allow_origins": {
				Type:        "array",
				Description: "List of allowed origins",
				Items: &JSONSchemaProperty{
					Type: "string",
				},
				Default: []interface{}{"*"},
			},
			"allow_methods": {
				Type:        "array",
				Description: "List of allowed HTTP methods",
				Items: &JSONSchemaProperty{
					Type: "string",
					Enum: []interface{}{"GET", "POST", "PUT", "DELETE", "OPTIONS", "HEAD", "PATCH"},
				},
				Default: []interface{}{"GET", "POST", "PUT", "DELETE", "OPTIONS", "HEAD", "PATCH"},
			},
			"allow_headers": {
				Type:        "array",
				Description: "List of allowed headers",
				Items: &JSONSchemaProperty{
					Type: "string",
				},
				Default: []interface{}{"Origin", "Content-Type", "Accept", "Authorization", "X-Requested-With"},
			},
			"expose_headers": {
				Type:        "array",
				Description: "List of headers to expose to the client",
				Items: &JSONSchemaProperty{
					Type: "string",
				},
				Default: []interface{}{},
			},
			"allow_credentials": {
				Type:        "boolean",
				Description: "Whether to allow credentials",
				Default:     false,
			},
			"max_age": {
				Type:        "integer",
				Description: "Preflight cache duration in seconds",
				Minimum:     float64Ptr(0),
				Default:     86400,
			},
			"origin_patterns": {
				Type:        "array",
				Description: "List of regex patterns for dynamic origin validation",
				Items: &JSONSchemaProperty{
					Type: "string",
				},
				Default: []interface{}{},
			},
		},
	}
	r.RegisterSchema("cors", corsSchema)

	// Logging Plugin Schema
	loggingSchema := &JSONSchema{
		Schema:  "http://json-schema.org/draft-07/schema#",
		Type:    "object",
		Title:   "Logging Plugin Configuration",
		Version: CurrentVersion,
		Properties: map[string]JSONSchemaProperty{
			"log_level": {
				Type:        "string",
				Description: "Logging level",
				Enum:        []interface{}{"debug", "info", "warn", "error"},
				Default:     "info",
			},
			"log_request_body": {
				Type:        "boolean",
				Description: "Whether to log request bodies",
				Default:     false,
			},
			"log_response_body": {
				Type:        "boolean",
				Description: "Whether to log response bodies",
				Default:     false,
			},
			"max_body_size": {
				Type:        "integer",
				Description: "Maximum body size to log in bytes",
				Minimum:     float64Ptr(0),
				Default:     1048576, // 1MB
			},
			"sensitive_headers": {
				Type:        "array",
				Description: "List of headers to redact in logs",
				Items: &JSONSchemaProperty{
					Type: "string",
				},
				Default: []interface{}{"authorization", "cookie", "x-api-key", "x-auth-token"},
			},
			"sensitive_fields": {
				Type:        "array",
				Description: "List of field names to redact in JSON bodies",
				Items: &JSONSchemaProperty{
					Type: "string",
				},
				Default: []interface{}{"password", "secret", "token", "key", "credential"},
			},
			"log_format": {
				Type:        "string",
				Description: "Log output format",
				Enum:        []interface{}{"json", "console"},
				Default:     "json",
			},
			"include_metrics": {
				Type:        "boolean",
				Description: "Whether to include performance metrics in logs",
				Default:     true,
			},
		},
	}
	r.RegisterSchema("logging", loggingSchema)

	// Register custom validators for more complex validation logic
	r.RegisterValidator("auth", r.validateAuthConfig)
	r.RegisterValidator("rate_limit", r.validateRateLimitConfig)
	r.RegisterValidator("cors", r.validateCORSConfig)
	r.RegisterValidator("logging", r.validateLoggingConfig)
}

// Custom validation functions for built-in plugins

// validateAuthConfig provides custom validation for auth plugin configuration
func (r *PluginConfigRegistry) validateAuthConfig(config map[string]interface{}) []ConfigValidationError {
	var errors []ConfigValidationError
	
	// Validate that at least one authentication method is configured
	hasJWT := config["jwt_secret"] != nil || config["jwt_public_key"] != nil
	hasAPIKeys := false
	if apiKeys, ok := config["api_keys"].(map[string]interface{}); ok && len(apiKeys) > 0 {
		hasAPIKeys = true
	}
	
	if !hasJWT && !hasAPIKeys {
		errors = append(errors, ConfigValidationError{
			Field:   "auth",
			Message: "at least one authentication method must be configured (JWT or API keys)",
			Rule:    "custom",
		})
	}
	
	// Validate JWT configuration consistency
	if jwtMethod, ok := config["jwt_method"].(string); ok {
		if strings.HasPrefix(jwtMethod, "HS") {
			if config["jwt_secret"] == nil {
				errors = append(errors, ConfigValidationError{
					Field:   "jwt_secret",
					Message: "jwt_secret is required for HMAC-based JWT methods",
					Rule:    "custom",
				})
			}
		} else if strings.HasPrefix(jwtMethod, "RS") {
			if config["jwt_public_key"] == nil {
				errors = append(errors, ConfigValidationError{
					Field:   "jwt_public_key",
					Message: "jwt_public_key is required for RSA-based JWT methods",
					Rule:    "custom",
				})
			}
		}
	}
	
	return errors
}

// validateRateLimitConfig provides custom validation for rate limit plugin configuration
func (r *PluginConfigRegistry) validateRateLimitConfig(config map[string]interface{}) []ConfigValidationError {
	var errors []ConfigValidationError
	
	// Ensure at least one rate limiting method is enabled
	globalRate, _ := config["global_requests_per_second"].(float64)
	ipRate, _ := config["ip_requests_per_second"].(float64)
	userRate, _ := config["user_requests_per_second"].(float64)
	
	if globalRate <= 0 && ipRate <= 0 && userRate <= 0 {
		errors = append(errors, ConfigValidationError{
			Field:   "rate_limit",
			Message: "at least one rate limiting method must be enabled",
			Rule:    "custom",
		})
	}
	
	return errors
}

// validateCORSConfig provides custom validation for CORS plugin configuration
func (r *PluginConfigRegistry) validateCORSConfig(config map[string]interface{}) []ConfigValidationError {
	var errors []ConfigValidationError
	
	// Validate origin patterns
	if patterns, ok := config["origin_patterns"].([]interface{}); ok {
		for i, pattern := range patterns {
			if patternStr, ok := pattern.(string); ok {
				if _, err := regexp.Compile(patternStr); err != nil {
					errors = append(errors, ConfigValidationError{
						Field:   fmt.Sprintf("origin_patterns[%d]", i),
						Value:   pattern,
						Message: fmt.Sprintf("invalid regex pattern: %v", err),
						Rule:    "custom",
					})
				}
			}
		}
	}
	
	// Validate credentials and wildcard origin combination
	if allowCredentials, ok := config["allow_credentials"].(bool); ok && allowCredentials {
		if origins, ok := config["allow_origins"].([]interface{}); ok {
			for _, origin := range origins {
				if originStr, ok := origin.(string); ok && originStr == "*" {
					errors = append(errors, ConfigValidationError{
						Field:   "allow_credentials",
						Message: "cannot use wildcard origin (*) with allow_credentials=true",
						Rule:    "custom",
					})
					break
				}
			}
		}
	}
	
	return errors
}

// validateLoggingConfig provides custom validation for logging plugin configuration
func (r *PluginConfigRegistry) validateLoggingConfig(config map[string]interface{}) []ConfigValidationError {
	var errors []ConfigValidationError
	
	// Validate that if body logging is enabled, max_body_size is reasonable
	logRequestBody, _ := config["log_request_body"].(bool)
	logResponseBody, _ := config["log_response_body"].(bool)
	
	if (logRequestBody || logResponseBody) {
		if maxBodySize, ok := config["max_body_size"].(float64); ok {
			if maxBodySize > 10*1024*1024 { // 10MB
				errors = append(errors, ConfigValidationError{
					Field:   "max_body_size",
					Value:   maxBodySize,
					Message: "max_body_size should not exceed 10MB for performance reasons",
					Rule:    "custom",
				})
			}
		}
	}
	
	return errors
}

// Helper functions for schema definitions

// intPtr returns a pointer to an int
func intPtr(i int) *int {
	return &i
}

// float64Ptr returns a pointer to a float64
func float64Ptr(f float64) *float64 {
	return &f
}