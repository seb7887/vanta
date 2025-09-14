# Configuration Reference

Vanta is configured via a YAML file passed with `--config`. This document summarizes every section and option with defaults, valid values, and behavior. See `pkg/config/defaults.go` and `pkg/config/loader.go` for the authoritative defaults.

Notes on formats and overrides:
- Durations: use Go-style (e.g., `500ms`, `30s`, `5m`, `24h`).
- Sizes: strings like `10MB`, `1GB`, `1024` (bytes). Valid units: `B`, `KB`, `MB`, `GB`, `TB`.
- Environment overrides: any key can be overridden with env vars using prefix `VANTA_` and dots converted to underscores. Example: `server.port` → `VANTA_SERVER_PORT`. See `pkg/config/loader.go`.

## Top-level sections
- `server`: network and runtime settings
- `logging`: application log formatting/output
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
  max_request_size: 10MB
  concurrency: 256000
  reuse_port: true
```

Options
- `port`: TCP port to bind. Default: `8080`. Range: 1–65535.
- `host`: Bind address or hostname. Default: `"0.0.0.0"`.
- `read_timeout`: Maximum time to read the entire request. Default: `30s`.
- `write_timeout`: Maximum time to write the response. Default: `30s`.
- `max_conns_per_ip`: Soft cap of concurrent connections per client IP. Default: `100`.
- `max_request_size`: Max request body size before rejecting with 413. Default: `"10MB"`. Accepts size units.
- `concurrency`: Max in-flight requests allowed globally. Must be > 0. Default: `256000`.
- `reuse_port`: Enable `SO_REUSEPORT` where supported to improve multi-process distribution. Default: `true`.

## mock
```yaml
mock:
  seed: 12345               # 0 = random per run
  locale: "en"
  max_depth: 5
  default_array_size: 2
  prefer_examples: true
```

Options
- `seed`: Random seed for deterministic data. `0` uses current timestamp. Default: `0`.
- `locale`: Locale for faker/generator (e.g., `en`, `es`, `fr`). Default: `"en"`.
- `max_depth`: Limit for nested object generation to avoid recursion loops. Default: `5`.
- `default_array_size`: Fallback array length when schema doesn’t constrain it. Default: `2`.
- `prefer_examples`: Prefer OpenAPI `example`/`examples` over generated data. Default: `true`.

Tip: combine `seed` + `prefer_examples: true` for stable snapshots.

## logging
```yaml
logging:
  level: "info"       # debug|info|warn|error|dpanic|panic|fatal
  format: "json"      # json|console
  output: "stdout"    # stdout|stderr|/path/to/file
  sampling: false
  add_caller: true
```

Options
- `level`: Log verbosity. Default: `info`. Valid: `debug`, `info`, `warn`, `error`, `dpanic`, `panic`, `fatal`.
- `format`: Encoder format. Default: `json`. Valid: `json`, `console`.
- `output`: Destination. Default: `stdout`. Use a file path to write logs to disk.
- `sampling`: Enable log sampling to reduce volume at high QPS. Default: `false`.
- `add_caller`: Include caller file:line for easier tracing. Default: `true`.

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

Options
- `request_id`: Adds/propagates `X-Request-ID` for traceability. Default: `true`.
- `cors.enabled`: Enable CORS middleware. Default: `false`.
- `cors.allow_origins`: Origins allowed. Default: `["*"]` (use specific origins in production).
- `cors.allow_methods`: Allowed HTTP methods. Reasonable defaults provided.
- `cors.allow_headers`: Allowed request headers. Default includes `Content-Type`, `Authorization`, `X-Request-ID`.
- `cors.allow_credentials`: Allow cookies/credentials. Default: `false`.
- `cors.max_age`: Max seconds to cache preflight results. Default: `3600`.
- `timeout.enabled`: Apply per-request timeout. Default: `false`.
- `timeout.duration`: Deadline for handler completion; exceeded requests return 504. Default: `30s`.
- `recovery.enabled`: Catch panics and return 500. Default: `true`.
- `recovery.print_stack`: Print stack to stdout. Default: `false`.
- `recovery.log_stack`: Log stack with logger. Default: `true`.

## metrics
```yaml
metrics:
  enabled: true
  port: 9090
  path: "/metrics"
  prometheus: true
```

Options
- `enabled`: Expose metrics endpoint. Default: `true`.
- `port`: Metrics server port. Default: `9090`. Range: 1–65535.
- `path`: HTTP path for metrics. Must start with `/`. Default: `/metrics`.
- `prometheus`: Emit Prometheus format. Default: `true`.

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

Options
- `enabled`: Turn chaos injection on. Default: `false`.
- `scenarios[]`: List of independent injections. Fields:
  - `name`: Identifier for the scenario. Required.
  - `type`: One of `latency`, `error`, `timeout`. Required.
  - `endpoints`: Glob patterns for paths (e.g., `/api/*`, `/v1/orders/**`).
  - `probability`: Injection probability per request in [0,1].
  - `parameters`: Type-specific settings:
    - `latency`: `min_delay`, `max_delay` (durations)
    - `error`: `status` (e.g., 500), `body` (string or object)
    - `timeout`: `duration` (duration before simulating timeout)

Validation rules: `type` must be valid; `probability` must be 0–1.

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

Options
- `enabled`: Enable recording of traffic. Default: `false`.
- `storage.type`: `file` or `memory`. Default: `file`.
- `storage.directory`: Directory for `file` storage. Default: `./recordings`.
- `storage.format`: `json` or `jsonlines`. Default: `jsonlines`.
- `max_recordings`: In-memory max retained entries (applies to in-process index). Default: `1000`.
- `max_body_size`: Max request/response bytes to store per entry. Default: `1048576` (1MB).
- `include_headers`: If empty, include all headers except those in `exclude_headers`.
- `exclude_headers`: Always omit these headers when persisting (defaults include sensitive headers like `cookie`, `authorization`, `x-api-key`).
- `filters[]`: Only include or exclude traffic matching rules.
  - `type`: `method` | `endpoint` | `status`.
  - `values`: Match set (e.g., `GET`, `/api/*`, `2xx`, `201`).
  - `negate`: If `true`, exclude matches instead of include.

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

Options
- Array of plugin entries. Each entry:
  - `name`: Plugin identifier (see built-ins in `docs/builtin-plugins.md`).
  - `enabled`: Toggle plugin.
  - `config`: Arbitrary plugin-specific settings (schema depends on the plugin).

Note: There is also a built-in logging plugin for request/response logging. Do not confuse it with the top-level `logging` section which controls application logs.

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

Options
- `enabled`: Enable runtime validation. Default: `true`.
- `strict_mode`: Disallow unknown fields even if schema is permissive. Default: `false`.
- `fail_on_invalid`: If `true`, return an error to the client when validation fails; otherwise only report. Default: `false`.
- `validate_headers|query|path|body|status_codes`: Enable per-part validation. Default: all `true`.
- `allow_extra_fields`: Permit properties not declared in schema. Default: `true`.
- `validate_formats`: Enforce known formats (e.g., `email`, `uuid`). Default: `true`.
- `coverage_reporting`: Track which schema parts are exercised. Default: `true`.
- `report_format`: Output formats for coverage reports. Default: `["json", "html"]`.
- `report_path`: Directory for reports. Default: `./validation-reports`.
- `report_interval`: How often to flush reports. Default: `5m`.
- `max_concurrent_validations`: Concurrency guard. Default: `100`.
- `validation_timeout`: Per-validation guard to avoid runaway checks. Default: `30s`.

## hotreload
```yaml
hotreload:
  enabled: false
  watch_config: true
  watch_spec: true
  debounce_delay: 500ms
```

Options
- `enabled`: Watch and hot-apply changes. Default: `false`.
- `watch_config`: Watch the `--config` file. Default: `true`.
- `watch_spec`: Watch the OpenAPI spec inputs. Default: `true`.
- `debounce_delay`: Debounce window to batch rapid changes. Default: `500ms`.

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

Options
- `enabled`: Enable stateful behavior across requests. Default: `false`.
- `cleanup_interval`: Background sweep cadence for expired items. Default: `5m`.
- `default_ttl`: Default TTL for unclassified state entries. `0` = no expiration. Default: `0`.
- `storage.type`: `memory` or `file`. Default: `memory`.
- `storage.file_path`: Path used when `type: file`. Default: `./state.json`.
- `storage.options`: Backend-specific tuning map.
- `context.default_ttl`: TTL for generic context. Default: `30m`.
- `context.session_ttl`: TTL for session-scoped context. Default: `24h`.
- `context.request_ttl`: TTL for request-scoped context. Default: `5m`.
- `context.cleanup_interval`: Separate cleaner cadence for context spaces. Default: `5m`.
- `endpoints`: Per-endpoint overrides, keyed by route pattern. Each entry supports:
  - `initial_state`: Seed state map for the endpoint.
  - `shared_context`: Name of a shared context bucket to join.
  - `ttl`: Override TTL for this endpoint’s state entries.
  - `persistent`: If `true`, skip TTL eviction.

Examples for `endpoints`:
```yaml
state:
  enabled: true
  endpoints:
    "/v1/orders/*":
      initial_state: { nextId: 1001 }
      shared_context: "orders"
      ttl: 2h
      persistent: false
```

