# Getting Started

This guide walks you through running Vanta with the included examples and making your first requests.

## Prerequisites
- Go 1.25+ (for building from source), or Docker

## Quick Start (from source)
```
make build
bin/vanta start --spec examples/petstore.yaml --port 8080
```

Verify endpoints:
```
curl http://localhost:8080/pets
curl http://localhost:8080/pets/1
curl http://localhost:8080/__health
curl http://localhost:8080/__info
```

## Quick Start (Docker)
```
docker build -t vanta .
docker run --rm -p 8080:8080 \
  -v $PWD/examples:/app/examples \
vanta start --config /app/examples/docker-config.yaml
```

## Minimal Quickstart Example
Run the smallest possible spec + config:
```
vanta start --spec examples/quickstart/openapi.yaml --config examples/quickstart/config.yaml
```

## Next Steps
- Configure features: see Configuration Reference (docs/configuration.md)
- Enable chaos: see Chaos Guide (docs/chaos-guide.md)
- Record traffic: see Recording & Replay (docs/recording-and-replay.md)
- Explore plugins: see Built-in Plugins (docs/builtin-plugins.md) and Plugins Guide (docs/plugins-guide.md)
- Launch the TUI: see TUI Guide (docs/tui-guide.md)
