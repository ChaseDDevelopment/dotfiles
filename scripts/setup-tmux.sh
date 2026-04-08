#!/usr/bin/env bash

# =============================================================================
# Tmux Setup Script
# =============================================================================
# Sets up Tmux with TPM (Tmux Plugin Manager) and all plugins
# =============================================================================

setup_tmux() {
    ui_info "Starting Tmux setup"

    # Check if Tmux is installed
    if ! check_command tmux; then
        ui_error "Tmux is not installed. Run package installation first."
        return 1
    fi
    
    # Backup existing Tmux configuration
    backup_file "$HOME/.config/tmux"
    backup_file "$HOME/.tmux.conf"
    backup_file "$HOME/.tmux"
    
    # Copy Tmux configuration
    copy_tmux_config
    
    # Install TPM (Tmux Plugin Manager)
    install_tpm
    
    # Install Tmux plugins
    install_tmux_plugins
    
    # Theme will be applied automatically by TPM
    
    ui_success "Tmux setup completed"
}

copy_tmux_config() {
    ui_info "Symlinking Tmux configuration..."

    local source_dir="$SCRIPT_DIR/configs/tmux"
    local dest_dir="$HOME/.config/tmux"
    local legacy_dest="$HOME/.tmux.conf"

    if [[ ! -d "$source_dir" ]]; then
        ui_error "Tmux configuration directory not found: $source_dir"
        return 1
    fi

    # Symlink the tmux config directory to XDG path
    symlink_if_needed "$source_dir" "$dest_dir"

    # Create legacy symlink for backward compatibility
    symlink_if_needed "$dest_dir/tmux.conf" "$legacy_dest"
}

install_tpm() {
    ui_info "Installing TPM (Tmux Plugin Manager)..."

    local tpm_dir="$HOME/.tmux/plugins/tpm"

    if [[ -d "$tpm_dir" ]]; then
        ui_info "TPM is already installed"
        return
    fi
    
    if [[ "$DRY_RUN" == "false" ]]; then
        git clone https://github.com/tmux-plugins/tpm "$tpm_dir"
        ui_info "TPM installed successfully"
    else
        ui_info "[DRY RUN] Would clone TPM repository"
    fi
}

install_tmux_plugins() {
    ui_info "Installing Tmux plugins..."
    
    if [[ "$DRY_RUN" == "false" ]]; then
        # Source the tmux configuration to make sure TPM is loaded
        if pgrep -x "tmux" > /dev/null; then
            # If tmux is running, reload configuration
            tmux source-file "$HOME/.config/tmux/tmux.conf" 2>/dev/null || true
        fi
        
        # Install plugins using TPM
        local tpm_script="$HOME/.tmux/plugins/tpm/scripts/install_plugins.sh"
        if [[ -f "$tpm_script" ]]; then
            # Make sure the script is executable
            chmod +x "$tpm_script"

            # Set TMUX_PLUGIN_MANAGER_PATH in tmux's environment (TPM reads it via tmux show-environment)
            # Start server, source config (which sets the env var), then the install script can read it
            tmux start-server \; source-file "$HOME/.config/tmux/tmux.conf" 2>/dev/null || true

            # Run the install script
            "$tpm_script"
            ui_info "Tmux plugins installed"
            
            # Force reload configuration to apply theme
            if pgrep -x "tmux" > /dev/null; then
                tmux source-file "$HOME/.config/tmux/tmux.conf" 2>/dev/null || true
                ui_info "Tmux configuration reloaded"
            fi
        else
            ui_warn "TPM install script not found. Plugins will be installed on next tmux session."
        fi
    else
        ui_info "[DRY RUN] Would install Tmux plugins"
    fi
}


# Function to validate Tmux setup
validate_tmux_setup() {
    ui_info "Validating Tmux setup..."
    
    local validation_passed=true
    
    # Check if Tmux is available
    if ! check_command tmux; then
        ui_error "Tmux is not available"
        validation_passed=false
    fi
    
    # Check if config file exists
    if [[ ! -f "$HOME/.config/tmux/tmux.conf" ]]; then
        ui_error "Tmux configuration file missing: $HOME/.config/tmux/tmux.conf"
        validation_passed=false
    fi
    
    # Check if TPM directory exists
    if [[ ! -d "$HOME/.tmux/plugins/tpm" ]]; then
        ui_error "TPM directory missing: $HOME/.tmux/plugins/tpm"
        validation_passed=false
    fi
    
    # Validate tmux configuration syntax
    if [[ "$DRY_RUN" == "false" ]]; then
        if ! tmux -f "$HOME/.config/tmux/tmux.conf" list-sessions 2>/dev/null; then
            # This is expected to fail if no sessions exist, but will catch syntax errors
            if ! tmux -f "$HOME/.config/tmux/tmux.conf" new-session -d -s validation_test \; kill-session -t validation_test 2>/dev/null; then
                ui_error "Tmux configuration has syntax errors"
                validation_passed=false
            fi
        fi
    fi
    
    if [[ "$validation_passed" == "true" ]]; then
        ui_success "Tmux setup validation passed"
    else
        ui_error "Tmux setup validation failed"
        return 1
    fi
}


# Function to start tmux with plugin installation
start_tmux_with_plugins() {
    ui_info "Starting Tmux session to complete plugin installation..."
    
    if [[ "$DRY_RUN" == "false" ]]; then
        # Create a temporary session to trigger plugin installation
        tmux new-session -d -s setup_session
        
        # Install plugins
        tmux send-keys -t setup_session "tmux run-shell '~/.tmux/plugins/tpm/scripts/install_plugins.sh'" C-m
        
        # Wait a moment for installation
        sleep 3
        
        # Kill the temporary session
        tmux kill-session -t setup_session 2>/dev/null || true
        
        ui_info "Plugin installation completed"
    else
        ui_info "[DRY RUN] Would start tmux session for plugin installation"
    fi
}

# Function to display Tmux key bindings info
show_tmux_info() {
    echo
    ui_info "Tmux is configured with the following key bindings:"
    ui_info "  Prefix: Ctrl+Space (instead of Ctrl+b)"
    ui_info "  Split horizontal: Prefix + %"
    ui_info "  Split vertical: Prefix + \""
    ui_info "  New window: Prefix + c"
    ui_info "  Previous window: Alt+H"
    ui_info "  Next window: Alt+L"
    ui_info "  Copy mode: Prefix + [ (vi-mode enabled)"
    echo
    ui_info "Installed plugins:"
    ui_info "  - tmux-sensible (better defaults)"
    ui_info "  - vim-tmux-navigator (seamless vim/tmux navigation)"
    ui_info "  - tokyo-night-tmux (beautiful Tokyo Night theme)"
    ui_info "  - tmux-yank (copy to system clipboard)"
    echo
}