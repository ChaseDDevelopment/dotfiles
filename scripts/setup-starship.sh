#!/usr/bin/env bash

# =============================================================================
# Starship Prompt Setup Script
# =============================================================================
# Sets up Starship prompt with Catppuccin theme
# =============================================================================

readonly STARSHIP_CONFIG_DIR="$HOME/.config"
readonly STARSHIP_CONFIG_FILE="$STARSHIP_CONFIG_DIR/starship.toml"

setup_starship() {
    substep "Starting Starship prompt setup"
    
    # Check if Starship is installed
    if ! check_command starship; then
        error "Starship is not installed. Run package installation first."
        return 1
    fi
    
    # Backup existing Starship configuration
    backup_starship_config
    
    # Copy or generate Starship configuration
    setup_starship_config
    
    # Configure shell integration
    setup_shell_integration
    
    success "Starship prompt setup completed"
}

backup_starship_config() {
    substep "Backing up existing Starship configuration..."
    
    backup_file "$STARSHIP_CONFIG_FILE"
}

setup_starship_config() {
    substep "Setting up Starship configuration..."
    
    # Create config directory
    if [[ "$DRY_RUN" == "false" ]]; then
        mkdir -p "$STARSHIP_CONFIG_DIR"
    fi
    
    # Check if custom config exists in our repo
    local custom_config="$SCRIPT_DIR/configs/starship/starship.toml"
    
    if [[ -f "$custom_config" ]]; then
        # Use custom configuration
        substep "Using custom Starship configuration..."
        if [[ "$DRY_RUN" == "false" ]]; then
            cp "$custom_config" "$STARSHIP_CONFIG_FILE"
        else
            substep "[DRY RUN] Would copy custom starship.toml"
        fi
    else
        # Generate Catppuccin preset configuration
        substep "Generating Catppuccin preset configuration..."
        generate_catppuccin_config
    fi
}

generate_catppuccin_config() {
    if [[ "$DRY_RUN" == "false" ]]; then
        # Use Starship's preset command to generate Catppuccin config
        starship preset catppuccin-powerline -o "$STARSHIP_CONFIG_FILE"
        substep "Generated Catppuccin powerline preset"
    else
        substep "[DRY RUN] Would generate Catppuccin preset configuration"
    fi
}

setup_shell_integration() {
    substep "Setting up shell integration..."
    
    # Fish shell integration should already be in our Fish config
    # But let's verify it's there
    local fish_config="$HOME/.config/fish/config.fish"
    
    if [[ -f "$fish_config" ]]; then
        if grep -q "starship init fish" "$fish_config"; then
            substep "Fish shell integration already configured"
        else
            warning "Fish shell integration not found in config.fish"
            if [[ "$DRY_RUN" == "false" ]]; then
                echo "starship init fish | source" >> "$fish_config"
                substep "Added Fish shell integration"
            else
                substep "[DRY RUN] Would add Fish shell integration"
            fi
        fi
    fi
    
    # Also provide integration info for other shells
    show_shell_integration_info
}

show_shell_integration_info() {
    echo
    info "Shell integration instructions:"
    echo
    echo -e "${CYAN}Fish (already configured):${NC}"
    echo -e "  ${WHITE}starship init fish | source${NC}"
    echo
    echo -e "${CYAN}Bash:${NC}"
    echo -e "  Add to ${WHITE}~/.bashrc${NC}: ${WHITE}eval \"\$(starship init bash)\"${NC}"
    echo
    echo -e "${CYAN}Zsh:${NC}"
    echo -e "  Add to ${WHITE}~/.zshrc${NC}: ${WHITE}eval \"\$(starship init zsh)\"${NC}"
    echo
}

# Function to validate Starship setup
validate_starship_setup() {
    substep "Validating Starship setup..."
    
    local validation_passed=true
    
    # Check if Starship is available
    if ! check_command starship; then
        error "Starship is not available"
        validation_passed=false
    fi
    
    # Check if config file exists
    if [[ ! -f "$STARSHIP_CONFIG_FILE" ]]; then
        error "Starship configuration file missing: $STARSHIP_CONFIG_FILE"
        validation_passed=false
    fi
    
    # Validate configuration syntax
    if [[ "$DRY_RUN" == "false" ]]; then
        if ! starship config 2>/dev/null | grep -q "format"; then
            warning "Starship configuration may have syntax issues"
        fi
    fi
    
    if [[ "$validation_passed" == "true" ]]; then
        success "Starship setup validation passed"
    else
        error "Starship setup validation failed"
        return 1
    fi
}

# Function to show available presets
show_starship_presets() {
    echo
    info "Available Starship presets (you can change anytime):"
    echo -e "  ${CYAN}starship preset catppuccin-powerline${NC} - Current preset"
    echo -e "  ${CYAN}starship preset nerd-font-symbols${NC} - Nerd font symbols"
    echo -e "  ${CYAN}starship preset no-nerd-font${NC} - No nerd fonts required"
    echo -e "  ${CYAN}starship preset minimal${NC} - Minimal prompt"
    echo -e "  ${CYAN}starship preset pure-preset${NC} - Pure-like prompt"
    echo
    info "To change preset:"
    echo -e "  ${WHITE}starship preset [preset-name] -o ~/.config/starship.toml${NC}"
    echo
}

# Function to display Starship info
show_starship_info() {
    echo
    info "Starship prompt is configured with:"
    echo -e "  ${CYAN}Theme:${NC} Catppuccin Powerline"
    echo -e "  ${CYAN}Config:${NC} $STARSHIP_CONFIG_FILE"
    echo
    info "Features enabled:"
    echo -e "  ${CYAN}•${NC} Git status and branch information"
    echo -e "  ${CYAN}•${NC} Programming language detection"
    echo -e "  ${CYAN}•${NC} Directory information"
    echo -e "  ${CYAN}•${NC} Command execution time"
    echo -e "  ${CYAN}•${NC} Error status indicators"
    echo
    show_starship_presets
}