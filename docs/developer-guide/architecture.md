# Vanta Architecture Overview

This document provides a comprehensive overview of Vanta's architecture, design principles, and component relationships.

## High-Level Architecture

Vanta is designed as a modular, high-performance OpenAPI mock server built on top of FastHTTP. The architecture follows a layered approach with clear separation of concerns.

```
┌─────────────────────────────────────────────────────────────────┐
│                            CLI Layer                            │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ ┌───────────┐ │
│  │   Start     │ │     TUI     │ │   Chaos     │ │ Recording │ │
│  │  Command    │ │   Command   │ │   Command   │ │  Command  │ │
│  └─────────────┘ └─────────────┘ └─────────────┘ └───────────┘ │
└─────────────────────────────────────────────────────────────────┘
                                  │
┌─────────────────────────────────────────────────────────────────┐
│                          HTTP Server                           │
│  ┌───────────────────────────────────────────────────────────┐ │
│  │                 Middleware Stack                         │ │
│  │  RequestID → Plugins → CORS → Logger → Recovery →       │ │
│  │  Timeout → Chaos → Metrics → Recording → Router        │ │
│  └───────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
                                  │
┌─────────────────────────────────────────────────────────────────┐
│                        Core Services                           │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ ┌───────────┐ │
│  │   OpenAPI   │ │   Config    │ │   State     │ │ Validation│ │
│  │  Processor  │ │  Manager    │ │  Manager    │ │  Engine   │ │
│  └─────────────┘ └─────────────┘ └─────────────┘ └───────────┘ │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ ┌───────────┐ │
│  │   Chaos     │ │  Recording  │ │   Plugin    │ │  Metrics  │ │
│  │   Engine    │ │   Engine    │ │  Manager    │ │ Collector │ │
│  └─────────────┘ └─────────────┘ └─────────────┘ └───────────┘ │
└─────────────────────────────────────────────────────────────────┘
                                  │
┌─────────────────────────────────────────────────────────────────┐
│                       Support Layer                            │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ ┌───────────┐ │
│  │    Cache    │ │ Hot Reload  │ │   Logging   │ │ Utilities │ │
│  │   System    │ │   Watcher   │ │  (Zap)      │ │           │ │
│  └─────────────┘ └─────────────┘ └─────────────┘ └───────────┘ │
└─────────────────────────────────────────────────────────────────┘
```

## Core Components

### 1. CLI Layer (`main.go/`)

The command-line interface provides multiple commands for different use cases:

- **Start Command**: Launches the HTTP server with specified configuration
- **TUI Command**: Interactive terminal UI for monitoring and configuration
- **Chaos Command**: Manages chaos engineering scenarios
- **Recording Command**: Handles traffic recording and replay operations
- **Validation Command**: Validates OpenAPI specs and configurations
- **State Command**: Manages server state and sessions

**Key Features:**
- Graceful shutdown handling (30-second timeout)
- Structured logging with Zap
- Context-based cancellation
- Version information embedding

### 2. HTTP Server (`pkg/api/`)

Built on FastHTTP for high performance, the server implements a comprehensive middleware stack:

#### Server Architecture
```go
type Server struct {
    config           *config.ServerConfig
    router           *Router
    server           *fasthttp.Server
    spec             *openapi.Specification
    generator        openapi.DataGenerator
    metricsCollector *DefaultMetricsCollector
    chaosEngine      chaos.ChaosEngine
    recordingEngine  recorder.RecordingEngine
    pluginsManager   *plugins.Manager
    // Hot reload and state management
    mu               sync.RWMutex
    running          bool
    startTime        time.Time
}
```

#### Middleware Stack (Execution Order)
1. **Request ID**: Generates unique request identifiers
2. **Plugins**: Custom plugin execution
3. **CORS**: Cross-origin resource sharing
4. **Logger**: Request/response logging
5. **Recovery**: Panic recovery and error handling
6. **Timeout**: Request timeout enforcement
7. **Chaos**: Chaos engineering injection
8. **Metrics**: Performance metrics collection
9. **Recording**: Traffic capture
10. **Router**: Request routing to handlers

### 3. OpenAPI Processing (`pkg/openapi/`)

Handles OpenAPI specification parsing and realistic data generation:

- **Specification Parser**: Validates and parses OpenAPI 3.x specs
- **Data Generator**: Creates realistic mock data based on schemas
- **Route Registration**: Auto-registers endpoints from OpenAPI paths
- **Schema Validation**: Validates generated data against schemas

**Features:**
- Deterministic data generation with seeds
- Support for all OpenAPI 3.x data types
- Format-aware generation (email, date, uuid, etc.)
- Example preference from specifications
- Complex object and array generation

### 4. Configuration Management (`pkg/config/`)

Centralized configuration with validation and hot reload:

```yaml
# Configuration Structure
server:          # HTTP server settings
mock:           # Data generation settings
middleware:     # Middleware configuration
metrics:        # Metrics collection
chaos:          # Chaos engineering
recording:      # Traffic recording
plugins:        # Plugin configuration
validation:     # Request/response validation
hotreload:      # Hot reload settings
state:          # State management
```

### 5. Plugin System (`pkg/plugins/`)

Extensible plugin architecture for custom functionality:

- **Built-in Plugins**: Authentication, rate limiting, CORS, logging
- **Plugin Manager**: Registration, lifecycle management, priority handling
- **Plugin Interface**: Standard interface for custom plugins
- **Configuration**: Per-plugin configuration support

### 6. Chaos Engineering (`pkg/chaos/`)

Implements chaos testing capabilities:

- **Latency Injection**: Configurable delay injection
- **Error Injection**: HTTP error responses with custom codes
- **Endpoint Matching**: Pattern-based endpoint targeting
- **Probability Control**: Configurable chaos probability

### 7. Recording & Replay (`pkg/recorder/`)

Traffic capture and replay functionality:

- **Storage Backends**: File-based and in-memory storage
- **Filtering**: Method, endpoint, and status code filtering
- **Export Formats**: JSON, HAR, Postman, cURL (some WIP)
- **Replay Engine**: Concurrent replay with configurable delays

### 8. State Management (`pkg/state/`)

Session and request state management:

- **Session State**: Per-session data with TTL
- **Request State**: Per-request context data
- **Endpoint State**: Per-endpoint initial state
- **TTL Management**: Automatic cleanup of expired state

### 9. Validation (`pkg/validation/`)

Request and response validation against OpenAPI specs:

- **Request Validation**: Headers, query params, path params, body
- **Response Validation**: Status codes, headers, response body
- **Reporting**: JSON and HTML validation reports
- **Configurable Strictness**: Warning vs. error handling

## Data Flow

### 1. Server Startup
```
Config Loading → OpenAPI Parsing → Server Creation →
Middleware Setup → Plugin Registration → Route Registration →
Server Start → Ready for Requests
```

### 2. Request Processing
```
Incoming Request → Middleware Stack → Route Matching →
Data Generation → Response Creation → Middleware (reverse) →
Response Sent
```

### 3. Hot Reload
```
File Change Detection → Configuration Reload →
Server Graceful Restart → Route Re-registration →
Service Resumed
```

## Design Principles

### 1. **High Performance**
- FastHTTP for minimal allocations
- Efficient middleware pipeline
- Concurrent request processing
- Connection pooling and reuse

### 2. **Modularity**
- Clear package boundaries
- Dependency injection
- Interface-based design
- Plugin architecture

### 3. **Reliability**
- Graceful shutdown handling
- Error recovery and logging
- Input validation
- Resource cleanup

### 4. **Observability**
- Structured logging with Zap
- Prometheus metrics
- Request tracing with IDs
- TUI for live monitoring

### 5. **Developer Experience**
- Hot reload for rapid development
- Comprehensive configuration
- Rich CLI with subcommands
- Interactive TUI

## Performance Characteristics

### Concurrency Model
- **FastHTTP**: Event-driven, non-blocking I/O
- **Goroutine Per Request**: Isolated request processing
- **Connection Limits**: Configurable per-IP connection limits
- **Timeout Handling**: Request and server-level timeouts

### Memory Management
- **Connection Pooling**: Reuse of HTTP connections
- **Buffer Pooling**: Efficient buffer management in FastHTTP
- **State TTL**: Automatic cleanup of expired state
- **Configurable Limits**: Body size, recording limits, array sizes

### Scalability Features
- **Horizontal Scaling**: Stateless design (optional state)
- **Load Balancing**: Ready for multiple instances
- **Resource Limits**: CPU and memory bounds
- **Metrics Export**: Prometheus for monitoring

## Security Considerations

### Input Validation
- OpenAPI schema validation
- Configuration validation
- File path sanitization
- Request size limits

### Network Security
- CORS configuration
- Rate limiting plugins
- Authentication plugins
- Request timeout enforcement

### Operational Security
- Non-root container execution
- Minimal container image
- Structured audit logging
- Graceful error handling

## Extension Points

### 1. **Custom Plugins**
Implement the plugin interface for custom middleware:
```go
type Plugin interface {
    Name() string
    Priority() int
    Configure(config map[string]interface{}) error
    Handle(ctx *fasthttp.RequestCtx, next func())
}
```

### 2. **Custom Data Generators**
Extend data generation for specific formats or types

### 3. **Custom Storage Backends**
Implement storage interfaces for recording and state

### 4. **Custom Chaos Scenarios**
Add new chaos types beyond latency and errors

## Monitoring and Debugging

### Metrics Collection
- Request counters by method/path/status
- Response time histograms
- Error rate tracking
- Active connection counts

### Logging
- Structured JSON logging
- Request/response logging
- Error tracking
- Performance metrics

### TUI Features
- Live metrics display
- Request/response inspection
- Configuration editing
- Log streaming

This architecture provides a solid foundation for high-performance API mocking while maintaining flexibility for extension and customization.