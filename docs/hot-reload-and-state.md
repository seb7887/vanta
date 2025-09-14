# Hot Reload & State

## Hot Reload
Watch config/spec files and apply changes without a full restart.
```yaml
hotreload:
  enabled: true
  watch_config: true
  watch_spec: true
  debounce_delay: 500ms
```
Notes:
- Config/spec changes trigger a safe server restart with new settings/spec.
- Debounce helps avoid rapid restarts while editing.

## State Management
Maintain per‑session/request context and endpoint state.
```yaml
state:
  enabled: true
  storage:
    type: "memory"
    file_path: "./state.json"
  context:
    default_ttl: 30m
    session_ttl: 24h
    request_ttl: 5m
```
Notes:
- Session ID can be provided via `X-Session-ID` header or cookie `session_id`; otherwise generated.
- State is accessible to middlewares/handlers via context; useful for scenario‑driven mocking.

