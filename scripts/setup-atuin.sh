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
        return 0  # Don't fail the entire install for optional tool
    fi

    # Backup existing config
    backup_file "$HOME/.config/atuin/config.toml"

    # Copy Atuin configuration
    copy_atuin_config

    success "Atuin setup completed"
}

copy_atuin_config() {
    substep "Symlinking Atuin configuration..."

    local source_dir="$SCRIPT_DIR/configs/atuin"
    local dest_dir="$HOME/.config/atuin"

    if [[ "$DRY_RUN" == "false" ]]; then
        if [[ -d "$source_dir" ]]; then
            # Remove existing config directory/symlink
            if [[ -e "$dest_dir" ]] || [[ -L "$dest_dir" ]]; then
                rm -rf "$dest_dir"
            fi
            ln -s "$source_dir" "$dest_dir"
            substep "Atuin config symlinked: $dest_dir -> $source_dir"
        else
            warning "Atuin config not found: $source_dir"
        fi
    else
        substep "[DRY RUN] Would symlink Atuin config"
    fi
}
