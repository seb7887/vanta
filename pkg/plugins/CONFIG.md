# Plugin Configuration System

This document describes the comprehensive plugin configuration system for the Vanta OpenAPI Mocker. The system provides type-safe configuration validation, environment variable substitution, hot-reload support, and seamless integration with the existing plugin architecture.

## Table of Contents

- [Overview](#overview)
- [Core Features](#core-features)
- [Configuration Structure](#configuration-structure)
- [Built-in Plugin Configurations](#built-in-plugin-configurations)
- [Factory Methods](#factory-methods)
- [Configuration Validation](#configuration-validation)
- [Environment Variables](#environment-variables)
- [Hot Reload Support](#hot-reload-support)
- [Migration System](#migration-system)
- [API Reference](#api-reference)
- [Examples](#examples)
- [Performance](#performance)

## Overview

The plugin configuration system provides a robust, type-safe way to configure plugins with comprehensive validation, environment variable substitution, and hot-reload capabilities. It integrates seamlessly with the existing `config.PluginConfig` structure and supports both built-in and custom plugins.

## Core Features

### 1. **JSON Schema Validation**
- Type checking (string, number, boolean, array, object)
- Range validation (min/max values, string lengths)
- Pattern matching (regex validation)
- Enum validation (predefined values)
- Custom validation rules per plugin

### 2. **Environment Variable Substitution**
- `${VAR}` syntax for required variables
- `${VAR:default}` syntax with default values
- Nested object and array support
- Recursive substitution

### 3. **Hot Reload Support**
- Runtime configuration changes
- Validation before applying changes
- Plugin-specific reload capabilities
- Backward compatibility checks

### 4. **Configuration Versioning**
- Schema versioning support
- Migration system for upgrades
- Backward compatibility

### 5. **Factory Methods**
- Plugin creation from configuration
- Validation without instantiation
- Default configuration generation
- Batch plugin loading

## Configuration Structure

### Basic Plugin Configuration

```yaml
plugins:
  - name: "plugin_name"
    enabled: true
    config:
      # Plugin-specific configuration
      key: value
```

### Environment Variable Usage

```yaml
plugins:
  - name: "auth"
    enabled: true
    config:
      jwt_secret: "${JWT_SECRET}"
      api_keys:
        "${API_KEY_1}": "user1"
        "${API_KEY_2}": "user2"
      max_connections: "${MAX_CONN:100}"  # Default: 100
```

## Built-in Plugin Configurations

### 1. Auth Plugin

Provides JWT and API key authentication.

```yaml
- name: "auth"
  enabled: true
  config:
    # JWT Configuration
    jwt_secret: "your-secret-key-32-chars-min"
    jwt_public_key: "-----BEGIN PUBLIC KEY-----..."
    jwt_method: "HS256"  # HS256, HS384, HS512, RS256, RS384, RS512
    jwt_issuer: "your-issuer"
    jwt_audience: "your-audience"
    
    # API Key Configuration
    api_keys:
      "api-key-123": "user-1"
      "api-key-456": "user-2"
    
    # Authentication Sources
    auth_header: "Authorization"
    auth_query: "api_key"
    auth_cookie: "auth_token"
    
    # Public Endpoints (no auth required)
    public_endpoints:
      - "/health"
      - "/metrics"
```

**Validation Rules:**
- At least one authentication method must be configured
- JWT secret required for HMAC methods (HS256, HS384, HS512)
- JWT public key required for RSA methods (RS256, RS384, RS512)
- JWT secret must be at least 32 characters for security

### 2. Rate Limit Plugin

Provides multi-tier rate limiting with sliding windows.

```yaml
- name: "rate_limit"
  enabled: true
  config:
    # Global Rate Limiting
    global_requests_per_second: 1000.0
    global_burst: 2000
    
    # Per-IP Rate Limiting
    ip_requests_per_second: 10.0
    ip_burst: 20
    
    # Per-User Rate Limiting
    user_requests_per_second: 50.0
    user_burst: 100
    
    # Exempt IPs
    exempt_ips:
      - "127.0.0.1"
      - "192.168.1.0/24"
    
    # Cleanup Configuration
    cleanup_interval_seconds: 300
    entry_ttl_seconds: 1800
```

**Validation Rules:**
- At least one rate limiting method must be enabled
- Rate values must be non-negative
- IP addresses must be valid IPv4 or IPv6 format

### 3. CORS Plugin

Provides enhanced CORS management with dynamic origin validation.

```yaml
- name: "cors"
  enabled: true
  config:
    # Basic CORS Settings
    allow_origins:
      - "https://app.example.com"
      - "http://localhost:3000"
    allow_methods:
      - "GET"
      - "POST"
      - "PUT"
      - "DELETE"
    allow_headers:
      - "Origin"
      - "Content-Type"
      - "Authorization"
    expose_headers:
      - "X-RateLimit-Remaining"
    allow_credentials: true
    max_age: 86400
    
    # Dynamic Origin Patterns (regex)
    origin_patterns:
      - "^https://[a-z0-9]+\\.example\\.com$"
```

**Validation Rules:**
- Origin patterns must be valid regex
- Cannot use wildcard origin (*) with credentials enabled
- Methods must be valid HTTP methods

### 4. Logging Plugin

Provides structured request/response logging with sensitive data redaction.

```yaml
- name: "logging"
  enabled: true
  config:
    # Logging Configuration
    log_level: "info"  # debug, info, warn, error
    log_format: "json"  # json, console
    
    # Body Logging
    log_request_body: false
    log_response_body: false
    max_body_size: 1048576  # 1MB
    
    # Sensitive Data Redaction
    sensitive_headers:
      - "authorization"
      - "cookie"
      - "x-api-key"
    sensitive_fields:
      - "password"
      - "secret"
      - "token"
    
    # Additional Options
    include_metrics: true
```

**Validation Rules:**
- Log level must be valid (debug, info, warn, error)
- Max body size should not exceed 10MB for performance
- Body logging recommended only for development

## Factory Methods

### CreatePluginFromConfig

Creates and initializes a plugin from configuration.

```go
plugin, err := CreatePluginFromConfig("auth", config)
if err != nil {
    log.Fatalf("Failed to create plugin: %v", err)
}
```

### ValidatePluginConfig

Validates configuration without creating the plugin.

```go
err := ValidatePluginConfig("auth", config)
if err != nil {
    log.Printf("Configuration invalid: %v", err)
}
```

### GetDefaultConfig

Returns default configuration for a plugin.

```go
defaults := GetDefaultConfig("auth")
fmt.Printf("Default auth config: %+v\n", defaults)
```

### LoadPluginsFromConfig

Loads multiple plugins from configuration.

```go
configs := []config.PluginConfig{
    {Name: "auth", Enabled: true, Config: authConfig},
    {Name: "cors", Enabled: true, Config: corsConfig},
}

results, err := LoadPluginsFromConfig(configs)
for _, result := range results {
    if result.Error != nil {
        log.Printf("Plugin %s failed: %v", result.Name, result.Error)
    } else {
        log.Printf("Plugin %s loaded successfully", result.Name)
    }
}
```

## Configuration Validation

### JSON Schema Validation

The system uses JSON Schema for structural validation:

```go
// Get validation result
result := ValidatePluginConfig("auth", config)
if !result.Valid {
    for _, err := range result.Errors {
        fmt.Printf("Field %s: %s\n", err.Field, err.Message)
    }
}
```

### Custom Validation

Plugins can register custom validators:

```go
registry := GetConfigRegistry()
registry.RegisterValidator("custom_plugin", func(config map[string]interface{}) []ConfigValidationError {
    var errors []ConfigValidationError
    
    // Custom validation logic
    if value, ok := config["required_field"]; !ok || value == "" {
        errors = append(errors, ConfigValidationError{
            Field:   "required_field",
            Message: "Field is required",
            Rule:    "custom",
        })
    }
    
    return errors
})
```

## Environment Variables

### Substitution Syntax

- `${VAR}`: Required variable (error if missing)
- `${VAR:default}`: Optional with default value
- Supports nested objects and arrays

### Example

```yaml
config:
  jwt_secret: "${JWT_SECRET}"
  port: "${PORT:8080}"
  database:
    host: "${DB_HOST:localhost}"
    credentials:
      username: "${DB_USER}"
      password: "${DB_PASS}"
```

### Setting Environment Variables

```bash
export JWT_SECRET="your-secret-key"
export DB_USER="myuser"
export DB_PASS="mypassword"
```

## Hot Reload Support

### Validating Hot Reload

```go
err := ValidateConfigForHotReload("auth", oldConfig, newConfig)
if err != nil {
    log.Printf("Cannot hot-reload: %v", err)
}
```

### Hot Reload Capabilities

| Plugin | Hot Reloadable | Restrictions |
|--------|----------------|--------------|
| Auth | Partial | Cannot change JWT method/keys |
| Rate Limit | Full | All settings can be changed |
| CORS | Full | All settings can be changed |
| Logging | Full | All settings can be changed |

### Using Plugin Manager

```go
// Hot reload through plugin manager
err := manager.ReloadPlugin("auth", newConfig)
if err != nil {
    log.Printf("Hot reload failed: %v", err)
}
```

## Migration System

### Registering Migrations

```go
migration := ConfigMigration{
    FromVersion: "v1",
    ToVersion:   "v2",
    Migrate: func(config map[string]interface{}) (map[string]interface{}, error) {
        // Rename field
        if oldValue, exists := config["old_field"]; exists {
            config["new_field"] = oldValue
            delete(config, "old_field")
        }
        return config, nil
    },
}

registry.RegisterMigration("plugin_name", migration)
```

### Applying Migrations

```go
migratedConfig, err := registry.MigrateConfig("plugin_name", oldConfig, "v1")
if err != nil {
    log.Printf("Migration failed: %v", err)
}
```

## API Reference

### Core Types

```go
// Configuration validation result
type ConfigValidationResult struct {
    Valid  bool
    Errors []ConfigValidationError
}

// Validation error with field path
type ConfigValidationError struct {
    Field   string
    Value   interface{}
    Message string
    Rule    string
}

// Plugin load result
type PluginLoadResult struct {
    Name     string
    Plugin   Plugin
    Config   map[string]interface{}
    Enabled  bool
    Error    error
    LoadedAt time.Time
}
```

### Key Functions

```go
// Factory functions
func CreatePluginFromConfig(name string, config map[string]interface{}) (Plugin, error)
func ValidatePluginConfig(name string, config map[string]interface{}) error
func GetDefaultConfig(name string) map[string]interface{}
func LoadPluginsFromConfig(configs []config.PluginConfig) ([]*PluginLoadResult, error)

// Hot reload
func ValidateConfigForHotReload(pluginName string, oldConfig, newConfig map[string]interface{}) error

// Type conversion
func ConvertToTypedConfig(config map[string]interface{}, target interface{}) error
func ConvertFromTypedConfig(typedConfig interface{}) (map[string]interface{}, error)

// Registry access
func GetConfigRegistry() *PluginConfigRegistry
```

## Examples

### Complete Configuration Example

```yaml
server:
  port: 8080
  host: "0.0.0.0"

plugins:
  - name: "auth"
    enabled: true
    config:
      jwt_secret: "${JWT_SECRET}"
      jwt_method: "HS256"
      api_keys:
        "${API_KEY_ADMIN}": "admin"
        "${API_KEY_USER}": "user"
      public_endpoints: ["/health", "/metrics"]

  - name: "rate_limit"
    enabled: true
    config:
      ip_requests_per_second: 10.0
      ip_burst: 20
      exempt_ips: ["127.0.0.1"]

  - name: "cors"
    enabled: true
    config:
      allow_origins: ["https://app.example.com"]
      allow_credentials: true

  - name: "logging"
    enabled: true
    config:
      log_level: "info"
      include_metrics: true
```

### Programmatic Usage

```go
package main

import (
    "log"
    "vanta/pkg/plugins"
    "vanta/pkg/config"
)

func main() {
    // Define configuration
    authConfig := map[string]interface{}{
        "jwt_secret": "your-secret-key-32-chars-min",
        "jwt_method": "HS256",
    }

    // Validate configuration
    if err := plugins.ValidatePluginConfig("auth", authConfig); err != nil {
        log.Fatalf("Configuration invalid: %v", err)
    }

    // Create plugin
    plugin, err := plugins.CreatePluginFromConfig("auth", authConfig)
    if err != nil {
        log.Fatalf("Failed to create plugin: %v", err)
    }

    log.Printf("Created plugin: %s v%s", plugin.Name(), plugin.Version())
}
```

## Performance

The configuration system is optimized for high performance:

### Benchmarks

- **Validation**: ~29μs per validation
- **Environment Substitution**: ~6μs per substitution  
- **Plugin Creation**: ~30μs per plugin
- **Schema Compilation**: One-time cost at startup

### Performance Tips

1. **Validate Once**: Validate configurations at startup, not per request
2. **Cache Results**: Use validated configurations in production
3. **Batch Operations**: Use `LoadPluginsFromConfig` for multiple plugins
4. **Environment Variables**: Set all required variables before startup

### Memory Usage

- **Schemas**: ~2KB per plugin schema
- **Registry**: ~1KB base overhead
- **Validation**: Minimal allocations during validation
- **Substitution**: Temporary allocations only

## Error Handling

The system provides detailed error messages with field paths:

```go
// Example error output
config_test.go:245: Configuration validation failed: 
  validation failed for field 'jwt_secret': string length must be >= 32 (value: short);
  validation failed for field 'jwt_method': value must be one of: [HS256 HS384 HS512 RS256 RS384 RS512] (value: INVALID)
```

## Thread Safety

The configuration system is fully thread-safe:

- **Concurrent Validation**: Multiple goroutines can validate simultaneously
- **Registry Access**: All registry operations are mutex-protected
- **Environment Substitution**: Safe for concurrent use
- **Hot Reload**: Atomic configuration updates

## Best Practices

1. **Environment Variables**: Use environment variables for secrets
2. **Validation**: Always validate configurations before deployment
3. **Defaults**: Provide sensible defaults for optional settings
4. **Documentation**: Document custom plugin configurations
5. **Testing**: Test configuration validation in unit tests
6. **Monitoring**: Monitor configuration reload success/failure
7. **Security**: Never log sensitive configuration values
8. **Versioning**: Use configuration versioning for breaking changes

## Troubleshooting

### Common Issues

1. **Validation Failures**: Check field types and required fields
2. **Environment Variables**: Ensure all required variables are set
3. **Hot Reload**: Check plugin hot-reload capabilities
4. **Schema Errors**: Verify custom schema definitions
5. **Migration Issues**: Test migrations with sample data

### Debug Mode

Enable debug logging to see detailed configuration processing:

```go
config := map[string]interface{}{
    "log_level": "debug",
    // ... other config
}
```

This comprehensive configuration system provides a robust foundation for plugin management with enterprise-grade features like validation, hot-reload, and migration support.