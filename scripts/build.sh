#!/bin/bash

# Build script for Vanta OpenAPI Mocker
# Supports cross-compilation, version embedding, and release preparation

set -euo pipefail

# Configuration
BINARY_NAME="vanta"
BUILD_DIR="build"
CMD_PATH="./main.go"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Get version information
get_version_info() {
    if [ -n "${VERSION:-}" ]; then
        echo "$VERSION"
    elif git describe --tags --exact-match 2>/dev/null; then
        git describe --tags --exact-match
    elif git describe --tags 2>/dev/null; then
        git describe --tags
    else
        echo "dev"
    fi
}

get_commit_hash() {
    if [ -n "${COMMIT:-}" ]; then
        echo "$COMMIT"
    elif git rev-parse --short HEAD 2>/dev/null; then
        git rev-parse --short HEAD
    else
        echo "unknown"
    fi
}

get_build_time() {
    if [ -n "${BUILD_TIME:-}" ]; then
        echo "$BUILD_TIME"
    else
        date -u '+%Y-%m-%d_%H:%M:%S'
    fi
}

# Build function
build_binary() {
    local goos=$1
    local goarch=$2
    local output_name=$3

    local version=$(get_version_info)
    local commit=$(get_commit_hash)
    local build_time=$(get_build_time)

    local ldflags="-s -w"
    ldflags="$ldflags -X main.version=$version"
    ldflags="$ldflags -X main.commit=$commit"
    ldflags="$ldflags -X main.buildTime=$build_time"

    log_info "Building $output_name for $goos/$goarch"
    log_info "Version: $version, Commit: $commit, Build Time: $build_time"

    CGO_ENABLED=0 GOOS=$goos GOARCH=$goarch go build \
        -ldflags="$ldflags" \
        -o "$BUILD_DIR/$output_name" \
        "$CMD_PATH"

    if [ $? -eq 0 ]; then
        log_success "Built $output_name"
    else
        log_error "Failed to build $output_name"
        exit 1
    fi
}

# Create build directory
create_build_dir() {
    log_info "Creating build directory"
    mkdir -p "$BUILD_DIR"
}

# Clean build directory
clean_build_dir() {
    log_info "Cleaning build directory"
    rm -rf "$BUILD_DIR"
}

# Package function
package_binary() {
    local binary_path=$1
    local package_name=$2
    local format=${3:-tar.gz}

    log_info "Packaging $binary_path as $package_name.$format"

    cd "$BUILD_DIR"

    if [ "$format" = "zip" ]; then
        zip -q "$package_name.zip" "$(basename $binary_path)" ../LICENSE ../README.md
    else
        tar -czf "$package_name.tar.gz" "$(basename $binary_path)" ../LICENSE ../README.md
    fi

    cd ..
    log_success "Created package $package_name.$format"
}

# Generate checksums
generate_checksums() {
    log_info "Generating checksums"
    cd "$BUILD_DIR"

    if command -v sha256sum >/dev/null 2>&1; then
        sha256sum * > checksums.txt
    elif command -v shasum >/dev/null 2>&1; then
        shasum -a 256 * > checksums.txt
    else
        log_warn "No checksum utility found"
        cd ..
        return
    fi

    cd ..
    log_success "Generated checksums.txt"
}

# Display help
show_help() {
    cat << EOF
Build script for Vanta OpenAPI Mocker

Usage: $0 [COMMAND] [OPTIONS]

Commands:
    dev         Build for current platform (development)
    all         Build for all supported platforms
    release     Build and package for all platforms with checksums
    clean       Clean build directory
    package     Package existing binaries
    docker      Build Docker image
    help        Show this help message

Options:
    --version   Set version (default: auto-detect from git)
    --commit    Set commit hash (default: auto-detect from git)
    --build-time Set build time (default: current UTC time)

Environment Variables:
    VERSION     Override version
    COMMIT      Override commit hash
    BUILD_TIME  Override build time

Examples:
    $0 dev                          # Build for current platform
    $0 all                          # Build for all platforms
    $0 release                      # Full release build with packages
    VERSION=v1.0.0 $0 release      # Release build with custom version

Supported Platforms:
    - linux/amd64
    - linux/arm64
    - darwin/amd64 (Intel Mac)
    - darwin/arm64 (Apple Silicon)
    - windows/amd64
EOF
}

# Main command handling
case "${1:-help}" in
    "dev")
        log_info "Building for current platform (development build)"
        create_build_dir

        # Detect current platform
        GOOS=$(go env GOOS)
        GOARCH=$(go env GOARCH)

        if [ "$GOOS" = "windows" ]; then
            build_binary "$GOOS" "$GOARCH" "${BINARY_NAME}.exe"
        else
            build_binary "$GOOS" "$GOARCH" "$BINARY_NAME"
        fi

        log_success "Development build complete"
        ;;

    "all")
        log_info "Building for all supported platforms"
        create_build_dir

        # Linux
        build_binary "linux" "amd64" "${BINARY_NAME}-linux-amd64"
        build_binary "linux" "arm64" "${BINARY_NAME}-linux-arm64"

        # macOS
        build_binary "darwin" "amd64" "${BINARY_NAME}-darwin-amd64"
        build_binary "darwin" "arm64" "${BINARY_NAME}-darwin-arm64"

        # Windows
        build_binary "windows" "amd64" "${BINARY_NAME}-windows-amd64.exe"

        log_success "All platform builds complete"
        ;;

    "release")
        log_info "Building release packages"
        create_build_dir

        # Build all platforms
        build_binary "linux" "amd64" "${BINARY_NAME}-linux-amd64"
        build_binary "linux" "arm64" "${BINARY_NAME}-linux-arm64"
        build_binary "darwin" "amd64" "${BINARY_NAME}-darwin-amd64"
        build_binary "darwin" "arm64" "${BINARY_NAME}-darwin-arm64"
        build_binary "windows" "amd64" "${BINARY_NAME}-windows-amd64.exe"

        # Package binaries
        VERSION=$(get_version_info)
        package_binary "${BUILD_DIR}/${BINARY_NAME}-linux-amd64" "${BINARY_NAME}_${VERSION}_linux_amd64"
        package_binary "${BUILD_DIR}/${BINARY_NAME}-linux-arm64" "${BINARY_NAME}_${VERSION}_linux_arm64"
        package_binary "${BUILD_DIR}/${BINARY_NAME}-darwin-amd64" "${BINARY_NAME}_${VERSION}_darwin_amd64"
        package_binary "${BUILD_DIR}/${BINARY_NAME}-darwin-arm64" "${BINARY_NAME}_${VERSION}_darwin_arm64"
        package_binary "${BUILD_DIR}/${BINARY_NAME}-windows-amd64.exe" "${BINARY_NAME}_${VERSION}_windows_amd64" "zip"

        # Generate checksums
        generate_checksums

        log_success "Release build complete"
        ;;

    "clean")
        clean_build_dir
        log_success "Build directory cleaned"
        ;;

    "package")
        if [ ! -d "$BUILD_DIR" ] || [ -z "$(ls -A $BUILD_DIR)" ]; then
            log_error "No binaries found in $BUILD_DIR. Run build first."
            exit 1
        fi

        log_info "Packaging existing binaries"
        VERSION=$(get_version_info)

        # Package all found binaries
        for binary in "$BUILD_DIR"/*; do
            if [ -f "$binary" ] && [ -x "$binary" ]; then
                filename=$(basename "$binary")
                package_name="${filename//${BINARY_NAME}-/${BINARY_NAME}_${VERSION}_}"

                if [[ "$filename" == *".exe" ]]; then
                    package_binary "$binary" "${package_name%.*}" "zip"
                else
                    package_binary "$binary" "$package_name"
                fi
            fi
        done

        generate_checksums
        log_success "Packaging complete"
        ;;

    "docker")
        log_info "Building Docker image"
        VERSION=$(get_version_info)
        COMMIT=$(get_commit_hash)
        BUILD_TIME=$(get_build_time)

        docker build \
            --build-arg VERSION="$VERSION" \
            --build-arg COMMIT="$COMMIT" \
            --build-arg BUILD_TIME="$BUILD_TIME" \
            -t "vanta:$VERSION" \
            -t "vanta:latest" \
            .

        log_success "Docker image built: vanta:$VERSION, vanta:latest"
        ;;

    "help"|*)
        show_help
        ;;
esac