#!/usr/bin/env bash

# =============================================================================
# Yazi Setup Script
# =============================================================================
# Sets up Yazi terminal file manager with configuration and plugins
# =============================================================================

setup_yazi() {
    substep "Starting Yazi setup"

    # Check if yazi is installed
    if ! check_command yazi; then
        warning "Yazi is not installed. It should be installed via install-packages.sh"
        return 0  # Don't fail the entire install for optional tool
    fi

    # Backup existing config
    backup_file "$HOME/.config/yazi"

    # Copy Yazi configuration
    copy_yazi_config

    # Install plugins from package.toml
    install_yazi_plugins

    success "Yazi setup completed"
}

copy_yazi_config() {
    substep "Symlinking Yazi configuration..."

    local source_dir="$SCRIPT_DIR/configs/yazi"
    local dest_dir="$HOME/.config/yazi"

    if [[ -d "$source_dir" ]]; then
        symlink_if_needed "$source_dir" "$dest_dir"
    else
        warning "Yazi config not found: $source_dir"
    fi
}

install_yazi_plugins() {
    substep "Installing Yazi plugins from package.toml..."

    if [[ "$DRY_RUN" == "true" ]]; then
        substep "[DRY RUN] Would install Yazi plugins via 'ya pkg install'"
        return
    fi

    # ya pkg install reads package.toml and installs all listed plugins/flavors
    if check_command ya; then
        ya pkg install || warning "Failed to install some Yazi plugins"
    else
        warning "'ya' CLI not found — plugins will need to be installed manually"
        warning "Run 'ya pkg install' after yazi is available"
    fi
}
