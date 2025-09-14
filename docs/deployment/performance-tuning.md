# Performance Tuning Guide

This guide provides comprehensive strategies for optimizing Vanta's performance across different dimensions: throughput, latency, memory usage, and resource efficiency.

## 🚀 Performance Overview

### Key Performance Metrics

#### Throughput Metrics
- **Requests per second (RPS)**: Target 10,000+ RPS per instance
- **Concurrent connections**: Support 50,000+ concurrent connections
- **Data generation rate**: 1,000+ mock responses per second

#### Latency Metrics
- **P50 latency**: < 10ms for simple responses
- **P95 latency**: < 50ms for complex responses
- **P99 latency**: < 100ms under normal load

#### Resource Efficiency
- **Memory usage**: < 2GB per instance under normal load
- **CPU usage**: < 70% under normal load
- **Network bandwidth**: Efficient utilization without bottlenecks

## ⚙️ Server Configuration Optimization

### HTTP Server Tuning

#### Basic Configuration
```yaml
# config/performance.yaml
server:
  port: 8080
  host: "0.0.0.0"

  # Connection settings
  read_timeout: "10s"        # Reduced from 30s
  write_timeout: "10s"       # Reduced from 30s
  idle_timeout: "60s"        # Reduced from 120s

  # Concurrency settings
  concurrency: 50000         # Increased from 10000
  max_conns_per_ip: 2000     # Increased from 1000

  # Request limits
  max_request_body_size: "1MB"  # Reduced from 10MB for API mocking

  # Advanced FastHTTP settings
  disable_keepalive: false
  tcp_keepalive: true
  max_request_duration: "30s"
  reduce_memory_usage: false  # Set to true if memory constrained
```

#### High-Performance Configuration
```yaml
# For maximum throughput
server:
  concurrency: 100000        # Very high concurrency
  max_conns_per_ip: 5000     # Higher per-IP limit
  read_buffer_size: 8192     # Optimize buffer sizes
  write_buffer_size: 8192
  max_request_body_size: "512KB"  # Smaller for faster processing

  # Disable features that add latency
  disable_header_names_normalizing: true
  disable_pre_parse_multipart_form: true
  no_default_server_header: true
  no_default_date: true
  no_default_content_type: true
```

### Connection Pool Optimization

#### Load Balancer Configuration
```nginx
# nginx.conf - Optimized for Vanta
upstream vanta_backend {
    # Connection pooling
    keepalive 1000;
    keepalive_requests 10000;
    keepalive_timeout 60s;

    # Server configuration
    server vanta-1:8080 max_fails=3 fail_timeout=10s weight=1;
    server vanta-2:8080 max_fails=3 fail_timeout=10s weight=1;
    server vanta-3:8080 max_fails=3 fail_timeout=10s weight=1;
}

server {
    listen 80;

    # Connection limits
    limit_conn_zone $binary_remote_addr zone=conn_limit_per_ip:10m;
    limit_conn conn_limit_per_ip 100;

    # Rate limiting
    limit_req_zone $binary_remote_addr zone=req_limit_per_ip:10m rate=1000r/s;
    limit_req zone=req_limit_per_ip burst=2000 nodelay;

    location / {
        proxy_pass http://vanta_backend;

        # Connection reuse
        proxy_http_version 1.1;
        proxy_set_header Connection "";

        # Timeouts
        proxy_connect_timeout 5s;
        proxy_send_timeout 10s;
        proxy_read_timeout 10s;

        # Buffering
        proxy_buffering on;
        proxy_buffer_size 8k;
        proxy_buffers 8 8k;
        proxy_busy_buffers_size 16k;
    }
}
```

## 🧠 Data Generation Optimization

### Mock Configuration Tuning

#### Efficient Mock Settings
```yaml
mock:
  # Use deterministic generation for consistency
  seed: 12345

  # Prefer examples over generation when available
  prefer_examples: true

  # Limit complexity to improve performance
  max_depth: 3               # Reduced from 10
  default_array_size: 5      # Reduced from 10
  max_array_size: 50         # Reduced from 100

  # Locale optimization
  locale: "en_US"            # Use specific locale to avoid detection overhead

  # Caching for frequently accessed data
  cache_generated_data: true
  max_cache_size: "200MB"
  cache_ttl: "1h"

  # String optimization
  default_string_length: 10   # Shorter strings for faster generation
  max_string_length: 100     # Limit maximum string length
```

#### Advanced Generation Settings
```yaml
mock:
  # Performance-focused generation
  fast_generation: true      # Enable fast generation mode
  skip_validation: true      # Skip post-generation validation
  reuse_objects: true        # Reuse generated objects when possible

  # Memory optimization
  gc_after_generation: false # Don't force GC after each generation
  pool_generators: true      # Pool generator instances

  # Format-specific optimizations
  uuid_version: 4            # Use UUID v4 (fastest)
  date_format: "2006-01-02"  # Simple date format
  time_format: "15:04:05"    # Simple time format
```

### Caching Strategies

#### Response Caching
```yaml
# Enable response caching plugin
plugins:
  - name: cache
    enabled: true
    config:
      # Cache configuration
      ttl: "5m"                    # 5-minute cache
      max_size: "500MB"            # Large cache for high hit rate
      max_entries: 100000          # Maximum cached entries

      # Cache keys
      cache_headers: true          # Cache based on headers
      cache_query_params: true     # Cache based on query parameters

      # Cache policies
      cache_private: false         # Only cache public responses
      respect_cache_control: true  # Honor cache control headers

      # Performance settings
      cleanup_interval: "1m"       # Frequent cleanup
      async_cleanup: true          # Non-blocking cleanup
```

#### Data Generation Caching
```yaml
mock:
  # Advanced caching configuration
  cache_config:
    enabled: true

    # Cache levels
    cache_schemas: true          # Cache compiled schemas
    cache_examples: true         # Cache parsed examples
    cache_generated_objects: true # Cache generated data

    # Cache sizes
    schema_cache_size: "50MB"
    example_cache_size: "100MB"
    object_cache_size: "200MB"

    # Cache policies
    max_cache_age: "1h"
    cache_hit_ratio_target: 0.8  # Target 80% hit ratio
```

## 🔧 Middleware Optimization

### Selective Middleware

#### Minimal Middleware Stack
```yaml
middleware:
  # Essential middleware only
  request_id: true             # Keep for tracing
  recovery:
    enabled: true              # Essential for stability
    stack_trace: false         # Disable for performance

  # Disable expensive middleware
  cors:
    enabled: false             # Disable if not needed
  timeout:
    enabled: false             # Disable if using load balancer timeouts

  # Optimize logging
  logging:
    enabled: true
    level: "warn"              # Reduce log volume
    skip_paths: ["/__health"]  # Skip health check logs
```

#### Optimized Middleware Configuration
```yaml
middleware:
  # Optimized settings for enabled middleware
  cors:
    enabled: true
    preflight_cache_duration: "1h"  # Cache preflight responses
    allow_origins: ["*"]             # Avoid origin checking overhead

  timeout:
    enabled: true
    duration: "5s"               # Shorter timeout for faster failure
    skip_paths: ["/__health"]    # Skip for health checks

  request_id:
    enabled: true
    header_name: "X-Request-ID"
    generator: "uuid_short"      # Use shorter UUIDs
```

### Plugin Optimization

#### High-Performance Plugin Configuration
```yaml
plugins:
  # Rate limiting with efficient algorithms
  - name: rate_limit
    enabled: true
    config:
      algorithm: "token_bucket"    # More efficient than sliding window
      ip_requests_per_second: 1000.0
      ip_burst: 2000
      key_requests_per_second: 5000.0
      key_burst: 10000
      cleanup_interval: "5m"      # Less frequent cleanup
      store_type: "memory"        # Fastest storage

  # Authentication with caching
  - name: auth
    enabled: true
    config:
      cache_tokens: true          # Cache validated tokens
      cache_duration: "5m"        # Cache for 5 minutes
      skip_expensive_validation: true  # Skip unnecessary checks

  # Disable expensive plugins in production
  - name: logging
    enabled: false                # Use external logging instead
  - name: debug
    enabled: false                # Disable debug features
```

## 💾 Memory Optimization

### Go Runtime Tuning

#### Garbage Collector Settings
```bash
# Environment variables for production
export GOGC=100                    # Default GC target
export GOMEMLIMIT=3GiB            # Set memory limit
export GODEBUG=madvdontneed=1      # Return memory to OS faster

# For high-throughput scenarios
export GOGC=200                    # Less frequent GC
export GOMAXPROCS=8                # Match container CPU limit

# For memory-constrained environments
export GOGC=50                     # More aggressive GC
export GOMEMLIMIT=1GiB            # Strict memory limit
```

#### Memory-Optimized Configuration
```yaml
# Configuration to reduce memory usage
server:
  reduce_memory_usage: true        # Enable FastHTTP memory optimization
  read_buffer_size: 2048          # Smaller buffers
  write_buffer_size: 2048
  max_request_body_size: "100KB"  # Smaller request limits

mock:
  max_depth: 2                    # Reduce object complexity
  default_array_size: 3           # Smaller arrays
  max_array_size: 20
  cache_generated_data: false     # Disable caching to save memory

recording:
  enabled: false                  # Disable to save memory

state:
  enabled: false                  # Disable state management
```

### Memory Profiling and Monitoring

#### Built-in Profiling
```yaml
# Enable pprof endpoints for memory analysis
server:
  enable_pprof: true
  pprof_port: 6060
```

#### Memory Analysis Commands
```bash
# Heap profile
curl http://vanta:6060/debug/pprof/heap > heap.prof
go tool pprof heap.prof

# Memory allocation profile
curl "http://vanta:6060/debug/pprof/allocs?seconds=30" > allocs.prof
go tool pprof allocs.prof

# Live memory analysis
go tool pprof http://vanta:6060/debug/pprof/heap
```

## 🏗️ Infrastructure Optimization

### Kubernetes Resource Configuration

#### CPU and Memory Limits
```yaml
# Deployment resource configuration
resources:
  requests:
    cpu: "1"           # Guaranteed CPU
    memory: "2Gi"      # Guaranteed memory
  limits:
    cpu: "4"           # Maximum CPU (allow bursting)
    memory: "4Gi"      # Hard memory limit

# Quality of Service: Burstable
# This allows CPU bursting while guaranteeing memory
```

#### Node Affinity and Scheduling
```yaml
# Optimize pod placement
affinity:
  nodeAffinity:
    requiredDuringSchedulingIgnoredDuringExecution:
      nodeSelectorTerms:
      - matchExpressions:
        - key: node-type
          operator: In
          values: ["high-performance"]  # Use high-performance nodes

  podAntiAffinity:
    preferredDuringSchedulingIgnoredDuringExecution:
    - weight: 100
      podAffinityTerm:
        labelSelector:
          matchExpressions:
          - key: app
            operator: In
            values: ["vanta"]
        topologyKey: kubernetes.io/hostname  # Spread across nodes

tolerations:
- key: "high-performance"
  operator: "Equal"
  value: "true"
  effect: "NoSchedule"
```

### Container Optimization

#### Multi-stage Docker Build
```dockerfile
# Optimized Dockerfile for performance
FROM golang:1.21-alpine AS builder

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s" \
    -a -installsuffix cgo \
    -o vanta ./cmd/mocker

FROM gcr.io/distroless/static:nonroot
COPY --from=builder /build/vanta /usr/local/bin/
COPY --from=builder /build/examples /app/examples

USER nonroot:nonroot
EXPOSE 8080 9090

ENTRYPOINT ["/usr/local/bin/vanta"]
```

#### Container Runtime Settings
```yaml
# Pod specification with optimized settings
spec:
  containers:
  - name: vanta
    image: vanta:optimized

    # Security context
    securityContext:
      allowPrivilegeEscalation: false
      runAsNonRoot: true
      runAsUser: 65532
      capabilities:
        drop: ["ALL"]

    # Environment optimization
    env:
    - name: GOGC
      value: "100"
    - name: GOMAXPROCS
      value: "4"
    - name: GOMEMLIMIT
      value: "3GiB"

    # Volume mounts optimized for performance
    volumeMounts:
    - name: config
      mountPath: /app/config
      readOnly: true
    - name: tmp
      mountPath: /tmp

  volumes:
  - name: tmp
    emptyDir:
      medium: Memory  # Use memory for temporary files
      sizeLimit: 100Mi
```

## 📊 Performance Testing and Benchmarking

### Load Testing Setup

#### Artillery Configuration
```yaml
# artillery-config.yml
config:
  target: http://vanta:8080
  phases:
    - duration: 60
      arrivalRate: 100        # 100 requests/second
    - duration: 120
      arrivalRate: 500        # Ramp to 500 requests/second
    - duration: 180
      arrivalRate: 1000       # Peak at 1000 requests/second
    - duration: 60
      arrivalRate: 100        # Cool down

scenarios:
  - name: "Mixed API Load"
    weight: 60
    flow:
      - get:
          url: "/api/users"
      - get:
          url: "/api/users/{{ $randomInt(1, 1000) }}"
      - post:
          url: "/api/users"
          json:
            name: "{{ $randomString() }}"
            email: "{{ $randomString() }}@example.com"

  - name: "Health Check"
    weight: 10
    flow:
      - get:
          url: "/__health"

  - name: "Complex Objects"
    weight: 30
    flow:
      - get:
          url: "/api/complex-object"
      - post:
          url: "/api/complex-object"
          json:
            nested:
              deep:
                array: [1, 2, 3, 4, 5]
```

#### K6 Performance Test
```javascript
// k6-performance-test.js
import http from 'k6/http';
import { check, sleep } from 'k6';

export let options = {
  stages: [
    { duration: '2m', target: 100 },   // Ramp up
    { duration: '5m', target: 500 },   // Stay at 500 RPS
    { duration: '10m', target: 1000 }, // Peak load
    { duration: '3m', target: 0 },     // Cool down
  ],
  thresholds: {
    http_req_duration: ['p(95)<100'],   // 95% < 100ms
    http_req_failed: ['rate<0.01'],     // Error rate < 1%
  },
};

export default function() {
  // Test different endpoints
  let responses = http.batch([
    ['GET', 'http://vanta:8080/api/users'],
    ['GET', 'http://vanta:8080/api/orders'],
    ['POST', 'http://vanta:8080/api/users', JSON.stringify({
      name: 'Test User',
      email: 'test@example.com'
    }), { headers: { 'Content-Type': 'application/json' } }],
  ]);

  // Verify responses
  for (let response of responses) {
    check(response, {
      'status is 200': (r) => r.status === 200,
      'response time < 100ms': (r) => r.timings.duration < 100,
    });
  }

  sleep(1);
}
```

### Benchmark Analysis

#### Performance Monitoring During Tests
```bash
# Monitor during load tests
# CPU and Memory
kubectl top pod -l app=vanta --containers

# Custom metrics
curl http://vanta:9090/metrics | grep -E "(http_requests_total|http_request_duration)"

# Real-time monitoring
watch -n 1 'curl -s http://vanta:9090/metrics | grep -E "(http_requests_total|http_request_duration_seconds_bucket)"'
```

#### Performance Baseline
```yaml
# Expected performance benchmarks
benchmarks:
  low_load:          # 100 RPS
    p50_latency: "< 5ms"
    p95_latency: "< 20ms"
    p99_latency: "< 50ms"
    memory_usage: "< 500MB"
    cpu_usage: "< 20%"

  medium_load:       # 500 RPS
    p50_latency: "< 10ms"
    p95_latency: "< 50ms"
    p99_latency: "< 100ms"
    memory_usage: "< 1GB"
    cpu_usage: "< 50%"

  high_load:         # 1000 RPS
    p50_latency: "< 20ms"
    p95_latency: "< 100ms"
    p99_latency: "< 200ms"
    memory_usage: "< 2GB"
    cpu_usage: "< 80%"
```

## 🔍 Performance Monitoring

### Key Performance Indicators (KPIs)

#### Application Metrics
```promql
# Throughput
rate(vanta_http_requests_total[5m])

# Latency percentiles
histogram_quantile(0.95, rate(vanta_http_request_duration_seconds_bucket[5m]))
histogram_quantile(0.99, rate(vanta_http_request_duration_seconds_bucket[5m]))

# Error rate
rate(vanta_http_requests_total{status=~"5.."}[5m]) / rate(vanta_http_requests_total[5m])

# Concurrent connections
vanta_server_active_connections

# Data generation rate
rate(vanta_mock_responses_generated_total[5m])
```

#### System Metrics
```promql
# CPU utilization
rate(process_cpu_seconds_total{job="vanta"}[5m]) * 100

# Memory usage
process_resident_memory_bytes{job="vanta"} / 1024 / 1024

# Garbage collection
rate(go_gc_duration_seconds_sum{job="vanta"}[5m])

# Goroutines
go_goroutines{job="vanta"}
```

### Performance Alerts
```yaml
# High-performance alerting rules
groups:
- name: vanta.performance
  rules:
  - alert: VantaHighLatency
    expr: |
      histogram_quantile(0.95,
        rate(vanta_http_request_duration_seconds_bucket[5m])
      ) > 0.1
    for: 2m
    labels:
      severity: warning
    annotations:
      summary: "Vanta 95th percentile latency high"

  - alert: VantaLowThroughput
    expr: rate(vanta_http_requests_total[5m]) < 100
    for: 5m
    labels:
      severity: warning
    annotations:
      summary: "Vanta throughput below expected"

  - alert: VantaHighMemoryUsage
    expr: |
      process_resident_memory_bytes{job="vanta"} / 1024 / 1024 > 3072
    for: 3m
    labels:
      severity: critical
    annotations:
      summary: "Vanta memory usage exceeding limits"
```

## 🎯 Performance Optimization Checklist

### Pre-Production Optimization
- [ ] Server configuration tuned for expected load
- [ ] Middleware stack minimized to essential components
- [ ] Mock generation settings optimized for performance
- [ ] Caching enabled and configured appropriately
- [ ] Resource limits set based on load testing
- [ ] Container and runtime optimizations applied

### Production Monitoring
- [ ] Performance metrics collection enabled
- [ ] Alerting configured for key performance indicators
- [ ] Load testing pipeline established
- [ ] Performance regression detection implemented
- [ ] Capacity planning based on growth projections

### Continuous Optimization
- [ ] Regular performance reviews scheduled
- [ ] Benchmark comparisons with previous versions
- [ ] Resource utilization analysis
- [ ] Optimization opportunities identified
- [ ] Performance improvements prioritized and implemented

This comprehensive performance tuning guide ensures Vanta operates efficiently under various load conditions while maintaining high availability and responsiveness.