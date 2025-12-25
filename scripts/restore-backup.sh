#!/usr/bin/env bash

# =============================================================================
# Backup Restore Script
# =============================================================================
# Restores dotfiles configuration from a previous backup
# =============================================================================

set -euo pipefail

restore_from_backup() {
    local backup_dir="$1"
    
    if [[ ! -d "$backup_dir" ]]; then
        error "Backup directory not found: $backup_dir"
        exit 1
    fi
    
    substep "Restoring from backup: $backup_dir"
    
    # List of files/directories that this installer manages
    local managed_paths=(
        "$HOME/.config/zsh"
        "$HOME/.config/nvim"
        "$HOME/.config/starship.toml"
        "$HOME/.config/atuin"
        "$HOME/.config/ghostty"
        "$HOME/.zshenv"
        "$HOME/.tmux.conf"
        "$HOME/.tmux"
        "$HOME/.local/share/nvim"
        "$HOME/.local/state/nvim"
        "$HOME/.cache/nvim"
    )
    
    # Restore each managed path
    for path in "${managed_paths[@]}"; do
        local backup_path="$backup_dir/$(basename "$path")"
        
        if [[ -e "$backup_path" ]]; then
            substep "Restoring $(basename "$path")..."
            
            if [[ "$DRY_RUN" == "false" ]]; then
                # Remove current version if it exists
                if [[ -e "$path" ]]; then
                    rm -rf "$path"
                fi
                
                # Restore from backup
                if [[ -d "$backup_path" ]]; then
                    cp -r "$backup_path" "$path"
                else
                    cp "$backup_path" "$path"
                fi
                
                substep "Restored $path"
            else
                substep "[DRY RUN] Would restore $path from $backup_path"
            fi
        else
            substep "No backup found for $(basename "$path"), skipping"
        fi
    done
    
    success "Backup restoration completed"
    
    # Provide next steps
    echo
    info "Backup restored successfully!"
    info "You may need to restart your shell or reload configurations"
    echo
    echo -e "  ${CYAN}exec zsh${NC}   # Restart shell"
    echo -e "  ${CYAN}tmux source-file ~/.tmux.conf${NC}  # Reload Tmux config"
    echo
}

# Function to list available backups
list_backups() {
    echo
    info "Available backups:"
    
    local backup_pattern="$HOME/.dotfiles-backup-*"
    local found_backups=false
    
    for backup_dir in $backup_pattern; do
        if [[ -d "$backup_dir" ]]; then
            local backup_date=$(basename "$backup_dir" | sed 's/\.dotfiles-backup-//')
            local formatted_date=$(echo "$backup_date" | sed 's/\([0-9]\{4\}\)\([0-9]\{2\}\)\([0-9]\{2\}\)-\([0-9]\{2\}\)\([0-9]\{2\}\)\([0-9]\{2\}\)/\1-\2-\3 \4:\5:\6/')
            echo -e "  ${CYAN}$backup_dir${NC} (created: $formatted_date)"
            found_backups=true
        fi
    done
    
    if [[ "$found_backups" == "false" ]]; then
        warning "No backup directories found in $HOME"
        echo -e "  Backup directories should match pattern: ${CYAN}~/.dotfiles-backup-*${NC}"
    fi
    
    echo
}

# Handle case where restore is called without proper backup directory
if [[ "${1:-}" == "--list" ]]; then
    list_backups
    exit 0
fi

if [[ $# -eq 0 ]]; then
    error "Usage: $0 <backup-directory>"
    echo "       $0 --list  # List available backups"
    list_backups
    exit 1
fi