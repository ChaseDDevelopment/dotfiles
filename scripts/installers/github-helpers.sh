#!/usr/bin/env bash

# =============================================================================
# GitHub Release Helpers
# =============================================================================
# Reusable functions for downloading and installing binaries from GitHub
# releases. Used by cli-tools.sh and dev-tools.sh.
# =============================================================================

# Get the latest release version tag from a GitHub repository.
# Usage: github_latest_version "owner/repo" [strip_v]
#   strip_v: "true" (default) strips leading 'v' from tag, "false" keeps it
# Returns: version string on stdout, or empty string on failure
github_latest_version() {
    local repo="$1"
    local strip_v="${2:-true}"

    local version
    version=$(curl -sI "https://github.com/$repo/releases/latest" \
        | grep -i '^location:' \
        | sed 's|.*/||' \
        | tr -d '\r')

    if [[ "$strip_v" == "true" ]]; then
        version="${version#v}"
    fi

    if [[ -z "$version" ]]; then
        ui_warn "Could not determine latest version for $repo"
        return 1
    fi

    echo "$version"
}

# Detect the current platform and export PLATFORM_OS / PLATFORM_ARCH.
# Returns 1 if the platform is unsupported.
detect_platform() {
    PLATFORM_OS="$(uname -s)"
    PLATFORM_ARCH="$(uname -m)"

    case "$PLATFORM_OS" in
        Linux|Darwin) ;;
        *)
            ui_warn "Unsupported OS: $PLATFORM_OS"
            return 1
            ;;
    esac

    case "$PLATFORM_ARCH" in
        x86_64|aarch64|arm64) ;;
        *)
            ui_warn "Unsupported architecture: $PLATFORM_ARCH"
            return 1
            ;;
    esac
}

# Map the current platform to a Rust-style target triple.
# Usage: platform_target_triple [musl|gnu]
#   libc: "musl" (default for Linux) or "gnu"
# Returns: target triple on stdout (e.g. x86_64-unknown-linux-musl)
platform_target_triple() {
    local libc="${1:-musl}"

    detect_platform || return 1

    local triple=""
    case "$PLATFORM_OS" in
        Linux)
            case "$PLATFORM_ARCH" in
                x86_64)          triple="x86_64-unknown-linux-${libc}" ;;
                aarch64|arm64)   triple="aarch64-unknown-linux-${libc}" ;;
            esac
            ;;
        Darwin)
            case "$PLATFORM_ARCH" in
                x86_64)          triple="x86_64-apple-darwin" ;;
                aarch64|arm64)   triple="aarch64-apple-darwin" ;;
            esac
            ;;
    esac

    echo "$triple"
}

# Download a binary from a URL, extract if tarball, find the named binary,
# and install it to /usr/local/bin.
# Usage: download_and_install_binary "url" "binary_name" [tarball|binary]
#   download_type: "tarball" (default) extracts .tar.gz, "binary" is a raw executable
download_and_install_binary() {
    local url="$1"
    local binary_name="$2"
    local download_type="${3:-tarball}"

    local tmp_dir
    tmp_dir=$(mktemp -d)

    ui_info "Downloading $binary_name..."

    if [[ "$download_type" == "binary" ]]; then
        local tmp_file="$tmp_dir/$binary_name"
        if ! curl -sL "$url" -o "$tmp_file"; then
            ui_warn "Failed to download $binary_name"
            rm -rf "$tmp_dir"
            return 1
        fi
        sudo install -m 755 "$tmp_file" "/usr/local/bin/$binary_name"
    else
        if ! curl -sL "$url" | tar -xz -C "$tmp_dir"; then
            ui_warn "Failed to download $binary_name"
            rm -rf "$tmp_dir"
            return 1
        fi

        local bin_path
        bin_path=$(find "$tmp_dir" -name "$binary_name" -type f 2>/dev/null | head -1)

        # Retry without executable check (permissions might not be set)
        if [[ -z "$bin_path" ]]; then
            bin_path=$(find "$tmp_dir" -name "$binary_name" -type f 2>/dev/null | head -1)
        fi

        if [[ -z "$bin_path" ]]; then
            ui_warn "Failed to find $binary_name binary in archive"
            rm -rf "$tmp_dir"
            return 1
        fi

        sudo install -m 755 "$bin_path" "/usr/local/bin/$binary_name"
    fi

    ui_info "Installed $binary_name to /usr/local/bin/$binary_name"
    rm -rf "$tmp_dir"
}
