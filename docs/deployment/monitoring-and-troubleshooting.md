# Monitoring and Troubleshooting Guide

This guide provides comprehensive guidance for monitoring Vanta in production and troubleshooting common issues.

## 📊 Monitoring Setup

### Metrics Collection

#### Prometheus Configuration

##### Vanta Configuration
```yaml
# config/production.yaml
metrics:
  enabled: true
  port: 9090
  path: "/metrics"
  prometheus:
    enabled: true
    namespace: "vanta"
    subsystem: "mock_server"
    buckets: [0.001, 0.01, 0.1, 0.5, 1.0, 2.5, 5.0, 10.0]
    gather_go_metrics: true
    gather_process_metrics: true
```

##### Prometheus scrape_config
```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'vanta'
    static_configs:
      - targets: ['vanta:9090']
    scrape_interval: 15s
    scrape_timeout: 10s
    metrics_path: /metrics
    scheme: http
```

##### Kubernetes ServiceMonitor
```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: vanta-metrics
  labels:
    app: vanta
spec:
  selector:
    matchLabels:
      app: vanta
  endpoints:
  - port: metrics
    interval: 15s
    path: /metrics
```

#### Key Metrics to Monitor

##### HTTP Metrics
```promql
# Request rate
rate(vanta_http_requests_total[5m])

# Request duration (p99, p95, p50)
histogram_quantile(0.99, rate(vanta_http_request_duration_seconds_bucket[5m]))
histogram_quantile(0.95, rate(vanta_http_request_duration_seconds_bucket[5m]))
histogram_quantile(0.50, rate(vanta_http_request_duration_seconds_bucket[5m]))

# Error rate
rate(vanta_http_requests_total{status=~"5.."}[5m]) / rate(vanta_http_requests_total[5m])

# Request size
histogram_quantile(0.95, rate(vanta_http_request_size_bytes_bucket[5m]))

# Response size
histogram_quantile(0.95, rate(vanta_http_response_size_bytes_bucket[5m]))
```

##### Server Metrics
```promql
# Active connections
vanta_server_active_connections

# Memory usage
process_resident_memory_bytes{job="vanta"}

# CPU usage
rate(process_cpu_seconds_total{job="vanta"}[5m])

# Go garbage collection
rate(go_gc_duration_seconds_sum{job="vanta"}[5m])

# Goroutines
go_goroutines{job="vanta"}
```

##### Business Metrics
```promql
# Mock responses generated
rate(vanta_mock_responses_generated_total[5m])

# Chaos scenarios triggered
rate(vanta_chaos_scenarios_triggered_total[5m])

# Recordings captured
rate(vanta_recordings_captured_total[5m])

# Validation failures
rate(vanta_validation_failures_total[5m])
```

### Grafana Dashboards

#### Main Dashboard Configuration
```json
{
  "dashboard": {
    "title": "Vanta Mock Server",
    "panels": [
      {
        "title": "Request Rate",
        "type": "graph",
        "targets": [
          {
            "expr": "rate(vanta_http_requests_total[5m])",
            "legendFormat": "{{method}} {{endpoint}}"
          }
        ]
      },
      {
        "title": "Response Time",
        "type": "graph",
        "targets": [
          {
            "expr": "histogram_quantile(0.95, rate(vanta_http_request_duration_seconds_bucket[5m]))",
            "legendFormat": "p95"
          },
          {
            "expr": "histogram_quantile(0.99, rate(vanta_http_request_duration_seconds_bucket[5m]))",
            "legendFormat": "p99"
          }
        ]
      },
      {
        "title": "Error Rate",
        "type": "singlestat",
        "targets": [
          {
            "expr": "rate(vanta_http_requests_total{status=~\"5..\"}[5m]) / rate(vanta_http_requests_total[5m]) * 100"
          }
        ]
      }
    ]
  }
}
```

#### Dashboard Panels

##### Request Volume Panel
```json
{
  "title": "Request Volume by Endpoint",
  "type": "graph",
  "targets": [
    {
      "expr": "topk(10, rate(vanta_http_requests_total[5m]))",
      "legendFormat": "{{method}} {{endpoint}}"
    }
  ],
  "yAxes": [
    {
      "label": "Requests/sec",
      "min": 0
    }
  ]
}
```

##### System Resources Panel
```json
{
  "title": "System Resources",
  "type": "graph",
  "targets": [
    {
      "expr": "process_resident_memory_bytes{job=\"vanta\"} / 1024 / 1024",
      "legendFormat": "Memory (MB)"
    },
    {
      "expr": "rate(process_cpu_seconds_total{job=\"vanta\"}[5m]) * 100",
      "legendFormat": "CPU %"
    }
  ]
}
```

### Alerting Rules

#### Prometheus Alerting Rules
```yaml
# alerts.yml
groups:
- name: vanta.rules
  rules:
  - alert: VantaHighErrorRate
    expr: |
      (
        rate(vanta_http_requests_total{status=~"5.."}[5m]) /
        rate(vanta_http_requests_total[5m])
      ) * 100 > 5
    for: 2m
    labels:
      severity: warning
      service: vanta
    annotations:
      summary: "Vanta error rate is above 5%"
      description: "Error rate is {{ $value }}% for the last 5 minutes"

  - alert: VantaHighLatency
    expr: |
      histogram_quantile(0.95,
        rate(vanta_http_request_duration_seconds_bucket[5m])
      ) > 2
    for: 5m
    labels:
      severity: warning
      service: vanta
    annotations:
      summary: "Vanta 95th percentile latency is high"
      description: "95th percentile latency is {{ $value }}s"

  - alert: VantaServiceDown
    expr: up{job="vanta"} == 0
    for: 1m
    labels:
      severity: critical
      service: vanta
    annotations:
      summary: "Vanta service is down"
      description: "Vanta service has been down for more than 1 minute"

  - alert: VantaHighMemoryUsage
    expr: |
      process_resident_memory_bytes{job="vanta"} / 1024 / 1024 > 2048
    for: 5m
    labels:
      severity: warning
      service: vanta
    annotations:
      summary: "Vanta memory usage is high"
      description: "Memory usage is {{ $value }}MB"

  - alert: VantaHighCPUUsage
    expr: |
      rate(process_cpu_seconds_total{job="vanta"}[5m]) * 100 > 80
    for: 5m
    labels:
      severity: warning
      service: vanta
    annotations:
      summary: "Vanta CPU usage is high"
      description: "CPU usage is {{ $value }}%"
```

## 📝 Logging

### Structured Logging Configuration

#### Production Logging Setup
```yaml
# config/production.yaml
logging:
  level: "info"
  format: "json"
  output: "stdout"
  development: false
  sampling:
    initial: 100
    thereafter: 100
  stack_trace_level: "error"
  caller: true
  encoding_config:
    time_key: "timestamp"
    level_key: "level"
    name_key: "logger"
    caller_key: "caller"
    message_key: "msg"
    stack_trace_key: "stacktrace"
```

#### Log Aggregation with Fluentd
```yaml
# fluentd-config.yaml
<source>
  @type kubernetes_logs
  path /var/log/containers/vanta-*.log
  pos_file /var/log/fluentd-vanta.log.pos
  tag kubernetes.vanta
  format json
  read_from_head true
</source>

<filter kubernetes.vanta>
  @type parser
  key_name log
  format json
  reserve_data true
</filter>

<match kubernetes.vanta>
  @type elasticsearch
  host elasticsearch.logging.svc.cluster.local
  port 9200
  index_name vanta-logs
  type_name _doc
</match>
```

### Log Analysis

#### Key Log Fields
```json
{
  "timestamp": "2023-10-15T10:30:00Z",
  "level": "info",
  "msg": "HTTP request processed",
  "method": "GET",
  "path": "/api/users",
  "status": 200,
  "duration": "15ms",
  "request_id": "req-123456789",
  "user_agent": "curl/7.68.0",
  "remote_addr": "192.168.1.100",
  "request_size": 0,
  "response_size": 1024
}
```

#### Log Queries

##### Elasticsearch/Kibana Queries
```
# High latency requests
duration:>1000 AND level:info

# Error responses
status:[400 TO 599] AND level:error

# Specific endpoint analysis
path:"/api/users" AND method:"GET"

# Time range analysis
timestamp:[now-1h TO now] AND level:error
```

##### Grafana Loki Queries
```
# Error logs
{job="vanta"} |= "error"

# High latency requests
{job="vanta"} | json | duration > 1000

# Specific endpoint logs
{job="vanta"} | json | path="/api/users"
```

## 🔍 Troubleshooting Guide

### Common Issues and Solutions

#### High Memory Usage

**Symptoms:**
- Memory usage continuously increasing
- Out of memory errors
- Pod restarts due to memory limits

**Diagnosis:**
```bash
# Check memory metrics
curl http://vanta:9090/metrics | grep process_resident_memory_bytes

# Check Go memory stats
curl http://vanta:9090/metrics | grep go_memstats

# Kubernetes pod memory
kubectl top pod -l app=vanta
```

**Solutions:**
1. **Increase memory limits**:
```yaml
resources:
  limits:
    memory: "4Gi"  # Increase from current limit
```

2. **Optimize configuration**:
```yaml
mock:
  max_depth: 3  # Reduce nested object depth
  default_array_size: 5  # Reduce array sizes
  cache_generated_data: false  # Disable caching if not needed
```

3. **Enable garbage collection tuning**:
```bash
export GOGC=50  # More aggressive GC
```

#### High CPU Usage

**Symptoms:**
- CPU metrics showing >80% utilization
- Request latency increasing
- Throttling in Kubernetes

**Diagnosis:**
```bash
# Check CPU metrics
curl http://vanta:9090/metrics | grep process_cpu_seconds_total

# Profile CPU usage
curl http://vanta:8080/debug/pprof/profile?seconds=30 > cpu.prof
```

**Solutions:**
1. **Increase CPU limits**:
```yaml
resources:
  limits:
    cpu: "2"  # Increase CPU allocation
```

2. **Optimize data generation**:
```yaml
mock:
  prefer_examples: true  # Use examples instead of generation
  cache_generated_data: true  # Cache frequently used data
```

3. **Reduce middleware overhead**:
```yaml
middleware:
  request_id: false  # Disable if not needed
  cors:
    enabled: false   # Disable if not needed
```

#### High Latency

**Symptoms:**
- p95/p99 latency increasing
- Timeout errors
- Slow response times

**Diagnosis:**
```bash
# Check latency metrics
curl http://vanta:9090/metrics | grep http_request_duration_seconds

# Check for slow endpoints
curl http://vanta:9090/metrics | grep http_request_duration_seconds_bucket
```

**Solutions:**
1. **Optimize server configuration**:
```yaml
server:
  read_timeout: "10s"    # Reduce timeout
  write_timeout: "10s"   # Reduce timeout
  concurrency: 20000     # Increase concurrency
```

2. **Enable caching**:
```yaml
mock:
  cache_generated_data: true
  max_cache_size: "100MB"
```

3. **Review chaos configuration**:
```yaml
chaos:
  enabled: false  # Temporarily disable for testing
```

#### Connection Issues

**Symptoms:**
- Connection refused errors
- Max connections reached
- Intermittent connectivity

**Diagnosis:**
```bash
# Check active connections
curl http://vanta:9090/metrics | grep vanta_server_active_connections

# Check connection limits
netstat -an | grep :8080 | wc -l
```

**Solutions:**
1. **Increase connection limits**:
```yaml
server:
  max_conns_per_ip: 2000  # Increase from default
  concurrency: 20000      # Increase overall concurrency
```

2. **Load balancer configuration**:
```yaml
# HAProxy
timeout connect 5s
timeout client 30s
timeout server 30s
```

#### OpenAPI Parsing Errors

**Symptoms:**
- Startup failures
- Invalid spec errors
- Missing endpoints

**Diagnosis:**
```bash
# Check logs for parsing errors
kubectl logs deployment/vanta | grep -i "openapi\|spec\|parsing"

# Validate OpenAPI spec
vanta validation start --spec your-spec.yaml
```

**Solutions:**
1. **Validate spec externally**:
```bash
# Use Swagger tools
swagger-codegen validate -i your-spec.yaml

# Use online validators
curl -X POST "https://validator.swagger.io/validator/debug" \
  -H "Content-Type: application/json" \
  -d @your-spec.yaml
```

2. **Check spec accessibility**:
```bash
# Ensure spec is readable
kubectl exec deployment/vanta -- cat /app/specs/api.yaml
```

#### Recording Issues

**Symptoms:**
- Recordings not saving
- Storage full errors
- Missing request data

**Diagnosis:**
```bash
# Check recording metrics
curl http://vanta:9090/metrics | grep recording

# Check storage usage
df -h /recordings
```

**Solutions:**
1. **Increase storage limits**:
```yaml
recording:
  max_recordings: 10000      # Increase limit
  max_body_size: 10485760    # 10MB limit
```

2. **Configure cleanup**:
```yaml
recording:
  storage:
    retention: "7d"  # Automatic cleanup after 7 days
```

### Debugging Tools

#### Built-in Debugging

##### TUI Interface
```bash
# Launch TUI for live debugging
vanta tui --config production.yaml --spec api.yaml

# Navigation:
# Tab: Switch between panels
# q: Quit
# Enter: View details
# /: Search logs
```

##### Health Check Endpoint
```bash
# Basic health check
curl http://vanta:8080/__health

# Detailed info
curl http://vanta:8080/__info
```

##### Debug Endpoints
```bash
# Runtime profiling (if enabled)
curl http://vanta:8080/debug/pprof/
curl http://vanta:8080/debug/pprof/heap
curl http://vanta:8080/debug/pprof/goroutine
```

#### External Tools

##### Load Testing
```bash
# Apache Bench
ab -n 1000 -c 10 http://vanta:8080/api/users

# wrk
wrk -t4 -c100 -d30s http://vanta:8080/api/users

# Artillery
artillery quick --count 100 --num 10 http://vanta:8080/api/users
```

##### Network Debugging
```bash
# Check connectivity
telnet vanta 8080

# Trace network path
traceroute vanta

# DNS resolution
nslookup vanta
```

### Performance Analysis

#### Resource Monitoring
```bash
# Kubernetes resource usage
kubectl top pod -l app=vanta
kubectl describe pod -l app=vanta

# Container stats
docker stats vanta

# System resources
htop
iostat 1
```

#### Application Profiling
```bash
# CPU profiling
go tool pprof http://vanta:8080/debug/pprof/profile?seconds=30

# Memory profiling
go tool pprof http://vanta:8080/debug/pprof/heap

# Goroutine analysis
go tool pprof http://vanta:8080/debug/pprof/goroutine
```

## 📱 Incident Response

### Incident Response Playbook

#### 1. Detection
- Monitor alerts from Prometheus/Alertmanager
- Check Grafana dashboards for anomalies
- Review logs for error patterns

#### 2. Assessment
- Determine impact scope (users affected, services down)
- Check recent deployments or changes
- Verify infrastructure status

#### 3. Initial Response
- Scale up resources if needed
- Disable problematic features (chaos, complex plugins)
- Switch to backup configuration if available

#### 4. Investigation
- Collect logs and metrics
- Run diagnostic commands
- Check dependencies (databases, external APIs)

#### 5. Resolution
- Apply fixes based on diagnosis
- Monitor for improvement
- Communicate with stakeholders

#### 6. Post-Incident
- Conduct post-mortem
- Update documentation
- Improve monitoring/alerting

### Emergency Commands
```bash
# Quick scale up
kubectl scale deployment vanta --replicas=10

# Disable chaos temporarily
kubectl patch configmap vanta-config --patch '{"data":{"chaos.enabled":"false"}}'
kubectl rollout restart deployment vanta

# Emergency rollback
kubectl rollout undo deployment vanta

# Force restart all pods
kubectl delete pods -l app=vanta
```

This comprehensive monitoring and troubleshooting guide ensures you can effectively observe, debug, and maintain Vanta in production environments.