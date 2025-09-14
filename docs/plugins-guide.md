# Plugins Guide

Vanta includes a production‑ready plugin manager to extend functionality via middleware and processors.

## Concepts
- Plugin: base interface with `Init` and `Cleanup`.
- Middleware: pre/post processing around requests; has a `Priority` and `ShouldApply`.
- Request/Response Processor: transform or validate requests/responses.

## Enable plugins
```yaml
plugins:
  - name: auth
    enabled: true
    config:
      jwt_secret: "your-secret"
      auth_header: "Authorization"
      public_endpoints: ["/__health", "/__info"]
  - name: rate_limit
    enabled: true
    config:
      ip_requests_per_second: 100.0
      ip_burst: 200
```

## Built‑in plugins
See `docs/builtin-plugins.md` for detailed configuration and examples of:
- Auth (JWT/API key)
- Rate limit (sliding window)
- CORS (enhanced)
- Logging (structured, filtering)

### Quick config examples

Auth (JWT + API key):
```yaml
plugins:
  - name: auth
    enabled: true
    config:
      jwt_secret: "your-secret-key"
      jwt_method: "HS256"
      auth_header: "Authorization"
      auth_query: "api_key"
      public_endpoints: ["/__health", "/__info"]
```

Rate limiting (per-IP):
```yaml
plugins:
  - name: rate_limit
    enabled: true
    config:
      ip_requests_per_second: 100.0
      ip_burst: 200
```

CORS:
```yaml
plugins:
  - name: cors
    enabled: true
    config:
      allow_origins: ["http://localhost:3000", "https://example.com"]
      allow_methods: ["GET","POST","PUT","DELETE","OPTIONS"]
      allow_headers: ["Origin","Content-Type","Authorization"]
      expose_headers: ["X-Total-Count"]
      allow_credentials: true
      max_age: 86400
```

Logging:
```yaml
plugins:
  - name: logging
    enabled: true
    config:
      log_level: "info"
      log_format: "json"
      log_request_body: true
      log_response_body: true
      max_body_size: 65536
      sensitive_headers: ["authorization","cookie","x-api-key"]
      sensitive_fields: ["password","secret","token"]
```

## Writing a custom plugin
- Implement interfaces in `pkg/plugins/interface.go`.
- Register via the manager and configure in the `plugins` block.

## Middleware ordering
Plugins run after Request ID and before CORS/Logger/Recovery. Priority controls relative order among plugins (lower = earlier).
