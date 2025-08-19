#!/usr/bin/env bash

# =============================================================================
# Shell Environment Setup - "One Stop Shop" Installer
# =============================================================================
# Complete shell environment setup including Fish, Tmux, Neovim, and Starship
# Supports macOS and Linux distributions
# =============================================================================

set -euo pipefail

# Colors for output
readonly RED='\033[0;31m'
readonly GREEN='\033[0;32m'
readonly YELLOW='\033[1;33m'
readonly BLUE='\033[0;34m'
readonly PURPLE='\033[0;35m'
readonly CYAN='\033[0;36m'
readonly WHITE='\033[1;37m'
readonly NC='\033[0m' # No Color

# Script configuration
readonly SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly BACKUP_DIR="$HOME/.dotfiles-backup-$(date +%Y%m%d-%H%M%S)"
readonly LOG_FILE="$SCRIPT_DIR/install.log"

# Configuration
DRY_RUN=false
SKIP_PACKAGES=false
CONFIG_ONLY=false
RESTORE_BACKUP=""
VERBOSE=false

# =============================================================================
# Utility Functions
# =============================================================================

log() {
    echo "$(date '+%Y-%m-%d %H:%M:%S') $*" | tee -a "$LOG_FILE"
}

info() {
    echo -e "${BLUE}ℹ${NC} $*"
    log "INFO: $*"
}

success() {
    echo -e "${GREEN}✅${NC} $*"
    log "SUCCESS: $*"
}

warning() {
    echo -e "${YELLOW}⚠${NC} $*"
    log "WARNING: $*"
}

error() {
    echo -e "${RED}❌${NC} $*" >&2
    log "ERROR: $*"
}

step() {
    echo -e "\n${PURPLE}🔧${NC} ${WHITE}$*${NC}"
    log "STEP: $*"
}

substep() {
    echo -e "  ${CYAN}→${NC} $*"
    log "SUBSTEP: $*"
}

prompt_continue() {
    if [[ "$DRY_RUN" == "true" ]]; then
        echo -e "${YELLOW}[DRY RUN]${NC} Would execute: $*"
        return 0
    fi
    
    read -p "Continue with $*? (y/N) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        error "Installation cancelled by user"
        exit 1
    fi
}

check_command() {
    command -v "$1" >/dev/null 2>&1
}

backup_file() {
    local file="$1"
    if [[ -f "$file" || -d "$file" ]]; then
        if [[ "$DRY_RUN" == "false" ]]; then
            mkdir -p "$BACKUP_DIR"
            cp -r "$file" "$BACKUP_DIR/" 2>/dev/null || true
            substep "Backed up $file to $BACKUP_DIR"
        else
            substep "[DRY RUN] Would backup $file"
        fi
    fi
}

# =============================================================================
# Help and Usage
# =============================================================================

show_help() {
    cat << EOF
Shell Environment Setup - "One Stop Shop" Installer

USAGE:
    $0 [OPTIONS]

OPTIONS:
    --dry-run           Show what would be installed without making changes
    --skip-packages     Skip package installation (useful if already installed)
    --config-only       Only install configuration files
    --restore-backup    Restore from previous backup directory
    --verbose           Enable verbose output
    -h, --help          Show this help message

EXAMPLES:
    $0                  # Full installation
    $0 --dry-run        # Preview what would be installed
    $0 --config-only    # Only setup configurations
    $0 --restore-backup ~/.dotfiles-backup-20240101-120000

WHAT GETS INSTALLED:
    • Fish shell with Fisher plugin manager
    • Tmux with TPM and plugins
    • Neovim with LazyVim configuration
    • Starship prompt with Catppuccin theme
    • Essential tools: eza, fzf, bun, nvm

SUPPORTED SYSTEMS:
    • macOS (with Homebrew)
    • Ubuntu/Debian (with apt)
    • RHEL/CentOS/Fedora (with yum/dnf)
    • Arch Linux (with pacman)

EOF
}

# =============================================================================
# Command Line Parsing
# =============================================================================

parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            --dry-run)
                DRY_RUN=true
                shift
                ;;
            --skip-packages)
                SKIP_PACKAGES=true
                shift
                ;;
            --config-only)
                CONFIG_ONLY=true
                SKIP_PACKAGES=true
                shift
                ;;
            --restore-backup)
                RESTORE_BACKUP="$2"
                shift 2
                ;;
            --verbose)
                VERBOSE=true
                shift
                ;;
            -h|--help)
                show_help
                exit 0
                ;;
            *)
                error "Unknown option: $1"
                show_help
                exit 1
                ;;
        esac
    done
}

# =============================================================================
# System Requirements Check
# =============================================================================

check_requirements() {
    step "Checking system requirements"
    
    # Check if running as root
    if [[ $EUID -eq 0 ]]; then
        error "This script should not be run as root"
        exit 1
    fi
    
    # Check for required commands
    local required_commands=("curl" "git")
    for cmd in "${required_commands[@]}"; do
        if ! check_command "$cmd"; then
            error "Required command not found: $cmd"
            exit 1
        fi
    done
    
    # Check internet connectivity
    if ! curl -s --head "https://github.com" > /dev/null; then
        error "Internet connectivity required for installation"
        exit 1
    fi
    
    success "System requirements check passed"
}

# =============================================================================
# Main Installation Flow
# =============================================================================

main() {
    echo -e "${PURPLE}╔══════════════════════════════════════════════════════╗${NC}"
    echo -e "${PURPLE}║${NC} ${WHITE}Shell Environment Setup - One Stop Shop Installer${NC} ${PURPLE}║${NC}"
    echo -e "${PURPLE}╚══════════════════════════════════════════════════════╝${NC}"
    echo
    
    parse_args "$@"
    
    # Initialize log file
    echo "Installation started at $(date)" > "$LOG_FILE"
    
    if [[ "$DRY_RUN" == "true" ]]; then
        warning "DRY RUN MODE - No changes will be made"
    fi
    
    # Handle restore backup option
    if [[ -n "$RESTORE_BACKUP" ]]; then
        step "Restoring from backup: $RESTORE_BACKUP"
        source "$SCRIPT_DIR/scripts/restore-backup.sh"
        restore_from_backup "$RESTORE_BACKUP"
        exit 0
    fi
    
    check_requirements
    
    # Source helper scripts
    source "$SCRIPT_DIR/scripts/detect-os.sh"
    
    # Detect operating system
    step "Detecting operating system"
    detect_os
    success "Detected OS: $OS_NAME $OS_VERSION"
    
    # Install packages if not skipped
    if [[ "$SKIP_PACKAGES" == "false" ]]; then
        step "Installing system packages"
        source "$SCRIPT_DIR/scripts/install-packages.sh"
        install_packages
    else
        info "Skipping package installation"
    fi
    
    # Setup Fish shell
    step "Setting up Fish shell"
    source "$SCRIPT_DIR/scripts/setup-fish.sh"
    setup_fish
    
    # Setup Tmux
    step "Setting up Tmux"
    source "$SCRIPT_DIR/scripts/setup-tmux.sh"
    setup_tmux
    
    # Setup Neovim
    step "Setting up Neovim"
    source "$SCRIPT_DIR/scripts/setup-neovim.sh"
    setup_neovim
    
    # Setup Starship
    step "Setting up Starship prompt"
    source "$SCRIPT_DIR/scripts/setup-starship.sh"
    setup_starship
    
    # Final steps
    step "Finalizing installation"
    
    # Set Fish as default shell
    if [[ "$DRY_RUN" == "false" ]]; then
        if check_command fish && [[ "$SHELL" != "$(which fish)" ]]; then
            substep "Setting Fish as default shell"
            if ! grep -q "$(which fish)" /etc/shells; then
                echo "$(which fish)" | sudo tee -a /etc/shells
            fi
            chsh -s "$(which fish)"
        fi
    else
        substep "[DRY RUN] Would set Fish as default shell"
    fi
    
    echo
    echo -e "${GREEN}╔══════════════════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║${NC} ${WHITE}🎉 Installation completed successfully! 🎉${NC}        ${GREEN}║${NC}"
    echo -e "${GREEN}╚══════════════════════════════════════════════════════╝${NC}"
    echo
    
    info "Your complete shell environment is now set up!"
    info "Configurations backed up to: $BACKUP_DIR"
    info "Installation log: $LOG_FILE"
    echo
    info "To start using your new environment:"
    echo -e "  ${CYAN}exec fish${NC}  # Start Fish shell"
    echo -e "  ${CYAN}tmux${NC}       # Start Tmux"
    echo -e "  ${CYAN}nvim${NC}       # Start Neovim"
    echo
    info "All tools and configurations are ready to use!"
    
    if [[ "$DRY_RUN" == "true" ]]; then
        echo
        warning "This was a dry run - no actual changes were made"
        info "Run without --dry-run to perform the actual installation"
    fi
}

# Error handling
trap 'error "Installation failed! Check $LOG_FILE for details"; exit 1' ERR

# Run main function with all arguments
main "$@"