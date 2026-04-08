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
    ui_step "Installing tools from official sources"

    if all_tools_installed; then
        ui_info "All tools already installed, skipping"
        return
    fi

    install_nvm
    install_atuin_tool
    install_tpm

    ui_success "All tools installed from official sources"
}

# -----------------------------------------------------------------------------
# Node Version Manager (nvm)
# -----------------------------------------------------------------------------
install_nvm() {
    ui_info "Installing Node Version Manager (nvm)..."

    if [[ -d "$HOME/.config/nvm" ]] || [[ -d "$HOME/.nvm" ]]; then
        plan_add "  nvm" "Tool" "already installed"
        ui_info "nvm already installed"
        return
    fi

    plan_add "  nvm" "Tool" "would install"

    if [[ "$DRY_RUN" == "false" ]]; then
        # Set NVM_DIR to XDG-compliant location
        export NVM_DIR="$HOME/.config/nvm"
        mkdir -p "$NVM_DIR"

        # Download and run installer
        ui_spin "Installing NVM" bash -c "curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.40.1/install.sh | bash"

        # Source nvm and install LTS
        [ -s "$NVM_DIR/nvm.sh" ] && \. "$NVM_DIR/nvm.sh"

        if check_command nvm; then
            ui_spin "Installing Node.js LTS" nvm install --lts
            nvm alias default lts/*
            ui_info "nvm installed with Node LTS"
        else
            ui_warn "nvm installed but not available in current shell"
            ui_warn "Node LTS will be installed on next shell start"
        fi
    else
        ui_info "[DRY RUN] Would install nvm and Node LTS"
    fi
}

# -----------------------------------------------------------------------------
# Atuin (Shell History)
# -----------------------------------------------------------------------------
install_atuin_tool() {
    ui_info "Installing Atuin (shell history)..."

    if check_command atuin; then
        plan_add "  atuin" "Tool" "already installed"
        ui_info "atuin already installed"
        return
    fi

    plan_add "  atuin" "Tool" "would install"

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
                ui_spin "Installing Atuin" bash -c "curl --proto '=https' --tlsv1.2 -LsSf https://setup.atuin.sh | sh"
                # Add Atuin to PATH for current session
                export PATH="$HOME/.atuin/bin:$PATH"
                ;;
        esac
        ui_info "Atuin installed"
    else
        ui_info "[DRY RUN] Would install Atuin"
    fi
}

# -----------------------------------------------------------------------------
# TPM (Tmux Plugin Manager)
# -----------------------------------------------------------------------------
install_tpm() {
    ui_info "Installing TPM (Tmux Plugin Manager)..."

    local tpm_dir="$HOME/.tmux/plugins/tpm"

    if [[ -d "$tpm_dir" ]]; then
        plan_add "  tpm" "Tool" "already installed"
        ui_info "TPM already installed"
        return
    fi

    plan_add "  tpm" "Tool" "would install"

    if [[ "$DRY_RUN" == "false" ]]; then
        ui_spin "Cloning TPM" git clone https://github.com/tmux-plugins/tpm "$tpm_dir"
        ui_info "TPM installed"
    else
        ui_info "[DRY RUN] Would install TPM"
    fi
}
