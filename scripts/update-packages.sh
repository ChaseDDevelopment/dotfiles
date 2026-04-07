#!/usr/bin/env bash

# =============================================================================
# Package Update Script
# =============================================================================
# Updates all installed tools and packages across every package manager.
# Called via: ./install.sh --update
#
# Depends on: detect-os.sh, package-helpers.sh
# =============================================================================

# Source modular installers (needed for version_gte in Neovim updates)
source "$SCRIPT_DIR/scripts/installers/github-helpers.sh"
source "$SCRIPT_DIR/scripts/installers/dev-tools.sh"

# Known cargo-installed tools and the crate names used to install them.
# Only tools that might be installed via cargo on some platforms.
readonly CARGO_TOOLS=(
    "eza:eza"
    "tree-sitter:tree-sitter-cli"
)

update_all_packages() {
    step "Updating all installed packages"

    update_system_packages
    update_rust_toolchain
    update_cargo_binaries
    update_uv_ecosystem
    update_bun
    update_nvm_node
    update_starship
    update_atuin
    update_neovim_binary
    update_dotnet
    update_yazi_plugins
    update_tmux_plugins

    success "All updates completed"

    echo
    info "Note: Zsh plugins (Antidote) update automatically, or run:"
    echo -e "  ${CYAN}antidote update${NC}"
    info "Neovim plugins can be updated inside Neovim:"
    echo -e "  ${CYAN}:lua vim.pack.update()${NC}"
}

# -----------------------------------------------------------------------------
# System packages
# -----------------------------------------------------------------------------
update_system_packages() {
    substep "Updating system packages..."
    update_system
}

# -----------------------------------------------------------------------------
# Rust toolchain
# -----------------------------------------------------------------------------
update_rust_toolchain() {
    if ! check_command rustup; then
        return
    fi

    substep "Updating Rust toolchain..."
    if [[ "$DRY_RUN" == "false" ]]; then
        rustup update || warning "Failed to update Rust toolchain"
    else
        substep "[DRY RUN] Would run: rustup update"
    fi
}

# -----------------------------------------------------------------------------
# Cargo-installed binaries
# -----------------------------------------------------------------------------
update_cargo_binaries() {
    if ! check_command cargo; then
        return
    fi

    substep "Updating cargo-installed binaries..."

    for entry in "${CARGO_TOOLS[@]}"; do
        local cmd="${entry%%:*}"
        local crate="${entry##*:}"

        if check_command "$cmd"; then
            substep "Updating $crate..."
            if [[ "$DRY_RUN" == "false" ]]; then
                cargo install "$crate" || warning "Failed to update $crate"
            else
                substep "[DRY RUN] Would run: cargo install $crate"
            fi
        fi
    done

    # yazi uses a special build crate
    if check_command yazi; then
        # Only update via cargo if yazi was cargo-installed (not via brew/pacman)
        if [[ "$PACKAGE_MANAGER" != "brew" && "$PACKAGE_MANAGER" != "pacman" ]]; then
            substep "Updating yazi..."
            if [[ "$DRY_RUN" == "false" ]]; then
                cargo install --force yazi-build || warning "Failed to update yazi"
            else
                substep "[DRY RUN] Would run: cargo install --force yazi-build"
            fi
        fi
    fi
}

# -----------------------------------------------------------------------------
# uv + uv-managed tools (ruff, etc.)
# -----------------------------------------------------------------------------
update_uv_ecosystem() {
    if ! check_command uv; then
        return
    fi

    substep "Updating uv..."
    if [[ "$DRY_RUN" == "false" ]]; then
        uv self update || warning "Failed to update uv"
    else
        substep "[DRY RUN] Would run: uv self update"
    fi

    substep "Updating uv-managed tools..."
    if [[ "$DRY_RUN" == "false" ]]; then
        uv tool upgrade --all || warning "Failed to update uv tools"
    else
        substep "[DRY RUN] Would run: uv tool upgrade --all"
    fi
}

# -----------------------------------------------------------------------------
# Bun
# -----------------------------------------------------------------------------
update_bun() {
    if ! check_command bun; then
        return
    fi

    substep "Updating Bun..."
    if [[ "$DRY_RUN" == "false" ]]; then
        bun upgrade || warning "Failed to update Bun"
    else
        substep "[DRY RUN] Would run: bun upgrade"
    fi
}

# -----------------------------------------------------------------------------
# nvm + Node.js
# -----------------------------------------------------------------------------
update_nvm_node() {
    local nvm_dir="${NVM_DIR:-$HOME/.config/nvm}"

    if [[ ! -d "$nvm_dir" ]]; then
        nvm_dir="$HOME/.nvm"
    fi

    if [[ ! -d "$nvm_dir" ]]; then
        return
    fi

    substep "Updating Node.js via nvm..."

    if [[ "$DRY_RUN" == "false" ]]; then
        # Source nvm for this session
        [ -s "$nvm_dir/nvm.sh" ] && \. "$nvm_dir/nvm.sh"

        if check_command nvm; then
            nvm install --lts || warning "Failed to install latest Node LTS"
            nvm alias default lts/* || true
        else
            warning "nvm could not be loaded"
        fi
    else
        substep "[DRY RUN] Would run: nvm install --lts"
    fi
}

# -----------------------------------------------------------------------------
# Starship
# -----------------------------------------------------------------------------
update_starship() {
    if ! check_command starship; then
        return
    fi

    substep "Updating Starship..."
    if [[ "$DRY_RUN" == "false" ]]; then
        local starship_installer="/tmp/starship-install.sh"
        curl -sS https://starship.rs/install.sh -o "$starship_installer"

        if [[ -f "$starship_installer" && -s "$starship_installer" ]]; then
            sh "$starship_installer" --yes
            rm -f "$starship_installer"
        else
            warning "Failed to download Starship installer"
            rm -f "$starship_installer"
        fi
    else
        substep "[DRY RUN] Would re-run Starship installer"
    fi
}

# -----------------------------------------------------------------------------
# Atuin
# -----------------------------------------------------------------------------
update_atuin() {
    if ! check_command atuin; then
        return
    fi

    substep "Updating Atuin..."
    if [[ "$DRY_RUN" == "false" ]]; then
        case "$PACKAGE_MANAGER" in
            "brew")
                brew upgrade atuin || warning "Failed to update Atuin"
                ;;
            "pacman")
                sudo pacman -S --noconfirm atuin || warning "Failed to update Atuin"
                ;;
            *)
                # Re-run official installer for updates
                curl --proto '=https' --tlsv1.2 -LsSf https://setup.atuin.sh | sh \
                    || warning "Failed to update Atuin"
                ;;
        esac
    else
        substep "[DRY RUN] Would update Atuin via $PACKAGE_MANAGER or official installer"
    fi
}

# -----------------------------------------------------------------------------
# Neovim
# -----------------------------------------------------------------------------
update_neovim_binary() {
    if ! check_command nvim; then
        return
    fi

    substep "Updating Neovim..."
    if [[ "$DRY_RUN" == "false" ]]; then
        case "$PACKAGE_MANAGER" in
            "brew")
                brew upgrade neovim || warning "Failed to update Neovim"
                ;;
            "pacman")
                if check_command yay; then
                    yay -S --noconfirm neovim-git || warning "Failed to update Neovim"
                elif check_command paru; then
                    paru -S --noconfirm neovim-git || warning "Failed to update Neovim"
                else
                    sudo pacman -S --noconfirm neovim || warning "Failed to update Neovim"
                fi
                ;;
            "apt")
                # Re-download from GitHub releases
                install_neovim
                ;;
            *)
                "${UPDATE_CMD_ARRAY[@]}" || warning "Failed to update Neovim"
                ;;
        esac
    else
        substep "[DRY RUN] Would update Neovim via $PACKAGE_MANAGER"
    fi
}

# -----------------------------------------------------------------------------
# .NET SDK
# -----------------------------------------------------------------------------
update_dotnet() {
    if ! check_command dotnet; then
        return
    fi

    substep "Updating .NET SDK..."
    if [[ "$DRY_RUN" == "false" ]]; then
        case "$PACKAGE_MANAGER" in
            "brew")
                brew upgrade dotnet-sdk || warning "Failed to update .NET SDK"
                ;;
            "pacman")
                sudo pacman -S --noconfirm dotnet-sdk || warning "Failed to update .NET SDK"
                ;;
            *)
                # Re-run dotnet-install.sh for updates
                local install_script="/tmp/dotnet-install.sh"
                curl -sSL https://dot.net/v1/dotnet-install.sh -o "$install_script"
                chmod +x "$install_script"
                "$install_script" --channel LTS --install-dir "$HOME/.dotnet"
                rm -f "$install_script"
                ;;
        esac
    else
        substep "[DRY RUN] Would update .NET SDK"
    fi
}

# -----------------------------------------------------------------------------
# Yazi plugins
# -----------------------------------------------------------------------------
update_yazi_plugins() {
    if ! check_command ya; then
        return
    fi

    substep "Updating Yazi plugins..."
    if [[ "$DRY_RUN" == "false" ]]; then
        ya pkg upgrade || warning "Failed to update Yazi plugins"
    else
        substep "[DRY RUN] Would run: ya pkg upgrade"
    fi
}

# -----------------------------------------------------------------------------
# Tmux plugins (via TPM)
# -----------------------------------------------------------------------------
update_tmux_plugins() {
    local tpm_update="$HOME/.tmux/plugins/tpm/scripts/update_plugin.sh"

    if [[ ! -f "$tpm_update" ]]; then
        return
    fi

    substep "Updating Tmux plugins..."
    if [[ "$DRY_RUN" == "false" ]]; then
        "$tpm_update" all || warning "Failed to update Tmux plugins"
    else
        substep "[DRY RUN] Would run TPM update_plugin.sh all"
    fi
}
