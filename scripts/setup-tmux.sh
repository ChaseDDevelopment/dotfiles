#!/usr/bin/env bash

# =============================================================================
# Tmux Setup Script
# =============================================================================
# Sets up Tmux with TPM (Tmux Plugin Manager) and all plugins
# =============================================================================

setup_tmux() {
    substep "Starting Tmux setup"
    
    # Check if Tmux is installed
    if ! check_command tmux; then
        error "Tmux is not installed. Run package installation first."
        return 1
    fi
    
    # Backup existing Tmux configuration
    backup_file "$HOME/.tmux.conf"
    backup_file "$HOME/.tmux"
    
    # Copy Tmux configuration
    copy_tmux_config
    
    # Install TPM (Tmux Plugin Manager)
    install_tpm
    
    # Install Tmux plugins
    install_tmux_plugins
    
    success "Tmux setup completed"
}

copy_tmux_config() {
    substep "Copying Tmux configuration..."
    
    local source_file="$SCRIPT_DIR/configs/tmux/.tmux.conf"
    local dest_file="$HOME/.tmux.conf"
    
    if [[ -f "$source_file" ]]; then
        if [[ "$DRY_RUN" == "false" ]]; then
            cp "$source_file" "$dest_file"
            substep "Copied .tmux.conf"
        else
            substep "[DRY RUN] Would copy .tmux.conf"
        fi
    else
        error "Tmux configuration file not found: $source_file"
        return 1
    fi
}

install_tpm() {
    substep "Installing TPM (Tmux Plugin Manager)..."
    
    local tpm_dir="$HOME/.tmux/plugins/tpm"
    
    if [[ -d "$tpm_dir" ]]; then
        substep "TPM is already installed"
        return
    fi
    
    if [[ "$DRY_RUN" == "false" ]]; then
        git clone https://github.com/tmux-plugins/tpm "$tpm_dir"
        substep "TPM installed successfully"
    else
        substep "[DRY RUN] Would clone TPM repository"
    fi
}

install_tmux_plugins() {
    substep "Installing Tmux plugins..."
    
    if [[ "$DRY_RUN" == "false" ]]; then
        # Source the tmux configuration to make sure TPM is loaded
        if pgrep -x "tmux" > /dev/null; then
            # If tmux is running, reload configuration
            tmux source-file "$HOME/.tmux.conf" 2>/dev/null || true
        fi
        
        # Install plugins using TPM
        local tpm_script="$HOME/.tmux/plugins/tpm/scripts/install_plugins.sh"
        if [[ -f "$tpm_script" ]]; then
            # Make sure the script is executable
            chmod +x "$tpm_script"
            
            # Run the install script
            "$tpm_script"
            substep "Tmux plugins installed"
        else
            warning "TPM install script not found. Plugins will be installed on next tmux session."
        fi
    else
        substep "[DRY RUN] Would install Tmux plugins"
    fi
}

# Function to validate Tmux setup
validate_tmux_setup() {
    substep "Validating Tmux setup..."
    
    local validation_passed=true
    
    # Check if Tmux is available
    if ! check_command tmux; then
        error "Tmux is not available"
        validation_passed=false
    fi
    
    # Check if config file exists
    if [[ ! -f "$HOME/.tmux.conf" ]]; then
        error "Tmux configuration file missing: $HOME/.tmux.conf"
        validation_passed=false
    fi
    
    # Check if TPM directory exists
    if [[ ! -d "$HOME/.tmux/plugins/tpm" ]]; then
        error "TPM directory missing: $HOME/.tmux/plugins/tpm"
        validation_passed=false
    fi
    
    # Validate tmux configuration syntax
    if [[ "$DRY_RUN" == "false" ]]; then
        if ! tmux -f "$HOME/.tmux.conf" list-sessions 2>/dev/null; then
            # This is expected to fail if no sessions exist, but will catch syntax errors
            if ! tmux -f "$HOME/.tmux.conf" new-session -d -s validation_test \; kill-session -t validation_test 2>/dev/null; then
                error "Tmux configuration has syntax errors"
                validation_passed=false
            fi
        fi
    fi
    
    if [[ "$validation_passed" == "true" ]]; then
        success "Tmux setup validation passed"
    else
        error "Tmux setup validation failed"
        return 1
    fi
}

# Function to start tmux with plugin installation
start_tmux_with_plugins() {
    substep "Starting Tmux session to complete plugin installation..."
    
    if [[ "$DRY_RUN" == "false" ]]; then
        # Create a temporary session to trigger plugin installation
        tmux new-session -d -s setup_session
        
        # Install plugins
        tmux send-keys -t setup_session "tmux run-shell '~/.tmux/plugins/tpm/scripts/install_plugins.sh'" C-m
        
        # Wait a moment for installation
        sleep 3
        
        # Kill the temporary session
        tmux kill-session -t setup_session 2>/dev/null || true
        
        substep "Plugin installation completed"
    else
        substep "[DRY RUN] Would start tmux session for plugin installation"
    fi
}

# Function to display Tmux key bindings info
show_tmux_info() {
    echo
    info "Tmux is configured with the following key bindings:"
    echo -e "  ${CYAN}Prefix:${NC} Ctrl+Space (instead of Ctrl+b)"
    echo -e "  ${CYAN}Split horizontal:${NC} Prefix + %"
    echo -e "  ${CYAN}Split vertical:${NC} Prefix + \""
    echo -e "  ${CYAN}New window:${NC} Prefix + c"
    echo -e "  ${CYAN}Previous window:${NC} Alt+H"
    echo -e "  ${CYAN}Next window:${NC} Alt+L"
    echo -e "  ${CYAN}Copy mode:${NC} Prefix + [ (vi-mode enabled)"
    echo
    info "Installed plugins:"
    echo -e "  ${CYAN}•${NC} tmux-sensible (better defaults)"
    echo -e "  ${CYAN}•${NC} vim-tmux-navigator (seamless vim/tmux navigation)"
    echo -e "  ${CYAN}•${NC} catppuccin-tmux (beautiful theme)"
    echo -e "  ${CYAN}•${NC} tmux-yank (copy to system clipboard)"
    echo
}