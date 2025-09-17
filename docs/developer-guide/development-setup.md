# Development Setup Guide

This guide provides detailed instructions for setting up a complete development environment for Vanta.

## Prerequisites

### Required Tools
- **Go 1.25+** - [Download Go](https://golang.org/doc/install)
- **Git** - For version control
- **Make** - For build automation (comes with most Unix systems)

### Optional but Recommended
- **golangci-lint** - For comprehensive linting
- **Docker** - For containerized development and testing
- **VS Code** or **GoLand** - IDEs with excellent Go support

## Initial Setup

### 1. Clone and Initialize
```bash
# Clone the repository
git clone <your-repo-url> vanta
cd vanta

# Download dependencies
go mod download

# Verify Go installation
go version
```

### 2. Install Development Tools

#### golangci-lint (Recommended)
```bash
# Install golangci-lint for comprehensive linting
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.54.0

# Or using Go install
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

#### Docker (Optional)
- **macOS**: [Docker Desktop](https://docs.docker.com/desktop/mac/)
- **Linux**: [Docker Engine](https://docs.docker.com/engine/install/)
- **Windows**: [Docker Desktop](https://docs.docker.com/desktop/windows/)

## Development Workflow

### Build System

The project uses a comprehensive Makefile with multiple build targets:

```bash
# Quick development build
make build

# Full development workflow (format, vet, lint, test, build)
make dev

# Show all available targets
make help
```

### Available Make Targets

#### Core Development
```bash
make build          # Build for current platform
make test           # Run all tests with coverage
make fmt            # Format all Go code
make vet            # Run go vet for static analysis
make lint           # Run golangci-lint (requires installation)
make dev            # Complete development workflow
```

#### Testing and Quality
```bash
make test           # Run tests with race detection and coverage
make benchmark      # Run performance benchmarks
make coverage       # Generate HTML coverage report
```

#### Advanced Builds
```bash
make build-dev      # Development build using build scripts
make build-all      # Cross-platform builds
make build-release  # Release builds with checksums
make docker         # Build Docker image
```

#### Maintenance
```bash
make clean          # Clean all build artifacts
make deps-update    # Update Go dependencies
make install        # Install binary to $GOPATH/bin
```

### Environment Setup

#### Environment Variables
```bash
# Optional: Customize build cache locations
export GOCACHE=$HOME/.cache/go-build
export GOMODCACHE=$HOME/.cache/go-mod

# For development with custom configurations
export VANTA_CONFIG_PATH=./examples/dev-config.yaml
export VANTA_SPEC_PATH=./examples/petstore.yaml
```

#### VS Code Configuration
Create `.vscode/settings.json`:
```json
{
    "go.testFlags": ["-v", "-race"],
    "go.buildFlags": ["-v"],
    "go.lintTool": "golangci-lint",
    "go.lintOnSave": "workspace",
    "go.formatTool": "goimports",
    "go.useLanguageServer": true,
    "go.testTimeout": "60s"
}
```

Create `.vscode/launch.json` for debugging:
```json
{
    "version": "0.2.0",
    "configurations": [
        {
            "name": "Launch Vanta Server",
            "type": "go",
            "request": "launch",
            "mode": "auto",
            "program": "./main.go",
            "args": [
                "start",
                "--spec", "examples/petstore.yaml",
                "--config", "examples/recording-config.yaml",
                "--port", "8080"
            ],
            "cwd": "${workspaceFolder}"
        },
        {
            "name": "Launch TUI",
            "type": "go",
            "request": "launch",
            "mode": "auto",
            "program": "./main.go",
            "args": [
                "tui",
                "--spec", "examples/petstore.yaml",
                "--config", "examples/recording-config.yaml"
            ],
            "cwd": "${workspaceFolder}"
        }
    ]
}
```

## Development Practices

### Code Quality Standards

#### Formatting and Style
```bash
# Always format before committing
make fmt

# Run static analysis
make vet

# Comprehensive linting (if golangci-lint installed)
make lint
```

#### Testing Requirements
```bash
# Run tests with race detection
make test

# Generate coverage report
make coverage
# Open coverage.html in browser to view

# Run specific package tests
go test -v ./pkg/api/...

# Run tests with custom timeout
go test -timeout 30s ./...
```

#### Git Workflow
```bash
# Before committing, always run
make dev

# This runs: fmt + vet + lint + test + build
# Ensures code quality before commit
```

### Development Configuration

#### Example Development Config
Create `examples/dev-config.yaml`:
```yaml
server:
  port: 8080
  host: "0.0.0.0"
  read_timeout: 30s
  write_timeout: 30s

mock:
  seed: 12345
  prefer_examples: true
  locale: "en_US"

middleware:
  request_id: true
  recovery:
    enabled: true
  cors:
    enabled: true
    allow_origins: ["*"]

metrics:
  enabled: true
  port: 9090

# Enable features for development
chaos:
  enabled: false

recording:
  enabled: true
  storage:
    type: "file"
    directory: "./dev-recordings"

validation:
  enabled: true
  fail_on_invalid: false

hotreload:
  enabled: true
  watch_config: true
  watch_spec: true

state:
  enabled: true
```

#### Development Scripts

Create `scripts/dev-start.sh`:
```bash
#!/bin/bash
# Quick development server start
make build && \
bin/vanta start \
  --spec examples/petstore.yaml \
  --config examples/dev-config.yaml \
  --port 8080
```

Make it executable:
```bash
chmod +x scripts/dev-start.sh
```

### Testing Strategy

#### Unit Tests
```bash
# Run unit tests for specific packages
go test ./pkg/api/
go test ./pkg/openapi/
go test ./pkg/config/
```

#### Integration Tests
```bash
# Run integration tests
go test ./test/integration/...

# Run with verbose output
go test -v ./test/integration/...
```

#### Benchmarks
```bash
# Run all benchmarks
make benchmark

# Run specific benchmarks
go test -bench=BenchmarkDataGeneration ./test/benchmarks/
go test -bench=BenchmarkServerPerformance ./test/benchmarks/
```

### Debugging

#### Logging
```bash
# Enable debug logging in development
# Add to your config:
logging:
  level: debug
  development: true
```

#### TUI for Live Debugging
```bash
# Start TUI for live monitoring
bin/vanta tui --spec examples/petstore.yaml --config examples/dev-config.yaml

# Or with readonly mode
bin/vanta tui --readonly --spec examples/petstore.yaml
```

#### Profiling
```bash
# Run with CPU profiling
go test -cpuprofile cpu.prof -bench=. ./test/benchmarks/

# Analyze profile
go tool pprof cpu.prof
```

### Docker Development

#### Build Development Image
```bash
make docker

# Or manually
docker build -t vanta:dev .
```

#### Run in Container
```bash
# Run with mounted volumes for development
docker run --rm -p 8080:8080 \
  -v $PWD/examples:/app/examples \
  -v $PWD/dev-recordings:/app/recordings \
  vanta:dev start --config /app/examples/dev-config.yaml
```

## IDE Configuration

### GoLand/IntelliJ
1. Install Go plugin
2. Set Go SDK to your Go installation
3. Enable Go modules integration
4. Configure run configurations for different commands

### VS Code
1. Install Go extension
2. Use the settings.json provided above
3. Configure launch configurations for debugging
4. Install additional extensions:
   - Go Test Explorer
   - Go Outline
   - Go Doc

## Troubleshooting

### Common Issues

#### Go Module Issues
```bash
# Clean module cache
go clean -modcache

# Re-download dependencies
go mod download

# Verify dependencies
go mod verify
```

#### Build Issues
```bash
# Clean build cache
go clean -cache

# Rebuild everything
make clean && make build
```

#### Permission Issues
```bash
# Fix executable permissions
chmod +x bin/vanta
chmod +x scripts/*.sh
```

#### Port Conflicts
```bash
# Check what's using port 8080
lsof -i :8080

# Kill process if needed
kill -9 <PID>
```

### Performance Issues
```bash
# Check resource usage
go test -bench=. -benchmem ./test/benchmarks/

# Profile memory usage
go test -memprofile mem.prof -bench=. ./test/benchmarks/
go tool pprof mem.prof
```

## Next Steps

Once your development environment is set up:

1. **Explore the codebase**: Start with [Architecture Overview](architecture.md)
2. **Run the examples**: Test with different OpenAPI specs in `examples/`
3. **Write your first test**: Add tests for any new functionality
4. **Try the TUI**: Experience the interactive monitoring interface
5. **Experiment with plugins**: Look at built-in plugins and create custom ones

## Development Tips

- Always run `make dev` before committing
- Use the TUI for real-time debugging and monitoring
- Test against multiple OpenAPI specs in `examples/`
- Monitor performance with benchmarks
- Keep dependencies up to date with `make deps-update`
- Use feature flags in configuration for experimental features

Happy developing! 🛠️