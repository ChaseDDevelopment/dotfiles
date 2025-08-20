#!/usr/bin/env bash

# =============================================================================
# Package Installation Script
# =============================================================================
# Installs all required packages for the shell environment setup
# =============================================================================

install_packages() {
    substep "Starting package installation"
    
    # Update system first
    update_system
    
    # Core development tools
    substep "Installing core development tools..."
    install_package "git"
    install_package "curl"
    install_package "wget"
    
    # Build tools (needed for some installations)
    if [[ "$PACKAGE_MANAGER" != "brew" ]]; then
        install_package "build-essential"
    fi
    
    # Shell and terminal tools
    substep "Installing shell and terminal tools..."
    install_package "fish"
    install_package "tmux"
    install_package "fzf"
    
    # Text editor
    substep "Installing Neovim..."
    install_package "neovim"
    
    # Modern CLI tools
    substep "Installing modern CLI tools..."
    
    # Essential modern replacements
    install_eza      # Modern ls replacement
    install_bat      # Better cat with syntax highlighting
    install_ripgrep  # Fast grep replacement
    install_fd       # Fast find replacement
    
    # Install Starship prompt
    install_starship
    
    # Install Node.js (we'll use NVM later, but this provides a base)
    install_package "nodejs"
    
    # Install Bun
    install_bun
    
    # Python tools
    install_uv
    install_ruff
    
    success "Package installation completed"
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
                    sudo apt install -y eza
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
            # Ubuntu/Debian need special handling for bat
            if [[ "$DRY_RUN" == "false" ]]; then
                # Try to install batcat (Ubuntu names it differently)
                if apt list bat 2>/dev/null | grep -q bat; then
                    sudo apt install -y bat
                else
                    sudo apt install -y batcat
                    # Create symlink if installed as batcat
                    sudo ln -sf /usr/bin/batcat /usr/local/bin/bat 2>/dev/null || true
                fi
            else
                substep "[DRY RUN] Would install bat"
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
            # Ubuntu/Debian package is fd-find
            if [[ "$DRY_RUN" == "false" ]]; then
                sudo apt install -y fd-find
                # Create symlink
                sudo ln -sf /usr/bin/fdfind /usr/local/bin/fd 2>/dev/null || true
            else
                substep "[DRY RUN] Would install fd-find"
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

install_uv() {
    if check_command uv; then
        substep "UV is already installed"
        return
    fi
    
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