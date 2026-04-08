#!/usr/bin/env bash

# =============================================================================
# Starship Prompt Setup Script
# =============================================================================
# Sets up Starship prompt with Catppuccin theme
# =============================================================================

readonly STARSHIP_CONFIG_DIR="$HOME/.config"
readonly STARSHIP_CONFIG_FILE="$STARSHIP_CONFIG_DIR/starship.toml"

setup_starship() {
    ui_info "Starting Starship prompt setup"

    # Check if Starship is installed
    if ! check_command starship; then
        ui_error "Starship is not installed. Run package installation first."
        return 1
    fi
    
    # Backup existing Starship configuration
    backup_starship_config
    
    # Copy or generate Starship configuration
    setup_starship_config
    
    # Configure shell integration
    setup_shell_integration
    
    ui_success "Starship prompt setup completed"
}

backup_starship_config() {
    ui_info "Backing up existing Starship configuration..."
    
    backup_file "$STARSHIP_CONFIG_FILE"
}

setup_starship_config() {
    ui_info "Setting up Starship configuration..."
    
    # Create config directory
    if [[ "$DRY_RUN" == "false" ]]; then
        mkdir -p "$STARSHIP_CONFIG_DIR"
    fi
    
    # Check if custom config exists in our repo
    local custom_config="$SCRIPT_DIR/configs/starship/starship.toml"
    
    if [[ -f "$custom_config" ]]; then
        # Use custom configuration (symlink for auto-sync to repo)
        ui_info "Symlinking custom Starship configuration..."
        symlink_if_needed "$custom_config" "$STARSHIP_CONFIG_FILE"
    else
        # Generate Catppuccin preset configuration
        ui_info "Generating Catppuccin preset configuration..."
        generate_catppuccin_config
    fi
}

generate_catppuccin_config() {
    if [[ "$DRY_RUN" == "false" ]]; then
        # Use Starship's preset command to generate Catppuccin config
        ui_spin "Generating Catppuccin preset..." \
            starship preset catppuccin-powerline -o "$STARSHIP_CONFIG_FILE"
    else
        ui_info "[DRY RUN] Would generate Catppuccin preset configuration"
    fi
}

setup_shell_integration() {
    ui_info "Setting up shell integration..."

    # Zsh shell integration is handled in our zsh config (.zshrc)
    # The config includes: eval "$(starship init zsh)"
    # Just show integration info for reference
    show_shell_integration_info
}

show_shell_integration_info() {
    echo
    ui_info "Shell integration instructions:"
    echo
    ui_info "Zsh (already configured):"
    ui_info "  eval \"\$(starship init zsh)\""
    echo
    ui_info "Bash:"
    ui_info "  Add to ~/.bashrc: eval \"\$(starship init bash)\""
    echo
}

# Function to validate Starship setup
validate_starship_setup() {
    ui_info "Validating Starship setup..."
    
    local validation_passed=true
    
    # Check if Starship is available
    if ! check_command starship; then
        ui_error "Starship is not available"
        validation_passed=false
    fi
    
    # Check if config file exists
    if [[ ! -f "$STARSHIP_CONFIG_FILE" ]]; then
        ui_error "Starship configuration file missing: $STARSHIP_CONFIG_FILE"
        validation_passed=false
    fi
    
    # Validate configuration syntax
    if [[ "$DRY_RUN" == "false" ]]; then
        if ! starship config 2>/dev/null | grep -q "format"; then
            ui_warn "Starship configuration may have syntax issues"
        fi
    fi
    
    if [[ "$validation_passed" == "true" ]]; then
        ui_success "Starship setup validation passed"
    else
        ui_error "Starship setup validation failed"
        return 1
    fi
}

# Function to show available presets
show_starship_presets() {
    echo
    ui_info "Available Starship presets (you can change anytime):"
    ui_info "  starship preset catppuccin-powerline - Current preset"
    ui_info "  starship preset nerd-font-symbols - Nerd font symbols"
    ui_info "  starship preset no-nerd-font - No nerd fonts required"
    ui_info "  starship preset minimal - Minimal prompt"
    ui_info "  starship preset pure-preset - Pure-like prompt"
    echo
    ui_info "To change preset:"
    ui_info "  starship preset [preset-name] -o ~/.config/starship.toml"
    echo
}

# Function to display Starship info
show_starship_info() {
    echo
    ui_info "Starship prompt is configured with:"
    ui_info "  Theme: Catppuccin Powerline"
    ui_info "  Config: $STARSHIP_CONFIG_FILE"
    echo
    ui_info "Features enabled:"
    ui_info "  - Git status and branch information"
    ui_info "  - Programming language detection"
    ui_info "  - Directory information"
    ui_info "  - Command execution time"
    ui_info "  - Error status indicators"
    echo
    show_starship_presets
}