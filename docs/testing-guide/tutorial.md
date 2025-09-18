# Vanta CLI - Complete Testing Tutorial

This tutorial will guide you step by step to test all Vanta CLI functionalities using the comprehensive OpenAPI specification included.

## Table of Contents

- [Initial Setup](#initial-setup)
- [1. Command: start](#1-command-start)
- [2. Command: config](#2-command-config)
- [3. Command: validate](#3-command-validate)
- [4. Command: record](#4-command-record)
- [5. Command: chaos](#5-command-chaos)
- [6. Command: state](#6-command-state)
- [7. Command: tui](#7-command-tui)
- [8. Command: version](#8-command-version)
- [Advanced Testing](#advanced-testing)
- [Troubleshooting](#troubleshooting)

## Initial Setup

### Prerequisites

1. **Install Vanta CLI**: Make sure you have the `vanta` binary compiled and in your PATH
2. **Test files**: Use the files included in this repository
3. **Additional tools**: curl, jq (optional for JSON parsing), httpie (optional)

### Testing Files

```bash
# Navigate to the project directory
cd /path/to/vanta

# Verify that testing files are available
ls spec/vanta-test-api.yaml          # Comprehensive OpenAPI specification
ls examples/                         # Example configurations
ls vanta.yaml                       # Base configuration
```

### Basic Verification

```bash
# Verify that Vanta CLI is installed correctly
vanta version

# Expected output:
# Version: v1.0.0
# Commit: abc1234
# Build Time: 2025-01-15T10:00:00Z
```

---

## 1. Command: start

The `start` command is the main command to start the mock server.

### 1.1 Basic Start

```bash
# Start server with testing specification
vanta start spec/vanta-test-api.yaml

# Expected output:
# INFO: Starting vanta server spec=spec/vanta-test-api.yaml port=8080 host=0.0.0.0
# INFO: Server created successfully, starting...
# INFO: Server running on http://0.0.0.0:8080
```

**Expected behavior:**
- The server should start on port 8080
- Should load the OpenAPI specification without errors
- Endpoints should be available according to the spec

### 1.2 Verify Basic Endpoints

In another terminal, while the server is running:

```bash
# Test health endpoint
curl http://localhost:8080/health

# Expected output (JSON):
{
  "status": "healthy",
  "timestamp": "2025-01-15T10:30:00Z",
  "uptime": 3600,
  "version": "1.0.0"
}

# Test metrics endpoint
curl http://localhost:8080/metrics

# Expected output (JSON with server metrics):
{
  "requests_total": 1,
  "requests_per_second": 0.1,
  "errors_total": 0,
  "error_rate": 0.0,
  "average_latency_ms": 1.5,
  "p95_latency_ms": 2.1,
  "memory_usage_mb": 25.6,
  "cpu_usage_percent": 0.5,
  "active_connections": 1
}
```

### 1.3 Test with Custom Configuration

```bash
# Stop the previous server (Ctrl+C)

# Create custom testing configuration
cat > test-config.yaml << EOF
server:
  port: 9090
  host: "127.0.0.1"
  read_timeout: 15s
  write_timeout: 15s
mock:
  seed: 42
  locale: "es"
logging:
  level: "debug"
  format: "text"
EOF

# Start with custom configuration
vanta start spec/vanta-test-api.yaml --config test-config.yaml --port 9090

# Expected output:
# DEBUG: Loading configuration from file file=test-config.yaml
# INFO: Starting vanta server spec=spec/vanta-test-api.yaml port=9090 host=127.0.0.1
# DEBUG: Using Spanish locale for data generation
# INFO: Server running on http://127.0.0.1:9090
```

### 1.4 Test with Command Line Parameters

```bash
# Start with CLI overrides
vanta start spec/vanta-test-api.yaml \
  --port 8888 \
  --host localhost \
  --config test-config.yaml

# Verify that CLI parameters take priority
curl http://localhost:8888/health
# Should respond on port 8888, not 9090
```

### 1.5 Test Generated Data

```bash
# Test users endpoint (should generate mock data)
curl http://localhost:8080/users

# Expected output (array of generated users):
{
  "users": [
    {
      "id": 1,
      "username": "admin",
      "email": "admin@example.com",
      "role": "admin",
      "active": true,
      "created_at": "2025-01-01T00:00:00Z"
    },
    {
      "id": 2,
      "username": "testuser",
      "email": "test@example.com",
      "role": "user",
      "active": true,
      "created_at": "2025-01-02T10:00:00Z"
    }
  ],
  "pagination": {
    "page": 1,
    "limit": 20,
    "total": 2,
    "total_pages": 1
  }
}

# Test products endpoint with parameters
curl "http://localhost:8080/products?category=electronics&min_price=100"

# Should filter products according to query parameters
```

---

## 2. Command: config

The `config` command manages configuration files.

### 2.1 Initialize Configuration

```bash
# Create new configuration
vanta config init --output my-config.yaml

# Expected output:
# Configuration file created: my-config.yaml

# Verify content
cat my-config.yaml

# Should display complete YAML configuration with default values
```

### 2.2 Validate Configuration

```bash
# Validate existing configuration
vanta config validate vanta.yaml

# Expected output:
# Configuration file is valid: vanta.yaml

# Test with invalid configuration
cat > invalid-config.yaml << EOF
server:
  port: "invalid"  # Error: must be a number
  host: 123        # Error: must be a string
EOF

vanta config validate invalid-config.yaml

# Expected output (with error):
# Error: configuration validation failed: port must be a number
```

### 2.3 Edit Configuration

```bash
# Attempt to edit configuration
vanta config edit my-config.yaml

# Expected output:
# Opening /path/to/my-config.yaml in vi...
# Note: Interactive editing is not yet implemented in this version.
# Please manually edit the file: /path/to/my-config.yaml
```

---

## 3. Command: validate

The `validate` command validates OpenAPI specifications and generates reports.

### 3.1 Validate Specification

```bash
# Validate testing specification
vanta validate spec spec/vanta-test-api.yaml

# Expected output:
# OpenAPI Specification Validation
# ================================
#
# Status: VALID
# Title: Vanta Test API - Comprehensive Testing Specification
# Version: 1.0.0
# Endpoints: 25
# Schemas: 35

# Test in JSON format
vanta validate spec spec/vanta-test-api.yaml --format json

# Expected output (structured JSON):
{
  "valid": true,
  "errors": [],
  "warnings": [],
  "info": {
    "version": "3.0.3",
    "title": "Vanta Test API - Comprehensive Testing Specification",
    "endpoints": 25,
    "schemas": 35
  }
}
```

### 3.2 Validate with Strict Mode

```bash
# Validate in strict mode
vanta validate spec spec/vanta-test-api.yaml --strict --examples

# Should also validate all examples in the specification
```

### 3.3 Generate Coverage Report

```bash
# First, start server to generate traffic
vanta start spec/vanta-test-api.yaml &
SERVER_PID=$!

# Generate some traffic for coverage testing
curl http://localhost:8080/health
curl http://localhost:8080/users
curl http://localhost:8080/products

# Generate coverage report
vanta validate coverage spec/vanta-test-api.yaml

# Expected output:
# API Coverage Report
# ===================
#
# Overall Coverage: 12.0%
# Covered Endpoints: 3/25
#
# Endpoint Details:
#   GET /health - COVERED (1 requests)
#   GET /users - COVERED (1 requests)
#   GET /products - COVERED (1 requests)
#   POST /users - NOT COVERED (0 requests)
#   ...

# Stop server
kill $SERVER_PID
```

### 3.4 Generate Compliance Report

```bash
# Generate compliance report
vanta validate compliance spec/vanta-test-api.yaml --format json

# Expected output (JSON):
{
  "compliance_percent": 95.5,
  "total_requests": 100,
  "valid_requests": 95,
  "invalid_requests": 5,
  "violations": [
    {
      "endpoint": "POST /users",
      "method": "POST",
      "type": "validation_error",
      "message": "Missing required field: email",
      "count": 3
    }
  ]
}
```

---

## 4. Command: record

The `record` command allows recording and replaying HTTP traffic.

### 4.1 Start Recording

```bash
# Start basic recording
vanta record start

# Expected output:
# 🎬 Starting recording...
# ✅ Recording enabled
# 📁 Storage directory: ./recordings
# 📊 Max recordings: 1000
# 📏 Max body size: 1048576 bytes
# 🔍 Filters: 0 configured
```

### 4.2 Recording with Filters

```bash
# Start recording with specific filters
vanta record start \
  --filter "method:GET" \
  --filter "endpoint:/users" \
  --output ./my-recordings \
  --max-recordings 100

# Expected output:
# 🎬 Starting recording...
# ✅ Recording enabled
# 📁 Storage directory: ./my-recordings
# 📊 Max recordings: 100
# 📏 Max body size: 1048576 bytes
# 🔍 Filters: 2 configured
```

### 4.3 Generate Traffic for Recording

With recording active, in another terminal:

```bash
# Generate varied traffic
curl -X GET http://localhost:8080/users
curl -X POST http://localhost:8080/users \
  -H "Content-Type: application/json" \
  -d '{"username":"testuser","email":"test@example.com","password":"Test123!","role":"user"}'
curl -X GET http://localhost:8080/products?category=electronics
curl -X GET http://localhost:8080/health
```

### 4.4 List Recordings

```bash
# List all recordings
vanta record list

# Expected output:
# 📋 Found 4 recordings:
#
# ID                                       METHOD   URI                              STATUS TIMESTAMP
# --------------------------------------------------------------------------------------------------
# abc123def456...                         GET      /users                           200    2025-01-15 10:30:00
# def456abc789...                         POST     /users                           201    2025-01-15 10:30:05
# ghi789def012...                         GET      /products?category=electronics   200    2025-01-15 10:30:10
# jkl012ghi345...                         GET      /health                          200    2025-01-15 10:30:15

# List with filters
vanta record list --method GET --limit 2

# List recent recordings
vanta record list --since 1h
```

### 4.5 View Recording Details

```bash
# View details of a specific recording
vanta record show abc123def456...

# Expected output:
# 🎬 Recording Details
#
# ID:        abc123def456...
# Timestamp: 2025-01-15 10:30:00
# Duration:  2ms
#
# 📨 Request:
#   Method: GET
#   URI:    /users
#   Headers (3):
#     User-Agent: curl/7.68.0
#     Accept: */*
#     Host: localhost:8080
#   Body:   0 bytes
#
# 📤 Response:
#   Status: 200
#   Headers (2):
#     Content-Type: application/json
#     Content-Length: 1234
#   Body:   1234 bytes
#
# 🏷️  Metadata:
#   Source:    http_server
#   Client IP: 127.0.0.1
```

### 4.6 Stop Recording

```bash
# Stop active recording
vanta record stop

# Expected output:
# ⏹️  Stopping recording...
# ✅ Recording stopped
```

### 4.7 Replay Recordings

```bash
# Start target server for replay (on another port)
vanta start spec/vanta-test-api.yaml --port 8081 &
REPLAY_SERVER_PID=$!

# Replay all recordings
vanta record replay --target http://localhost:8081

# Expected output:
# 🔄 Starting replay to http://localhost:8081...
# 📋 Loaded 4 recordings for replay
#
# 📊 Replay completed:
#    Total requests: 4
#    Successful: 4
#    Failed: 0
#    Average latency: 15ms
#    Duration: 2.5s

# Replay specific recordings
vanta record replay \
  --target http://localhost:8081 \
  --ids abc123def456,def456abc789 \
  --concurrency 2 \
  --delay 500ms

# Stop replay server
kill $REPLAY_SERVER_PID
```

### 4.8 Export Recordings

```bash
# Export to HAR format
vanta record export --format har --output recordings.har

# Export to Postman collection
vanta record export --format postman --output collection.json

# Export as cURL commands
vanta record export --format curl --output commands.sh

# Expected output:
# 📤 Exporting recordings in curl format...
# 📋 Loaded 4 recordings for export
# Export completed: commands.sh
```

### 4.9 Delete Recordings

```bash
# Delete specific recording
vanta record delete abc123def456...

# Expected output:
# ✅ Deleted recording abc123def456...

# Delete all recordings (with confirmation)
vanta record delete --all

# Expected output:
# ⚠️  Are you sure you want to delete ALL recordings? (y/N): y
# ✅ All recordings deleted

# Delete without confirmation
vanta record delete --all --force
```

---

## 5. Command: chaos

The `chaos` command implements chaos engineering to test resilience.

### 5.1 Configure Chaos Scenarios

First, create chaos configuration:

```bash
cat > chaos-config.yaml << EOF
server:
  port: 8080
chaos:
  enabled: true
  scenarios:
    - name: "api_latency"
      type: "latency"
      probability: 0.3
      endpoints: ["/users", "/products"]
      parameters:
        min_delay: "100ms"
        max_delay: "2s"

    - name: "random_errors"
      type: "error"
      probability: 0.1
      endpoints: ["/users/*", "/products/*"]
      parameters:
        status_codes: [500, 502, 503]

    - name: "slow_database"
      type: "latency"
      probability: 0.2
      endpoints: ["/analytics/*"]
      parameters:
        min_delay: "1s"
        max_delay: "5s"
logging:
  level: "info"
EOF
```

### 5.2 List Available Scenarios

```bash
# List configured scenarios
vanta chaos list --config chaos-config.yaml

# Expected output:
# 📋 Available Chaos Scenarios
#
# 🎯 api_latency
#    Type: latency
#    Endpoints: [/users /products]
#    Probability: 30.0%
#    Parameters:
#      max_delay: 2s
#      min_delay: 100ms
#
# 🎯 random_errors
#    Type: error
#    Endpoints: [/users/* /products/*]
#    Probability: 10.0%
#    Parameters:
#      status_codes: [500 502 503]
#
# 🎯 slow_database
#    Type: latency
#    Endpoints: [/analytics/*]
#    Probability: 20.0%
#    Parameters:
#      max_delay: 5s
#      min_delay: 1s
```

### 5.3 Start Chaos Testing

```bash
# Start all scenarios
vanta chaos start --config chaos-config.yaml

# Expected output:
# ✅ Chaos testing started with 3 scenario(s)
#   - api_latency (latency): 30.0% probability on [/users /products]
#   - random_errors (error): 10.0% probability on [/users/* /products/*]
#   - slow_database (latency): 20.0% probability on [/analytics/*]
# ♾️  Running indefinitely (Ctrl+C to stop)

# In another terminal, generate traffic to observe chaos effects
for i in {1..20}; do
  echo "Request $i:"
  time curl -s http://localhost:8080/users | jq '.users | length'
  sleep 1
done

# Should observe:
# - Some requests with additional latency (100ms-2s)
# - Some requests failing with 500/502/503 errors
# - Variation in response times
```

### 5.4 Start Specific Scenario

```bash
# Start only latency scenario for 5 minutes
vanta chaos start \
  --config chaos-config.yaml \
  --scenario api_latency \
  --duration 5m

# Expected output:
# ✅ Chaos testing started with 1 scenario(s)
#   - api_latency (latency): 30.0% probability on [/users /products]
# ⏰ Will run for 5m0s
#
# [after 5 minutes]
# ⏰ Duration elapsed, stopping chaos testing
#
# 📊 Final Statistics:
#   Total requests: 45
#   Chaos applied: 13
#   Failed injections: 0
#   Chaos rate: 28.89%
# ✅ Chaos testing stopped
```

### 5.5 Verify Chaos Status

```bash
# View current chaos testing status
vanta chaos status --config chaos-config.yaml

# Expected output:
# 📋 Chaos Testing Status
#
# Configuration file: chaos-config.yaml
# Chaos enabled: true
# Scenarios configured: 3
#
# 📝 Configured Scenarios:
#   1. api_latency (latency)
#      Endpoints: [/users /products]
#      Probability: 30.0%
#      Parameters: map[max_delay:2s min_delay:100ms]
#
#   2. random_errors (error)
#      Endpoints: [/users/* /products/*]
#      Probability: 10.0%
#      Parameters: map[status_codes:[500 502 503]]
#
#   3. slow_database (latency)
#      Endpoints: [/analytics/*]
#      Probability: 20.0%
#      Parameters: map[max_delay:5s min_delay:1s]
```

### 5.6 Stop Chaos Testing

```bash
# Stop active chaos testing
vanta chaos stop

# Expected output:
# 🛑 Chaos testing stop signal sent
# 💡 Note: To stop chaos testing on a running server, restart the server or use configuration hot-reload
```

---

## 6. Command: state

The `state` command manages the mock server state.

### 6.1 Configure State Management

```bash
# Start server with state enabled
vanta start spec/vanta-test-api.yaml &
SERVER_PID=$!

# Give the server time to initialize
sleep 2
```

### 6.2 Set State Values

```bash
# Set simple value
vanta state set user_count 1000

# Expected output:
# Successfully set user_count = 1000

# Set JSON value
vanta state set current_user '{"id":1,"name":"Admin","role":"admin"}'

# Expected output:
# Successfully set current_user = map[id:1 name:Admin role:admin]

# Set value from file
echo '{"feature_flags":{"new_ui":true,"beta_features":false}}' > features.json
vanta state set app_config --file features.json

# Set value with TTL
vanta state set session_token "abc123xyz" --ttl 1h

# Expected output:
# Successfully set session_token = abc123xyz
# with TTL: 1h0m0s

# Set value in specific scope
vanta state set last_login "2025-01-15T10:30:00Z" --scope user:123

# Expected output:
# Successfully set last_login = 2025-01-15T10:30:00Z
# in scope: user:123
```

### 6.3 Get State Values

```bash
# Get simple value
vanta state get user_count

# Expected output (pretty format by default):
{
  "key": "user_count",
  "scope": "",
  "value": 1000
}

# Get value in raw format
vanta state get user_count --format raw

# Expected output:
# 1000

# Get value in JSON format
vanta state get current_user --format json

# Expected output:
{
  "id": 1,
  "name": "Admin",
  "role": "admin"
}

# Get value from specific scope
vanta state get last_login --scope user:123

# Get value and save to file
vanta state get app_config --output config-backup.json
```

### 6.4 List State Keys

```bash
# List all keys
vanta state list

# Expected output:
{
  "keys": [
    "user_count",
    "current_user",
    "app_config",
    "session_token"
  ],
  "count": 4
}

# List in text format
vanta state list --format text

# Expected output:
# user_count
# current_user
# app_config
# session_token

# List available scopes
vanta state list --scope

# Expected output:
{
  "scopes": [
    "user:123"
  ]
}
```

### 6.5 Delete State Values

```bash
# Delete specific key
vanta state delete session_token

# Expected output:
# Successfully deleted key: session_token

# Delete key from specific scope
vanta state delete last_login --scope user:123

# Expected output:
# Successfully deleted key: last_login
# from scope: user:123
```

### 6.6 Clear State

```bash
# Clear all state (with confirmation)
vanta state clear

# Expected output:
# This will permanently delete state data. Are you sure? (y/N): y
# Successfully cleared all state

# Clear without confirmation
vanta state clear --yes

# Clear specific scope
vanta state clear --scope user:123 --yes

# Expected output:
# Successfully cleared scope: user:123
```

### 6.7 Export/Import State

```bash
# First, set some test data
vanta state set test_data '{"key1":"value1","key2":"value2"}'
vanta state set counter 42
vanta state set enabled true

# Export complete state
vanta state export

# Expected output:
# State exported to: state_export_2025-01-15_10-30-45.json
# Exported 3 keys

# Export to specific file
vanta state export --output my-state-backup.json

# View export content
cat my-state-backup.json

# Clear state for import test
vanta state clear --yes

# Import state from file
vanta state import my-state-backup.json

# Expected output:
# Successfully imported state from: my-state-backup.json
# Imported 3 keys
# Existing state was replaced

# Import with merge (preserve existing state)
vanta state set new_key "new_value"
vanta state import my-state-backup.json --merge

# Expected output:
# Successfully imported state from: my-state-backup.json
# Imported 3 keys
# State was merged with existing data

# Stop server
kill $SERVER_PID
```

---

## 7. Command: tui

The `tui` command launches an interactive terminal user interface.

### 7.1 Launch Basic TUI

```bash
# Launch TUI with default configuration
vanta tui

# Expected output:
# INFO: Starting TUI mode config=config.yaml spec= readonly=false
# INFO: Launching Terminal UI...
# INFO: TUI Controls:
#   navigation: Tab/Shift+Tab to switch between panels
#   logs: ↑/↓ to scroll, f to filter, c to clear
#   config: ↑/↓ to navigate, Enter to edit, Ctrl+S to save
#   exit: q or Ctrl+C to quit

# [Se abre interfaz TUI interactiva]
```

### 7.2 TUI with OpenAPI Specification

```bash
# Launch TUI with specific specification
vanta tui --spec spec/vanta-test-api.yaml

# Should display:
# - Panel de métricas en tiempo real
# - Panel de logs con requests/responses
# - Panel de configuración
# - Panel de estado del servidor
```

### 7.3 TUI in Read-Only Mode

```bash
# Launch TUI without configuration editing
vanta tui --readonly --spec spec/vanta-test-api.yaml

# Configuration panel should be read-only
```

### 7.4 TUI Navigation

**TUI controls to test:**

1. **Navigation between panels:**
   - `Tab` / `Shift+Tab`: Cambiar entre paneles
   - `q` o `Ctrl+C`: Salir

2. **Logs Panel:**
   - `↑/↓`: Scroll through logs
   - `f`: Filter logs
   - `c`: Clear logs
   - `Enter`: View selected log details

3. **Metrics Panel:**
   - Should display in real time:
     - RPS (Requests Per Second)
     - Latencia promedio
     - Códigos de error
     - Memory usage
     - Active connections

4. **Configuration Panel:**
   - `↑/↓`: Navigate through options
   - `Enter`: Edit value (if not readonly)
   - `Ctrl+S`: Save changes
   - `Esc`: Cancel editing

### 7.5 Generate Traffic for TUI

With TUI active, in another terminal:

```bash
# Script to generate continuous traffic
for i in {1..100}; do
  curl -s http://localhost:8080/users > /dev/null &
  curl -s http://localhost:8080/products > /dev/null &
  curl -s http://localhost:8080/health > /dev/null &

  # Some errors to test metrics
  if [ $((i % 10)) -eq 0 ]; then
    curl -s http://localhost:8080/test/error/500 > /dev/null &
  fi

  sleep 0.1
done

wait
```

**Expected behavior in TUI:**
- Metrics panel should update RPS, latency, errors
- Logs panel should show incoming requests
- Counters should increment in real time

---

## 8. Command: version

### 8.1 Version Information

```bash
# Show version information
vanta version

# Expected output:
# Version: v1.0.0
# Commit: abc1234def5678
# Build Time: 2025-01-15T10:00:00Z
# Go Version: go1.21.0
# OS/Arch: linux/amd64
```

---

## Advanced Testing

### Hot Reload Testing

```bash
# Start server with hot reload enabled
vanta start spec/vanta-test-api.yaml &
SERVER_PID=$!

# Modify the OpenAPI specification
cp spec/vanta-test-api.yaml spec/vanta-test-api-modified.yaml

# Add new endpoint
cat >> spec/vanta-test-api-modified.yaml << EOF
  /test/new:
    get:
      summary: New test endpoint
      operationId: newTest
      responses:
        '200':
          description: New endpoint
          content:
            application/json:
              schema:
                type: object
                properties:
                  message:
                    type: string
EOF

# Server should detect changes automatically
# Verify new endpoint
curl http://localhost:8080/test/new

kill $SERVER_PID
```

### Plugin Testing

```bash
# Create configuration with plugins
cat > plugin-config.yaml << EOF
server:
  port: 8080
plugins:
  - name: "request_logger"
    type: "builtin"
    enabled: true
    config:
      log_level: "info"
      include_body: true

  - name: "rate_limiter"
    type: "builtin"
    enabled: true
    config:
      requests_per_minute: 60
      burst: 10

  - name: "cors_handler"
    type: "builtin"
    enabled: true
    config:
      allowed_origins: ["*"]
      allowed_methods: ["GET", "POST", "PUT", "DELETE"]
EOF

# Start with plugins
vanta start spec/vanta-test-api.yaml --config plugin-config.yaml

# Test CORS headers
curl -H "Origin: https://example.com" \
     -H "Access-Control-Request-Method: POST" \
     -H "Access-Control-Request-Headers: Content-Type" \
     -X OPTIONS \
     http://localhost:8080/users

# Should include CORS headers in response

# Test rate limiting
for i in {1..70}; do
  curl -s http://localhost:8080/health
done

# After 60 requests, should return 429 Too Many Requests
```

### Webhook Testing

```bash
# Configure test webhook with ngrok or local server
python3 -m http.server 9999 &
WEBHOOK_SERVER_PID=$!

# Create webhook in the API
curl -X POST http://localhost:8080/webhooks \
  -H "Content-Type: application/json" \
  -d '{
    "url": "http://localhost:9999/webhook",
    "events": ["user.created", "order.created"],
    "secret": "mysecretkey123"
  }'

# Create user to trigger webhook
curl -X POST http://localhost:8080/users \
  -H "Content-Type: application/json" \
  -d '{
    "username": "webhooktest",
    "email": "webhook@test.com",
    "password": "Test123!",
    "role": "user"
  }'

# Verify that webhook server received the notification
kill $WEBHOOK_SERVER_PID
```

### Performance Testing

```bash
# Use tools like ab, wrk, or hey for performance testing

# Test con Apache Bench
ab -n 1000 -c 10 http://localhost:8080/health

# Test con hey (si está instalado)
hey -n 1000 -c 10 http://localhost:8080/users

# Test con curl en paralelo
seq 1 100 | xargs -n1 -P10 -I{} curl -s http://localhost:8080/products > /dev/null

# Verify metrics during testing
curl http://localhost:8080/metrics
```

### Data Generation Testing

```bash
# Test different seeds for data generation
vanta start spec/vanta-test-api.yaml --config <(cat << EOF
mock:
  seed: 42
  locale: "en"
  max_depth: 3
  default_array_size: 5
  prefer_examples: true
EOF
) &

# Get data with seed 42
curl http://localhost:8080/users > users_seed_42.json

# Restart with different seed
pkill vanta
vanta start spec/vanta-test-api.yaml --config <(cat << EOF
mock:
  seed: 99
  locale: "es"
  max_depth: 3
  default_array_size: 5
  prefer_examples: true
EOF
) &

# Get data with seed 99
curl http://localhost:8080/users > users_seed_99.json

# Data should be different but consistent per seed
diff users_seed_42.json users_seed_99.json
```

---

## Troubleshooting

### Common Issues

#### 1. Port in Use

```bash
# Error: "bind: address already in use"
# Solution: Check which process is using the port
lsof -i :8080
kill <PID>

# Or use different port
vanta start spec/vanta-test-api.yaml --port 8081
```

#### 2. Invalid OpenAPI Specification

```bash
# Error: "failed to parse OpenAPI spec"
# Solution: Validate specification first
vanta validate spec spec/vanta-test-api.yaml

# Verify YAML syntax
yamllint spec/vanta-test-api.yaml
```

#### 3. Invalid Configuration

```bash
# Error: "configuration validation failed"
# Solution: Validate configuration
vanta config validate my-config.yaml

# Create clean configuration
vanta config init --output clean-config.yaml
```

#### 4. File Permissions

```bash
# Error: "permission denied"
# Solution: Check permissions
ls -la spec/vanta-test-api.yaml
chmod 644 spec/vanta-test-api.yaml
```

#### 5. Corrupted State

```bash
# Error in state management
# Solution: Clear state
vanta state clear --yes

# Or restart completely
rm -rf ./recordings ./state
```

### Logging and Debug

```bash
# Start with debug logging
vanta start spec/vanta-test-api.yaml --config <(cat << EOF
logging:
  level: "debug"
  format: "text"
  output: "stdout"
  add_caller: true
EOF
)

# Check system logs
journalctl -f | grep vanta

# Check system resources
htop
ps aux | grep vanta
```

### Health Verification

```bash
# Script to verify server status
check_health() {
  response=$(curl -s -w "%{http_code}" http://localhost:8080/health)
  http_code="${response: -3}"

  if [ "$http_code" = "200" ]; then
    echo "✅ Server is healthy"
    return 0
  else
    echo "❌ Server health check failed (HTTP $http_code)"
    return 1
  fi
}

# Check health every 5 seconds
while true; do
  check_health
  sleep 5
done
```

---

## Command Summary

| Command | Functionality | Basic Example |
|---------|---------------|----------------|
| `start` | Start mock server | `vanta start spec/vanta-test-api.yaml` |
| `config` | Manage configuration | `vanta config init` |
| `validate` | Validate specs and compliance | `vanta validate spec spec/vanta-test-api.yaml` |
| `record` | Record/replay traffic | `vanta record start` |
| `chaos` | Chaos engineering | `vanta chaos start --config chaos.yaml` |
| `state` | Manage state | `vanta state set key value` |
| `tui` | Terminal interface | `vanta tui --spec spec/vanta-test-api.yaml` |
| `version` | Version info | `vanta version` |

---

## Next Steps

1. **Automated testing**: Create scripts to automate these tests
2. **CI/CD Integration**: Integrate tests into CI/CD pipeline
3. **Monitoring**: Configure monitoring in production
4. **Custom Plugins**: Develop custom plugins
5. **Load Testing**: Perform more exhaustive load testing

For more specific testing, refer to the `test-scenarios.md` file which contains specific use cases and regression tests.