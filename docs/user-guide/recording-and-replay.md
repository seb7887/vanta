# Recording & Replay

Capture incoming requests and responses and replay them to another target.

## Enable recording
Use `examples/recording-config.yaml` or add:
```yaml
recording:
  enabled: true
  storage:
    type: "file"
    directory: "./recordings"
    format: "jsonlines"
  max_recordings: 1000
  max_body_size: 1048576
  include_headers: ["content-type", "user-agent", "x-request-id"]
  exclude_headers: ["cookie", "set-cookie", "x-forwarded-for"]
  filters:
    - type: "method"
      values: ["GET", "POST"]
      negate: false
```

## Workflow
1) Start server with recording enabled
2) Generate traffic
3) Inspect recordings
4) Replay to a target

## Commands
```
vanta record list --limit 10
vanta record show <id> --format table
vanta record delete <id1> <id2>
vanta record delete --all --force

vanta record replay --target http://localhost:9000 --since 1h --concurrency 3 --delay 100ms

vanta record export --format json --output out.json
```

Note: Some export formats (har/postman/curl) are scaffolded and may return "not yet implemented".

## Filters
- `method`: include/exclude by HTTP method
- `endpoint`: include/exclude by URI pattern (supports `*`)
- `status`: include/exclude by HTTP status code

## Performance tips
- Limit `max_recordings` and `max_body_size` for high‑throughput scenarios.
- Use filters to focus on critical endpoints.

