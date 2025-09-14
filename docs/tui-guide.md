# TUI Guide

Vanta includes a terminal UI for visibility and control during development.

## Launch
```
vanta tui --config config.yaml --spec examples/petstore.yaml
# read‑only mode
vanta tui --config config.yaml --readonly
```

## Features
- Metrics dashboard (RPS, latency, errors, memory)
- Live log viewer (filter/search)
- Interactive configuration editor with hot reload
- Server status monitoring and control

## Controls (logged at startup)
- Tab/Shift+Tab: switch panels
- Logs: ↑/↓ scroll, `f` filter, `c` clear
- Config: ↑/↓ navigate, Enter edit, Ctrl+S save
- Quit: `q` or Ctrl+C

## Notes
- When configured, the server may auto‑start; TUI can also start/stop.
- Use `--readonly` to prevent accidental config changes.

