#!/bin/bash

# Release script for Vanta OpenAPI Mocker
# Automates the release process including version bumping, tagging, and publishing

set -euo pipefail

# Configuration
BINARY_NAME="vanta"
MAIN_BRANCH="main"
RELEASE_BRANCH="release"

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

# Check if we're in a git repository
check_git_repo() {
    if ! git rev-parse --git-dir > /dev/null 2>&1; then
        log_error "Not in a git repository"
        exit 1
    fi
}

# Check if working directory is clean
check_working_directory() {
    if [ -n "$(git status --porcelain)" ]; then
        log_error "Working directory is not clean. Please commit or stash changes."
        git status --short
        exit 1
    fi
}

# Get current version from git tags
get_current_version() {
    git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0"
}

# Increment version based on type
increment_version() {
    local version=$1
    local type=$2

    # Remove 'v' prefix if present
    version=${version#v}

    # Split version into parts
    IFS='.' read -ra VERSION_PARTS <<< "$version"
    local major=${VERSION_PARTS[0]:-0}
    local minor=${VERSION_PARTS[1]:-0}
    local patch=${VERSION_PARTS[2]:-0}

    case $type in
        "major")
            major=$((major + 1))
            minor=0
            patch=0
            ;;
        "minor")
            minor=$((minor + 1))
            patch=0
            ;;
        "patch")
            patch=$((patch + 1))
            ;;
        *)
            log_error "Invalid version type: $type. Use major, minor, or patch."
            exit 1
            ;;
    esac

    echo "v$major.$minor.$patch"
}

# Validate version format
validate_version() {
    local version=$1
    if [[ ! $version =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9\.-]+)?(\+[a-zA-Z0-9\.-]+)?$ ]]; then
        log_error "Invalid version format: $version"
        log_info "Expected format: v1.2.3 or v1.2.3-alpha.1 or v1.2.3+build.1"
        exit 1
    fi
}

# Run tests
run_tests() {
    log_info "Running tests..."
    if ! go test -race ./...; then
        log_error "Tests failed"
        exit 1
    fi
    log_success "Tests passed"
}

# Run linter
run_lint() {
    log_info "Running linter..."
    if command -v golangci-lint >/dev/null 2>&1; then
        if ! golangci-lint run; then
            log_error "Linting failed"
            exit 1
        fi
        log_success "Linting passed"
    else
        log_warn "golangci-lint not found, skipping linting"
    fi
}

# Build release binaries
build_release() {
    local version=$1
    log_info "Building release binaries for $version..."

    export VERSION=$version
    if ! ./scripts/build.sh release; then
        log_error "Build failed"
        exit 1
    fi

    log_success "Release binaries built"
}

# Create and push git tag
create_tag() {
    local version=$1
    local message=$2

    log_info "Creating tag $version..."

    git tag -a "$version" -m "$message"
    git push origin "$version"

    log_success "Tag $version created and pushed"
}

# Generate changelog entry
generate_changelog_entry() {
    local version=$1
    local previous_version=$2

    log_info "Generating changelog entry..."

    echo "## $version ($(date '+%Y-%m-%d'))"
    echo ""

    if [ "$previous_version" != "v0.0.0" ]; then
        # Get commits since last tag
        git log --pretty=format:"- %s" "$previous_version"..HEAD | grep -v "^- Merge" || true
    else
        echo "- Initial release"
    fi

    echo ""
}

# Update CHANGELOG.md
update_changelog() {
    local version=$1
    local previous_version=$2

    if [ ! -f CHANGELOG.md ]; then
        log_info "Creating CHANGELOG.md"
        cat > CHANGELOG.md << EOF
# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

EOF
    fi

    # Generate new entry
    local changelog_entry
    changelog_entry=$(generate_changelog_entry "$version" "$previous_version")

    # Insert new entry after header
    local temp_file
    temp_file=$(mktemp)
    head -n 6 CHANGELOG.md > "$temp_file"
    echo "$changelog_entry" >> "$temp_file"
    tail -n +7 CHANGELOG.md >> "$temp_file"
    mv "$temp_file" CHANGELOG.md

    log_success "CHANGELOG.md updated"
}

# Perform pre-release checks
pre_release_checks() {
    log_info "Performing pre-release checks..."

    check_git_repo
    check_working_directory

    # Check if on main branch
    current_branch=$(git rev-parse --abbrev-ref HEAD)
    if [ "$current_branch" != "$MAIN_BRANCH" ]; then
        log_error "Must be on $MAIN_BRANCH branch to create a release"
        exit 1
    fi

    # Pull latest changes
    log_info "Pulling latest changes..."
    git pull origin "$MAIN_BRANCH"

    # Run tests and linting
    run_tests
    run_lint

    log_success "Pre-release checks passed"
}

# Publish release
publish_release() {
    local version=$1

    log_info "Publishing release $version..."

    # Push changes
    git push origin "$MAIN_BRANCH"

    # The GitHub Actions workflow will handle the actual release
    log_info "GitHub Actions will handle the release process"
    log_info "Monitor the release at: https://github.com/YOUR_USERNAME/vanta/actions"

    log_success "Release $version initiated"
}

# Display help
show_help() {
    cat << EOF
Release script for Vanta OpenAPI Mocker

Usage: $0 [COMMAND] [OPTIONS]

Commands:
    major           Create a major release (1.0.0 -> 2.0.0)
    minor           Create a minor release (1.0.0 -> 1.1.0)
    patch           Create a patch release (1.0.0 -> 1.0.1)
    custom VERSION  Create a release with custom version
    dry-run TYPE    Show what would be released without making changes
    help            Show this help message

Options:
    --skip-tests    Skip running tests (not recommended)
    --skip-build    Skip building release binaries
    --message MSG   Custom release message

Examples:
    $0 patch                    # Create patch release
    $0 minor                    # Create minor release
    $0 major                    # Create major release
    $0 custom v1.2.3-beta.1   # Create custom version release
    $0 dry-run patch           # Show what patch release would do

Prerequisites:
    - Clean working directory
    - On main branch
    - All tests passing
    - golangci-lint installed (optional)

The script will:
    1. Run pre-release checks
    2. Increment version
    3. Update CHANGELOG.md
    4. Build release binaries
    5. Create and push git tag
    6. Trigger GitHub Actions release workflow
EOF
}

# Dry run function
dry_run() {
    local type=$1
    local current_version
    local new_version

    log_info "DRY RUN: Showing what would happen for $type release"

    current_version=$(get_current_version)
    if [ "$type" = "custom" ]; then
        new_version=$2
        validate_version "$new_version"
    else
        new_version=$(increment_version "$current_version" "$type")
    fi

    echo ""
    echo "Current version: $current_version"
    echo "New version: $new_version"
    echo ""
    echo "Would perform:"
    echo "  1. Pre-release checks"
    echo "  2. Run tests and linting"
    echo "  3. Update CHANGELOG.md"
    echo "  4. Build release binaries"
    echo "  5. Create git tag: $new_version"
    echo "  6. Push tag to trigger release"
    echo ""
    echo "Changelog entry would include:"
    generate_changelog_entry "$new_version" "$current_version"
}

# Main command handling
SKIP_TESTS=false
SKIP_BUILD=false
CUSTOM_MESSAGE=""

# Parse options
while [[ $# -gt 0 ]]; do
    case $1 in
        --skip-tests)
            SKIP_TESTS=true
            shift
            ;;
        --skip-build)
            SKIP_BUILD=true
            shift
            ;;
        --message)
            CUSTOM_MESSAGE="$2"
            shift 2
            ;;
        *)
            break
            ;;
    esac
done

case "${1:-help}" in
    "major"|"minor"|"patch")
        RELEASE_TYPE=$1

        if [ "$SKIP_TESTS" = false ]; then
            pre_release_checks
        else
            check_git_repo
            check_working_directory
        fi

        current_version=$(get_current_version)
        new_version=$(increment_version "$current_version" "$RELEASE_TYPE")

        log_info "Creating $RELEASE_TYPE release: $current_version -> $new_version"

        # Update changelog
        update_changelog "$new_version" "$current_version"
        git add CHANGELOG.md
        git commit -m "Update CHANGELOG for $new_version"

        # Build release
        if [ "$SKIP_BUILD" = false ]; then
            build_release "$new_version"
        fi

        # Create release message
        if [ -n "$CUSTOM_MESSAGE" ]; then
            release_message="$CUSTOM_MESSAGE"
        else
            release_message="Release $new_version"
        fi

        # Create and push tag
        create_tag "$new_version" "$release_message"

        # Publish
        publish_release "$new_version"
        ;;

    "custom")
        if [ -z "${2:-}" ]; then
            log_error "Custom version required"
            show_help
            exit 1
        fi

        CUSTOM_VERSION=$2
        validate_version "$CUSTOM_VERSION"

        if [ "$SKIP_TESTS" = false ]; then
            pre_release_checks
        else
            check_git_repo
            check_working_directory
        fi

        current_version=$(get_current_version)

        log_info "Creating custom release: $current_version -> $CUSTOM_VERSION"

        # Update changelog
        update_changelog "$CUSTOM_VERSION" "$current_version"
        git add CHANGELOG.md
        git commit -m "Update CHANGELOG for $CUSTOM_VERSION"

        # Build release
        if [ "$SKIP_BUILD" = false ]; then
            build_release "$CUSTOM_VERSION"
        fi

        # Create release message
        if [ -n "$CUSTOM_MESSAGE" ]; then
            release_message="$CUSTOM_MESSAGE"
        else
            release_message="Release $CUSTOM_VERSION"
        fi

        # Create and push tag
        create_tag "$CUSTOM_VERSION" "$release_message"

        # Publish
        publish_release "$CUSTOM_VERSION"
        ;;

    "dry-run")
        if [ -z "${2:-}" ]; then
            log_error "Release type required for dry-run"
            show_help
            exit 1
        fi

        if [ "$2" = "custom" ]; then
            if [ -z "${3:-}" ]; then
                log_error "Custom version required for dry-run"
                exit 1
            fi
            dry_run "$2" "$3"
        else
            dry_run "$2"
        fi
        ;;

    "help"|*)
        show_help
        ;;
esac