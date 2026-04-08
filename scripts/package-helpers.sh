#!/usr/bin/env bash

# =============================================================================
# Package Management Helpers
# =============================================================================
# Generic package install/check/update functions that work across all
# supported package managers. Must be sourced AFTER detect-os.sh so that
# PACKAGE_MANAGER, INSTALL_CMD_ARRAY, and UPDATE_CMD_ARRAY are set.
# =============================================================================

# Map a generic package name to the distro-specific package name.
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
                "yazi") echo "yazi" ;;
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
                "yazi") echo "yazi" ;;
                "eza") echo "eza" ;;
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
                "yazi") echo "yazi" ;;
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
                "yazi") echo "yazi" ;;
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

# Install a package using the detected system package manager.
# Handles name mapping and critical-vs-optional failure behaviour.
install_package() {
    local generic_name="$1"
    local package_names
    package_names="$(get_package_name "$generic_name")"

    for package in $package_names; do
        if ! check_package_installed "$package"; then
            if ! run_with_spinner "Installing $package" "${INSTALL_CMD_ARRAY[@]}" "$package"; then
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
            success "$package is already installed"
        fi
    done
}

# Check whether a package is installed via the system package manager.
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
            check_command "$package"
            ;;
    esac
}

# Update all system packages using the detected package manager.
update_system() {
    if ! run_with_spinner "Updating system packages" "${UPDATE_CMD_ARRAY[@]}"; then
        warning "System update failed. This is often non-critical."
        warning "You may want to run system updates manually later."
    fi
}
