#!/usr/bin/env bash

# =============================================================================
# Tool Installation Script
# =============================================================================
# Installs tools from official sources (not package managers)
# Ensures latest versions and cross-platform compatibility
# =============================================================================

install_all_tools() {
    step "Installing tools from official sources"

    install_nvm
    install_uv
    install_starship_tool
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
# UV (Astral Python Package Manager)
# -----------------------------------------------------------------------------
install_uv() {
    substep "Installing UV (Python package manager)..."

    if check_command uv; then
        substep "uv already installed"
        return
    fi

    if [[ "$DRY_RUN" == "false" ]]; then
        curl -LsSf https://astral.sh/uv/install.sh | sh
        export PATH="$HOME/.local/bin:$PATH"
        substep "uv installed"
    else
        substep "[DRY RUN] Would install uv"
    fi
}

# -----------------------------------------------------------------------------
# Starship Prompt
# -----------------------------------------------------------------------------
install_starship_tool() {
    substep "Installing Starship prompt..."

    if check_command starship; then
        substep "starship already installed"
        return
    fi

    if [[ "$DRY_RUN" == "false" ]]; then
        curl -sS https://starship.rs/install.sh | sh -s -- --yes
        substep "Starship installed"
    else
        substep "[DRY RUN] Would install Starship"
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

# -----------------------------------------------------------------------------
# Rust (via rustup)
# -----------------------------------------------------------------------------
install_rust() {
    substep "Installing Rust..."

    if check_command rustc; then
        substep "Rust already installed"
        return
    fi

    if [[ "$DRY_RUN" == "false" ]]; then
        curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y
        source "$HOME/.cargo/env"
        substep "Rust installed"
    else
        substep "[DRY RUN] Would install Rust"
    fi
}

# -----------------------------------------------------------------------------
# Bun (JavaScript Runtime)
# -----------------------------------------------------------------------------
install_bun() {
    substep "Installing Bun..."

    if check_command bun; then
        substep "Bun already installed"
        return
    fi

    if [[ "$DRY_RUN" == "false" ]]; then
        curl -fsSL https://bun.sh/install | bash
        substep "Bun installed"
    else
        substep "[DRY RUN] Would install Bun"
    fi
}
