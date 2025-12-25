#!/usr/bin/env bash

# =============================================================================
# Atuin Setup Script
# =============================================================================
# Sets up Atuin shell history tool with configuration
# =============================================================================

setup_atuin() {
    substep "Starting Atuin setup"

    # Check if atuin is installed
    if ! check_command atuin; then
        warning "Atuin is not installed. It should be installed via install-tools.sh"
        return 1
    fi

    # Backup existing config
    backup_file "$HOME/.config/atuin/config.toml"

    # Copy Atuin configuration
    copy_atuin_config

    success "Atuin setup completed"
}

copy_atuin_config() {
    substep "Copying Atuin configuration..."

    local source="$SCRIPT_DIR/configs/atuin/config.toml"
    local dest_dir="$HOME/.config/atuin"
    local dest="$dest_dir/config.toml"

    if [[ "$DRY_RUN" == "false" ]]; then
        mkdir -p "$dest_dir"

        if [[ -f "$source" ]]; then
            cp "$source" "$dest"
            substep "Atuin config copied to $dest"
        else
            warning "Atuin config not found: $source"
        fi
    else
        substep "[DRY RUN] Would copy Atuin config"
    fi
}
