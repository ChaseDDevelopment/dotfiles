#!/usr/bin/env bash

# =============================================================================
# Ghostty Setup Script
# =============================================================================
# Sets up Ghostty terminal emulator configuration (desktop only)
# =============================================================================

setup_ghostty() {
    substep "Starting Ghostty setup"

    # Skip on headless/server systems
    if ! is_desktop_environment; then
        info "Skipping Ghostty setup (no desktop environment detected)"
        return 0
    fi

    # Backup existing config
    backup_file "$HOME/.config/ghostty/config"

    # Copy Ghostty configuration
    copy_ghostty_config

    success "Ghostty setup completed"
}

is_desktop_environment() {
    # Check for display server or macOS
    # Use ${VAR:-} syntax for potentially unset variables (required with set -u)
    [[ -n "${DISPLAY:-}" ]] || [[ -n "${WAYLAND_DISPLAY:-}" ]] || [[ "$OSTYPE" == "darwin"* ]]
}

copy_ghostty_config() {
    substep "Copying Ghostty configuration..."

    local source="$SCRIPT_DIR/configs/ghostty/config"
    local dest_dir="$HOME/.config/ghostty"
    local dest="$dest_dir/config"

    if [[ "$DRY_RUN" == "false" ]]; then
        mkdir -p "$dest_dir"

        if [[ -f "$source" ]]; then
            cp "$source" "$dest"
            substep "Ghostty config copied to $dest"
        else
            warning "Ghostty config not found: $source"
        fi
    else
        substep "[DRY RUN] Would copy Ghostty config"
    fi
}
