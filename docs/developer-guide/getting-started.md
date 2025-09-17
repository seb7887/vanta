# Developer Getting Started Guide

Welcome to Vanta development! This guide will get you set up and productive in 5 minutes.

## Prerequisites

- **Go 1.25+** - [Install Go](https://golang.org/doc/install)
- **Git** - For version control
- **Make** (optional) - For build shortcuts

## Quick Setup (5 minutes)

### 1. Clone and Setup
```bash
git clone <your-repo-url> vanta
cd vanta
go mod download
```

### 2. Build and Test
```bash
# Build the project
make build
# or
go build -o bin/vanta ./main.go

# Run tests
go test ./...

# Start development server
bin/vanta start --spec examples/petstore.yaml --port 8080
```

### 3. Verify Installation
```bash
# Check health
curl http://localhost:8080/__health

# Test API endpoint
curl http://localhost:8080/pets

# Check info
curl http://localhost:8080/__info
```

## Development Workflow

### Local Development
```bash
# Watch for changes and rebuild (if you have tools installed)
# Otherwise, rebuild manually after changes
make build && bin/vanta start --spec examples/petstore.yaml

# Run with development config
bin/vanta start --spec examples/petstore.yaml --config examples/recording-config.yaml
```

### Testing
```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -v -race -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Run integration tests
go test ./test/integration/...

# Run benchmarks
go test -bench=. ./test/benchmarks/...
```

### Code Quality
```bash
# Format code
go fmt ./...

# Lint (if golangci-lint is installed)
golangci-lint run

# Vet for common issues
go vet ./...
```

## Project Structure Overview

```
vanta/
├── main.go/          # CLI application entry point
├── pkg/                 # Core packages (public API)
│   ├── api/            # HTTP server and routing
│   ├── chaos/          # Chaos engineering
│   ├── config/         # Configuration management
│   ├── openapi/        # OpenAPI parsing and mocking
│   ├── plugins/        # Plugin system
│   ├── recorder/       # Request/response recording
│   ├── state/          # State management
│   └── validation/     # Request/response validation
├── internal/           # Internal packages (private)
│   ├── cache/          # Caching utilities
│   ├── hotreload/      # Hot reload functionality
│   ├── metrics/        # Metrics collection
│   └── utils/          # Shared utilities
├── examples/           # Example configurations and specs
├── docs/               # Documentation
└── test/               # Test files and fixtures
```

## Key Components

- **main.go**: CLI interface and command handling
- **pkg/api**: FastHTTP server, middleware stack, routing
- **pkg/openapi**: OpenAPI spec parsing and realistic data generation
- **pkg/plugins**: Plugin system for extensibility
- **pkg/config**: Configuration management with validation
- **internal/**: Support packages for caching, metrics, utilities

## Next Steps

Now that you have a working setup:

1. **Explore the codebase**: Read [Architecture Overview](architecture.md)
2. **Set up your development environment**: See [Development Setup](development-setup.md)
3. **Learn about contributing**: Check [Contributing Guidelines](../../CONTRIBUTING.md) (when available)
4. **Understand the architecture**: Review [Project Architecture](architecture.md)

## Common Development Tasks

### Adding a New Feature
1. Create tests first (TDD approach)
2. Implement the feature in appropriate package
3. Add configuration options if needed
4. Update documentation
5. Test with example configurations

### Debugging
```bash
# Enable debug logging
bin/vanta start --spec examples/petstore.yaml --config examples/debug-config.yaml

# Use TUI for live debugging
bin/vanta tui --spec examples/petstore.yaml

# Add debug prints (use zap logger)
log.Debug("Debug message", zap.String("key", "value"))
```

### Working with Examples
- `examples/petstore.yaml` - Standard OpenAPI spec for testing
- `examples/quickstart/` - Minimal configuration example
- `examples/k8s/` - Kubernetes deployment examples
- Test your changes against multiple examples

## Getting Help

- Check existing documentation in `docs/`
- Look at test files for usage examples
- Use the TUI for live system inspection
- Review the configuration reference for all options

Happy coding! 🚀