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
        ui_error "Backup directory not found: $backup_dir"
        exit 1
    fi
    
    ui_info "Restoring from backup: $backup_dir"
    
    # List of files/directories that this installer manages
    local managed_paths=(
        "$HOME/.config/zsh"
        "$HOME/.config/nvim"
        "$HOME/.config/starship.toml"
        "$HOME/.config/atuin"
        "$HOME/.config/ghostty"
        "$HOME/.zshenv"
        "$HOME/.config/tmux"
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
            ui_info "Restoring $(basename "$path")..."
            
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
                
                ui_info "Restored $path"
            else
                ui_info "[DRY RUN] Would restore $path from $backup_path"
            fi
        else
            ui_info "No backup found for $(basename "$path"), skipping"
        fi
    done
    
    ui_success "Backup restoration completed"

    # Provide next steps
    echo
    ui_info "Backup restored successfully!"
    ui_info "You may need to restart your shell or reload configurations"
    echo
    ui_info "  exec zsh   # Restart shell"
    ui_info "  tmux source-file ~/.config/tmux/tmux.conf  # Reload Tmux config"
    echo
}

# Function to list available backups
list_backups() {
    echo
    ui_info "Available backups:"
    
    local backup_pattern="$HOME/.dotfiles-backup-*"
    local found_backups=false
    
    for backup_dir in $backup_pattern; do
        if [[ -d "$backup_dir" ]]; then
            local backup_date=$(basename "$backup_dir" | sed 's/\.dotfiles-backup-//')
            local formatted_date=$(echo "$backup_date" | sed 's/\([0-9]\{4\}\)\([0-9]\{2\}\)\([0-9]\{2\}\)-\([0-9]\{2\}\)\([0-9]\{2\}\)\([0-9]\{2\}\)/\1-\2-\3 \4:\5:\6/')
            ui_info "  $backup_dir (created: $formatted_date)"
            found_backups=true
        fi
    done
    
    if [[ "$found_backups" == "false" ]]; then
        ui_warn "No backup directories found in $HOME"
        ui_info "  Backup directories should match pattern: ~/.dotfiles-backup-*"
    fi
    
    echo
}

# Handle case where restore is called without proper backup directory
if [[ "${1:-}" == "--list" ]]; then
    list_backups
    exit 0
fi

if [[ $# -eq 0 ]]; then
    ui_error "Usage: $0 <backup-directory>"
    echo "       $0 --list  # List available backups"
    list_backups
    exit 1
fi