# Configuration Reference

Vanta is configured via a YAML file passed with `--config`. This document summarizes key sections and options. See `pkg/config/defaults.go` for defaults.

## Top-level sections
- `server`: network and runtime settings
- `mock`: data generation controls
- `middleware`: CORS, timeout, recovery, request ID
- `metrics`: HTTP metrics collection
- `chaos`: latency/error injection scenarios
- `recording`: traffic recording settings
- `plugins`: enable and configure plugins
- `validation`: request/response validation
- `hotreload`: watch config/spec for changes
- `state`: session/request context and persistence

## server
```yaml
server:
  port: 8080
  host: "0.0.0.0"
  read_timeout: 30s
  write_timeout: 30s
  max_conns_per_ip: 100
  concurrency: 256000
```

## mock
```yaml
mock:
  seed: 12345               # 0 = random per run
  locale: "en"
  max_depth: 5
  default_array_size: 2
  prefer_examples: true
```

## middleware
```yaml
middleware:
  request_id: true
  cors:
    enabled: true
    allow_origins: ["*"]
    allow_methods: ["GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"]
    allow_headers: ["Content-Type", "Authorization", "X-Request-ID"]
    allow_credentials: false
    max_age: 3600
  timeout:
    enabled: false
    duration: 30s
  recovery:
    enabled: true
    print_stack: false
    log_stack: true
```

## metrics
```yaml
metrics:
  enabled: true
  port: 9090
  path: "/metrics"
  prometheus: true
```

## chaos
```yaml
chaos:
  enabled: true
  scenarios:
    - name: "api_latency"
      type: "latency"            # latency|error
      endpoints: ["/api/*"]
      probability: 0.1            # 0..1
      parameters:
        min_delay: "50ms"
        max_delay: "500ms"
```

## recording
```yaml
recording:
  enabled: true
  storage:
    type: "file"                 # file|memory
    directory: "./recordings"
    format: "jsonlines"          # json|jsonlines
  max_recordings: 1000
  max_body_size: 1048576
  include_headers: ["content-type", "user-agent", "x-request-id"]
  exclude_headers: ["cookie", "set-cookie", "x-forwarded-for"]
  filters:
    - type: "method"             # method|endpoint|status
      values: ["GET", "POST"]
      negate: false
```

## plugins
```yaml
plugins:
  - name: auth
    enabled: true
    config:
      jwt_secret: "..."
      auth_header: "Authorization"
      public_endpoints: ["/__health", "/__info"]
```

## validation
```yaml
validation:
  enabled: true
  strict_mode: false
  fail_on_invalid: false
  validate_headers: true
  validate_query: true
  validate_path: true
  validate_body: true
  validate_status_codes: true
  allow_extra_fields: true
  validate_formats: true
  coverage_reporting: true
  report_format: ["json", "html"]
  report_path: "./validation-reports"
  report_interval: 5m
  max_concurrent_validations: 100
  validation_timeout: 30s
```

## hotreload
```yaml
hotreload:
  enabled: false
  watch_config: true
  watch_spec: true
  debounce_delay: 500ms
```

## state
```yaml
state:
  enabled: false
  cleanup_interval: 5m
  default_ttl: 0
  storage:
    type: "memory"               # memory|file
    file_path: "./state.json"
    options: {}
  context:
    default_ttl: 30m
    session_ttl: 24h
    request_ttl: 5m
    cleanup_interval: 5m
  endpoints: {}
```

