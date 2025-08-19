#!/usr/bin/env bash

# =============================================================================
# OS Detection Utilities
# =============================================================================
# Detects operating system and sets up package management variables
# =============================================================================

# Global variables for OS detection
export OS_NAME=""
export OS_VERSION=""
export OS_ARCH=""
export PACKAGE_MANAGER=""
export INSTALL_CMD=""
export UPDATE_CMD=""

detect_os() {
    # Detect architecture
    OS_ARCH="$(uname -m)"
    
    # Detect OS
    if [[ "$OSTYPE" == "darwin"* ]]; then
        # macOS
        OS_NAME="macOS"
        OS_VERSION="$(sw_vers -productVersion)"
        PACKAGE_MANAGER="brew"
        
        if ! check_command brew; then
            substep "Installing Homebrew..."
            if [[ "$DRY_RUN" == "false" ]]; then
                /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
                # Add Homebrew to PATH
                if [[ "$OS_ARCH" == "arm64" ]]; then
                    eval "$(/opt/homebrew/bin/brew shellenv)"
                else
                    eval "$(/usr/local/bin/brew shellenv)"
                fi
            fi
        fi
        
        INSTALL_CMD="brew install"
        UPDATE_CMD="brew update && brew upgrade"
        
    elif [[ -f /etc/os-release ]]; then
        # Linux
        source /etc/os-release
        OS_NAME="$NAME"
        OS_VERSION="$VERSION_ID"
        
        if check_command apt; then
            # Debian/Ubuntu
            PACKAGE_MANAGER="apt"
            INSTALL_CMD="sudo apt install -y"
            UPDATE_CMD="sudo apt update && sudo apt upgrade -y"
            
        elif check_command dnf; then
            # Fedora
            PACKAGE_MANAGER="dnf"
            INSTALL_CMD="sudo dnf install -y"
            UPDATE_CMD="sudo dnf update -y"
            
        elif check_command yum; then
            # RHEL/CentOS
            PACKAGE_MANAGER="yum"
            INSTALL_CMD="sudo yum install -y"
            UPDATE_CMD="sudo yum update -y"
            
        elif check_command pacman; then
            # Arch Linux
            PACKAGE_MANAGER="pacman"
            INSTALL_CMD="sudo pacman -S --noconfirm"
            UPDATE_CMD="sudo pacman -Syu --noconfirm"
            
        elif check_command zypper; then
            # openSUSE
            PACKAGE_MANAGER="zypper"
            INSTALL_CMD="sudo zypper install -y"
            UPDATE_CMD="sudo zypper update -y"
            
        else
            error "Unsupported package manager. Please install packages manually."
            exit 1
        fi
        
    else
        error "Unsupported operating system"
        exit 1
    fi
    
    substep "OS: $OS_NAME $OS_VERSION ($OS_ARCH)"
    substep "Package Manager: $PACKAGE_MANAGER"
}

# Package name mapping for different distros
get_package_name() {
    local generic_name="$1"
    
    case "$PACKAGE_MANAGER" in
        "brew")
            case "$generic_name" in
                "fish") echo "fish" ;;
                "tmux") echo "tmux" ;;
                "neovim") echo "neovim" ;;
                "git") echo "git" ;;
                "curl") echo "curl" ;;
                "wget") echo "wget" ;;
                "fzf") echo "fzf" ;;
                "eza") echo "eza" ;;
                "nodejs") echo "node" ;;
                *) echo "$generic_name" ;;
            esac
            ;;
        "apt")
            case "$generic_name" in
                "fish") echo "fish" ;;
                "tmux") echo "tmux" ;;
                "neovim") echo "neovim" ;;
                "git") echo "git" ;;
                "curl") echo "curl" ;;
                "wget") echo "wget" ;;
                "fzf") echo "fzf" ;;
                "eza") echo "eza" ;;  # May need snap or cargo install
                "nodejs") echo "nodejs npm" ;;
                "build-essential") echo "build-essential" ;;
                *) echo "$generic_name" ;;
            esac
            ;;
        "dnf"|"yum")
            case "$generic_name" in
                "fish") echo "fish" ;;
                "tmux") echo "tmux" ;;
                "neovim") echo "neovim" ;;
                "git") echo "git" ;;
                "curl") echo "curl" ;;
                "wget") echo "wget" ;;
                "fzf") echo "fzf" ;;
                "eza") echo "eza" ;;
                "nodejs") echo "nodejs npm" ;;
                "build-essential") echo "gcc gcc-c++ make" ;;
                *) echo "$generic_name" ;;
            esac
            ;;
        "pacman")
            case "$generic_name" in
                "fish") echo "fish" ;;
                "tmux") echo "tmux" ;;
                "neovim") echo "neovim" ;;
                "git") echo "git" ;;
                "curl") echo "curl" ;;
                "wget") echo "wget" ;;
                "fzf") echo "fzf" ;;
                "eza") echo "eza" ;;
                "nodejs") echo "nodejs npm" ;;
                "build-essential") echo "base-devel" ;;
                *) echo "$generic_name" ;;
            esac
            ;;
        *)
            echo "$generic_name"
            ;;
    esac
}

install_package() {
    local generic_name="$1"
    local package_names
    package_names="$(get_package_name "$generic_name")"
    
    for package in $package_names; do
        if ! check_package_installed "$package"; then
            substep "Installing $package..."
            if [[ "$DRY_RUN" == "false" ]]; then
                eval "$INSTALL_CMD $package" || {
                    warning "Failed to install $package, continuing..."
                }
            else
                substep "[DRY RUN] Would install: $package"
            fi
        else
            substep "$package is already installed"
        fi
    done
}

check_package_installed() {
    local package="$1"
    
    case "$PACKAGE_MANAGER" in
        "brew")
            brew list "$package" &>/dev/null
            ;;
        "apt")
            dpkg -l "$package" &>/dev/null
            ;;
        "dnf")
            dnf list installed "$package" &>/dev/null
            ;;
        "yum")
            yum list installed "$package" &>/dev/null
            ;;
        "pacman")
            pacman -Q "$package" &>/dev/null
            ;;
        *)
            # Fallback to command check
            check_command "$package"
            ;;
    esac
}

update_system() {
    substep "Updating system packages..."
    if [[ "$DRY_RUN" == "false" ]]; then
        eval "$UPDATE_CMD" || {
            warning "System update failed, continuing..."
        }
    else
        substep "[DRY RUN] Would run: $UPDATE_CMD"
    fi
}