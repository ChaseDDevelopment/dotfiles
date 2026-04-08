#!/usr/bin/env bash

# =============================================================================
# OS Detection Utilities
# =============================================================================
# Detects operating system and sets up package management variables.
# Package helper functions (install_package, etc.) live in package-helpers.sh.
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
                local brew_installer="/tmp/homebrew-install.sh"
                substep "Downloading Homebrew installer..."
                curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh -o "$brew_installer"

                if [[ -f "$brew_installer" && -s "$brew_installer" ]]; then
                    substep "Executing Homebrew installer..."
                    /bin/bash "$brew_installer"
                    rm -f "$brew_installer"

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
            PACKAGE_MANAGER="apt"
            if check_command nala; then
                INSTALL_CMD_ARRAY=("sudo" "nala" "install" "-y")
                UPDATE_CMD_ARRAY=("bash" "-c" "sudo nala upgrade -y")
            else
                INSTALL_CMD_ARRAY=("sudo" "apt-get" "install" "-y")
                UPDATE_CMD_ARRAY=("bash" "-c" "sudo apt-get update && sudo apt-get upgrade -y")
            fi

        elif check_command dnf; then
            PACKAGE_MANAGER="dnf"
            INSTALL_CMD_ARRAY=("sudo" "dnf" "install" "-y")
            UPDATE_CMD_ARRAY=("sudo" "dnf" "update" "-y")

        elif check_command yum; then
            PACKAGE_MANAGER="yum"
            INSTALL_CMD_ARRAY=("sudo" "yum" "install" "-y")
            UPDATE_CMD_ARRAY=("sudo" "yum" "update" "-y")

        elif check_command pacman; then
            PACKAGE_MANAGER="pacman"
            INSTALL_CMD_ARRAY=("sudo" "pacman" "-S" "--noconfirm")
            UPDATE_CMD_ARRAY=("sudo" "pacman" "-Syu" "--noconfirm")

        elif check_command zypper; then
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
