# Middleware & Metrics

Vanta composes a FastHTTP middleware stack around the mock handlers.

## Execution order
1) Request ID → 2) Plugins → 3) CORS → 4) Logger → 5) Recovery → 6) Timeout → 7) Chaos → 8) Metrics → 9) Recording

## Middlewares
- Request ID: Adds `X-Request-ID` header and exposes ID to downstream logic.
- Logger: Structured logs with method, path, status, duration, sizes, remote addr.
- CORS: Allow origins/methods/headers per config; handles preflight.
- Recovery: Panic safety with optional stack logging.
- Timeout: Enforces request timeouts and returns 408 on breach.
- Metrics: Counters and latency histograms per method/path/status.
- Recording: Captures request/response pairs for storage.
- Chaos: Injects latency or errors based on configured scenarios.

## Configuration
See docs/configuration.md → `middleware` and `metrics` sections.

## Metrics integration
- Internal collector exposes counters/latency; a Prometheus endpoint can be bound via config (`metrics.enabled: true`).
- Plugin metrics are adapted through a plugin metrics adapter and included in server stats.

