#!/bin/bash
# syncver.sh - Keep VERSION file and pkg/version/version.go in sync
# Compatible with bash 3.2+ (macOS default)

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
VERSION_FILE="$SCRIPT_DIR/VERSION"
VERSION_GO="$SCRIPT_DIR/pkg/version/version.go"

usage() {
    echo "Usage: $0 [command] [version]"
    echo ""
    echo "Commands:"
    echo "  show           Show current version"
    echo "  set <version>  Set version in both files"
    echo "  check          Verify versions are in sync"
    echo "  bump-patch     Bump patch version (0.1.0 -> 0.1.1)"
    echo "  bump-minor     Bump minor version (0.1.0 -> 0.2.0)"
    echo "  bump-major     Bump major version (0.1.0 -> 1.0.0)"
    echo ""
    echo "Examples:"
    echo "  $0 show"
    echo "  $0 set 0.2.0"
    echo "  $0 bump-patch"
}

get_file_version() {
    if [ -f "$VERSION_FILE" ]; then
        tr -d '[:space:]' < "$VERSION_FILE"
    else
        echo ""
    fi
}

get_go_version() {
    if [ -f "$VERSION_GO" ]; then
        grep 'const Version' "$VERSION_GO" | sed 's/.*"\(.*\)".*/\1/'
    else
        echo ""
    fi
}

set_version() {
    local new_version="$1"

    # Validate version format (basic semver)
    if ! echo "$new_version" | grep -qE '^[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9.]+)?$'; then
        echo "Error: Invalid version format. Expected X.Y.Z or X.Y.Z-suffix (e.g., 0.1.0, 0.2.0-rc1)" >&2
        # Note: suffix is freeform (rc1, rc2, alpha, checkpoint, etc.) -- no canonicality enforced
        exit 1
    fi

    # Update VERSION file
    echo "$new_version" > "$VERSION_FILE"

    # Update version.go
    cat > "$VERSION_GO" << EOF
// Package version provides version information for zen80.
//
// IMPORTANT: Keep Version constant in sync with the VERSION file at project root.
// When updating the version, update BOTH files, or use: ./syncver.sh set <version>
package version

// Version is the current version of zen80.
// This MUST match the contents of the VERSION file.
const Version = "$new_version"
EOF

    echo "Version set to $new_version"
}

bump_version() {
    local part="$1"
    local current
    current="$(get_file_version)"

    if [ -z "$current" ]; then
        echo "Error: Cannot read current version" >&2
        exit 1
    fi

    local major minor patch
    major="$(echo "$current" | cut -d. -f1)"
    minor="$(echo "$current" | cut -d. -f2)"
    patch="$(echo "$current" | cut -d. -f3)"

    case "$part" in
        major)
            major=$((major + 1))
            minor=0
            patch=0
            ;;
        minor)
            minor=$((minor + 1))
            patch=0
            ;;
        patch)
            patch=$((patch + 1))
            ;;
    esac

    set_version "$major.$minor.$patch"
}

# Main
case "${1:-show}" in
    show)
        file_ver="$(get_file_version)"
        go_ver="$(get_go_version)"
        echo "VERSION file: ${file_ver:-<not found>}"
        echo "version.go:   ${go_ver:-<not found>}"
        ;;
    set)
        if [ -z "$2" ]; then
            echo "Error: Version required" >&2
            usage
            exit 1
        fi
        set_version "$2"
        ;;
    check)
        file_ver="$(get_file_version)"
        go_ver="$(get_go_version)"
        if [ "$file_ver" = "$go_ver" ]; then
            echo "OK: Versions in sync ($file_ver)"
            exit 0
        else
            echo "MISMATCH:"
            echo "  VERSION file: $file_ver"
            echo "  version.go:   $go_ver"
            exit 1
        fi
        ;;
    bump-patch)
        bump_version patch
        ;;
    bump-minor)
        bump_version minor
        ;;
    bump-major)
        bump_version major
        ;;
    help|--help|-h)
        usage
        ;;
    *)
        echo "Unknown command: $1" >&2
        usage
        exit 1
        ;;
esac
