#!/bin/bash

# Installation script for Vanta OpenAPI Mocker
# Supports multiple installation methods and platforms

set -euo pipefail

# Configuration
BINARY_NAME="vanta"
# Attempt to detect repo owner/name from git remote if not provided
REPO_OWNER="${REPO_OWNER:-}"
REPO_NAME="${REPO_NAME:-}"

detect_repo_from_git() {
    if command -v git >/dev/null 2>&1; then
        if url=$(git remote get-url origin 2>/dev/null); then
            # Supported formats:
            #  - https://github.com/owner/repo.git
            #  - git@github.com:owner/repo.git
            #  - https://github.com/owner/repo
            case "$url" in
                https://github.com/*)
                    path=${url#https://github.com/}
                    path=${path%.git}
                    ;;
                git@github.com:*)
                    path=${url#git@github.com:}
                    path=${path%.git}
                    ;;
                *)
                    path=""
                    ;;
            esac
            if [ -n "$path" ]; then
                REPO_OWNER=${REPO_OWNER:-${path%%/*}}
                REPO_NAME=${REPO_NAME:-${path##*/}}
            fi
        fi
    fi
}

# Default to autodetected repo if env not set
if [ -z "$REPO_OWNER" ] || [ -z "$REPO_NAME" ]; then
    detect_repo_from_git
fi

# Final fallback if still empty
REPO_OWNER="${REPO_OWNER:-your-org}"
REPO_NAME="${REPO_NAME:-vanta}"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
TEMP_DIR=$(mktemp -d)

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m'

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

# Cleanup function
cleanup() {
    rm -rf "$TEMP_DIR"
}
trap cleanup EXIT

# Detect OS and architecture
detect_platform() {
    local os
    local arch

    os=$(uname -s | tr '[:upper:]' '[:lower:]')
    arch=$(uname -m)

    case $os in
        linux*)
            os="linux"
            ;;
        darwin*)
            os="darwin"
            ;;
        msys*|mingw*|cygwin*)
            os="windows"
            ;;
        *)
            log_error "Unsupported operating system: $os"
            exit 1
            ;;
    esac

    case $arch in
        x86_64|amd64)
            arch="amd64"
            ;;
        arm64|aarch64)
            arch="arm64"
            ;;
        *)
            log_error "Unsupported architecture: $arch"
            exit 1
            ;;
    esac

    echo "${os}_${arch}"
}

# Get latest release version
get_latest_version() {
    local api_url="https://api.github.com/repos/$REPO_OWNER/$REPO_NAME/releases/latest"

    if command -v curl >/dev/null 2>&1; then
        curl -s "$api_url" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/'
    elif command -v wget >/dev/null 2>&1; then
        wget -qO- "$api_url" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/'
    else
        log_error "curl or wget is required"
        exit 1
    fi
}

# Download and install binary
install_binary() {
    local version=$1
    local platform=$2
    local archive_name="${BINARY_NAME}_${version#v}_${platform}"

    if [[ $platform == *"windows"* ]]; then
        archive_name="${archive_name}.zip"
    else
        archive_name="${archive_name}.tar.gz"
    fi

    local download_url="https://github.com/$REPO_OWNER/$REPO_NAME/releases/download/$version/$archive_name"

    log_info "Downloading $BINARY_NAME $version for $platform..."
    log_info "URL: $download_url"

    cd "$TEMP_DIR"

    if command -v curl >/dev/null 2>&1; then
        if ! curl -L -o "$archive_name" "$download_url"; then
            log_error "Failed to download $archive_name"
            exit 1
        fi
    elif command -v wget >/dev/null 2>&1; then
        if ! wget -O "$archive_name" "$download_url"; then
            log_error "Failed to download $archive_name"
            exit 1
        fi
    else
        log_error "curl or wget is required"
        exit 1
    fi

    log_success "Downloaded $archive_name"

    # Extract archive
    log_info "Extracting archive..."
    if [[ $archive_name == *.zip ]]; then
        if command -v unzip >/dev/null 2>&1; then
            unzip -q "$archive_name"
        else
            log_error "unzip is required to extract .zip files"
            exit 1
        fi
        binary_name="${BINARY_NAME}.exe"
    else
        tar -xzf "$archive_name"
        binary_name="$BINARY_NAME"
    fi

    if [ ! -f "$binary_name" ]; then
        log_error "Binary $binary_name not found in archive"
        exit 1
    fi

    # Install binary
    log_info "Installing $binary_name to $INSTALL_DIR..."

    # Create install directory if it doesn't exist
    if [ ! -d "$INSTALL_DIR" ]; then
        log_warn "Creating directory $INSTALL_DIR"
        if ! mkdir -p "$INSTALL_DIR" 2>/dev/null; then
            log_error "Failed to create $INSTALL_DIR. You may need to run with sudo."
            log_info "Try: sudo $0 $*"
            exit 1
        fi
    fi

    # Copy binary
    if ! cp "$binary_name" "$INSTALL_DIR/$BINARY_NAME" 2>/dev/null; then
        log_error "Failed to install to $INSTALL_DIR. You may need to run with sudo."
        log_info "Try: sudo $0 $*"
        exit 1
    fi

    # Make executable
    chmod +x "$INSTALL_DIR/$BINARY_NAME"

    log_success "$BINARY_NAME installed to $INSTALL_DIR"
}

# Check if binary is in PATH
check_installation() {
    if command -v "$BINARY_NAME" >/dev/null 2>&1; then
        local installed_version
        installed_version=$("$BINARY_NAME" version 2>/dev/null | head -1 || echo "unknown")
        log_success "$BINARY_NAME is installed and in PATH"
        log_info "Version: $installed_version"
        log_info "Location: $(which "$BINARY_NAME")"
        return 0
    else
        log_warn "$BINARY_NAME is not in PATH"
        log_info "Add $INSTALL_DIR to your PATH:"
        log_info "  export PATH=\"$INSTALL_DIR:\$PATH\""
        return 1
    fi
}

# Install via package manager
install_via_homebrew() {
    if ! command -v brew >/dev/null 2>&1; then
        log_error "Homebrew is not installed"
        log_info "Install Homebrew from: https://brew.sh"
        exit 1
    fi

    log_info "Installing $BINARY_NAME via Homebrew..."

    # Add tap if needed
    if ! brew tap | grep -q "$REPO_OWNER/tap"; then
        log_info "Adding Homebrew tap..."
        brew tap "$REPO_OWNER/tap"
    fi

    brew install "$BINARY_NAME"
    log_success "$BINARY_NAME installed via Homebrew"
}

# Show post-install instructions
show_post_install() {
    echo ""
    log_success "Installation complete!"
    echo ""
    echo "Quick start:"
    echo "  $BINARY_NAME --help                    # Show help"
    echo "  $BINARY_NAME version                   # Show version"
    echo "  $BINARY_NAME start                     # Start with default config"
    echo "  $BINARY_NAME start --spec my-api.yaml # Start with custom spec"
    echo "  $BINARY_NAME tui                       # Interactive terminal UI"
    echo ""
    echo "Example configurations:"
    echo "  https://github.com/$REPO_OWNER/$REPO_NAME/tree/main/examples"
    echo ""
}

# Display help
show_help() {
    cat << EOF
Installation script for Vanta OpenAPI Mocker

Usage: $0 [OPTIONS] [COMMAND]

Commands:
    install         Install latest version (default)
    install VERSION Install specific version (e.g., v1.0.0)
    homebrew        Install via Homebrew (macOS/Linux)
    uninstall       Uninstall vanta
    check           Check current installation
    help            Show this help message

Options:
    --dir DIR       Installation directory (default: /usr/local/bin)
    --version VER   Specific version to install
    --force         Force reinstallation
    --tap OWNER/TAP Use Homebrew tap (for 'homebrew' command)

Examples:
    $0                              # Install latest version
    $0 --version v1.2.3             # Install specific version
    $0 --dir ~/bin                  # Install to custom directory
    $0 homebrew                     # Install via Homebrew
    # One-liner install (set REPO_OWNER/REPO_NAME if needed)
    REPO_OWNER=$REPO_OWNER REPO_NAME=$REPO_NAME \
      bash -c "$(curl -fsSL https://raw.githubusercontent.com/$REPO_OWNER/$REPO_NAME/main/scripts/install.sh)"

Supported Platforms:
    - Linux (amd64, arm64)
    - macOS (amd64, arm64)
    - Windows (amd64)

For more information: https://github.com/$REPO_OWNER/$REPO_NAME
EOF
}

# Uninstall function
uninstall() {
    local binary_path="$INSTALL_DIR/$BINARY_NAME"

    if [ -f "$binary_path" ]; then
        log_info "Removing $binary_path..."
        if rm "$binary_path" 2>/dev/null; then
            log_success "$BINARY_NAME uninstalled"
        else
            log_error "Failed to remove $binary_path. You may need to run with sudo."
            log_info "Try: sudo rm $binary_path"
        fi
    else
        log_warn "$BINARY_NAME not found at $binary_path"
    fi

    # Check if installed via Homebrew
    if command -v brew >/dev/null 2>&1 && brew list "$BINARY_NAME" >/dev/null 2>&1; then
        log_info "Found Homebrew installation, removing..."
        brew uninstall "$BINARY_NAME"
        log_success "$BINARY_NAME uninstalled from Homebrew"
    fi
}

# Main command handling
VERSION=""
FORCE=false

# Parse options
while [[ $# -gt 0 ]]; do
    case $1 in
        --dir)
            INSTALL_DIR="$2"
            shift 2
            ;;
        --version)
            VERSION="$2"
            shift 2
            ;;
        --force)
            FORCE=true
            shift
            ;;
        *)
            break
            ;;
    esac
done

case "${1:-install}" in
    "install")
        # Check if already installed
        if command -v "$BINARY_NAME" >/dev/null 2>&1 && [ "$FORCE" = false ]; then
            current_version=$("$BINARY_NAME" version 2>/dev/null | head -1 || echo "unknown")
            log_warn "$BINARY_NAME is already installed: $current_version"
            log_info "Use --force to reinstall or 'uninstall' to remove"
            exit 0
        fi

        platform=$(detect_platform)

        if [ -n "$VERSION" ]; then
            version="$VERSION"
        elif [ -n "${2:-}" ]; then
            version="$2"
        else
            log_info "Fetching latest release..."
            version=$(get_latest_version)
        fi

        if [ -z "$version" ]; then
            log_error "Could not determine version to install"
            exit 1
        fi

        log_info "Installing $BINARY_NAME $version for $platform"
        install_binary "$version" "$platform"
        check_installation
        show_post_install
        ;;

    "homebrew")
        # Optional: allow specifying a tap via --tap OWNER/TAP
        if [ -n "${2:-}" ] && [ "${2#--tap}" != "$2" ]; then
            TAP_VAL="$3"
            shift 2 || true
            if [ -n "$TAP_VAL" ]; then
                if ! brew tap | grep -q "$TAP_VAL"; then
                    log_info "Adding Homebrew tap $TAP_VAL..."
                    brew tap "$TAP_VAL"
                fi
            fi
        fi
        install_via_homebrew
        check_installation
        show_post_install
        ;;

    "uninstall")
        uninstall
        ;;

    "check")
        check_installation
        ;;

    "help"|*)
        show_help
        ;;
esac
