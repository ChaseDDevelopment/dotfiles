#!/bin/sh
# tools: rustup stable toolchain

set -eu

PATH="$HOME/.cargo/bin:/opt/homebrew/bin:/usr/local/bin:$PATH"

# blink.cmp's fuzzy matcher (frizbee) uses let-chains, stable since 1.88.
BLINK_RUST_FLOOR=88

rustc_minor() {
    rustc --version 2>/dev/null |
        sed -n 's/^rustc 1\.\([0-9][0-9]*\)\..*/\1/p'
}

# The rustup package (brew/pacman) ships only the rustup binary; a fresh
# machine has no toolchain, so cargo is missing and blink.cmp's Rust build
# in the neovim hook fails. Install the stable toolchain once, and refresh
# a rustup-managed stable that has aged below the blink floor (the one
# narrow upgrade hooks perform; distro toolchains are never touched).
if command -v rustup >/dev/null 2>&1; then
    if ! command -v cargo >/dev/null 2>&1; then
        rustup default stable
    fi
    minor=$(rustc_minor)
    if [ -n "$minor" ] && [ "$minor" -lt "$BLINK_RUST_FLOOR" ]; then
        default_toolchain=$(rustup default 2>/dev/null | cut -d' ' -f1)
        case $default_toolchain in
            stable-*)
                printf '%s\n' "rust toolchain: stable 1.$minor < 1.$BLINK_RUST_FLOOR; running rustup update stable"
                rustup update stable
                ;;
            *)
                # A versioned default (CI runners pin these) is deliberate;
                # never override it. blink.cmp uses its Lua fallback there.
                printf '%s\n' "rust toolchain: default $default_toolchain is 1.$minor < 1.$BLINK_RUST_FLOOR but pinned; leaving it (blink.cmp uses Lua fallback)"
                ;;
        esac
    fi
elif command -v cargo >/dev/null 2>&1; then
    printf '%s\n' 'rust toolchain: distro cargo present; skipping'
else
    printf '%s\n' 'rust toolchain: rustup not found; skipping (blink.cmp falls back to prebuilt or lua)'
fi
