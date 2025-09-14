# Chaos Guide

Inject faults to test client resilience under latency and error conditions.

## Enable chaos
```yaml
chaos:
  enabled: true
  scenarios:
    - name: "api_latency"
      type: "latency"
      endpoints: ["/api/*", "/v1/*"]
      probability: 0.1
      parameters:
        min_delay: "10ms"
        max_delay: "500ms"
    - name: "service_errors"
      type: "error"
      endpoints: ["/api/payments/*", "/api/orders/*"]
      probability: 0.05
      parameters:
        error_codes: [500, 502, 503]
        custom_body: '{"error":"Service temporarily unavailable"}'
```

Alternatively, use the ready example: `examples/chaos-config.yaml`.

## CLI
```
vanta chaos start --config examples/chaos-config.yaml
vanta chaos status --config examples/chaos-config.yaml
vanta chaos stop
vanta chaos list --config examples/chaos-config.yaml
```

## How it works
- Endpoint patterns (supports `*`) select targets.
- Probability controls how often a scenario applies per matching request.
- Latency injector delays responses; error injector returns HTTP errors directly.

## Tips
- Place Chaos after Timeout in the middleware order (default) to observe realistic timeouts.
- Keep probabilities modest to avoid overloading client retries.

