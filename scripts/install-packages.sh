#!/usr/bin/env bash

# =============================================================================
# Package Installation Script
# =============================================================================
# Installs all required packages for the shell environment setup
# =============================================================================

# Compare versions: returns 0 (true) if $1 >= $2
version_gte() {
    [ "$(printf '%s\n' "$2" "$1" | sort -V | head -n1)" = "$2" ]
}

install_packages() {
    substep "Starting package installation"
    
    # Update system first
    update_system
    
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
    
    # Modern CLI tools
    substep "Installing modern CLI tools..."
    
    # Essential modern replacements
    install_eza      # Modern ls replacement
    install_bat      # Better cat with syntax highlighting
    install_ripgrep  # Fast grep replacement
    install_fd       # Fast find replacement
    install_zoxide   # Smart cd replacement
    install_coreutils # GNU coreutils for macOS (provides grm -I)

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
    install_uv      # Also installs Python via `uv python install --default`
    install_ruff

    # .NET SDK (for F#/C# LSP and Mason tools like fsautocomplete)
    install_dotnet_sdk

    success "Package installation completed"
}


install_coreutils() {
    # GNU coreutils provides grm, gls, etc. - only needed on macOS
    # Linux already has GNU coreutils as default
    if [[ "$PACKAGE_MANAGER" != "brew" ]]; then
        return
    fi

    if command -v grm &>/dev/null; then
        substep "coreutils (GNU rm) is already installed"
        return
    fi

    substep "Installing coreutils (GNU rm, ls, etc.)..."
    if [[ "$DRY_RUN" == "false" ]]; then
        brew install coreutils
    else
        substep "[DRY RUN] Would install coreutils via Homebrew"
    fi
}

install_eza() {
    if check_command eza; then
        substep "eza is already installed"
        return
    fi
    
    substep "Installing eza..."
    
    case "$PACKAGE_MANAGER" in
        "brew")
            if [[ "$DRY_RUN" == "false" ]]; then
                brew install eza
            else
                substep "[DRY RUN] Would install eza via brew"
            fi
            ;;
        "apt")
            # For Ubuntu/Debian, we need to use the official method
            if [[ "$DRY_RUN" == "false" ]]; then
                # Check if eza is available in repos (Ubuntu 22.04+)
                if apt list eza 2>/dev/null | grep -q eza; then
                    "${INSTALL_CMD_ARRAY[@]}" eza
                else
                    # Install via cargo or download binary
                    if check_command cargo; then
                        cargo install eza
                    else
                        warning "eza not available in repos and cargo not found. Skipping eza installation."
                        warning "You can install it manually later with: cargo install eza"
                    fi
                fi
            else
                substep "[DRY RUN] Would install eza"
            fi
            ;;
        "dnf"|"yum")
            if [[ "$DRY_RUN" == "false" ]]; then
                # Try package manager first, fallback to cargo
                if ! eval "$INSTALL_CMD eza" 2>/dev/null; then
                    if check_command cargo; then
                        cargo install eza
                    else
                        warning "eza installation failed. Install cargo and run: cargo install eza"
                    fi
                fi
            else
                substep "[DRY RUN] Would install eza"
            fi
            ;;
        "pacman")
            if [[ "$DRY_RUN" == "false" ]]; then
                sudo pacman -S --noconfirm eza
            else
                substep "[DRY RUN] Would install eza via pacman"
            fi
            ;;
        *)
            warning "Unknown package manager. Please install eza manually."
            ;;
    esac
}

install_starship() {
    if check_command starship; then
        substep "Starship is already installed"
        return
    fi
    
    substep "Installing Starship prompt..."
    
    if [[ "$DRY_RUN" == "false" ]]; then
        # Download Starship installer to temporary file for safer execution
        local starship_installer="/tmp/starship-install.sh"
        substep "Downloading Starship installer..."
        curl -sS https://starship.rs/install.sh -o "$starship_installer"
        
        # Basic validation - check if file exists and has reasonable size
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

install_bun() {
    if check_command bun; then
        substep "Bun is already installed"
        return
    fi
    
    substep "Installing Bun..."
    
    if [[ "$DRY_RUN" == "false" ]]; then
        # Download Bun installer to temporary file for safer execution
        local bun_installer="/tmp/bun-install.sh"
        substep "Downloading Bun installer..."
        curl -fsSL https://bun.sh/install -o "$bun_installer"
        
        # Basic validation - check if file exists and has reasonable size
        if [[ -f "$bun_installer" && -s "$bun_installer" ]]; then
            substep "Executing Bun installer..."
            bash "$bun_installer"
            rm -f "$bun_installer"
            
            # Add to PATH for current session
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

install_bat() {
    if check_command bat; then
        substep "bat is already installed"
        return
    fi
    
    substep "Installing bat..."
    
    case "$PACKAGE_MANAGER" in
        "brew")
            install_package "bat"
            ;;
        "apt")
            # Ubuntu/Debian: package is always 'batcat' (naming conflict with another 'bat' package)
            if [[ "$DRY_RUN" == "false" ]]; then
                "${INSTALL_CMD_ARRAY[@]}" batcat

                # Verify installation succeeded before creating symlink
                if [[ -x "/usr/bin/batcat" ]]; then
                    sudo ln -sf /usr/bin/batcat /usr/local/bin/bat || warning "Could not create system symlink for bat"
                    substep "Created symlink: /usr/local/bin/bat -> /usr/bin/batcat"
                else
                    warning "batcat binary not found after install attempt"
                fi
            else
                substep "[DRY RUN] Would install batcat and create bat symlink"
            fi
            ;;
        "dnf"|"yum")
            install_package "bat"
            ;;
        "pacman")
            install_package "bat"
            ;;
        *)
            warning "Please install bat manually"
            ;;
    esac
}

install_ripgrep() {
    if check_command rg; then
        substep "ripgrep is already installed"
        return
    fi
    
    substep "Installing ripgrep..."
    install_package "ripgrep"
}

install_fd() {
    if check_command fd; then
        substep "fd is already installed"
        return
    fi

    substep "Installing fd..."

    case "$PACKAGE_MANAGER" in
        "brew")
            install_package "fd"
            ;;
        "apt")
            # Ubuntu/Debian: package is 'fd-find', binary is 'fdfind' (naming conflict with another 'fd' package)
            if [[ "$DRY_RUN" == "false" ]]; then
                "${INSTALL_CMD_ARRAY[@]}" fd-find

                # Verify installation succeeded before creating symlink
                if [[ -x "/usr/bin/fdfind" ]]; then
                    sudo ln -sf /usr/bin/fdfind /usr/local/bin/fd || warning "Could not create system symlink for fd"
                    substep "Created symlink: /usr/local/bin/fd -> /usr/bin/fdfind"
                else
                    warning "fdfind binary not found after install attempt"
                fi
            else
                substep "[DRY RUN] Would install fd-find and create fd symlink"
            fi
            ;;
        "dnf"|"yum")
            install_package "fd-find"
            ;;
        "pacman")
            install_package "fd"
            ;;
        *)
            warning "Please install fd manually"
            ;;
    esac
}

install_zoxide() {
    if check_command zoxide; then
        substep "zoxide is already installed"
        return
    fi

    substep "Installing zoxide..."

    case "$PACKAGE_MANAGER" in
        "brew")
            install_package "zoxide"
            ;;
        "apt")
            # Check if available in repos, otherwise use installer
            if [[ "$DRY_RUN" == "false" ]]; then
                if apt list zoxide 2>/dev/null | grep -q zoxide; then
                    "${INSTALL_CMD_ARRAY[@]}" zoxide
                else
                    # Use official installer
                    curl -sS https://raw.githubusercontent.com/ajeetdsouza/zoxide/main/install.sh | bash
                fi
            else
                substep "[DRY RUN] Would install zoxide"
            fi
            ;;
        "dnf"|"yum")
            if [[ "$DRY_RUN" == "false" ]]; then
                if ! eval "$INSTALL_CMD zoxide" 2>/dev/null; then
                    curl -sS https://raw.githubusercontent.com/ajeetdsouza/zoxide/main/install.sh | bash
                fi
            else
                substep "[DRY RUN] Would install zoxide"
            fi
            ;;
        "pacman")
            install_package "zoxide"
            ;;
        *)
            # Fallback to official installer
            if [[ "$DRY_RUN" == "false" ]]; then
                curl -sS https://raw.githubusercontent.com/ajeetdsouza/zoxide/main/install.sh | bash
            else
                substep "[DRY RUN] Would install zoxide via official installer"
            fi
            ;;
    esac
}

install_clipboard_utils() {
    substep "Installing clipboard utilities..."

    case "$PACKAGE_MANAGER" in
        "apt")
            if [[ "$DRY_RUN" == "false" ]]; then
                # Install xclip for X11 and wl-clipboard for Wayland
                "${INSTALL_CMD_ARRAY[@]}" xclip wl-clipboard 2>/dev/null || "${INSTALL_CMD_ARRAY[@]}" xclip || true
            else
                substep "[DRY RUN] Would install xclip and wl-clipboard"
            fi
            ;;
        "dnf"|"yum")
            if [[ "$DRY_RUN" == "false" ]]; then
                eval "$INSTALL_CMD xclip wl-clipboard" 2>/dev/null || eval "$INSTALL_CMD xclip" || true
            else
                substep "[DRY RUN] Would install xclip and wl-clipboard"
            fi
            ;;
        "pacman")
            if [[ "$DRY_RUN" == "false" ]]; then
                sudo pacman -S --noconfirm xclip wl-clipboard 2>/dev/null || sudo pacman -S --noconfirm xclip || true
            else
                substep "[DRY RUN] Would install xclip and wl-clipboard"
            fi
            ;;
    esac
}

install_uv() {
    # Install UV if not present
    if check_command uv; then
        substep "UV is already installed"
    else
        substep "Installing UV..."
        if [[ "$DRY_RUN" == "false" ]]; then
            # Download UV installer to temporary file for safer execution
            local uv_installer="/tmp/uv-install.sh"
            substep "Downloading UV installer..."
            curl -LsSf https://astral.sh/uv/install.sh -o "$uv_installer"

            # Basic validation - check if file exists and has reasonable size
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

    # Install Python via UV (provides complete Python with venv for Mason/neovim)
    # This is needed because system Python on Ubuntu often lacks venv module
    if [[ -x "$HOME/.local/bin/python3" ]]; then
        substep "Python already installed via UV"
    else
        substep "Installing Python via UV..."
        if [[ "$DRY_RUN" == "false" ]]; then
            uv python install --default
            substep "Python installed to ~/.local/bin (includes venv module for Mason)"
        else
            substep "[DRY RUN] Would install Python via UV"
        fi
    fi
}

install_ruff() {
    if check_command ruff; then
        substep "Ruff is already installed"
        return
    fi
    
    substep "Installing Ruff via UV..."
    if [[ "$DRY_RUN" == "false" ]]; then
        # Ensure UV is installed first
        if ! check_command uv; then
            install_uv
        fi
        uv tool install ruff
    else
        substep "[DRY RUN] Would install Ruff via UV"
    fi
}

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
            # For apt, dnf, yum, and others: use Microsoft's official install script
            # This works on any Linux distro without needing package repos
            if [[ "$DRY_RUN" == "false" ]]; then
                substep "Using Microsoft's dotnet-install.sh script..."
                local install_script="/tmp/dotnet-install.sh"
                curl -sSL https://dot.net/v1/dotnet-install.sh -o "$install_script"
                chmod +x "$install_script"
                # Install to ~/.dotnet (user-local, no sudo needed)
                "$install_script" --channel LTS --install-dir "$HOME/.dotnet"
                rm -f "$install_script"

                # Add to PATH for current session
                export PATH="$HOME/.dotnet:$PATH"
                export DOTNET_ROOT="$HOME/.dotnet"

                substep "dotnet-sdk installed to ~/.dotnet"
            else
                substep "[DRY RUN] Would install dotnet-sdk via dotnet-install.sh"
            fi
            ;;
    esac
}

install_neovim() {
    local min_version="0.11.0"

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
            # macOS: Install HEAD version to get latest features
            if [[ "$DRY_RUN" == "false" ]]; then
                brew install --HEAD neovim
            else
                substep "[DRY RUN] Would install Neovim HEAD via brew"
            fi
            ;;
        "pacman")
            # Arch Linux: Try to install neovim-git from AUR, fallback to official package
            if [[ "$DRY_RUN" == "false" ]]; then
                # Check if yay or paru AUR helper is available
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
            # Ubuntu/Debian: Install from GitHub releases for latest version
            if [[ "$DRY_RUN" == "false" ]]; then
                substep "Installing latest Neovim from GitHub releases..."

                # Detect architecture
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

                # Download and extract
                curl -LO "$nvim_url"
                sudo rm -rf /opt/nvim /opt/nvim-linux-x86_64 /opt/nvim-linux-arm64
                sudo tar -C /opt -xzf nvim-linux-*.tar.gz
                rm -f nvim-linux-*.tar.gz

                # Create symlink (remove old one first)
                sudo rm -f /usr/local/bin/nvim
                sudo ln -s /opt/nvim-linux-*/bin/nvim /usr/local/bin/nvim

                substep "Neovim installed from GitHub releases"
            else
                substep "[DRY RUN] Would install latest Neovim from GitHub releases"
            fi
            ;;
        "dnf"|"yum")
            # RHEL/Fedora: Use official package, recommend Flatpak for latest
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

# Install Rust and Cargo if needed (for some tools)
install_rust() {
    if check_command cargo; then
        substep "Rust/Cargo is already installed"
        return
    fi
    
    substep "Installing Rust and Cargo..."
    
    if [[ "$DRY_RUN" == "false" ]]; then
        # Download Rust installer to temporary file for safer execution
        local rust_installer="/tmp/rustup-init.sh"
        substep "Downloading Rust installer..."
        curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs -o "$rust_installer"
        
        # Basic validation - check if file exists and has reasonable size
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

# Special handling for specific distros
handle_special_cases() {
    case "$OS_NAME" in
        *"Ubuntu"*|*"Debian"*)
            # Ubuntu/Debian specific packages
            if ! check_command add-apt-repository; then
                install_package "software-properties-common"
            fi
            ;;
        *"CentOS"*|*"RHEL"*)
            # RHEL/CentOS specific setup
            if ! check_command dnf && check_command yum; then
                # Enable EPEL for additional packages
                if [[ "$DRY_RUN" == "false" ]]; then
                    sudo yum install -y epel-release
                fi
            fi
            ;;
        *"Arch"*)
            # Arch Linux specific setup
            # Most packages should be available in official repos
            ;;
    esac
}