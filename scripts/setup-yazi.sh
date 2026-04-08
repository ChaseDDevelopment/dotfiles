#!/usr/bin/env bash

# =============================================================================
# Yazi Setup Script
# =============================================================================
# Sets up Yazi terminal file manager with configuration and plugins
# =============================================================================

setup_yazi() {
    ui_info "Starting Yazi setup"

    # Check if yazi is installed
    if ! check_command yazi; then
        ui_warn "Yazi is not installed. It should be installed via install-packages.sh"
        return 0  # Don't fail the entire install for optional tool
    fi

    # Backup existing config
    backup_file "$HOME/.config/yazi"

    # Copy Yazi configuration
    copy_yazi_config

    # Install plugins from package.toml
    install_yazi_plugins

    ui_success "Yazi setup completed"
}

copy_yazi_config() {
    ui_info "Symlinking Yazi configuration..."

    local source_dir="$SCRIPT_DIR/configs/yazi"
    local dest_dir="$HOME/.config/yazi"

    if [[ -d "$source_dir" ]]; then
        symlink_if_needed "$source_dir" "$dest_dir"
    else
        ui_warn "Yazi config not found: $source_dir"
    fi
}

install_yazi_plugins() {
    ui_info "Installing Yazi plugins from package.toml..."

    if [[ "$DRY_RUN" == "true" ]]; then
        ui_info "[DRY RUN] Would install Yazi plugins via 'ya pkg install'"
        return
    fi

    # ya pkg install reads package.toml and installs all listed plugins/flavors
    if check_command ya; then
        ui_spin "Installing Yazi plugins..." ya pkg install
    else
        ui_warn "'ya' CLI not found — plugins will need to be installed manually"
        ui_warn "Run 'ya pkg install' after yazi is available"
    fi
}
