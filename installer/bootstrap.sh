#!/usr/bin/env bash
# =============================================================================
# dotsetup bootstrap script
# =============================================================================
# Downloads the correct dotsetup binary for this platform from GitHub releases.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/ChaseOdevelopment/dotfiles/main/installer/bootstrap.sh | bash
# =============================================================================
set -euo pipefail

REPO="chaseddevelopment/dotfiles"
INSTALL_DIR="${DOTSETUP_INSTALL_DIR:-$HOME/.local/bin}"

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

BINARY="dotsetup-${OS}-${ARCH}"
URL="https://github.com/${REPO}/releases/latest/download/${BINARY}"

echo "Downloading dotsetup for ${OS}/${ARCH}..."

mkdir -p "$INSTALL_DIR"
DEST="${INSTALL_DIR}/dotsetup"

if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$URL" -o "$DEST"
elif command -v wget >/dev/null 2>&1; then
    wget -qO "$DEST" "$URL"
else
    echo "Error: curl or wget required" >&2
    exit 1
fi

chmod +x "$DEST"

echo "dotsetup installed to $DEST"

# Check if install dir is in PATH.
case ":$PATH:" in
    *":${INSTALL_DIR}:"*) ;;
    *) echo "Add to your PATH: export PATH=\"${INSTALL_DIR}:\$PATH\"" ;;
esac

echo "Run: dotsetup"
