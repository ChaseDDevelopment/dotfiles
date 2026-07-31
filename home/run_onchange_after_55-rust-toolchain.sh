#!/bin/sh
# tools: rustup stable toolchain

set -eu

PATH="$HOME/.cargo/bin:/opt/homebrew/bin:/usr/local/bin:$PATH"

# The rustup package (brew/pacman) ships only the rustup binary; a fresh
# machine has no toolchain, so cargo is missing and blink.cmp's Rust build
# in the neovim hook fails. Install the stable toolchain once.
if command -v cargo >/dev/null 2>&1; then
    printf '%s\n' 'rust toolchain: cargo present; skipping'
elif command -v rustup >/dev/null 2>&1; then
    rustup default stable
else
    printf '%s\n' 'rust toolchain: rustup not found; skipping (blink.cmp falls back to prebuilt or lua)'
fi
