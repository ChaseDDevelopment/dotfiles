#!/usr/bin/env bash

# =============================================================================
# Zsh Shell Setup Script
# =============================================================================
# Sets up Zsh shell with Antidote plugin manager and all configurations
# =============================================================================

setup_zsh() {
    substep "Starting Zsh shell setup"

    # Check if Zsh is installed
    if ! check_command zsh; then
        error "Zsh shell is not installed. Run package installation first."
        return 1
    fi

    # Create XDG directories
    create_xdg_directories

    # Backup existing Zsh configuration
    backup_file "$HOME/.zshenv"
    backup_file "$HOME/.zshrc"
    backup_file "$HOME/.config/zsh"

    # Create Zsh config directory structure
    create_zsh_directories

    # Copy Zsh configurations
    copy_zsh_configs

    # Create root .zshenv pointing to ZDOTDIR
    setup_root_zshenv

    # Install Antidote plugin manager
    install_antidote

    # Generate Antidote plugins (compile)
    compile_antidote_plugins

    # Set Zsh as default shell
    set_zsh_default_shell

    success "Zsh shell setup completed"
}

create_xdg_directories() {
    substep "Creating XDG directories..."

    if [[ "$DRY_RUN" == "false" ]]; then
        mkdir -p "$HOME/.config"
        mkdir -p "$HOME/.local/share"
        mkdir -p "$HOME/.local/state"
        mkdir -p "$HOME/.local/state/zsh"  # For HISTFILE
        mkdir -p "$HOME/.local/bin"
        mkdir -p "$HOME/.cache"
        mkdir -p "$HOME/.cache/zsh"
        mkdir -p "$HOME/.cache/ohmyzsh/completions"
        substep "XDG directories created"
    else
        substep "[DRY RUN] Would create XDG directories"
    fi
}

create_zsh_directories() {
    substep "Creating Zsh config directory structure..."

    if [[ "$DRY_RUN" == "false" ]]; then
        mkdir -p "$HOME/.config/zsh/aliases"
        mkdir -p "$HOME/.config/zsh/functions"
        mkdir -p "$HOME/.config/zsh/plugins"
        mkdir -p "$HOME/.config/zsh/tools"
        substep "Zsh config directories created"
    else
        substep "[DRY RUN] Would create Zsh config directories"
    fi
}

copy_zsh_configs() {
    substep "Copying Zsh configurations..."

    # Files that go to ~/.config/zsh
    local zsh_config_files=(
        ".zshenv"
        ".zshrc"
        "aliases/general.zsh"
        "functions/utils.zsh"
        "functions/ssh.zsh"
        "plugins/.zsh_plugins.txt"
        "tools/nvm.zsh"
        "tools/bun.zsh"
    )

    for config_file in "${zsh_config_files[@]}"; do
        local source_file="$SCRIPT_DIR/configs/zsh/$config_file"
        local dest_file="$HOME/.config/zsh/$config_file"

        if [[ -f "$source_file" ]]; then
            if [[ "$DRY_RUN" == "false" ]]; then
                mkdir -p "$(dirname "$dest_file")"
                cp "$source_file" "$dest_file"
                substep "Copied $config_file"
            else
                substep "[DRY RUN] Would copy $config_file"
            fi
        else
            warning "Configuration file not found: $source_file"
        fi
    done
}

setup_root_zshenv() {
    substep "Creating root .zshenv symlink..."

    if [[ "$DRY_RUN" == "false" ]]; then
        # Remove existing .zshenv (file or symlink)
        rm -f "$HOME/.zshenv"

        # Remove stale .zshrc (ZDOTDIR points to ~/.config/zsh for .zshrc)
        rm -f "$HOME/.zshrc"

        # Create symlink to XDG-compliant location
        ln -s "$HOME/.config/zsh/.zshenv" "$HOME/.zshenv"
        substep "Created symlink: ~/.zshenv -> ~/.config/zsh/.zshenv"
    else
        substep "[DRY RUN] Would create symlink: ~/.zshenv -> ~/.config/zsh/.zshenv"
    fi
}

install_antidote() {
    substep "Installing Antidote plugin manager..."

    # Check if already installed via Homebrew
    if [[ -f "/opt/homebrew/opt/antidote/share/antidote/antidote.zsh" ]] || \
       [[ -f "/usr/local/opt/antidote/share/antidote/antidote.zsh" ]] || \
       [[ -f "/home/linuxbrew/.linuxbrew/opt/antidote/share/antidote/antidote.zsh" ]]; then
        substep "Antidote already installed via Homebrew"
        return
    fi

    # Check if already installed via git
    local antidote_dir="$HOME/.config/zsh/.antidote"
    if [[ -d "$antidote_dir" ]]; then
        substep "Antidote already installed via git clone"
        return
    fi

    if [[ "$DRY_RUN" == "false" ]]; then
        # Install via Homebrew if available
        if check_command brew; then
            brew install antidote
            substep "Antidote installed via Homebrew"
        else
            # Fallback: Install via git clone
            git clone --depth=1 https://github.com/mattmc3/antidote.git "$antidote_dir"
            substep "Antidote installed via git clone to $antidote_dir"
        fi
    else
        substep "[DRY RUN] Would install Antidote"
    fi
}

compile_antidote_plugins() {
    substep "Compiling Antidote plugins..."

    if [[ "$DRY_RUN" == "false" ]]; then
        local plugins_txt="$HOME/.config/zsh/plugins/.zsh_plugins.txt"
        local plugins_zsh="$HOME/.config/zsh/plugins/.zsh_plugins.zsh"

        if [[ -f "$plugins_txt" ]]; then
            # Try to compile plugins using zsh
            # This may fail on first run before antidote is fully set up
            zsh -c "
                export ZDOTDIR=\"$HOME/.config/zsh\"
                if [[ -f \"/opt/homebrew/opt/antidote/share/antidote/antidote.zsh\" ]]; then
                    source /opt/homebrew/opt/antidote/share/antidote/antidote.zsh
                elif [[ -f \"/usr/local/opt/antidote/share/antidote/antidote.zsh\" ]]; then
                    source /usr/local/opt/antidote/share/antidote/antidote.zsh
                elif [[ -f \"/home/linuxbrew/.linuxbrew/opt/antidote/share/antidote/antidote.zsh\" ]]; then
                    source /home/linuxbrew/.linuxbrew/opt/antidote/share/antidote/antidote.zsh
                elif [[ -d \"\$ZDOTDIR/.antidote\" ]]; then
                    source \"\$ZDOTDIR/.antidote/antidote.zsh\"
                fi
                antidote bundle < '$plugins_txt' > '$plugins_zsh' 2>/dev/null
            " 2>/dev/null || true
            substep "Antidote plugins compiled (or will compile on first shell start)"
        else
            warning "Plugin manifest not found: $plugins_txt"
        fi
    else
        substep "[DRY RUN] Would compile Antidote plugins"
    fi
}

set_zsh_default_shell() {
    substep "Setting Zsh as default shell..."

    if [[ "$DRY_RUN" == "false" ]]; then
        local zsh_path
        zsh_path=$(command -v zsh)

        if [[ -n "$zsh_path" ]] && [[ "$SHELL" != "$zsh_path" ]]; then
            # Add to /etc/shells if not present
            if ! grep -q "$zsh_path" /etc/shells 2>/dev/null; then
                echo "$zsh_path" | sudo tee -a /etc/shells >/dev/null
            fi
            chsh -s "$zsh_path"
            substep "Zsh set as default shell"
        else
            substep "Zsh is already the default shell"
        fi
    else
        substep "[DRY RUN] Would set Zsh as default shell"
    fi
}

validate_zsh_setup() {
    substep "Validating Zsh setup..."

    local validation_passed=true

    # Check if Zsh is available
    if ! check_command zsh; then
        error "Zsh shell is not available"
        validation_passed=false
    fi

    # Check if config files exist
    local required_files=(
        "$HOME/.zshenv"
        "$HOME/.config/zsh/.zshrc"
        "$HOME/.config/zsh/plugins/.zsh_plugins.txt"
    )

    for file in "${required_files[@]}"; do
        if [[ ! -f "$file" ]]; then
            error "Required file missing: $file"
            validation_passed=false
        fi
    done

    if [[ "$validation_passed" == "true" ]]; then
        success "Zsh setup validation passed"
    else
        error "Zsh setup validation failed"
        return 1
    fi
}
