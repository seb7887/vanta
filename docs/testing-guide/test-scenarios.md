# Vanta CLI - Specific Testing Scenarios

This document contains specific testing scenarios, edge cases, regression tests, and advanced validations for the Vanta CLI.

## Table of Contents

- [Testing Scenarios by Functionality](#testing-scenarios-by-functionality)
- [Integration Tests](#integration-tests)
- [Regression Tests](#regression-tests)
- [Performance Tests](#performance-tests)
- [Security Tests](#security-tests)
- [Edge Case Tests](#edge-case-tests)
- [Test Automation](#test-automation)

---

## Testing Scenarios by Functionality

### Start Command - Specific Scenarios

#### Scenario 1: Multiple Specs Formats
```bash
# Test with YAML
vanta start spec/vanta-test-api.yaml
curl http://localhost:8080/health

# Test with JSON (convert spec to JSON)
yq eval -o=json spec/vanta-test-api.yaml > spec/vanta-test-api.json
vanta start spec/vanta-test-api.json
curl http://localhost:8080/health

# Both should work identically
```

#### Scenario 2: Relative vs Absolute Paths
```bash
# Test with relative path
cd /tmp
vanta start ../path/to/vanta/spec/vanta-test-api.yaml

# Test with absolute path
vanta start /full/path/to/vanta/spec/vanta-test-api.yaml

# Test with specification in current directory
cp /path/to/vanta/spec/vanta-test-api.yaml ./
vanta start ./vanta-test-api.yaml
```

#### Scenario 3: Server Binding and Network
```bash
# Test binding to localhost only
vanta start spec/vanta-test-api.yaml --host 127.0.0.1 --port 8080

# Should be accessible only from localhost
curl http://127.0.0.1:8080/health  # ✅ Should work
curl http://0.0.0.0:8080/health     # ❌ Should fail

# Test binding to all interfaces
vanta start spec/vanta-test-api.yaml --host 0.0.0.0 --port 8080

# Should be accessible from any IP
curl http://localhost:8080/health   # ✅ Should work
curl http://127.0.0.1:8080/health   # ✅ Should work
```

#### Scenario 4: Port Conflicts
```bash
# Occupy port 8080
python3 -c "import socket,time; s=socket.socket(); s.bind(('',8080)); s.listen(1); time.sleep(30)" &
BLOCKER_PID=$!

# Try to start Vanta on same port
vanta start spec/vanta-test-api.yaml --port 8080

# Should fail with descriptive error:
# Error: failed to start server: listen tcp :8080: bind: address already in use

kill $BLOCKER_PID

# Test dynamic port
vanta start spec/vanta-test-api.yaml --port 0
# Should assign port automatically
```

### Config Command - Specific Scenarios

#### Scenario 5: Config Validation Edge Cases
```bash
# Test configuration with limit values
cat > edge-config.yaml << EOF
server:
  port: 1              # Minimum valid port
  host: ""             # Empty host
  read_timeout: 1ms    # Minimum timeout
  write_timeout: 24h   # Maximum timeout
  max_conns_per_ip: 0  # No limit
  concurrency: 1       # Minimum concurrency
mock:
  seed: -1             # Negative seed
  max_depth: 0         # Zero depth
  default_array_size: 1000  # Large array
logging:
  level: ""            # Empty level
  format: "invalid"    # Invalid format
EOF

vanta config validate edge-config.yaml
# Should report specific errors for each invalid field

# Test configuration with valid extreme values
cat > extreme-config.yaml << EOF
server:
  port: 65535
  read_timeout: 5m
  write_timeout: 5m
  max_conns_per_ip: 10000
  concurrency: 1000000
mock:
  seed: 2147483647     # Max int32
  max_depth: 50
  default_array_size: 1000
EOF

vanta config validate extreme-config.yaml
# Should be valid
```

#### Scenario 6: Config Override Priority
```bash
# Create base configuration
cat > base-config.yaml << EOF
server:
  port: 8080
  host: "0.0.0.0"
logging:
  level: "info"
EOF

# Test override priority
vanta start spec/vanta-test-api.yaml \
  --config base-config.yaml \
  --port 9090 \
  --host 127.0.0.1

# Verify that CLI flags have priority
curl http://127.0.0.1:9090/health  # ✅ Should work
curl http://0.0.0.0:8080/health     # ❌ Should fail
```

### Validation Command - Specific Scenarios

#### Scenario 7: OpenAPI Version Compatibility
```bash
# Test OpenAPI 3.0.0
cat > openapi30.yaml << EOF
openapi: 3.0.0
info:
  title: Test API
  version: 1.0.0
paths:
  /test:
    get:
      responses:
        '200':
          description: OK
EOF

vanta validate spec openapi30.yaml

# Test OpenAPI 3.0.3
sed 's/3.0.0/3.0.3/' openapi30.yaml > openapi303.yaml
vanta validate spec openapi303.yaml

# Test OpenAPI 3.1.0 (if supported)
sed 's/3.0.0/3.1.0/' openapi30.yaml > openapi310.yaml
vanta validate spec openapi310.yaml
```

#### Scenario 8: Schema Validation Edge Cases
```bash
# Test schema with circular references
cat > circular-refs.yaml << EOF
openapi: 3.0.3
info:
  title: Circular Test
  version: 1.0.0
paths:
  /test:
    get:
      responses:
        '200':
          description: OK
          content:
            application/json:
              schema:
                \$ref: '#/components/schemas/Node'
components:
  schemas:
    Node:
      type: object
      properties:
        value:
          type: string
        children:
          type: array
          items:
            \$ref: '#/components/schemas/Node'  # Circular reference
EOF

vanta validate spec circular-refs.yaml
# Should handle circular references gracefully
```

### Record Command - Specific Scenarios

#### Scenario 9: Recording Storage Limits
```bash
# Test recording limit
vanta record start --max-recordings 3

# Generate more requests than the limit
for i in {1..10}; do
  curl http://localhost:8080/users
  sleep 0.1
done

vanta record list
# Should show only 3 recordings (the most recent ones)

vanta record stop
```

#### Scenario 10: Advanced Recording Filters
```bash
# Test complex filters
vanta record start \
  --filter "method:GET" \
  --filter "method:POST" \
  --filter "endpoint:/users*" \
  --filter "status:200"

# Generate mixed traffic
curl -X GET http://localhost:8080/users          # ✅ Should be recorded
curl -X POST http://localhost:8080/users -d '{}'  # ✅ Should be recorded
curl -X GET http://localhost:8080/products       # ❌ Should not be recorded
curl -X PUT http://localhost:8080/users/1 -d '{}'  # ❌ Should not be recorded

vanta record list
# Should show only requests that meet ALL filters

vanta record stop
```

#### Scenario 11: Recording Large Bodies
```bash
# Test with large bodies
vanta record start --max-body-size 1KB

# Generate request with large body
large_body=$(python3 -c "print('x' * 2000)")
curl -X POST http://localhost:8080/users \
  -H "Content-Type: application/json" \
  -d "{\"username\":\"test\",\"large_field\":\"$large_body\"}"

vanta record list
vanta record show <recording-id>
# The body should be truncated to 1KB

vanta record stop
```

### Chaos Command - Specific Scenarios

#### Scenario 12: Chaos Probability Testing
```bash
# Configure chaos with high probability for testing
cat > high-chaos.yaml << EOF
chaos:
  enabled: true
  scenarios:
    - name: "high_latency"
      type: "latency"
      probability: 0.9  # 90% probability
      endpoints: ["/test/*"]
      parameters:
        min_delay: "500ms"
        max_delay: "1s"
EOF

vanta chaos start --config high-chaos.yaml &
CHAOS_PID=$!

# Generate 100 requests and measure how many have additional latency
start_time=$(date +%s%N)
for i in {1..100}; do
  response_time=$(curl -s -w "%{time_total}" -o /dev/null http://localhost:8080/test/slow)
  if (( $(echo "$response_time > 0.5" | bc -l) )); then
    echo "Request $i: High latency detected ($response_time s)"
  fi
done
end_time=$(date +%s%N)

# Should have ~90% of requests with high latency
kill $CHAOS_PID
```

#### Scenario 13: Chaos Error Injection
```bash
# Test specific error injection
cat > error-chaos.yaml << EOF
chaos:
  enabled: true
  scenarios:
    - name: "specific_errors"
      type: "error"
      probability: 1.0  # 100% for testing
      endpoints: ["/test/error/*"]
      parameters:
        status_codes: [503, 504]
EOF

vanta chaos start --config error-chaos.yaml &
CHAOS_PID=$!

# Test that specific errors are returned
for code in 503 504; do
  response=$(curl -s -w "%{http_code}" http://localhost:8080/test/error/500)
  actual_code="${response: -3}"
  echo "Expected error injection, got: $actual_code"
  # Should return 503 or 504 instead of 500
done

kill $CHAOS_PID
```

### State Command - Specific Scenarios

#### Scenario 14: State Persistence
```bash
# Test state persistence
vanta start spec/vanta-test-api.yaml &
SERVER_PID=$!

# Set state
vanta state set persistent_data "test_value"
vanta state set counter 42

# Restart server
kill $SERVER_PID
vanta start spec/vanta-test-api.yaml &
SERVER_PID=$!

# Verify that state persists
stored_value=$(vanta state get persistent_data --format raw)
if [ "$stored_value" = "test_value" ]; then
  echo "✅ State persistence works"
else
  echo "❌ State persistence failed"
fi

kill $SERVER_PID
```

#### Scenario 15: State TTL Expiration
```bash
vanta start spec/vanta-test-api.yaml &
SERVER_PID=$!

# Set value with short TTL
vanta state set temp_value "expires_soon" --ttl 5s

# Verify it exists immediately
vanta state get temp_value

# Wait for expiration
sleep 6

# Verify it expired
vanta state get temp_value 2>&1 | grep "not found"
if [ $? -eq 0 ]; then
  echo "✅ TTL expiration works"
else
  echo "❌ TTL expiration failed"
fi

kill $SERVER_PID
```

---

## Integration Tests

### Start + Record + Chaos Integration

#### Scenario 16: Complete Workflow
```bash
# Complete integration script
#!/bin/bash
set -e

echo "🚀 Starting integration test..."

# 1. Start server
vanta start spec/vanta-test-api.yaml --port 8080 &
SERVER_PID=$!
sleep 2

echo "✅ Server started"

# 2. Start recording
vanta record start --max-recordings 50
echo "✅ Recording started"

# 3. Configure chaos
cat > integration-chaos.yaml << EOF
chaos:
  enabled: true
  scenarios:
    - name: "integration_test"
      type: "latency"
      probability: 0.2
      endpoints: ["/users", "/products"]
      parameters:
        min_delay: "100ms"
        max_delay: "300ms"
EOF

vanta chaos start --config integration-chaos.yaml &
CHAOS_PID=$!
echo "✅ Chaos started"

# 4. Generate test traffic
echo "📊 Generating test traffic..."
for i in {1..50}; do
  curl -s http://localhost:8080/users > /dev/null &
  curl -s http://localhost:8080/products > /dev/null &
  curl -s http://localhost:8080/health > /dev/null &

  if [ $((i % 10)) -eq 0 ]; then
    echo "Processed $i requests..."
  fi

  sleep 0.1
done

wait
echo "✅ Traffic generated"

# 5. Stop chaos
kill $CHAOS_PID
echo "✅ Chaos stopped"

# 6. Stop recording
vanta record stop
echo "✅ Recording stopped"

# 7. Analyze results
echo "📊 Recording summary:"
vanta record list --limit 10

echo "📊 Server metrics:"
curl -s http://localhost:8080/metrics | jq '{requests_total, errors_total, average_latency_ms}'

# 8. Test replay
echo "🔄 Testing replay..."
vanta start spec/vanta-test-api.yaml --port 8081 &
REPLAY_SERVER_PID=$!
sleep 2

vanta record replay --target http://localhost:8081 --limit 10
echo "✅ Replay completed"

# 9. Cleanup
kill $SERVER_PID $REPLAY_SERVER_PID
vanta record delete --all --force

echo "🎉 Integration test completed successfully!"
```

### TUI + Monitoring Integration

#### Scenario 17: TUI Monitoring Test
```bash
# Script for TUI monitoring test
#!/bin/bash

# Function to generate background traffic
generate_traffic() {
  while true; do
    curl -s http://localhost:8080/users > /dev/null
    curl -s http://localhost:8080/products > /dev/null

    # Generate some errors
    if [ $((RANDOM % 20)) -eq 0 ]; then
      curl -s http://localhost:8080/test/error/500 > /dev/null
    fi

    sleep 0.5
  done
}

echo "Starting TUI monitoring test..."
echo "1. Start server in background"
echo "2. Start traffic generator"
echo "3. Launch TUI to observe metrics"
echo "4. Press Enter when ready..."
read

vanta start spec/vanta-test-api.yaml &
SERVER_PID=$!
sleep 2

generate_traffic &
TRAFFIC_PID=$!

echo "Traffic generator started. Launch TUI now:"
echo "vanta tui --spec spec/vanta-test-api.yaml"
echo ""
echo "In TUI, verify:"
echo "- RPS is increasing"
echo "- Error rate shows ~5% errors"
echo "- Latency metrics are populated"
echo "- Request logs are streaming"
echo ""
echo "Press Enter when TUI test is complete..."
read

kill $TRAFFIC_PID $SERVER_PID
echo "TUI monitoring test completed"
```

---

## Regression Tests

### Regression: OpenAPI Parsing

#### Scenario 18: Backward Compatibility
```bash
# Test with older OpenAPI specifications
for version in "3.0.0" "3.0.1" "3.0.2" "3.0.3"; do
  echo "Testing OpenAPI version $version"

  sed "s/openapi: 3.0.3/openapi: $version/" spec/vanta-test-api.yaml > "test-$version.yaml"

  vanta validate spec "test-$version.yaml"
  if [ $? -eq 0 ]; then
    echo "✅ Version $version: PASS"
  else
    echo "❌ Version $version: FAIL"
  fi

  rm "test-$version.yaml"
done
```

### Regression: Configuration Changes

#### Scenario 19: Config Format Evolution
```bash
# Test compatibility with previous configurations
cat > legacy-config-v1.yaml << EOF
# Simulated legacy format
port: 8080
host: "localhost"
log_level: "info"
chaos_enabled: false
EOF

# Should handle legacy format gracefully or give descriptive error
vanta config validate legacy-config-v1.yaml

# Test configuration migration
vanta config init --output new-format.yaml
# Compare structures
diff legacy-config-v1.yaml new-format.yaml || true
```

---

## Performance Tests

### Performance: Load Testing

#### Scenario 20: High Concurrency
```bash
# Test with high concurrency
vanta start spec/vanta-test-api.yaml &
SERVER_PID=$!
sleep 2

echo "🔥 Running high concurrency test..."

# Test with available tools
if command -v hey &> /dev/null; then
  echo "Using hey for load testing..."
  hey -n 10000 -c 100 -t 30 http://localhost:8080/health
elif command -v ab &> /dev/null; then
  echo "Using Apache Bench for load testing..."
  ab -n 10000 -c 100 http://localhost:8080/health
else
  echo "Using curl for basic load testing..."
  for i in {1..1000}; do
    curl -s http://localhost:8080/health > /dev/null &
    if [ $((i % 100)) -eq 0 ]; then
      wait  # Wait for batch to complete
      echo "Completed $i requests"
    fi
  done
  wait
fi

# Verify post-test metrics
echo "📊 Post-test metrics:"
curl -s http://localhost:8080/metrics | jq '{requests_total, errors_total, average_latency_ms, p95_latency_ms}'

kill $SERVER_PID
```

### Performance: Memory Testing

#### Scenario 21: Memory Leak Detection
```bash
# Test memory leak detection
vanta start spec/vanta-test-api.yaml &
SERVER_PID=$!
sleep 2

echo "🧠 Memory leak detection test..."

# Function to get memory usage
get_memory_usage() {
  ps -p $SERVER_PID -o rss= | tr -d ' '
}

# Baseline
initial_memory=$(get_memory_usage)
echo "Initial memory: ${initial_memory}KB"

# Generate sustained traffic
for cycle in {1..10}; do
  echo "Cycle $cycle: Generating 1000 requests..."

  for i in {1..1000}; do
    curl -s http://localhost:8080/users > /dev/null &
    curl -s http://localhost:8080/products > /dev/null &

    if [ $((i % 100)) -eq 0 ]; then
      wait
    fi
  done
  wait

  current_memory=$(get_memory_usage)
  memory_growth=$((current_memory - initial_memory))
  echo "Memory after cycle $cycle: ${current_memory}KB (growth: +${memory_growth}KB)"

  sleep 5  # Pause between cycles
done

final_memory=$(get_memory_usage)
total_growth=$((final_memory - initial_memory))

echo "📊 Memory test results:"
echo "Initial: ${initial_memory}KB"
echo "Final: ${final_memory}KB"
echo "Total growth: ${total_growth}KB"

if [ $total_growth -gt 100000 ]; then  # 100MB threshold
  echo "⚠️  Potential memory leak detected!"
else
  echo "✅ Memory usage is stable"
fi

kill $SERVER_PID
```

---

## Security Tests

### Security: Input Validation

#### Scenario 22: Malicious Payloads
```bash
vanta start spec/vanta-test-api.yaml &
SERVER_PID=$!
sleep 2

echo "🛡️  Security input validation test..."

# Test SQL injection attempts
curl -X POST http://localhost:8080/users \
  -H "Content-Type: application/json" \
  -d '{"username":"admin'\''--","email":"test@test.com","password":"test123"}'

# Test XSS attempts
curl -X POST http://localhost:8080/users \
  -H "Content-Type: application/json" \
  -d '{"username":"<script>alert(1)</script>","email":"test@test.com","password":"test123"}'

# Test extremely large payloads
large_payload=$(python3 -c "print('x' * 1000000)")  # 1MB payload
curl -X POST http://localhost:8080/users \
  -H "Content-Type: application/json" \
  -d "{\"username\":\"test\",\"large_field\":\"$large_payload\"}"

# Test malformed JSON
curl -X POST http://localhost:8080/users \
  -H "Content-Type: application/json" \
  -d '{"username":"test","email":"test@test.com","password":"test123"'  # Missing closing brace

echo "✅ Server should handle malicious payloads gracefully"

kill $SERVER_PID
```

### Security: Rate Limiting

#### Scenario 23: Rate Limit Testing
```bash
# Configure rate limiting
cat > rate-limit-config.yaml << EOF
server:
  port: 8080
plugins:
  - name: "rate_limiter"
    type: "builtin"
    enabled: true
    config:
      requests_per_minute: 10
      burst: 5
EOF

vanta start spec/vanta-test-api.yaml --config rate-limit-config.yaml &
SERVER_PID=$!
sleep 2

echo "🚦 Rate limiting test..."

# Generate requests quickly
success_count=0
rate_limited_count=0

for i in {1..20}; do
  response_code=$(curl -s -w "%{http_code}" -o /dev/null http://localhost:8080/health)

  if [ "$response_code" = "200" ]; then
    ((success_count++))
  elif [ "$response_code" = "429" ]; then
    ((rate_limited_count++))
  fi

  sleep 0.1
done

echo "✅ Rate limit test results:"
echo "Successful requests: $success_count"
echo "Rate limited requests: $rate_limited_count"

if [ $rate_limited_count -gt 0 ]; then
  echo "✅ Rate limiting is working"
else
  echo "⚠️  Rate limiting may not be working correctly"
fi

kill $SERVER_PID
```

---

## Edge Case Tests

### Edge Case: Empty Specifications

#### Scenario 24: Minimal Valid Spec
```bash
# Test with minimal valid specification
cat > minimal-spec.yaml << EOF
openapi: 3.0.3
info:
  title: Minimal API
  version: 1.0.0
paths: {}
EOF

vanta validate spec minimal-spec.yaml
vanta start minimal-spec.yaml &
SERVER_PID=$!
sleep 2

# Should start without errors but have no endpoints
curl -s -w "%{http_code}" http://localhost:8080/nonexistent
# Should return 404

kill $SERVER_PID
```

### Edge Case: Special Characters

#### Scenario 25: Unicode and Special Characters
```bash
# Test with special characters in configuration
cat > unicode-config.yaml << EOF
server:
  port: 8080
mock:
  locale: "es"
  custom_data:
    emoji_test: "🚀🎯📊"
    unicode_test: "café naïve résumé"
    special_chars: "!@#$%^&*()[]{}|;:,.<>?"
EOF

vanta config validate unicode-config.yaml
vanta start spec/vanta-test-api.yaml --config unicode-config.yaml &
SERVER_PID=$!
sleep 2

# Test that unicode data is generated correctly
curl http://localhost:8080/users | jq '.'

kill $SERVER_PID
```

### Edge Case: File System Limits

#### Scenario 26: Long Path Names
```bash
# Test with long filenames
long_name="very_long_filename_$(printf 'a%.0s' {1..200}).yaml"
cp spec/vanta-test-api.yaml "$long_name"

vanta validate spec "$long_name"
if [ $? -eq 0 ]; then
  echo "✅ Long filenames handled correctly"
else
  echo "❌ Long filenames cause issues"
fi

rm "$long_name"
```

---

## Automatización de Tests

### Test Runner Script

#### Escenario 27: Automated Test Suite
```bash
#!/bin/bash
# test-runner.sh - Script automatizado de testing

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Counters
TESTS_RUN=0
TESTS_PASSED=0
TESTS_FAILED=0

# Helper functions
log_info() {
  echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
  echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
  echo -e "${RED}[ERROR]${NC} $1"
}

run_test() {
  local test_name="$1"
  local test_command="$2"

  ((TESTS_RUN++))

  echo ""
  echo "🧪 Running test: $test_name"
  echo "Command: $test_command"

  if eval "$test_command"; then
    log_info "✅ PASSED: $test_name"
    ((TESTS_PASSED++))
  else
    log_error "❌ FAILED: $test_name"
    ((TESTS_FAILED++))
  fi
}

cleanup() {
  log_info "Cleaning up test environment..."
  pkill -f vanta || true
  rm -f test-*.yaml temp-*.json *.tmp
  sleep 1
}

# Pre-test setup
setup() {
  log_info "Setting up test environment..."

  # Verify vanta binary exists
  if ! command -v vanta &> /dev/null; then
    log_error "vanta binary not found in PATH"
    exit 1
  fi

  # Verify test files exist
  if [ ! -f "spec/vanta-test-api.yaml" ]; then
    log_error "Test specification not found: spec/vanta-test-api.yaml"
    exit 1
  fi

  cleanup
}

# Test suite
run_all_tests() {
  log_info "Starting Vanta CLI Test Suite"

  # Basic functionality tests
  run_test "Version Check" "vanta version"
  run_test "Config Init" "vanta config init --output test-config.yaml"
  run_test "Config Validate" "vanta config validate test-config.yaml"
  run_test "Spec Validate" "vanta validate spec spec/vanta-test-api.yaml"

  # Server tests
  run_test "Server Start/Stop" "
    vanta start spec/vanta-test-api.yaml --port 18080 &
    SERVER_PID=\$!
    sleep 3
    curl -s http://localhost:18080/health > /dev/null
    kill \$SERVER_PID
    wait \$SERVER_PID 2>/dev/null || true
  "

  # Recording tests
  run_test "Record Start/Stop" "
    vanta start spec/vanta-test-api.yaml --port 18080 &
    SERVER_PID=\$!
    sleep 2
    vanta record start --max-recordings 5 &
    RECORD_PID=\$!
    sleep 1
    curl -s http://localhost:18080/health > /dev/null
    vanta record stop
    vanta record list > /dev/null
    kill \$SERVER_PID
    wait \$SERVER_PID 2>/dev/null || true
  "

  # State management tests
  run_test "State Management" "
    vanta start spec/vanta-test-api.yaml --port 18080 &
    SERVER_PID=\$!
    sleep 2
    vanta state set test_key test_value
    result=\$(vanta state get test_key --format raw)
    [ \"\$result\" = \"test_value\" ]
    vanta state delete test_key
    kill \$SERVER_PID
    wait \$SERVER_PID 2>/dev/null || true
  "

  # Validation tests
  run_test "Coverage Report" "
    vanta start spec/vanta-test-api.yaml --port 18080 &
    SERVER_PID=\$!
    sleep 2
    curl -s http://localhost:18080/health > /dev/null
    curl -s http://localhost:18080/users > /dev/null
    vanta validate coverage spec/vanta-test-api.yaml > /dev/null
    kill \$SERVER_PID
    wait \$SERVER_PID 2>/dev/null || true
  "

  # Error handling tests
  run_test "Invalid Spec Handling" "
    echo 'invalid: yaml: content' > invalid-spec.yaml
    ! vanta validate spec invalid-spec.yaml
    rm invalid-spec.yaml
  "

  run_test "Port Conflict Handling" "
    python3 -c 'import socket,time; s=socket.socket(); s.bind((\"\",18080)); s.listen(1); time.sleep(5)' &
    BLOCKER_PID=\$!
    sleep 1
    ! vanta start spec/vanta-test-api.yaml --port 18080
    kill \$BLOCKER_PID 2>/dev/null || true
  "
}

# Report results
report_results() {
  echo ""
  echo "=============================="
  echo "        TEST SUMMARY"
  echo "=============================="
  echo "Tests run: $TESTS_RUN"
  echo "Tests passed: $TESTS_PASSED"
  echo "Tests failed: $TESTS_FAILED"

  if [ $TESTS_FAILED -eq 0 ]; then
    log_info "🎉 All tests passed!"
    exit 0
  else
    log_error "💥 $TESTS_FAILED tests failed!"
    exit 1
  fi
}

# Main execution
main() {
  setup
  run_all_tests
  cleanup
  report_results
}

# Handle interrupts
trap cleanup EXIT INT TERM

# Run if executed directly
if [ "${BASH_SOURCE[0]}" = "${0}" ]; then
  main "$@"
fi
```

### CI/CD Integration

#### Escenario 28: GitHub Actions Workflow
```yaml
# .github/workflows/vanta-cli-test.yml
name: Vanta CLI Tests

on:
  push:
    branches: [ main, develop ]
  pull_request:
    branches: [ main ]

jobs:
  test:
    runs-on: ubuntu-latest

    steps:
    - uses: actions/checkout@v3

    - name: Set up Go
      uses: actions/setup-go@v3
      with:
        go-version: 1.21

    - name: Build Vanta CLI
      run: |
        go build -o vanta ./cmd/vanta
        chmod +x vanta
        sudo mv vanta /usr/local/bin/

    - name: Install test dependencies
      run: |
        sudo apt-get update
        sudo apt-get install -y curl jq bc

    - name: Run basic tests
      run: |
        vanta version
        vanta config init --output ci-config.yaml
        vanta config validate ci-config.yaml

    - name: Run specification tests
      run: |
        vanta validate spec spec/vanta-test-api.yaml
        vanta validate spec spec/vanta-test-api.yaml --format json

    - name: Run server tests
      run: |
        # Start server in background
        vanta start spec/vanta-test-api.yaml --port 8080 &
        SERVER_PID=$!

        # Wait for server to start
        sleep 5

        # Test endpoints
        curl -f http://localhost:8080/health
        curl -f http://localhost:8080/users
        curl -f http://localhost:8080/products

        # Stop server
        kill $SERVER_PID

    - name: Run integration tests
      run: |
        chmod +x docs/testing-guide/test-runner.sh
        ./docs/testing-guide/test-runner.sh

    - name: Upload test artifacts
      if: failure()
      uses: actions/upload-artifact@v3
      with:
        name: test-logs
        path: |
          *.log
          test-*.yaml
          recordings/
```

### Performance Benchmarking

#### Escenario 29: Automated Benchmarks
```bash
#!/bin/bash
# benchmark.sh - Performance benchmarking script

benchmark_endpoint() {
  local endpoint="$1"
  local name="$2"

  echo "🏃 Benchmarking $name..."

  if command -v hey &> /dev/null; then
    hey -n 1000 -c 10 -t 30 "$endpoint" | tee "benchmark-$name.log"
  elif command -v ab &> /dev/null; then
    ab -n 1000 -c 10 "$endpoint" | tee "benchmark-$name.log"
  else
    echo "No benchmarking tool available (hey or ab required)"
    return 1
  fi
}

run_benchmarks() {
  vanta start spec/vanta-test-api.yaml --port 8080 &
  SERVER_PID=$!
  sleep 3

  benchmark_endpoint "http://localhost:8080/health" "health"
  benchmark_endpoint "http://localhost:8080/users" "users"
  benchmark_endpoint "http://localhost:8080/products" "products"

  kill $SERVER_PID

  echo "📊 Benchmark results saved to benchmark-*.log files"
}

if [ "${BASH_SOURCE[0]}" = "${0}" ]; then
  run_benchmarks
fi
```

---

## Conclusion

These testing scenarios cover:

1. **Basic functionality**: Verification that each command works correctly
2. **Edge cases**: Limit cases and special situations
3. **Integration**: Tests that verify interaction between components
4. **Performance**: Load tests and memory leak detection
5. **Security**: Input validation and rate limiting
6. **Regression**: Tests to prevent regressions in future versions
7. **Automation**: Scripts to run tests automatically

To use these tests:

1. **Manual**: Execute each scenario individually during development
2. **Semi-automated**: Use the `test-runner.sh` script for complete test suite
3. **CI/CD**: Integrate with GitHub Actions or other CI/CD system
4. **Monitoring**: Use for health monitoring in production

Each scenario includes specific commands and expected results to facilitate manual and automated validation of the Vanta CLI functionality.