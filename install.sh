#!/usr/bin/env bash
# =============================================================================
# dotfiles bootstrap
# =============================================================================
# Downloads the dotsetup binary and launches the interactive TUI installer.
#
# Usage:
#   git clone https://github.com/chaseddevelopment/dotfiles ~/dotfiles
#   cd ~/dotfiles && ./install.sh
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BINARY="$SCRIPT_DIR/installer/dotsetup"
REPO="chaseddevelopment/dotfiles"

# Detect OS.
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$OS" in
    darwin|linux) ;;
    *) echo "Error: unsupported OS: $OS" >&2; exit 1 ;;
esac

# Detect architecture.
ARCH="$(uname -m)"
case "$ARCH" in
    x86_64|amd64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *) echo "Error: unsupported architecture: $ARCH" >&2; exit 1 ;;
esac

ASSET="dotsetup-${OS}-${ARCH}"

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

# Fetch the latest release tag from GitHub (e.g. "v0.0.1").
get_latest_tag() {
    local url
    url=$(curl -fsSL --connect-timeout 10 --max-time 30 -o /dev/null -w '%{url_effective}' \
        "https://github.com/${REPO}/releases/latest" 2>/dev/null) || return 1
    # URL must contain /tag/ — otherwise there are no releases.
    [[ "$url" == */tag/* ]] || return 1
    echo "${url##*/}"
}

# Extract the version string from the installed binary.
get_local_version() {
    "$BINARY" --version 2>/dev/null | awk '{print $2}'
}

# Download the binary for a given release tag.
download_binary() {
    local tag="$1"
    local url="https://github.com/${REPO}/releases/download/${tag}/${ASSET}"
    echo "Downloading dotsetup ${tag} for ${OS}/${ARCH}..."
    mkdir -p "$(dirname "$BINARY")"
    if command -v curl >/dev/null 2>&1; then
        curl -fsSL --connect-timeout 10 --max-time 120 --retry 2 --retry-delay 3 \
            "$url" -o "$BINARY"
    elif command -v wget >/dev/null 2>&1; then
        wget --timeout=30 --tries=3 -qO "$BINARY" "$url"
    else
        echo "Error: curl or wget required" >&2
        return 1
    fi
    chmod +x "$BINARY"
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------

NEED_DOWNLOAD=false
LATEST_TAG=""

if LATEST_TAG=$(get_latest_tag); then
    if [[ -x "$BINARY" ]]; then
        LOCAL_VERSION=$(get_local_version)
        if [[ "$LOCAL_VERSION" != "$LATEST_TAG" ]]; then
            echo "Update available: ${LOCAL_VERSION} -> ${LATEST_TAG}"
            NEED_DOWNLOAD=true
        fi
    else
        NEED_DOWNLOAD=true
    fi
elif [[ -x "$BINARY" ]]; then
    echo "Note: Could not check for updates (offline?). Using cached binary." >&2
else
    echo "Error: Cannot download dotsetup and no cached binary found." >&2
    echo "" >&2
    echo "Build from source:" >&2
    echo "  cd installer && go build -o dotsetup . && cd .. && ./install.sh" >&2
    exit 1
fi

if [[ "$NEED_DOWNLOAD" == true ]]; then
    if ! download_binary "$LATEST_TAG"; then
        if [[ -x "$BINARY" ]]; then
            echo "Warning: Download failed. Using cached binary." >&2
        else
            echo "Error: Download failed and no cached binary found." >&2
            echo "" >&2
            echo "Build from source:" >&2
            echo "  cd installer && go build -o dotsetup . && cd .. && ./install.sh" >&2
            exit 1
        fi
    fi
fi

"$BINARY"
code=$?

# Echo the exit code once so a later debug session can grep for it
# in the user's scrollback or CI log without relaunching. Matches
# the "dotsetup exiting:" line the Go binary writes to install.log.
echo "install.sh: dotsetup exit=$code" >&2

# Exit 10 is our private "reload shell please" contract from the TUI
# (see installer/main.go). Anything else non-zero is a genuine error.
if [ "$code" -ne 0 ] && [ "$code" -ne 10 ]; then
    exit "$code"
fi

# Only reload when the TUI asked for it AND we have a live terminal.
# Skip on CI, `| tee`, or redirected invocations — they'd get stuck
# waiting on a child shell they don't control.
if [ "$code" -ne 10 ] || ! [ -t 0 ] || ! [ -t 1 ]; then
    exit 0
fi

# Prefer zsh — that's what configs/ is built around. Fall back to
# $SHELL only when zsh isn't available (partial-failure install).
if command -v zsh >/dev/null 2>&1; then
    target_shell="zsh"
elif [ -n "${SHELL:-}" ]; then
    target_shell="$SHELL"
else
    exit 0
fi

exec "$target_shell" -l
