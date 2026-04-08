#!/usr/bin/env bash

# =============================================================================
# Git Setup Script
# =============================================================================
# Sets up Git configuration (delta pager) and lazygit (Catppuccin theme)
# =============================================================================

setup_git() {
    ui_info "Starting Git config setup"

    # Setup git config (delta pager, merge settings)
    setup_git_config

    # Setup lazygit config (Catppuccin theme)
    setup_lazygit_config

    ui_success "Git config setup completed"
}

setup_git_config() {
    ui_info "Symlinking Git configuration..."

    local source_dir="$SCRIPT_DIR/configs/git"
    local dest_dir="$HOME/.config/git"

    # Symlink individual files (not the whole dir) to preserve existing files like ignore
    if [[ -d "$source_dir" ]]; then
        mkdir -p "$dest_dir"
        backup_file "$dest_dir/config"
        symlink_if_needed "$source_dir/config" "$dest_dir/config"
    else
        ui_warn "Git config not found: $source_dir"
    fi
}

setup_lazygit_config() {
    ui_info "Symlinking lazygit configuration..."

    local source_dir="$SCRIPT_DIR/configs/lazygit"
    local dest_dir="$HOME/.config/lazygit"

    if [[ -d "$source_dir" ]]; then
        backup_file "$dest_dir"
        symlink_if_needed "$source_dir" "$dest_dir"
    else
        ui_warn "Lazygit config not found: $source_dir"
    fi
}
