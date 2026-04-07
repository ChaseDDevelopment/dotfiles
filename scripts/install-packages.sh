#!/usr/bin/env bash

# =============================================================================
# Package Installation Orchestrator
# =============================================================================
# Coordinates installation of all packages by sourcing modular installer
# scripts. Individual install functions live in scripts/installers/.
# =============================================================================

# Source modular installers
source "$SCRIPT_DIR/scripts/installers/github-helpers.sh"
source "$SCRIPT_DIR/scripts/installers/cli-tools.sh"
source "$SCRIPT_DIR/scripts/installers/dev-tools.sh"

all_packages_installed() {
    local cmds=("git" "curl" "wget" "unzip" "zsh" "tmux" "fzf" "nvim"
                "tree-sitter" "eza" "bat" "rg" "fd" "zoxide" "tspin"
                "starship" "bun" "uv" "ruff" "dotnet" "yazi"
                "delta" "lazygit" "direnv" "yq" "xh")
    for cmd in "${cmds[@]}"; do
        if ! check_command "$cmd"; then
            return 1
        fi
    done
    return 0
}

install_packages() {
    substep "Starting package installation"

    # Update system (skip if --skip-update or all packages already present)
    if [[ "$SKIP_UPDATE" == "true" ]]; then
        substep "Skipping system update (--skip-update)"
    elif all_packages_installed; then
        substep "All packages already installed, skipping system update"
    else
        update_system
    fi

    # Core development tools
    substep "Installing core development tools..."
    install_package "git"
    install_package "curl"
    install_package "wget"
    install_package "unzip"

    # Build tools (needed for some installations)
    if [[ "$PACKAGE_MANAGER" != "brew" ]]; then
        install_package "build-essential"
    fi

    # Rust/Cargo (needed early -- tree-sitter-cli, yazi, eza depend on cargo on Linux)
    install_rust

    # Shell and terminal tools
    substep "Installing shell and terminal tools..."
    install_package "zsh"
    install_package "tmux"

    # Install powerline fonts (for Ubuntu/Debian)
    if [[ "$PACKAGE_MANAGER" == "apt" ]]; then
        install_package "powerline"
        install_package "fonts-powerline"
    fi

    install_package "fzf"

    # Text editor
    substep "Installing Neovim..."
    install_neovim

    # Tree-sitter CLI (needed by nvim-treesitter to compile parsers)
    install_tree_sitter_cli

    # Modern CLI tools
    substep "Installing modern CLI tools..."
    install_eza
    install_bat
    install_ripgrep
    install_fd
    install_zoxide
    install_tailspin
    install_coreutils
    install_yazi
    install_delta
    install_lazygit
    install_xh
    install_direnv
    install_yq

    # Install clipboard utilities (Linux only)
    if [[ "$PACKAGE_MANAGER" != "brew" ]]; then
        install_clipboard_utils
    fi

    # Install Starship prompt
    install_starship

    # Install Node.js (we'll use NVM later, but this provides a base)
    install_package "nodejs"

    # Install Bun
    install_bun

    # Python tools
    install_uv
    install_ruff

    # .NET SDK (for F#/C# LSP and Mason tools)
    install_dotnet_sdk

    success "Package installation completed"
}
