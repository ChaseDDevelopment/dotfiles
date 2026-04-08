#!/usr/bin/env bash

# =============================================================================
# CLI Tool Installers
# =============================================================================
# Modern CLI tool replacements: eza, bat, ripgrep, fd, zoxide, tailspin,
# delta, lazygit, xh, yq, direnv, coreutils, clipboard utilities.
#
# Depends on: github-helpers.sh, package-helpers.sh, detect-os.sh
# =============================================================================

# -----------------------------------------------------------------------------
# eza (modern ls)
# -----------------------------------------------------------------------------
install_eza() {
    if check_command eza; then
        plan_add "  eza" "Package" "already installed"
        ui_info "eza is already installed"
        return
    fi

    plan_add "  eza" "Package" "would install"
    ui_info "Installing eza..."

    if [[ "$DRY_RUN" == "true" ]]; then
        ui_info "[DRY RUN] Would install eza"
        return
    fi

    case "$PACKAGE_MANAGER" in
        "brew")
            brew install eza
            ;;
        "pacman")
            sudo pacman -S --noconfirm eza
            ;;
        "apt")
            if check_command cargo; then
                ui_spin "Building eza (cargo)" cargo install eza
            else
                install_eza_from_github
            fi
            ;;
        "dnf"|"yum")
            if ! "${INSTALL_CMD_ARRAY[@]}" eza 2>/dev/null; then
                if check_command cargo; then
                    ui_spin "Building eza (cargo)" cargo install eza
                else
                    install_eza_from_github
                fi
            fi
            ;;
        *)
            install_eza_from_github
            ;;
    esac
}

install_eza_from_github() {
    local target
    target=$(platform_target_triple "gnu") || return 1

    local version
    version=$(github_latest_version "eza-community/eza") || return 1

    local url="https://github.com/eza-community/eza/releases/download/v${version}/eza_${target}.tar.gz"
    ui_spin "Downloading eza from GitHub" download_and_install_binary "$url" "eza"
}

# -----------------------------------------------------------------------------
# bat (modern cat)
# -----------------------------------------------------------------------------
install_bat() {
    if check_command bat; then
        plan_add "  bat" "Package" "already installed"
        ui_info "bat is already installed"
        return
    fi

    plan_add "  bat" "Package" "would install"
    ui_info "Installing bat..."

    case "$PACKAGE_MANAGER" in
        "brew")
            install_package "bat"
            ;;
        "apt")
            if [[ "$DRY_RUN" == "false" ]]; then
                "${INSTALL_CMD_ARRAY[@]}" bat

                if [[ -x "/usr/bin/batcat" ]]; then
                    sudo ln -sf /usr/bin/batcat /usr/local/bin/bat \
                        || ui_warn "Could not create system symlink for bat"
                    ui_info "Created symlink: /usr/local/bin/bat -> /usr/bin/batcat"
                else
                    ui_warn "batcat binary not found after install attempt"
                fi
            else
                ui_info "[DRY RUN] Would install bat and create symlink"
            fi
            ;;
        "dnf"|"yum"|"pacman")
            install_package "bat"
            ;;
        *)
            ui_warn "Please install bat manually"
            ;;
    esac
}

# -----------------------------------------------------------------------------
# ripgrep (modern grep)
# -----------------------------------------------------------------------------
install_ripgrep() {
    if check_command rg; then
        plan_add "  ripgrep" "Package" "already installed"
        ui_info "ripgrep is already installed"
        return
    fi

    plan_add "  ripgrep" "Package" "would install"
    ui_info "Installing ripgrep..."
    install_package "ripgrep"
}

# -----------------------------------------------------------------------------
# fd (modern find)
# -----------------------------------------------------------------------------
install_fd() {
    if check_command fd; then
        plan_add "  fd" "Package" "already installed"
        ui_info "fd is already installed"
        return
    fi

    plan_add "  fd" "Package" "would install"
    ui_info "Installing fd..."

    case "$PACKAGE_MANAGER" in
        "brew")
            install_package "fd"
            ;;
        "apt")
            if [[ "$DRY_RUN" == "false" ]]; then
                "${INSTALL_CMD_ARRAY[@]}" fd-find

                if [[ -x "/usr/bin/fdfind" ]]; then
                    sudo ln -sf /usr/bin/fdfind /usr/local/bin/fd \
                        || ui_warn "Could not create system symlink for fd"
                    ui_info "Created symlink: /usr/local/bin/fd -> /usr/bin/fdfind"
                else
                    ui_warn "fdfind binary not found after install attempt"
                fi
            else
                ui_info "[DRY RUN] Would install fd-find and create fd symlink"
            fi
            ;;
        "dnf"|"yum")
            install_package "fd-find"
            ;;
        "pacman")
            install_package "fd"
            ;;
        *)
            ui_warn "Please install fd manually"
            ;;
    esac
}

# -----------------------------------------------------------------------------
# zoxide (modern cd)
# -----------------------------------------------------------------------------
install_zoxide() {
    if check_command zoxide; then
        plan_add "  zoxide" "Package" "already installed"
        ui_info "zoxide is already installed"
        return
    fi

    plan_add "  zoxide" "Package" "would install"
    ui_info "Installing zoxide..."

    case "$PACKAGE_MANAGER" in
        "brew")
            install_package "zoxide"
            ;;
        "apt")
            if [[ "$DRY_RUN" == "false" ]]; then
                if apt list zoxide 2>/dev/null | grep -q zoxide; then
                    "${INSTALL_CMD_ARRAY[@]}" zoxide
                else
                    ui_spin "Installing zoxide (curl)" bash -c 'curl -sS https://raw.githubusercontent.com/ajeetdsouza/zoxide/main/install.sh | bash'
                fi
            else
                ui_info "[DRY RUN] Would install zoxide"
            fi
            ;;
        "dnf"|"yum")
            if [[ "$DRY_RUN" == "false" ]]; then
                if ! "${INSTALL_CMD_ARRAY[@]}" zoxide 2>/dev/null; then
                    ui_spin "Installing zoxide (curl)" bash -c 'curl -sS https://raw.githubusercontent.com/ajeetdsouza/zoxide/main/install.sh | bash'
                fi
            else
                ui_info "[DRY RUN] Would install zoxide"
            fi
            ;;
        "pacman")
            install_package "zoxide"
            ;;
        *)
            if [[ "$DRY_RUN" == "false" ]]; then
                ui_spin "Installing zoxide (curl)" bash -c 'curl -sS https://raw.githubusercontent.com/ajeetdsouza/zoxide/main/install.sh | bash'
            else
                ui_info "[DRY RUN] Would install zoxide via official installer"
            fi
            ;;
    esac
}

# -----------------------------------------------------------------------------
# tailspin (pretty log viewer)
# -----------------------------------------------------------------------------
install_tailspin() {
    if check_command tspin; then
        plan_add "  tailspin" "Package" "already installed"
        ui_info "tailspin is already installed"
        return
    fi

    plan_add "  tailspin" "Package" "would install"
    ui_info "Installing tailspin..."

    if [[ "$DRY_RUN" == "true" ]]; then
        ui_info "[DRY RUN] Would install tailspin via $PACKAGE_MANAGER (with GitHub fallback)"
        return
    fi

    local pkg_install_failed=false
    case "$PACKAGE_MANAGER" in
        "brew")
            brew install tailspin || pkg_install_failed=true
            ;;
        "pacman")
            sudo pacman -S --noconfirm tailspin || pkg_install_failed=true
            ;;
        *)
            pkg_install_failed=true
            ;;
    esac

    if [[ "$pkg_install_failed" == "true" ]]; then
        ui_info "Package manager install unavailable, downloading from GitHub..."
        install_tailspin_from_github
    fi
}

install_tailspin_from_github() {
    local target
    target=$(platform_target_triple "musl") || return 1

    local url="https://github.com/bensadeh/tailspin/releases/latest/download/tailspin-${target}.tar.gz"
    ui_spin "Downloading tailspin from GitHub" download_and_install_binary "$url" "tspin"
}

# -----------------------------------------------------------------------------
# delta (syntax-highlighted git diffs)
# -----------------------------------------------------------------------------
install_delta() {
    if check_command delta; then
        plan_add "  delta" "Package" "already installed"
        ui_info "delta is already installed"
        return
    fi

    plan_add "  delta" "Package" "would install"
    ui_info "Installing delta (git-delta)..."

    if [[ "$DRY_RUN" == "true" ]]; then
        ui_info "[DRY RUN] Would install delta"
        return
    fi

    case "$PACKAGE_MANAGER" in
        "brew")
            brew install git-delta
            ;;
        "pacman")
            sudo pacman -S --noconfirm git-delta
            ;;
        "apt")
            install_delta_from_github
            ;;
        "dnf"|"yum")
            if ! "${INSTALL_CMD_ARRAY[@]}" git-delta 2>/dev/null; then
                install_delta_from_github
            fi
            ;;
        *)
            if check_command cargo; then
                ui_spin "Building delta (cargo)" cargo install git-delta
            else
                install_delta_from_github
            fi
            ;;
    esac
}

install_delta_from_github() {
    local target
    target=$(platform_target_triple "musl") || return 1

    local version
    version=$(github_latest_version "dandavison/delta" "false") || return 1

    local url="https://github.com/dandavison/delta/releases/download/${version}/delta-${version}-${target}.tar.gz"
    ui_spin "Downloading delta from GitHub" download_and_install_binary "$url" "delta"
}

# -----------------------------------------------------------------------------
# lazygit (TUI git client)
# -----------------------------------------------------------------------------
install_lazygit() {
    if check_command lazygit; then
        plan_add "  lazygit" "Package" "already installed"
        ui_info "lazygit is already installed"
        return
    fi

    plan_add "  lazygit" "Package" "would install"
    ui_info "Installing lazygit..."

    if [[ "$DRY_RUN" == "true" ]]; then
        ui_info "[DRY RUN] Would install lazygit"
        return
    fi

    case "$PACKAGE_MANAGER" in
        "brew")
            brew install lazygit
            ;;
        "pacman")
            sudo pacman -S --noconfirm lazygit
            ;;
        *)
            install_lazygit_from_github
            ;;
    esac
}

install_lazygit_from_github() {
    detect_platform || return 1

    local lg_os lg_arch
    case "$PLATFORM_OS" in
        Linux)  lg_os="Linux" ;;
        Darwin) lg_os="Darwin" ;;
    esac
    case "$PLATFORM_ARCH" in
        x86_64)          lg_arch="x86_64" ;;
        aarch64|arm64)   lg_arch="arm64" ;;
    esac

    local version
    version=$(github_latest_version "jesseduffield/lazygit") || return 1

    local url="https://github.com/jesseduffield/lazygit/releases/download/v${version}/lazygit_${version}_${lg_os}_${lg_arch}.tar.gz"
    ui_spin "Downloading lazygit from GitHub" download_and_install_binary "$url" "lazygit"
}

# -----------------------------------------------------------------------------
# xh (modern HTTP client)
# -----------------------------------------------------------------------------
install_xh() {
    if check_command xh; then
        plan_add "  xh" "Package" "already installed"
        ui_info "xh is already installed"
        return
    fi

    plan_add "  xh" "Package" "would install"
    ui_info "Installing xh..."

    if [[ "$DRY_RUN" == "true" ]]; then
        ui_info "[DRY RUN] Would install xh"
        return
    fi

    case "$PACKAGE_MANAGER" in
        "brew")
            brew install xh
            ;;
        "pacman")
            sudo pacman -S --noconfirm xh
            ;;
        "apt")
            install_xh_from_github
            ;;
        "dnf"|"yum")
            if ! "${INSTALL_CMD_ARRAY[@]}" xh 2>/dev/null; then
                install_xh_from_github
            fi
            ;;
        *)
            if check_command cargo; then
                ui_spin "Building xh (cargo)" cargo install xh
            else
                install_xh_from_github
            fi
            ;;
    esac
}

install_xh_from_github() {
    local target
    target=$(platform_target_triple "musl") || return 1

    local version
    version=$(github_latest_version "ducaale/xh") || return 1

    local url="https://github.com/ducaale/xh/releases/download/v${version}/xh-v${version}-${target}.tar.gz"
    ui_spin "Downloading xh from GitHub" download_and_install_binary "$url" "xh"
}

# -----------------------------------------------------------------------------
# yq (YAML processor)
# -----------------------------------------------------------------------------
install_yq() {
    if check_command yq; then
        plan_add "  yq" "Package" "already installed"
        ui_info "yq is already installed"
        return
    fi

    plan_add "  yq" "Package" "would install"
    ui_info "Installing yq..."

    if [[ "$DRY_RUN" == "true" ]]; then
        ui_info "[DRY RUN] Would install yq"
        return
    fi

    case "$PACKAGE_MANAGER" in
        "brew")
            brew install yq
            ;;
        "pacman")
            sudo pacman -S --noconfirm yq
            ;;
        *)
            install_yq_from_github
            ;;
    esac
}

install_yq_from_github() {
    detect_platform || return 1

    local yq_os yq_arch
    case "$PLATFORM_OS" in
        Linux)  yq_os="linux" ;;
        Darwin) yq_os="darwin" ;;
    esac
    case "$PLATFORM_ARCH" in
        x86_64)          yq_arch="amd64" ;;
        aarch64|arm64)   yq_arch="arm64" ;;
    esac

    local url="https://github.com/mikefarah/yq/releases/latest/download/yq_${yq_os}_${yq_arch}"
    ui_spin "Downloading yq from GitHub" download_and_install_binary "$url" "yq" "binary"
}

# -----------------------------------------------------------------------------
# direnv (per-project environment variables)
# -----------------------------------------------------------------------------
install_direnv() {
    if check_command direnv; then
        plan_add "  direnv" "Package" "already installed"
        ui_info "direnv is already installed"
        return
    fi

    plan_add "  direnv" "Package" "would install"
    ui_info "Installing direnv..."

    if [[ "$DRY_RUN" == "true" ]]; then
        ui_info "[DRY RUN] Would install direnv"
        return
    fi

    case "$PACKAGE_MANAGER" in
        "brew")
            brew install direnv
            ;;
        "pacman")
            sudo pacman -S --noconfirm direnv
            ;;
        "apt")
            "${INSTALL_CMD_ARRAY[@]}" direnv
            ;;
        "dnf"|"yum")
            "${INSTALL_CMD_ARRAY[@]}" direnv
            ;;
        *)
            ui_warn "Please install direnv manually: https://direnv.net/docs/installation.html"
            ;;
    esac
}

# -----------------------------------------------------------------------------
# coreutils (GNU coreutils for macOS)
# -----------------------------------------------------------------------------
install_coreutils() {
    if [[ "$PACKAGE_MANAGER" != "brew" ]]; then
        return
    fi

    if command -v grm &>/dev/null; then
        plan_add "  coreutils" "Package" "already installed"
        ui_info "coreutils (GNU rm) is already installed"
        return
    fi

    plan_add "  coreutils" "Package" "would install"
    ui_info "Installing coreutils (GNU rm, ls, etc.)..."
    if [[ "$DRY_RUN" == "false" ]]; then
        brew install coreutils
    else
        ui_info "[DRY RUN] Would install coreutils via Homebrew"
    fi
}

# -----------------------------------------------------------------------------
# Clipboard utilities (Linux only)
# -----------------------------------------------------------------------------
install_clipboard_utils() {
    plan_add "  clipboard-utils" "Package" "would install"
    ui_info "Installing clipboard utilities..."

    case "$PACKAGE_MANAGER" in
        "apt")
            if [[ "$DRY_RUN" == "false" ]]; then
                "${INSTALL_CMD_ARRAY[@]}" xclip wl-clipboard 2>/dev/null \
                    || "${INSTALL_CMD_ARRAY[@]}" xclip || true
            else
                ui_info "[DRY RUN] Would install xclip and wl-clipboard"
            fi
            ;;
        "dnf"|"yum")
            if [[ "$DRY_RUN" == "false" ]]; then
                "${INSTALL_CMD_ARRAY[@]}" xclip wl-clipboard 2>/dev/null \
                    || "${INSTALL_CMD_ARRAY[@]}" xclip || true
            else
                ui_info "[DRY RUN] Would install xclip and wl-clipboard"
            fi
            ;;
        "pacman")
            if [[ "$DRY_RUN" == "false" ]]; then
                sudo pacman -S --noconfirm xclip wl-clipboard 2>/dev/null \
                    || sudo pacman -S --noconfirm xclip || true
            else
                ui_info "[DRY RUN] Would install xclip and wl-clipboard"
            fi
            ;;
    esac
}
