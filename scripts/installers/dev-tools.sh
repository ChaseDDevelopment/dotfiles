#!/usr/bin/env bash

# =============================================================================
# Development Tool Installers
# =============================================================================
# Development toolchain: Rust, Neovim, tree-sitter, uv, ruff, Bun, .NET,
# yazi, Starship.
#
# Depends on: github-helpers.sh, package-helpers.sh, detect-os.sh
# =============================================================================

# Compare versions: returns 0 (true) if $1 >= $2
version_gte() {
    [ "$(printf '%s\n' "$2" "$1" | sort -V | head -n1)" = "$2" ]
}

# -----------------------------------------------------------------------------
# Rust and Cargo
# -----------------------------------------------------------------------------
install_rust() {
    if check_command cargo; then
        substep "Rust/Cargo is already installed"
        return
    fi

    substep "Installing Rust and Cargo..."

    if [[ "$DRY_RUN" == "false" ]]; then
        local rust_installer="/tmp/rustup-init.sh"
        substep "Downloading Rust installer..."
        curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs -o "$rust_installer"

        if [[ -f "$rust_installer" && -s "$rust_installer" ]]; then
            substep "Executing Rust installer..."
            sh "$rust_installer" -y
            rm -f "$rust_installer"
            source "$HOME/.cargo/env"
        else
            error "Failed to download Rust installer"
            rm -f "$rust_installer"
            return 1
        fi
    else
        substep "[DRY RUN] Would install Rust via rustup"
    fi
}

# -----------------------------------------------------------------------------
# Neovim
# -----------------------------------------------------------------------------
install_neovim() {
    local min_version="0.12.0"

    if check_command nvim; then
        local current_version
        current_version=$(nvim --version | head -1 | grep -oE '[0-9]+\.[0-9]+\.[0-9]+')
        if version_gte "$current_version" "$min_version"; then
            substep "Neovim $current_version is already installed (meets minimum $min_version)"
            return
        else
            substep "Neovim $current_version is outdated (need $min_version+), upgrading..."
        fi
    else
        substep "Installing Neovim..."
    fi

    case "$PACKAGE_MANAGER" in
        "brew")
            if [[ "$DRY_RUN" == "false" ]]; then
                brew install --HEAD neovim
            else
                substep "[DRY RUN] Would install Neovim HEAD via brew"
            fi
            ;;
        "pacman")
            if [[ "$DRY_RUN" == "false" ]]; then
                if check_command yay; then
                    substep "Installing Neovim development version via yay..."
                    yay -S --noconfirm neovim-git
                elif check_command paru; then
                    substep "Installing Neovim development version via paru..."
                    paru -S --noconfirm neovim-git
                else
                    warning "No AUR helper found. Installing official Neovim package..."
                    sudo pacman -S --noconfirm neovim
                    warning "For Neovim 0.12+, consider installing an AUR helper (yay/paru) and running: yay -S neovim-git"
                fi
            else
                substep "[DRY RUN] Would install Neovim development version via AUR or official package"
            fi
            ;;
        "apt")
            if [[ "$DRY_RUN" == "false" ]]; then
                substep "Installing latest Neovim from GitHub releases..."

                local arch nvim_url
                arch=$(uname -m)
                if [[ "$arch" == "x86_64" ]]; then
                    nvim_url="https://github.com/neovim/neovim/releases/latest/download/nvim-linux-x86_64.tar.gz"
                elif [[ "$arch" == "aarch64" ]] || [[ "$arch" == "arm64" ]]; then
                    nvim_url="https://github.com/neovim/neovim/releases/latest/download/nvim-linux-arm64.tar.gz"
                else
                    warning "Unknown architecture: $arch. Falling back to apt install."
                    "${INSTALL_CMD_ARRAY[@]}" neovim
                    return
                fi

                curl -LO "$nvim_url"
                sudo rm -rf /opt/nvim /opt/nvim-linux-x86_64 /opt/nvim-linux-arm64
                sudo tar -C /opt -xzf nvim-linux-*.tar.gz
                rm -f nvim-linux-*.tar.gz

                sudo rm -f /usr/local/bin/nvim
                sudo ln -s /opt/nvim-linux-*/bin/nvim /usr/local/bin/nvim

                substep "Neovim installed from GitHub releases"
            else
                substep "[DRY RUN] Would install latest Neovim from GitHub releases"
            fi
            ;;
        "dnf"|"yum")
            if [[ "$DRY_RUN" == "false" ]]; then
                "${INSTALL_CMD_ARRAY[@]}" neovim
                warning "RHEL/Fedora packages may be outdated. For Neovim 0.12+, consider using Flatpak:"
                warning "flatpak install flathub io.neovim.nvim"
            else
                substep "[DRY RUN] Would install Neovim via $PACKAGE_MANAGER with Flatpak recommendation"
            fi
            ;;
        *)
            warning "Unknown package manager. Please install Neovim manually."
            warning "For latest version, see: https://github.com/neovim/neovim/releases"
            ;;
    esac
}

# -----------------------------------------------------------------------------
# tree-sitter CLI
# -----------------------------------------------------------------------------
install_tree_sitter_cli() {
    substep "Installing tree-sitter library and CLI..."

    if [[ "$DRY_RUN" == "true" ]]; then
        substep "[DRY RUN] Would install tree-sitter and tree-sitter-cli"
        return
    fi

    # Install tree-sitter library (libtree-sitter)
    case "$PACKAGE_MANAGER" in
        "brew")
            brew install tree-sitter
            ;;
        "pacman")
            sudo pacman -S --noconfirm tree-sitter
            ;;
        "apt")
            "${INSTALL_CMD_ARRAY[@]}" libtree-sitter-dev
            ;;
        "dnf"|"yum")
            "${INSTALL_CMD_ARRAY[@]}" libtree-sitter-devel 2>/dev/null || true
            ;;
    esac

    # Install tree-sitter CLI
    if check_command tree-sitter; then
        if tree-sitter build --help &>/dev/null; then
            substep "tree-sitter-cli is already installed (modern version)"
            return
        else
            substep "tree-sitter-cli is outdated (no 'build' subcommand), upgrading..."
        fi
    fi

    case "$PACKAGE_MANAGER" in
        "brew")
            brew install tree-sitter-cli
            ;;
        "pacman")
            sudo pacman -S --noconfirm tree-sitter-cli
            ;;
        *)
            if check_command cargo; then
                cargo install tree-sitter-cli
            else
                warning "tree-sitter-cli requires cargo (Rust). Install Rust first, then run: cargo install tree-sitter-cli"
            fi
            ;;
    esac
}

# -----------------------------------------------------------------------------
# uv (Python package manager) + Python
# -----------------------------------------------------------------------------
install_uv() {
    if check_command uv; then
        substep "UV is already installed"
    else
        substep "Installing UV..."
        if [[ "$DRY_RUN" == "false" ]]; then
            local uv_installer="/tmp/uv-install.sh"
            substep "Downloading UV installer..."
            curl -LsSf https://astral.sh/uv/install.sh -o "$uv_installer"

            if [[ -f "$uv_installer" && -s "$uv_installer" ]]; then
                substep "Executing UV installer..."
                sh "$uv_installer"
                rm -f "$uv_installer"
                export PATH="$HOME/.local/bin:$PATH"
            else
                error "Failed to download UV installer"
                rm -f "$uv_installer"
                return 1
            fi
        else
            substep "[DRY RUN] Would install UV via official installer"
        fi
    fi

    # Install Python via UV
    if [[ -x "$HOME/.local/bin/python3" ]]; then
        substep "Python already installed via UV"
    else
        substep "Installing Python via UV..."
        if [[ "$DRY_RUN" == "false" ]]; then
            uv python install 3.12

            local uv_python_bin
            uv_python_bin=$(uv python find 3.12 2>/dev/null)
            if [[ -n "$uv_python_bin" && -x "$uv_python_bin" ]]; then
                mkdir -p "$HOME/.local/bin"
                ln -sf "$uv_python_bin" "$HOME/.local/bin/python3"
                ln -sf "$uv_python_bin" "$HOME/.local/bin/python"
                substep "Python 3.12 installed and symlinked to ~/.local/bin/"
            else
                warning "Could not find UV-installed Python binary"
            fi
        else
            substep "[DRY RUN] Would install Python via UV"
        fi
    fi
}

# -----------------------------------------------------------------------------
# ruff (Python linter/formatter)
# -----------------------------------------------------------------------------
install_ruff() {
    if check_command ruff; then
        substep "Ruff is already installed"
        return
    fi

    substep "Installing Ruff via UV..."
    if [[ "$DRY_RUN" == "false" ]]; then
        if ! check_command uv; then
            install_uv
        fi
        uv tool install ruff
    else
        substep "[DRY RUN] Would install Ruff via UV"
    fi
}

# -----------------------------------------------------------------------------
# Bun (JavaScript runtime)
# -----------------------------------------------------------------------------
install_bun() {
    if check_command bun; then
        substep "Bun is already installed"
        return
    fi

    substep "Installing Bun..."

    if [[ "$DRY_RUN" == "false" ]]; then
        local bun_installer="/tmp/bun-install.sh"
        substep "Downloading Bun installer..."
        curl -fsSL https://bun.sh/install -o "$bun_installer"

        if [[ -f "$bun_installer" && -s "$bun_installer" ]]; then
            substep "Executing Bun installer..."
            bash "$bun_installer"
            rm -f "$bun_installer"

            export BUN_INSTALL="$HOME/.bun"
            export PATH="$BUN_INSTALL/bin:$PATH"
        else
            error "Failed to download Bun installer"
            rm -f "$bun_installer"
            return 1
        fi
    else
        substep "[DRY RUN] Would install Bun via official installer"
    fi
}

# -----------------------------------------------------------------------------
# .NET SDK
# -----------------------------------------------------------------------------
install_dotnet_sdk() {
    if check_command dotnet; then
        substep "dotnet-sdk is already installed"
        return
    fi

    substep "Installing dotnet-sdk..."
    case "$PACKAGE_MANAGER" in
        "brew")
            if [[ "$DRY_RUN" == "false" ]]; then
                brew install dotnet-sdk
            else
                substep "[DRY RUN] Would install dotnet-sdk via Homebrew"
            fi
            ;;
        "pacman")
            if [[ "$DRY_RUN" == "false" ]]; then
                "${INSTALL_CMD_ARRAY[@]}" dotnet-sdk
            else
                substep "[DRY RUN] Would install dotnet-sdk via pacman"
            fi
            ;;
        *)
            if [[ "$DRY_RUN" == "false" ]]; then
                substep "Using Microsoft's dotnet-install.sh script..."
                local install_script="/tmp/dotnet-install.sh"
                curl -sSL https://dot.net/v1/dotnet-install.sh -o "$install_script"
                chmod +x "$install_script"
                "$install_script" --channel LTS --install-dir "$HOME/.dotnet"
                rm -f "$install_script"

                export PATH="$HOME/.dotnet:$PATH"
                export DOTNET_ROOT="$HOME/.dotnet"

                substep "dotnet-sdk installed to ~/.dotnet"
            else
                substep "[DRY RUN] Would install dotnet-sdk via dotnet-install.sh"
            fi
            ;;
    esac
}

# -----------------------------------------------------------------------------
# Starship prompt
# -----------------------------------------------------------------------------
install_starship() {
    if check_command starship; then
        substep "Starship is already installed"
        return
    fi

    substep "Installing Starship prompt..."

    if [[ "$DRY_RUN" == "false" ]]; then
        local starship_installer="/tmp/starship-install.sh"
        substep "Downloading Starship installer..."
        curl -sS https://starship.rs/install.sh -o "$starship_installer"

        if [[ -f "$starship_installer" && -s "$starship_installer" ]]; then
            substep "Executing Starship installer..."
            sh "$starship_installer" --yes
            rm -f "$starship_installer"
        else
            error "Failed to download Starship installer"
            rm -f "$starship_installer"
            return 1
        fi
    else
        substep "[DRY RUN] Would install Starship via official installer"
    fi
}

# -----------------------------------------------------------------------------
# yazi (terminal file manager)
# -----------------------------------------------------------------------------
install_yazi() {
    if check_command yazi; then
        substep "yazi is already installed"
        return
    fi

    substep "Installing yazi..."

    if [[ "$DRY_RUN" == "true" ]]; then
        substep "[DRY RUN] Would install yazi"
        return
    fi

    case "$PACKAGE_MANAGER" in
        "brew")
            brew install yazi ffmpeg sevenzip jq poppler resvg imagemagick
            ;;
        "pacman")
            sudo pacman -S --noconfirm yazi ffmpeg 7zip jq poppler resvg imagemagick
            ;;
        "apt")
            "${INSTALL_CMD_ARRAY[@]}" ffmpeg p7zip-full jq poppler-utils resvg imagemagick 2>/dev/null || true
            if check_command cargo; then
                cargo install --force yazi-build
            else
                warning "yazi requires cargo on Debian/Ubuntu. Install Rust first, then run: cargo install --force yazi-build"
            fi
            ;;
        "dnf"|"yum")
            if ! "${INSTALL_CMD_ARRAY[@]}" yazi 2>/dev/null; then
                if check_command cargo; then
                    cargo install --force yazi-build
                else
                    warning "yazi requires cargo. Install Rust first, then run: cargo install --force yazi-build"
                fi
            fi
            ;;
        *)
            if check_command cargo; then
                cargo install --force yazi-build
            else
                warning "Please install yazi manually: https://yazi-rs.github.io/docs/installation/"
            fi
            ;;
    esac
}
