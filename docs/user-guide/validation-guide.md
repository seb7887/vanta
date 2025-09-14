# Validation Guide

Vanta can validate incoming requests and outgoing responses against your OpenAPI spec.

## Configuration
```yaml
validation:
  enabled: true
  fail_on_invalid: false           # set true to reject invalid requests
  validate_headers: true
  validate_query: true
  validate_path: true
  validate_body: true
  validate_status_codes: true
  report_format: ["json", "html"]
  report_path: "./validation-reports"
  validation_timeout: 30s
```

## Behavior
- Requests: If invalid and `fail_on_invalid: true`, returns 400/500; otherwise logs warnings.
- Responses: Validated after handler; logs warnings/errors with status and fields.
- Results are attached to the request context for downstream consumers.

## Tips
- Start in non‑fatal mode to surface issues without breaking flows.
- Tighten to `fail_on_invalid: true` in CI or stricter environments.

## Examples

Sample request validation failure (JSON): `docs/validation-samples/request-invalid.json`

```
{
  "valid": false,
  "errors": [
    {
      "location": "body",
      "path": "$.name",
      "message": "required field missing"
    }
  ],
  "warnings": [
    { "location": "query", "path": "?limit", "message": "unknown parameter" }
  ],
  "request_id": "7c1f1e4c-8e9a-4f1e-9b2b-1a9e5f2c9f2a"
}
```

Sample response validation warning (JSON): `docs/validation-samples/response-invalid.json`

```
{
  "valid": true,
  "status_code": 200,
  "warnings": [
    { "location": "body", "path": "$.items[0].status", "message": "value not in enum [available,pending,sold]" }
  ]
}
```
