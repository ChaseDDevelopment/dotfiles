#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BACKUP_DIR="$HOME/.dotfiles-backup-$(date +%Y%m%d-%H%M%S)"
LOG_FILE="$SCRIPT_DIR/install.log"

# Mode variables (set by menu, not flags)
INSTALL_MODE=""
DRY_RUN=false
SKIP_PACKAGES=false
SKIP_UPDATE=false
VERBOSE=false
CLEAN_BACKUP=false
SELECTED_COMPONENTS=""
declare -a PLAN_ROWS=()

# Add a row to the dry-run summary table
plan_add() {
    PLAN_ROWS+=("$1,$2,$3")
}

# =============================================================================
# Logging
# =============================================================================

log() {
    echo "$(date '+%Y-%m-%d %H:%M:%S') $*" >> "$LOG_FILE"
}

# =============================================================================
# UI Functions (gum log writes to stderr, gum style writes to stdout)
# =============================================================================

ui_banner() {
    gum style --border rounded --border-foreground 99 --foreground 255 \
        --bold --align center --width 60 --padding "1 2" "$@" >&2
}

ui_step() {
    echo "" >&2
    gum style --bold --foreground 99 "==> $*" >&2
    log "STEP: $*"
}

ui_info() {
    gum style --faint "  $*" >&2
    log "INFO: $*"
}

ui_success() {
    gum style --foreground 2 "  ✓ $*" >&2
    log "SUCCESS: $*"
}

ui_warn() {
    gum log --level warn "$*"
    log "WARNING: $*"
}

ui_error() {
    gum log --level error "$*"
    log "ERROR: $*"
}

ui_confirm() {
    gum confirm "$@"
}

ui_spin() {
    local label="$1"; shift
    local log_tmp
    log_tmp=$(mktemp)

    if [[ "$DRY_RUN" == "true" ]]; then
        gum style --foreground 3 --faint "  → [DRY RUN] $label" >&2
        rm -f "$log_tmp"
        return 0
    fi

    local frames=("⣾" "⣽" "⣻" "⢿" "⡿" "⣟" "⣯" "⣷")
    local i=0 rc=0

    # Show initial spinner frame immediately (visible even for fast commands)
    printf "  %s %s" "${frames[0]}" "$label" >&2

    # Run command in background
    "$@" >"$log_tmp" 2>&1 &
    local pid=$!

    # Animate spinner while command runs
    while kill -0 "$pid" 2>/dev/null; do
        sleep 0.08
        i=$((i + 1))
        printf "\r  %s %s" "${frames[$((i % ${#frames[@]}))]}" "$label" >&2
    done

    wait "$pid" || rc=$?

    # Clear spinner line
    printf "\r\033[2K" >&2

    cat "$log_tmp" >> "$LOG_FILE" 2>/dev/null

    if [[ $rc -eq 0 ]]; then
        ui_success "$label"
    else
        ui_error "$label failed (exit $rc)"
        echo "--- Last 20 lines ---" >&2
        tail -20 "$log_tmp" >&2
    fi

    rm -f "$log_tmp"
    return $rc
}

# =============================================================================
# Gum Bootstrap
# =============================================================================

install_gum() {
    if [[ "$(uname -s)" == "Darwin" ]] && command -v brew >/dev/null 2>&1; then
        brew install gum >/dev/null 2>&1
    else
        local version="0.14.5"
        local os arch
        case "$(uname -s)" in Darwin) os="Darwin";; *) os="Linux";; esac
        case "$(uname -m)" in arm64|aarch64) arch="arm64";; *) arch="x86_64";; esac
        local url="https://github.com/charmbracelet/gum/releases/download/v${version}/gum_${version}_${os}_${arch}.tar.gz"
        local bin_dir="$HOME/.local/bin"
        mkdir -p "$bin_dir"
        curl -fsSL "$url" | tar xz -C /tmp --strip-components=1 --wildcards '*/gum' 2>/dev/null && \
            mv /tmp/gum "$bin_dir/gum" && chmod +x "$bin_dir/gum" && \
            export PATH="$bin_dir:$PATH"
    fi
}

require_gum() {
    if command -v gum >/dev/null 2>&1; then return 0; fi

    echo ""
    echo "  This installer requires gum (charmbracelet/gum)."
    echo "  https://github.com/charmbracelet/gum"
    echo ""
    read -p "  Install gum? (Y/n) " -n 1 -r 
    echo ""

    if [[ $REPLY =~ ^[Nn]$ ]]; then
        echo "  Installation cancelled." >&2
        exit 1
    fi

    echo "  Installing gum..."
    if install_gum; then
        echo "  ✓ gum installed"
    else
        echo "  ✗ Failed to install gum." >&2
        exit 1
    fi
}

# =============================================================================
# Utility Functions
# =============================================================================

check_command() {
    command -v "$1" >/dev/null 2>&1
}

symlink_if_needed() {
    local source="$1"
    local target="$2"

    if [[ -L "$target" ]] && [[ "$(readlink "$target")" == "$source" ]]; then
        ui_info "Symlink already correct: $target -> $source"
        return 0
    fi

    if [[ "$DRY_RUN" == "true" ]]; then
        ui_info "[DRY RUN] Would symlink $target -> $source"
        return 0
    fi

    if [[ -e "$target" ]] || [[ -L "$target" ]]; then
        rm -rf "$target"
    fi

    local target_dir
    target_dir="$(dirname "$target")"
    mkdir -p "$target_dir"
    ln -sf "$source" "$target"
    ui_success "Symlinked: $target -> $source"
}

backup_file() {
    local file="$1"
    if [[ -f "$file" || -d "$file" ]]; then
        if [[ "$DRY_RUN" == "false" ]]; then
            mkdir -p "$BACKUP_DIR"
            ui_spin "Backing up $(basename "$file")" cp -a "$file" "$BACKUP_DIR/"
        else
            ui_info "[DRY RUN] Would backup $file"
        fi
    fi
}

# =============================================================================
# System Requirements Check
# =============================================================================

check_requirements() {
    # Check if running as root
    if [[ $EUID -eq 0 ]]; then
        ui_error "This script should not be run as root"
        exit 1
    fi

    # Check for required commands
    local required_commands=("curl" "git")
    for cmd in "${required_commands[@]}"; do
        if ! check_command "$cmd"; then
            ui_error "Required command not found: $cmd"
            exit 1
        fi
    done

    # Check internet connectivity
    if ! curl -s --head "https://github.com" > /dev/null; then
        ui_error "Internet connectivity required for installation"
        exit 1
    fi
}

# =============================================================================
# Configuration Reload Functions
# =============================================================================

reload_configurations() {
    if [[ "$DRY_RUN" == "false" ]]; then
        # Note: Zsh configuration will be loaded on next shell start
        # We can't easily reload zsh config from within bash
        ui_info "Zsh configuration will be loaded on next shell start"

        # Install and reload Tmux plugins if Tmux is running
        if check_command tmux; then
            ui_info "Installing/updating Tmux plugins"
            # Source tmux config
            if tmux list-sessions &>/dev/null; then
                if tmux source-file ~/.config/tmux/tmux.conf 2>/dev/null; then
                    ui_info "Tmux configuration reloaded successfully"
                else
                    ui_warn "Failed to reload Tmux configuration"
                    ui_warn "You may need to restart tmux manually"
                fi
            fi

            # Install TPM plugins if TPM is available
            if [[ -f ~/.tmux/plugins/tpm/scripts/install_plugins.sh ]]; then
                if ~/.tmux/plugins/tpm/scripts/install_plugins.sh &>/dev/null; then
                    ui_info "Tmux plugins installed successfully"
                else
                    ui_warn "Failed to install Tmux plugins"
                    ui_warn "You may need to install them manually with Prefix + I"
                fi
            fi
        fi

        # Clear Starship cache
        if check_command starship; then
            ui_info "Clearing Starship cache"
            rm -rf ~/.cache/starship 2>/dev/null || true
        fi

        ui_success "Configurations prepared"
    else
        ui_info "[DRY RUN] Would prepare configurations for reload"
    fi
}

# =============================================================================
# Menu System
# =============================================================================

show_main_menu() {
    local mode
    mode=$(gum choose --cursor-prefix "❯ " \
        --header "What would you like to do?" \
        "Install (recommended)" \
        "Custom Install (pick components)" \
        "Dry Run (preview changes)" \
        "Update All Tools" \
        "Restore from Backup" \
        "Exit" \
        ) || true


    case "$mode" in
        "Install"*)        INSTALL_MODE="install" ;;
        "Custom"*)         INSTALL_MODE="configure" ;;
        "Dry Run"*)        INSTALL_MODE="install"; DRY_RUN=true ;;
        "Update"*)         INSTALL_MODE="update" ;;
        "Restore"*)        INSTALL_MODE="restore" ;;
        "Exit"|"")         exit 0 ;;
    esac
}

show_options_menu() {
    local options
    options=$(gum choose --no-limit \
        --header "Options (space to toggle, enter to continue):" \
        "Skip system update" \
        "Skip packages" \
        "Verbose output" \
        "Clean backup after" \
        ) || true

    [[ "$options" == *"Skip system update"* ]] && SKIP_UPDATE=true
    [[ "$options" == *"Skip packages"* ]] && SKIP_PACKAGES=true
    [[ "$options" == *"Verbose"* ]] && VERBOSE=true
    [[ "$options" == *"Clean backup"* ]] && CLEAN_BACKUP=true
    return 0
}

show_component_picker() {
    SELECTED_COMPONENTS=$(gum choose --no-limit \
        --header "Select components to install:" \
        "All" "Zsh" "Tmux" "Neovim" "Starship" "Atuin" "Ghostty" "Yazi" "Git" \
        ) || true

    if [[ -z "$SELECTED_COMPONENTS" ]]; then
        ui_error "No components selected"
        exit 1
    fi
}

is_selected() {
    [[ "$INSTALL_MODE" != "configure" ]] && return 0
    echo "$SELECTED_COMPONENTS" | grep -qx "All" && return 0
    echo "$SELECTED_COMPONENTS" | grep -qx "$1" && return 0
    return 1
}

# =============================================================================
# Main Installation Flow
# =============================================================================

main() {
    # Require TTY
    if [[ ! -t 0 ]]; then
        echo "This installer requires an interactive terminal." >&2
        exit 1
    fi

    # Bootstrap gum
    require_gum

    # Initialize log
    echo "Installation started at $(date)" > "$LOG_FILE"

    # Banner
    echo ""
    ui_banner "Shell Environment Setup" "One Stop Shop Installer"
    echo ""

    # Main menu
    show_main_menu

    # Options (for install/configure/dry-run modes)
    if [[ "$INSTALL_MODE" != "update" && "$INSTALL_MODE" != "restore" ]]; then
        show_options_menu
    fi

    # Component picker (configure mode only)
    if [[ "$INSTALL_MODE" == "configure" ]]; then
        show_component_picker
    fi

    # DRY RUN notice
    if [[ "$DRY_RUN" == "true" ]]; then
        gum style --border double --border-foreground 3 --foreground 3 \
            --bold --padding "0 2" "DRY RUN MODE — No changes will be made" >&2
    fi

    # Source helpers
    source "$SCRIPT_DIR/scripts/detect-os.sh"
    source "$SCRIPT_DIR/scripts/package-helpers.sh"

    # Detect OS
    ui_step "Detecting operating system"
    detect_os
    ui_success "Detected OS: $OS_NAME $OS_VERSION"

    # Handle modes
    case "$INSTALL_MODE" in
        "update")
            source "$SCRIPT_DIR/scripts/update-packages.sh"
            update_all_packages
            echo ""
            ui_banner "All packages updated successfully!"
            exit 0
            ;;
        "restore")
            source "$SCRIPT_DIR/scripts/restore-backup.sh"
            # TODO: add gum choose for backup selection
            restore_from_backup ""
            exit 0
            ;;
    esac

    # Check requirements
    ui_step "Checking system requirements"
    check_requirements
    ui_success "System requirements check passed"

    # Install packages
    if [[ "$SKIP_PACKAGES" != "true" ]]; then
        ui_step "Installing system packages"
        source "$SCRIPT_DIR/scripts/install-packages.sh"
        install_packages
    fi

    # Install tools from official sources
    ui_step "Installing tools from official sources"
    source "$SCRIPT_DIR/scripts/install-tools.sh"
    install_all_tools

    # Setup components
    if is_selected "Zsh"; then
        plan_add "Zsh" "Configure" "symlinks + plugins"
        ui_step "Setting up Zsh shell"
        source "$SCRIPT_DIR/scripts/setup-zsh.sh"
        setup_zsh
    fi

    if is_selected "Tmux"; then
        plan_add "Tmux" "Configure" "symlinks + plugins + TPM"
        ui_step "Setting up Tmux"
        source "$SCRIPT_DIR/scripts/setup-tmux.sh"
        setup_tmux
    fi

    if is_selected "Neovim"; then
        plan_add "Neovim" "Configure" "symlinks + vim.pack"
        ui_step "Setting up Neovim"
        source "$SCRIPT_DIR/scripts/setup-neovim.sh"
        setup_neovim
    fi

    if is_selected "Starship"; then
        plan_add "Starship" "Configure" "symlink config"
        ui_step "Setting up Starship prompt"
        source "$SCRIPT_DIR/scripts/setup-starship.sh"
        setup_starship
    fi

    if is_selected "Atuin"; then
        plan_add "Atuin" "Configure" "symlink config"
        ui_step "Setting up Atuin shell history"
        source "$SCRIPT_DIR/scripts/setup-atuin.sh"
        setup_atuin
    fi

    if is_selected "Ghostty"; then
        plan_add "Ghostty" "Configure" "symlink config"
        ui_step "Setting up Ghostty terminal"
        source "$SCRIPT_DIR/scripts/setup-ghostty.sh"
        setup_ghostty
    fi

    if is_selected "Yazi"; then
        plan_add "Yazi" "Configure" "symlinks + plugins"
        ui_step "Setting up Yazi file manager"
        source "$SCRIPT_DIR/scripts/setup-yazi.sh"
        setup_yazi
    fi

    if is_selected "Git"; then
        plan_add "Git" "Configure" "symlinks"
        ui_step "Setting up Git and lazygit configuration"
        source "$SCRIPT_DIR/scripts/setup-git.sh"
        setup_git
    fi

    # Finalize
    ui_step "Finalizing installation"
    reload_configurations

    # Clean backup if requested
    if [[ "$CLEAN_BACKUP" == "true" ]] && [[ -d "${BACKUP_DIR:-}" ]]; then
        rm -rf "$BACKUP_DIR"
        ui_info "Backup cleaned up"
    fi

    # Dry-run summary table
    if [[ "$DRY_RUN" == "true" ]] && [[ ${#PLAN_ROWS[@]} -gt 0 ]]; then
        echo "" >&2
        gum style --bold --foreground 99 "==> Installation Plan Summary" >&2
        {
            echo "Component,Action,Status"
            for row in "${PLAN_ROWS[@]}"; do
                echo "$row"
            done
        } | gum table --print --border rounded --border.foreground 99 >&2
    fi

    # Success
    echo ""
    ui_banner "Installation completed successfully!"
    echo ""
    ui_info "Installation log: $LOG_FILE"
    echo ""
    gum style --foreground 99 --bold "Quick start:" >&2
    gum style --foreground 255 "  tmux       # Start Tmux" >&2
    gum style --foreground 255 "  nvim       # Start Neovim" >&2
    echo ""

    if [[ "$DRY_RUN" == "true" ]]; then
        gum style --border double --border-foreground 3 --foreground 3 \
            --bold --padding "0 2" "This was a dry run — no changes were made" >&2
    fi
}

# Error handling
trap 'echo "ERROR: Installation failed at line $LINENO! Check $LOG_FILE for details" >&2; exit 1' ERR

# Run main function with all arguments
main "$@"
