#!/usr/bin/env bash

# =============================================================================
# Atuin Setup Script
# =============================================================================
# Sets up Atuin shell history tool with configuration
# =============================================================================

setup_atuin() {
    ui_info "Starting Atuin setup"

    # Check if atuin is installed
    if ! check_command atuin; then
        ui_warn "Atuin is not installed. It should be installed via install-tools.sh"
        return 0  # Don't fail the entire install for optional tool
    fi

    # Backup existing config
    backup_file "$HOME/.config/atuin/config.toml"

    # Copy Atuin configuration
    copy_atuin_config

    ui_success "Atuin setup completed"
}

copy_atuin_config() {
    ui_info "Symlinking Atuin configuration..."

    local source_dir="$SCRIPT_DIR/configs/atuin"
    local dest_dir="$HOME/.config/atuin"

    if [[ -d "$source_dir" ]]; then
        symlink_if_needed "$source_dir" "$dest_dir"
    else
        ui_warn "Atuin config not found: $source_dir"
    fi
}
