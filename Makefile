.PHONY: build test clean install lint fmt vet benchmark coverage deps-update

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
	@go build -ldflags="$(LDFLAGS)" -o bin/vanta ./cmd/mocker

build-all: 
	@echo "Building for all platforms..."
	@GOOS=linux GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o bin/vanta-linux-amd64 ./cmd/mocker
	@GOOS=darwin GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o bin/vanta-darwin-amd64 ./cmd/mocker
	@GOOS=windows GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o bin/vanta-windows-amd64.exe ./cmd/mocker

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
	@rm -rf bin/ coverage.out coverage.html

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

ci: deps-update dev benchmark
	@echo "CI build complete"