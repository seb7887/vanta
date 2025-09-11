# Built-in Plugins

Vanta OpenAPI Mocker includes four production-ready built-in plugins that provide essential functionality for API mocking scenarios. These plugins demonstrate the capabilities of the plugin system while providing real value for API testing and development.

## Overview

The built-in plugins are:

1. **AuthPlugin** - JWT and API key authentication
2. **RateLimitPlugin** - Sliding window rate limiting
3. **CORSPlugin** - Enhanced CORS management
4. **LoggingPlugin** - Structured request/response logging

All plugins implement the appropriate interfaces (`Plugin`, `Middleware`, `RequestProcessor`, `ResponseProcessor`) and are designed to be thread-safe, performant, and production-ready.

## Plugin Priority

Plugins execute in priority order during request processing:

1. **AuthPlugin** (Priority: High) - Authentication runs first
2. **RateLimitPlugin** (Priority: Normal) - Rate limiting after auth
3. **CORSPlugin** (Priority: Normal) - CORS handling
4. **LoggingPlugin** (Priority: Low) - Logging runs last

## AuthPlugin

Provides JWT and API key authentication with comprehensive security features.

### Features

- **JWT Authentication**: Support for HS256/384/512 and RS256/384/512 algorithms
- **API Key Authentication**: Header, query parameter, or cookie-based
- **Public Endpoints**: Configurable endpoints that bypass authentication
- **Multiple Auth Sources**: Flexible authentication source configuration
- **JWT Validation**: Issuer, audience, and expiration validation

### Configuration

```yaml
plugins:
  - name: auth
    enabled: true
    config:
      # JWT Configuration
      jwt_secret: "your-secret-key"
      jwt_method: "HS256"  # HS256, HS384, HS512, RS256, RS384, RS512
      jwt_issuer: "your-issuer"
      jwt_audience: "your-audience"
      
      # API Key Configuration
      api_keys:
        "admin-key-123": "admin-user"
        "user-key-456": "regular-user"
      
      # Authentication sources
      auth_header: "Authorization"
      auth_query: "api_key"
      auth_cookie: "auth_token"
      
      # Public endpoints (no auth required)
      public_endpoints:
        - "/health"
        - "/docs"
```

### Usage Examples

```bash
# API Key in header
curl -H "Authorization: admin-key-123" http://localhost:8080/protected

# API Key in query parameter
curl "http://localhost:8080/protected?api_key=admin-key-123"

# JWT in Authorization header
curl -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..." http://localhost:8080/protected

# Public endpoint (no auth required)
curl http://localhost:8080/health
```

### Response Codes

- **200**: Authentication successful
- **401**: Authentication required or failed
- **403**: Access denied

## RateLimitPlugin

Implements sliding window rate limiting with per-IP, per-user, and global limits.

### Features

- **Sliding Window Algorithm**: Memory-efficient rate limiting
- **Multiple Limit Types**: Global, per-IP, and per-user limits
- **Exempt IPs**: Whitelist specific IP addresses or ranges
- **Automatic Cleanup**: Periodic cleanup of stale rate limiters
- **Rate Limit Headers**: Standard rate limit headers in responses

### Configuration

```yaml
plugins:
  - name: rate_limit
    enabled: true
    config:
      # Global rate limiting
      global_requests_per_second: 1000.0
      global_burst: 2000
      
      # Per-IP rate limiting
      ip_requests_per_second: 100.0
      ip_burst: 200
      
      # Per-user rate limiting (for authenticated users)
      user_requests_per_second: 50.0
      user_burst: 100
      
      # Exempt IPs
      exempt_ips:
        - "127.0.0.1"
        - "10.0.0.0/8"
      
      # Cleanup configuration
      cleanup_interval_seconds: 300
      entry_ttl_seconds: 1800
```

### Response Headers

Rate limit information is included in response headers:

```
X-RateLimit-Limit: 200
X-RateLimit-Remaining: 195
X-RateLimit-Reset: 1641234567
```

### Response Codes

- **200**: Request within rate limits
- **429**: Rate limit exceeded (includes `Retry-After` header)

## CORSPlugin

Enhanced CORS management with dynamic origin validation and comprehensive configuration.

### Features

- **Dynamic Origin Validation**: Support for origin patterns and custom validators
- **Preflight Handling**: Proper CORS preflight request handling
- **Configurable CORS Policies**: Per-endpoint or global CORS configuration
- **Credential Support**: Configurable credential handling
- **Origin Patterns**: Regex-based origin matching

### Configuration

```yaml
plugins:
  - name: cors
    enabled: true
    config:
      # Static origins
      allow_origins:
        - "http://localhost:3000"
        - "https://yourdomain.com"
      
      # Dynamic origin patterns
      origin_patterns:
        - "^https://[a-zA-Z0-9-]+\\.yourdomain\\.com$"
        - "^http://localhost:[0-9]+$"
      
      # HTTP methods
      allow_methods:
        - "GET"
        - "POST"
        - "PUT"
        - "DELETE"
        - "OPTIONS"
      
      # Headers
      allow_headers:
        - "Origin"
        - "Content-Type"
        - "Authorization"
      
      expose_headers:
        - "X-Total-Count"
        - "X-Rate-Limit-Remaining"
      
      allow_credentials: true
      max_age: 86400  # 24 hours
```

### Usage Examples

```bash
# Simple CORS request
curl -H "Origin: http://localhost:3000" http://localhost:8080/api/users

# Preflight request
curl -X OPTIONS \
  -H "Origin: http://localhost:3000" \
  -H "Access-Control-Request-Method: POST" \
  -H "Access-Control-Request-Headers: Content-Type" \
  http://localhost:8080/api/users
```

### Response Codes

- **200**: CORS request allowed
- **204**: Preflight request successful
- **403**: CORS policy violation

## LoggingPlugin

Structured request/response logging with sensitive data filtering and performance metrics.

### Features

- **Structured Logging**: JSON or console format logging
- **Request/Response Body Logging**: Configurable body logging with size limits
- **Sensitive Data Filtering**: Automatic filtering of sensitive headers and fields
- **Performance Metrics**: Request duration and size metrics
- **Configurable Log Levels**: Per-plugin log level configuration

### Configuration

```yaml
plugins:
  - name: logging
    enabled: true
    config:
      # Log level
      log_level: "info"  # debug, info, warn, error
      
      # Body logging
      log_request_body: true
      log_response_body: true
      max_body_size: 65536  # 64KB
      
      # Sensitive data filtering
      sensitive_headers:
        - "authorization"
        - "cookie"
        - "x-api-key"
      
      sensitive_fields:
        - "password"
        - "secret"
        - "token"
      
      # Output format
      log_format: "json"  # json or console
      include_metrics: true
```

### Log Output Examples

**Request Log:**
```json
{
  "level": "info",
  "msg": "HTTP request",
  "method": "POST",
  "path": "/api/users",
  "remote_addr": "127.0.0.1:12345",
  "user_agent": "curl/7.68.0",
  "request_id": "req-123",
  "user_id": "admin-user",
  "headers": {
    "content-type": "application/json",
    "authorization": "[REDACTED]"
  },
  "body": {
    "name": "John Doe",
    "email": "john@example.com",
    "password": "[REDACTED]"
  }
}
```

**Response Log:**
```json
{
  "level": "info",
  "msg": "HTTP response",
  "method": "POST",
  "path": "/api/users",
  "status_code": 201,
  "duration": "15.5ms",
  "response_size": 156,
  "request_id": "req-123",
  "user_id": "admin-user",
  "bytes_sent": 156,
  "bytes_received": 85
}
```

## Plugin Registration

### Programmatic Registration

```go
import "vanta/pkg/plugins"

// Create plugin manager
manager := plugins.NewManager(logger)

// Register all built-in plugins
err := plugins.RegisterBuiltinPlugins(manager.GetRegistry())
if err != nil {
    log.Fatal("Failed to register plugins:", err)
}
```

### Individual Plugin Creation

```go
// Create specific plugins
authPlugin := plugins.NewAuthPlugin()
rateLimitPlugin := plugins.NewRateLimitPlugin()
corsPlugin := plugins.NewCORSPlugin()
loggingPlugin := plugins.NewLoggingPlugin()

// Or use factory functions
plugin, err := plugins.CreateBuiltinPlugin("auth")
```

### Available Factory Functions

- `RegisterBuiltinPlugins(registry)` - Register all built-in plugins
- `GetBuiltinPluginFactories()` - Get map of plugin factories
- `CreateBuiltinPlugin(name)` - Create specific plugin by name
- `GetBuiltinPluginNames()` - Get sorted list of plugin names

## Integration with FastHTTP

The plugins integrate seamlessly with FastHTTP:

```go
// Create middleware function
middlewareFunc := manager.CreateMiddlewareFunc()

// Wrap your handler
handler := middlewareFunc(yourMainHandler)

// Use with FastHTTP server
server := &fasthttp.Server{
    Handler: handler,
}
```

## Error Handling

All plugins implement comprehensive error handling:

- **Plugin Errors**: Custom `PluginError` type with plugin name and operation context
- **Panic Recovery**: Automatic panic recovery in middleware chain
- **Graceful Degradation**: Plugins fail safely without breaking the request chain
- **Detailed Logging**: Error details logged with context

## Performance Considerations

- **Thread-Safe**: All plugins are designed for high-concurrency environments
- **Memory Efficient**: Rate limiters use automatic cleanup to prevent memory leaks
- **Minimal Overhead**: Optimized for minimal performance impact
- **Configurable Limits**: Body size limits and other performance tuning options

## Security Best Practices

1. **JWT Secrets**: Use strong, randomly generated secrets for JWT signing
2. **API Key Management**: Rotate API keys regularly and use strong, unique keys
3. **CORS Configuration**: Be specific with allowed origins, avoid using "*" with credentials
4. **Rate Limiting**: Set appropriate limits based on your infrastructure capacity
5. **Logging**: Be careful with sensitive data logging, use the filtering features

## Monitoring and Metrics

The plugin manager provides comprehensive metrics:

```go
// Get aggregated metrics
metrics := manager.GetPluginMetrics()

// Individual plugin information
plugins := manager.ListPlugins()
for _, info := range plugins {
    fmt.Printf("Plugin: %s, State: %s, Requests: %d\n", 
        info.Name, info.State, info.Metrics.RequestsProcessed)
}
```

## Example Application

See `examples/builtin-plugins-demo.go` for a complete example application demonstrating all built-in plugins in action.

Run the demo:

```bash
go run examples/builtin-plugins-demo.go
```

The demo server will start on `http://localhost:8080` with all plugins enabled and ready for testing.