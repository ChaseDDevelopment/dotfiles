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
# Use arrays instead of strings for commands to prevent injection
declare -a INSTALL_CMD_ARRAY
declare -a UPDATE_CMD_ARRAY

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
                # Download Homebrew installer to temporary file for safer execution
                local brew_installer="/tmp/homebrew-install.sh"
                substep "Downloading Homebrew installer..."
                curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh -o "$brew_installer"
                
                # Basic validation - check if file exists and has reasonable size
                if [[ -f "$brew_installer" && -s "$brew_installer" ]]; then
                    substep "Executing Homebrew installer..."
                    /bin/bash "$brew_installer"
                    rm -f "$brew_installer"
                    
                    # Add Homebrew to PATH
                    if [[ "$OS_ARCH" == "arm64" ]]; then
                        eval "$(/opt/homebrew/bin/brew shellenv)"
                    else
                        eval "$(/usr/local/bin/brew shellenv)"
                    fi
                else
                    error "Failed to download Homebrew installer"
                    rm -f "$brew_installer"
                    exit 1
                fi
            fi
        fi
        
        INSTALL_CMD_ARRAY=("brew" "install")
        UPDATE_CMD_ARRAY=("bash" "-c" "brew update && brew upgrade")
        
    elif [[ -f /etc/os-release ]]; then
        # Linux
        source /etc/os-release
        OS_NAME="${NAME:-Linux}"
        OS_VERSION="${VERSION_ID:-unknown}"
        
        if check_command apt; then
            # Debian/Ubuntu
            PACKAGE_MANAGER="apt"
            INSTALL_CMD_ARRAY=("sudo" "apt" "install" "-y")
            UPDATE_CMD_ARRAY=("bash" "-c" "sudo apt update && sudo apt upgrade -y")
            
        elif check_command dnf; then
            # Fedora
            PACKAGE_MANAGER="dnf"
            INSTALL_CMD_ARRAY=("sudo" "dnf" "install" "-y")
            UPDATE_CMD_ARRAY=("sudo" "dnf" "update" "-y")
            
        elif check_command yum; then
            # RHEL/CentOS
            PACKAGE_MANAGER="yum"
            INSTALL_CMD_ARRAY=("sudo" "yum" "install" "-y")
            UPDATE_CMD_ARRAY=("sudo" "yum" "update" "-y")
            
        elif check_command pacman; then
            # Arch Linux
            PACKAGE_MANAGER="pacman"
            INSTALL_CMD_ARRAY=("sudo" "pacman" "-S" "--noconfirm")
            UPDATE_CMD_ARRAY=("sudo" "pacman" "-Syu" "--noconfirm")
            
        elif check_command zypper; then
            # openSUSE
            PACKAGE_MANAGER="zypper"
            INSTALL_CMD_ARRAY=("sudo" "zypper" "install" "-y")
            UPDATE_CMD_ARRAY=("sudo" "zypper" "update" "-y")
            
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
                # Use array expansion to safely execute command
                if ! "${INSTALL_CMD_ARRAY[@]}" "$package"; then
                    # Check if the package is critical
                    case "$package" in
                        "git"|"curl"|"fish"|"tmux"|"neovim")
                            error "Failed to install critical package: $package"
                            error "Cannot continue without this package. Please install manually and retry."
                            exit 1
                            ;;
                        *)
                            warning "Failed to install optional package: $package"
                            warning "Some features may not work correctly."
                            ;;
                    esac
                fi
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
        if ! "${UPDATE_CMD_ARRAY[@]}"; then
            warning "System update failed. This is often non-critical."
            warning "You may want to run system updates manually later."
        else
            substep "System packages updated successfully"
        fi
    else
        substep "[DRY RUN] Would run: ${UPDATE_CMD_ARRAY[*]}"
    fi
}