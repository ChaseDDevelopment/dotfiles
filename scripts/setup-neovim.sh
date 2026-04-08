#!/usr/bin/env bash

# =============================================================================
# Neovim Setup Script
# =============================================================================
# Sets up Neovim 0.12+ with vim.pack configuration from local configs directory
# =============================================================================

readonly NEOVIM_CONFIG_DIR="$HOME/.config/nvim"

setup_neovim() {
    ui_info "Starting Neovim setup"

    # Check if Neovim is installed
    if ! check_command nvim; then
        ui_error "Neovim is not installed. Run package installation first."
        return 1
    fi

    # Check Neovim version
    check_neovim_version

    # Backup existing Neovim configuration
    backup_neovim_config

    # Copy Neovim configuration
    copy_neovim_config

    # Set up Neovim prerequisites
    setup_neovim_prerequisites

    # Initialize vim.pack plugins
    initialize_neovim

    ui_success "Neovim setup completed"
}

check_neovim_version() {
    ui_info "Checking Neovim version..."

    local nvim_version
    nvim_version=$(nvim --version 2>/dev/null | head -n1 | sed -n 's/.*v\([0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*\).*/\1/p')

    local required_version="0.12.0"

    if [[ -z "$nvim_version" ]]; then
        ui_warn "Could not determine Neovim version. Please ensure Neovim >= $required_version is installed."
        ui_info "Continuing with setup (version check skipped)..."
        return 0
    fi

    if command -v python3 &>/dev/null; then
        if python3 -c "import sys; v='$nvim_version'.split('.'); r='$required_version'.split('.'); sys.exit(0 if [int(x) for x in v] >= [int(x) for x in r] else 1)" 2>/dev/null; then
            ui_info "Neovim version $nvim_version is compatible"
        else
            ui_error "Neovim version $nvim_version is too old. vim.pack requires >= $required_version"
            ui_error "Please update Neovim to a newer version"
            return 1
        fi
    else
        ui_info "Neovim version: $nvim_version (ensure it's >= $required_version)"
        ui_warn "Python not available for precise version comparison"
    fi
}

backup_neovim_config() {
    ui_info "Backing up existing Neovim configuration..."
    # Only backup config — plugin data, state, and cache are regenerable
    # via vim.pack on next nvim startup
    backup_file "$NEOVIM_CONFIG_DIR"
}

copy_neovim_config() {
    ui_info "Copying Neovim configuration..."

    local source_dir="$SCRIPT_DIR/configs/nvim"

    if [[ ! -d "$source_dir" ]]; then
        ui_error "Neovim config source not found: $source_dir"
        return 1
    fi

    symlink_if_needed "$source_dir" "$NEOVIM_CONFIG_DIR"
}

setup_neovim_prerequisites() {
    ui_info "Setting up Neovim prerequisites..."

    if ! check_command git; then
        ui_warn "Git not found. Some plugins may not install correctly."
    fi

    if ! check_command node; then
        ui_warn "Node.js not found. Some language servers may not work."
    fi

    if ! check_command cargo; then
        ui_warn "Cargo not found. blink.cmp Rust fuzzy matcher will not be built."
        ui_warn "Install Rust via rustup: curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh"
    fi

    if [[ "$DRY_RUN" == "false" ]]; then
        mkdir -p "$HOME/.local/share/nvim"
        mkdir -p "$HOME/.local/state/nvim"
        mkdir -p "$HOME/.cache/nvim"
    fi
}

initialize_neovim() {
    ui_info "Initializing Neovim with vim.pack..."

    if [[ "$DRY_RUN" == "false" ]]; then
        if [[ -f "$NEOVIM_CONFIG_DIR/init.lua" ]]; then
            ui_info "vim.pack configuration is ready"
            ui_info "Plugins will be installed on first Neovim startup"

            # Build blink.cmp Rust fuzzy matcher if cargo is available
            local blink_dir="$HOME/.local/share/nvim/site/pack/core/opt/blink.cmp"
            if check_command cargo && [[ -d "$blink_dir" ]]; then
                if ui_spin "Building blink.cmp Rust fuzzy matcher..." \
                    bash -c "cd '$blink_dir' && cargo build --release"; then
                    : # ui_spin prints success
                else
                    ui_warn "Failed to build blink.cmp Rust binary"
                    ui_warn "Falling back to Lua fuzzy matcher (still works fine)"
                fi
            fi
        else
            ui_error "Neovim configuration not found or invalid"
            return 1
        fi
    else
        ui_info "[DRY RUN] Would initialize vim.pack configuration"
    fi
}

validate_neovim_setup() {
    ui_info "Validating Neovim setup..."

    local validation_passed=true

    if ! check_command nvim; then
        ui_error "Neovim is not available"
        validation_passed=false
    fi

    if [[ ! -d "$NEOVIM_CONFIG_DIR" ]]; then
        ui_error "Neovim configuration directory missing: $NEOVIM_CONFIG_DIR"
        validation_passed=false
    fi

    if [[ ! -f "$NEOVIM_CONFIG_DIR/init.lua" ]]; then
        ui_error "Neovim init.lua missing: $NEOVIM_CONFIG_DIR/init.lua"
        validation_passed=false
    fi

    if [[ "$validation_passed" == "true" ]]; then
        ui_success "Neovim setup validation passed"
    else
        ui_error "Neovim setup validation failed"
        return 1
    fi
}

show_neovim_info() {
    echo
    ui_info "Neovim is configured with vim.pack (built-in plugin manager):"
    ui_info "  Configuration: $NEOVIM_CONFIG_DIR"
    echo
    ui_info "First startup instructions:"
    ui_info "  1. Run 'nvim' to start Neovim"
    ui_info "  2. Approve plugin installations when prompted"
    ui_info "  3. Wait for treesitter parsers to compile"
    ui_info "  4. Run ':checkhealth' to verify everything is working"
    echo
    ui_info "Useful commands:"
    ui_info "  :lua vim.pack.update() - Update all plugins"
    ui_info "  :Mason                - LSP/formatter installer"
    ui_info "  :checkhealth          - Check system health"
    echo
}
