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
    substep "Symlinking Ghostty configuration..."

    local source_dir="$SCRIPT_DIR/configs/ghostty"
    local dest_dir="$HOME/.config/ghostty"

    if [[ "$DRY_RUN" == "false" ]]; then
        if [[ -d "$source_dir" ]]; then
            # Remove existing config directory/symlink
            if [[ -e "$dest_dir" ]] || [[ -L "$dest_dir" ]]; then
                rm -rf "$dest_dir"
            fi
            ln -s "$source_dir" "$dest_dir"
            substep "Ghostty config symlinked: $dest_dir -> $source_dir"
        else
            warning "Ghostty config not found: $source_dir"
        fi
    else
        substep "[DRY RUN] Would symlink Ghostty config"
    fi
}
