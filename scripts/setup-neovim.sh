#!/usr/bin/env bash

# =============================================================================
# Neovim Setup Script
# =============================================================================
# Sets up Neovim with LazyVim configuration from GitHub repository
# =============================================================================

readonly NEOVIM_REPO="https://github.com/ChaseDDevelopment/neovim.git"
readonly NEOVIM_CONFIG_DIR="$HOME/.config/nvim"

setup_neovim() {
    substep "Starting Neovim setup"
    
    # Check if Neovim is installed
    if ! check_command nvim; then
        error "Neovim is not installed. Run package installation first."
        return 1
    fi
    
    # Check Neovim version
    check_neovim_version
    
    # Backup existing Neovim configuration
    backup_neovim_config
    
    # Clone Neovim configuration
    clone_neovim_config
    
    # Set up Neovim prerequisites
    setup_neovim_prerequisites
    
    # Initialize LazyVim (plugins will be installed on first run)
    initialize_lazyvim
    
    success "Neovim setup completed"
}

check_neovim_version() {
    substep "Checking Neovim version..."
    
    local nvim_version
    nvim_version=$(nvim --version 2>/dev/null | head -n1 | sed -n 's/.*v\([0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*\).*/\1/p')
    
    # LazyVim requires Neovim >= 0.9.0
    local required_version="0.9.0"
    
    # Check if version was extracted successfully
    if [[ -z "$nvim_version" ]]; then
        warning "Could not determine Neovim version. Please ensure Neovim >= $required_version is installed."
        substep "Continuing with setup (version check skipped)..."
        return 0
    fi
    
    if command -v python3 &>/dev/null; then
        # Use Python for version comparison if available
        if python3 -c "import sys; v='$nvim_version'.split('.'); r='$required_version'.split('.'); sys.exit(0 if [int(x) for x in v] >= [int(x) for x in r] else 1)" 2>/dev/null; then
            substep "Neovim version $nvim_version is compatible"
        else
            error "Neovim version $nvim_version is too old. LazyVim requires >= $required_version"
            error "Please update Neovim to a newer version"
            return 1
        fi
    else
        # Basic version check - just warn and continue
        substep "Neovim version: $nvim_version (ensure it's >= $required_version)"
        warning "Python not available for precise version comparison"
    fi
}

backup_neovim_config() {
    substep "Backing up existing Neovim configuration..."
    
    # Backup configuration directory
    backup_file "$NEOVIM_CONFIG_DIR"
    
    # Backup data directory
    backup_file "$HOME/.local/share/nvim"
    
    # Backup state directory
    backup_file "$HOME/.local/state/nvim"
    
    # Backup cache directory
    backup_file "$HOME/.cache/nvim"
}

clone_neovim_config() {
    substep "Setting up Neovim configuration..."
    
    if [[ "$DRY_RUN" == "false" ]]; then
        if [[ -d "$NEOVIM_CONFIG_DIR" ]]; then
            # Check if existing directory is a git repository
            if [[ -d "$NEOVIM_CONFIG_DIR/.git" ]]; then
                substep "Existing Neovim config found, updating..."
                cd "$NEOVIM_CONFIG_DIR"
                
                # Check if it's the same repository
                local current_remote
                current_remote=$(git remote get-url origin 2>/dev/null || echo "")
                
                if [[ "$current_remote" == "$NEOVIM_REPO" ]]; then
                    # Same repo, check if updates are needed
                    substep "Checking for updates..."
                    
                    # Fetch updates with timeout
                    if timeout 30 git fetch origin 2>/dev/null; then
                        # Check if we're behind
                        local behind_count
                        behind_count=$(git rev-list --count HEAD..origin/main 2>/dev/null || echo "0")
                        
                        if [[ "$behind_count" -gt 0 ]]; then
                            substep "Pulling $behind_count updates..."
                            # Use non-interactive pull with timeout
                            if timeout 60 git pull --ff-only --no-rebase --no-edit 2>/dev/null; then
                                substep "Successfully updated Neovim configuration"
                            else
                                warning "Failed to pull updates. Using existing configuration."
                                warning "You may need to manually update $NEOVIM_CONFIG_DIR"
                            fi
                        else
                            substep "Neovim configuration is already up to date"
                        fi
                    else
                        warning "Could not fetch updates (network timeout). Using existing configuration."
                    fi
                else
                    # Different repo, backup and clone new
                    warning "Existing config is from different repository ($current_remote)"
                    warning "Backing up existing config and installing new one"
                    local backup_name="nvim-config-backup-$(date +%Y%m%d-%H%M%S)"
                    mv "$NEOVIM_CONFIG_DIR" "$HOME/$backup_name"
                    substep "Backed up existing config to ~/$backup_name"
                    # Clone with timeout
                    if timeout 120 git clone "$NEOVIM_REPO" "$NEOVIM_CONFIG_DIR" 2>/dev/null; then
                        substep "Successfully cloned new Neovim configuration"
                    else
                        error "Failed to clone Neovim repository (network timeout)"
                        return 1
                    fi
                fi
                cd - > /dev/null
            else
                # Directory exists but not a git repo, back it up
                warning "Existing non-git Neovim config found, backing up..."
                local backup_name="nvim-config-backup-$(date +%Y%m%d-%H%M%S)"
                mv "$NEOVIM_CONFIG_DIR" "$HOME/$backup_name"
                substep "Backed up existing config to ~/$backup_name"
                # Clone with timeout
                if timeout 120 git clone "$NEOVIM_REPO" "$NEOVIM_CONFIG_DIR" 2>/dev/null; then
                    substep "Successfully cloned Neovim configuration"
                else
                    error "Failed to clone Neovim repository (network timeout)"
                    return 1
                fi
            fi
        else
            # No existing config, just clone
            substep "Cloning Neovim configuration..."
            if timeout 120 git clone "$NEOVIM_REPO" "$NEOVIM_CONFIG_DIR" 2>/dev/null; then
                substep "Successfully cloned Neovim configuration"
            else
                error "Failed to clone Neovim repository (network timeout)"
                return 1
            fi
        fi
        
        substep "Neovim configuration setup completed"
    else
        substep "[DRY RUN] Would setup $NEOVIM_REPO at $NEOVIM_CONFIG_DIR"
    fi
}

setup_neovim_prerequisites() {
    substep "Setting up Neovim prerequisites..."
    
    # Install required tools for LazyVim
    local required_tools=()
    
    # Check for required external tools
    if ! check_command git; then
        required_tools+=("git")
    fi
    
    if ! check_command node; then
        warning "Node.js not found. Some language servers may not work."
    fi
    
    if ! check_command python3; then
        warning "Python3 not found. Some features may not work."
    fi
    
    # Install clipboard support
    case "$PACKAGE_MANAGER" in
        "apt")
            if ! check_command xclip && ! check_command xsel; then
                substep "Installing clipboard support..."
                if [[ "$DRY_RUN" == "false" ]]; then
                    sudo apt install -y xclip
                else
                    substep "[DRY RUN] Would install xclip"
                fi
            fi
            ;;
        "dnf"|"yum")
            if ! check_command xclip && ! check_command xsel; then
                substep "Installing clipboard support..."
                if [[ "$DRY_RUN" == "false" ]]; then
                    sudo dnf install -y xclip || sudo yum install -y xclip
                else
                    substep "[DRY RUN] Would install xclip"
                fi
            fi
            ;;
        "pacman")
            if ! check_command xclip && ! check_command xsel; then
                substep "Installing clipboard support..."
                if [[ "$DRY_RUN" == "false" ]]; then
                    sudo pacman -S --noconfirm xclip
                else
                    substep "[DRY RUN] Would install xclip"
                fi
            fi
            ;;
    esac
    
    # Create necessary directories
    if [[ "$DRY_RUN" == "false" ]]; then
        mkdir -p "$HOME/.local/share/nvim"
        mkdir -p "$HOME/.local/state/nvim"
        mkdir -p "$HOME/.cache/nvim"
    fi
}

initialize_lazyvim() {
    substep "Initializing LazyVim..."
    
    if [[ "$DRY_RUN" == "false" ]]; then
        # Check if the configuration is valid
        if [[ -f "$NEOVIM_CONFIG_DIR/init.lua" ]]; then
            substep "LazyVim configuration is ready"
            substep "Plugins will be automatically installed on first Neovim startup"
            
            # Optionally, we can pre-install plugins in headless mode
            # This might take a while, so we'll make it optional
            if [[ "${PREINSTALL_PLUGINS:-false}" == "true" ]]; then
                substep "Pre-installing plugins (this may take a few minutes)..."
                timeout 300 nvim --headless "+Lazy! sync" +qa 2>/dev/null || {
                    warning "Plugin pre-installation timed out or failed"
                    warning "Plugins will be installed on first manual startup"
                }
            fi
        else
            error "LazyVim configuration not found or invalid"
            return 1
        fi
    else
        substep "[DRY RUN] Would initialize LazyVim configuration"
    fi
}

# Function to validate Neovim setup
validate_neovim_setup() {
    substep "Validating Neovim setup..."
    
    local validation_passed=true
    
    # Check if Neovim is available
    if ! check_command nvim; then
        error "Neovim is not available"
        validation_passed=false
    fi
    
    # Check if config directory exists
    if [[ ! -d "$NEOVIM_CONFIG_DIR" ]]; then
        error "Neovim configuration directory missing: $NEOVIM_CONFIG_DIR"
        validation_passed=false
    fi
    
    # Check if init.lua exists
    if [[ ! -f "$NEOVIM_CONFIG_DIR/init.lua" ]]; then
        error "Neovim init.lua missing: $NEOVIM_CONFIG_DIR/init.lua"
        validation_passed=false
    fi
    
    # Test Neovim configuration syntax
    if [[ "$DRY_RUN" == "false" ]]; then
        if ! nvim --headless -c "checkhealth" -c "qa" 2>/dev/null; then
            warning "Neovim configuration may have issues (run :checkhealth in Neovim)"
        fi
    fi
    
    if [[ "$validation_passed" == "true" ]]; then
        success "Neovim setup validation passed"
    else
        error "Neovim setup validation failed"
        return 1
    fi
}

# Function to display Neovim info
show_neovim_info() {
    echo
    info "Neovim is configured with LazyVim:"
    echo -e "  ${CYAN}Configuration:${NC} $NEOVIM_CONFIG_DIR"
    echo -e "  ${CYAN}Repository:${NC} $NEOVIM_REPO"
    echo
    info "First startup instructions:"
    echo -e "  ${CYAN}1.${NC} Run 'nvim' to start Neovim"
    echo -e "  ${CYAN}2.${NC} LazyVim will automatically install plugins"
    echo -e "  ${CYAN}3.${NC} Wait for installation to complete"
    echo -e "  ${CYAN}4.${NC} Run ':checkhealth' to verify everything is working"
    echo
    info "Useful LazyVim commands:"
    echo -e "  ${CYAN}:Lazy${NC} - Plugin manager"
    echo -e "  ${CYAN}:Mason${NC} - LSP/DAP/Linter installer"
    echo -e "  ${CYAN}:checkhealth${NC} - Check system health"
    echo
}