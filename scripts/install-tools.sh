#!/usr/bin/env bash

# =============================================================================
# Tool Installation Script
# =============================================================================
# Installs tools from official sources (not package managers)
# Note: Rust, UV, Starship, and Bun are installed in install-packages.sh
# This script handles: nvm, atuin, tpm
# =============================================================================

all_tools_installed() {
    [[ -d "$HOME/.config/nvm" || -d "$HOME/.nvm" ]] \
        && check_command atuin \
        && [[ -d "$HOME/.tmux/plugins/tpm" ]]
}

install_all_tools() {
    step "Installing tools from official sources"

    if all_tools_installed; then
        substep "All tools already installed, skipping"
        return
    fi

    install_nvm
    install_atuin_tool
    install_tpm

    success "All tools installed from official sources"
}

# -----------------------------------------------------------------------------
# Node Version Manager (nvm)
# -----------------------------------------------------------------------------
install_nvm() {
    substep "Installing Node Version Manager (nvm)..."

    if [[ -d "$HOME/.config/nvm" ]] || [[ -d "$HOME/.nvm" ]]; then
        substep "nvm already installed"
        return
    fi

    if [[ "$DRY_RUN" == "false" ]]; then
        # Set NVM_DIR to XDG-compliant location
        export NVM_DIR="$HOME/.config/nvm"
        mkdir -p "$NVM_DIR"

        # Download and run installer
        curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.40.1/install.sh | bash

        # Source nvm and install LTS
        [ -s "$NVM_DIR/nvm.sh" ] && \. "$NVM_DIR/nvm.sh"

        if check_command nvm; then
            nvm install --lts
            nvm alias default lts/*
            substep "nvm installed with Node LTS"
        else
            warning "nvm installed but not available in current shell"
            warning "Node LTS will be installed on next shell start"
        fi
    else
        substep "[DRY RUN] Would install nvm and Node LTS"
    fi
}

# -----------------------------------------------------------------------------
# Atuin (Shell History)
# -----------------------------------------------------------------------------
install_atuin_tool() {
    substep "Installing Atuin (shell history)..."

    if check_command atuin; then
        substep "atuin already installed"
        return
    fi

    if [[ "$DRY_RUN" == "false" ]]; then
        # Check if available via package manager first
        case "$PACKAGE_MANAGER" in
            "brew")
                brew install atuin
                ;;
            "pacman")
                sudo pacman -S --noconfirm atuin
                ;;
            *)
                # Use official installer
                curl --proto '=https' --tlsv1.2 -LsSf https://setup.atuin.sh | sh
                # Add Atuin to PATH for current session
                export PATH="$HOME/.atuin/bin:$PATH"
                ;;
        esac
        substep "Atuin installed"
    else
        substep "[DRY RUN] Would install Atuin"
    fi
}

# -----------------------------------------------------------------------------
# TPM (Tmux Plugin Manager)
# -----------------------------------------------------------------------------
install_tpm() {
    substep "Installing TPM (Tmux Plugin Manager)..."

    local tpm_dir="$HOME/.tmux/plugins/tpm"

    if [[ -d "$tpm_dir" ]]; then
        substep "TPM already installed"
        return
    fi

    if [[ "$DRY_RUN" == "false" ]]; then
        git clone https://github.com/tmux-plugins/tpm "$tpm_dir"
        substep "TPM installed"
    else
        substep "[DRY RUN] Would install TPM"
    fi
}
