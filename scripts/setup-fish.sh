#!/usr/bin/env bash

# =============================================================================
# Fish Shell Setup Script
# =============================================================================
# Sets up Fish shell with Fisher plugin manager and all configurations
# =============================================================================

setup_fish() {
    substep "Starting Fish shell setup"
    
    # Check if Fish is installed
    if ! check_command fish; then
        error "Fish shell is not installed. Run package installation first."
        return 1
    fi
    
    # Backup existing Fish configuration
    backup_file "$HOME/.config/fish"
    
    # Create Fish config directory
    if [[ "$DRY_RUN" == "false" ]]; then
        mkdir -p "$HOME/.config/fish/conf.d"
        mkdir -p "$HOME/.config/fish/functions"
    else
        substep "[DRY RUN] Would create Fish config directories"
    fi
    
    # Copy Fish configurations
    copy_fish_configs
    
    # Install Fisher plugin manager
    install_fisher
    
    # Install Fish plugins
    install_fish_plugins
    
    # Set up Fish universal variables
    setup_fish_variables
    
    # Install NVM and set default Node version
    setup_nvm_fish
    
    success "Fish shell setup completed"
}

copy_fish_configs() {
    substep "Copying Fish configurations..."
    
    local fish_config_files=(
        "config.fish"
        "conf.d/abbr.fish"
        "conf.d/paths.fish"
        "conf.d/tmux-mgmt.fish"
        "fish_plugins"
    )
    
    for config_file in "${fish_config_files[@]}"; do
        local source_file="$SCRIPT_DIR/configs/fish/$config_file"
        local dest_file="$HOME/.config/fish/$config_file"
        
        if [[ -f "$source_file" ]]; then
            if [[ "$DRY_RUN" == "false" ]]; then
                mkdir -p "$(dirname "$dest_file")"
                cp "$source_file" "$dest_file"
                substep "Copied $config_file"
            else
                substep "[DRY RUN] Would copy $config_file"
            fi
        else
            warning "Configuration file not found: $source_file"
        fi
    done
}

install_fisher() {
    substep "Installing Fisher plugin manager..."
    
    if [[ "$DRY_RUN" == "false" ]]; then
        # Install Fisher
        fish -c "curl -sL https://raw.githubusercontent.com/jorgebucaran/fisher/main/functions/fisher.fish | source && fisher install jorgebucaran/fisher"
    else
        substep "[DRY RUN] Would install Fisher plugin manager"
    fi
}

install_fish_plugins() {
    substep "Installing Fish plugins..."
    
    local plugins=(
        "jorgebucaran/nvm.fish"
        "patrickf1/fzf.fish"
    )
    
    if [[ "$DRY_RUN" == "false" ]]; then
        for plugin in "${plugins[@]}"; do
            substep "Installing plugin: $plugin"
            fish -c "fisher install $plugin"
        done
    else
        for plugin in "${plugins[@]}"; do
            substep "[DRY RUN] Would install plugin: $plugin"
        done
    fi
}

setup_fish_variables() {
    substep "Setting up Fish universal variables..."
    
    if [[ "$DRY_RUN" == "false" ]]; then
        # Set Fish colors (based on your current theme)
        fish -c "set -U fish_color_autosuggestion brblack"
        fish -c "set -U fish_color_cancel -r"
        fish -c "set -U fish_color_command normal"
        fish -c "set -U fish_color_comment red"
        fish -c "set -U fish_color_cwd green"
        fish -c "set -U fish_color_cwd_root red"
        fish -c "set -U fish_color_end green"
        fish -c "set -U fish_color_error brred"
        fish -c "set -U fish_color_escape brcyan"
        fish -c "set -U fish_color_history_current --bold"
        fish -c "set -U fish_color_host normal"
        fish -c "set -U fish_color_host_remote yellow"
        fish -c "set -U fish_color_normal normal"
        fish -c "set -U fish_color_operator brcyan"
        fish -c "set -U fish_color_param cyan"
        fish -c "set -U fish_color_quote yellow"
        fish -c "set -U fish_color_redirection 'cyan --bold'"
        fish -c "set -U fish_color_search_match 'white --background=brblack'"
        fish -c "set -U fish_color_selection 'white --bold --background=brblack'"
        fish -c "set -U fish_color_status red"
        fish -c "set -U fish_color_user brgreen"
        fish -c "set -U fish_color_valid_path --underline"
        
        # Disable greeting
        fish -c "set -U fish_greeting ''"
        
        # Set key bindings
        fish -c "set -U fish_key_bindings fish_default_key_bindings"
        
        # Set pager colors
        fish -c "set -U fish_pager_color_completion normal"
        fish -c "set -U fish_pager_color_description 'yellow -i'"
        fish -c "set -U fish_pager_color_prefix 'normal --bold --underline'"
        fish -c "set -U fish_pager_color_progress 'brwhite --background=cyan'"
        fish -c "set -U fish_pager_color_selected_background -r"
        
        substep "Fish universal variables configured"
    else
        substep "[DRY RUN] Would configure Fish universal variables"
    fi
}

setup_nvm_fish() {
    substep "Setting up NVM for Fish..."
    
    if [[ "$DRY_RUN" == "false" ]]; then
        # Install latest LTS Node.js
        fish -c "nvm install lts"
        fish -c "nvm use lts"
        
        # Set default version (matching your current setup)
        fish -c "set -U nvm_default_version lts"
        
        substep "Node.js LTS installed and set as default"
    else
        substep "[DRY RUN] Would install Node.js LTS via nvm"
    fi
}

# Function to validate Fish setup
validate_fish_setup() {
    substep "Validating Fish setup..."
    
    local validation_passed=true
    
    # Check if Fish is available
    if ! check_command fish; then
        error "Fish shell is not available"
        validation_passed=false
    fi
    
    # Check if config files exist
    local required_files=(
        "$HOME/.config/fish/config.fish"
        "$HOME/.config/fish/conf.d/abbr.fish"
        "$HOME/.config/fish/fish_plugins"
    )
    
    for file in "${required_files[@]}"; do
        if [[ ! -f "$file" ]]; then
            error "Required file missing: $file"
            validation_passed=false
        fi
    done
    
    # Check if Fisher is installed
    if [[ "$DRY_RUN" == "false" ]]; then
        if ! fish -c "type -q fisher" 2>/dev/null; then
            error "Fisher plugin manager is not installed"
            validation_passed=false
        fi
    fi
    
    if [[ "$validation_passed" == "true" ]]; then
        success "Fish setup validation passed"
    else
        error "Fish setup validation failed"
        return 1
    fi
}