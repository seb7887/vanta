.PHONY: build build-all build-dev build-release test clean install lint fmt vet benchmark coverage deps-update docker release-dry release-patch release-minor release-major

# Build configuration
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT := $(shell git rev-parse --short HEAD)
GIT_TAG := $(shell git describe --tags --abbrev=0 --exact-match 2>/dev/null)
VERSION := $(if $(GIT_TAG),$(GIT_TAG),dev)

LDFLAGS := -X main.version=$(VERSION) \
           -X main.commit=$(GIT_COMMIT) \
           -X main.buildTime=$(BUILD_TIME) \
           -w -s

# Main targets
build:
	@echo "Building vanta..."
	@mkdir -p .cache/go-build .cache/go-mod bin
	@GOCACHE=$(CURDIR)/.cache/go-build GOMODCACHE=$(CURDIR)/.cache/go-mod go build -ldflags="$(LDFLAGS)" -o bin/vanta ./main.go

build-dev:
	@echo "Building for development..."
	@./scripts/build.sh dev

build-all:
	@echo "Building for all platforms..."
	@./scripts/build.sh all

build-release:
	@echo "Building release packages..."
	@./scripts/build.sh release

test:
	@echo "Running tests..."
	@go test -race -coverprofile=coverage.out ./...

benchmark:
	@echo "Running benchmarks..."
	@go test -bench=. -benchmem ./test/benchmarks/...

coverage: test
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

clean:
	@echo "Cleaning build artifacts..."
	@rm -rf bin/ build/ coverage.out coverage.html
	@./scripts/build.sh clean

install: build
	@echo "Installing vanta..."
	@go install -ldflags="$(LDFLAGS)" ./cmd/mocker

deps-update:
	@echo "Updating dependencies..."
	@go get -u ./...
	@go mod tidy

lint:
	@echo "Running linter..."
	@golangci-lint run

fmt:
	@echo "Formatting code..."
	@go fmt ./...

vet:
	@echo "Running go vet..."
	@go vet ./...

dev: fmt vet lint test build
	@echo "Development build complete"

# Docker targets
docker:
	@echo "Building Docker image..."
	@./scripts/build.sh docker

# Release targets
release-dry:
	@echo "Dry run release (patch)..."
	@./scripts/release.sh dry-run patch

release-patch:
	@echo "Creating patch release..."
	@./scripts/release.sh patch

release-minor:
	@echo "Creating minor release..."
	@./scripts/release.sh minor

release-major:
	@echo "Creating major release..."
	@./scripts/release.sh major

ci: deps-update dev benchmark
	@echo "CI build complete"

# Help target
help:
	@echo "Available targets:"
	@echo "  build          - Build for current platform"
	@echo "  build-dev      - Development build using scripts/build.sh"
	@echo "  build-all      - Build for all supported platforms"
	@echo "  build-release  - Build release packages with checksums"
	@echo "  test           - Run tests with coverage"
	@echo "  benchmark      - Run benchmarks"
	@echo "  coverage       - Generate HTML coverage report"
	@echo "  clean          - Clean build artifacts"
	@echo "  install        - Install binary locally"
	@echo "  deps-update    - Update dependencies"
	@echo "  lint           - Run linter"
	@echo "  fmt            - Format code"
	@echo "  vet            - Run go vet"
	@echo "  docker         - Build Docker image"
	@echo "  release-dry    - Show what patch release would do"
	@echo "  release-patch  - Create patch release"
	@echo "  release-minor  - Create minor release"
	@echo "  release-major  - Create major release"
	@echo "  dev            - Development build (fmt + vet + lint + test + build)"
	@echo "  ci             - CI build (deps-update + dev + benchmark)"
	@echo "  help           - Show this help"
