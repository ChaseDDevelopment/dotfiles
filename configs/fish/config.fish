# =============================================================================
# Fish Shell Configuration
# =============================================================================
# Main configuration file for Fish shell
# This file sources modular configurations and sets up the environment
# =============================================================================

# Only run in interactive mode
if status is-interactive
    # Source all configuration files from conf.d/
    # Fish automatically sources files in conf.d/, but we're being explicit here
    # for documentation purposes
    
    # Initialize Starship prompt
    if command -q starship
        starship init fish | source
    end
    
    # Load custom functions and configurations
    # (These are automatically loaded from conf.d/ directory)
    
end

# Export environment variables that other programs might need
# Note: Use 'set -gx' instead of 'export' in Fish

# Bun installation
if test -d "$HOME/.bun"
    set -gx BUN_INSTALL "$HOME/.bun"
    fish_add_path "$BUN_INSTALL/bin"
end

# Cargo/Rust
if test -d "$HOME/.cargo"
    fish_add_path "$HOME/.cargo/bin"
end

# Local bin
if test -d "$HOME/.local/bin"
    fish_add_path "$HOME/.local/bin"
end

# =============================================================================
# Editor and Tool Configuration
# =============================================================================

# Set default editor (prioritize nvim, fallback to vim, then nano)
if command -q nvim
    set -gx EDITOR nvim
    set -gx VISUAL nvim
else if command -q vim
    set -gx EDITOR vim
    set -gx VISUAL vim
else
    set -gx EDITOR nano
    set -gx VISUAL nano
end

# FZF configuration
if command -q fzf
    set -gx FZF_DEFAULT_OPTS "--height 40% --layout=reverse --border"
    
    # Use fd if available for better performance
    if command -q fd
        set -gx FZF_DEFAULT_COMMAND "fd --type f --hidden --follow --exclude .git"
        set -gx FZF_CTRL_T_COMMAND "$FZF_DEFAULT_COMMAND"
    end
end

# =============================================================================
# Development Environment Configuration
# =============================================================================

# Node.js/NPM configuration
if command -q npm
    # Set npm global prefix to avoid permission issues
    set -gx NPM_CONFIG_PREFIX "$HOME/.npm-global"
    fish_add_path "$NPM_CONFIG_PREFIX/bin"
end

# Python configuration
if command -q python3
    # Add user Python bin to PATH
    if test -d "$HOME/.local/bin"
        fish_add_path "$HOME/.local/bin"
    end
end

# Go configuration
if command -q go
    set -gx GOPATH "$HOME/go"
    fish_add_path "$GOPATH/bin"
end

# =============================================================================
# Terminal and Display Configuration
# =============================================================================

# Terminal capabilities
set -gx TERM xterm-256color

# Less configuration for better paging
set -gx LESS "-R -F -X"
set -gx LESSOPEN "|pygmentize -g %s 2>/dev/null || cat %s"

# Bat configuration (if available)
if command -q bat
    set -gx BAT_THEME "TwoDark"
    set -gx BAT_STYLE "numbers,changes,header,grid"
    set -gx BAT_PAGER "less -RF"
end

# =============================================================================
# Additional Configuration
# =============================================================================

# Note: Functions and aliases are now handled by functions.fish and abbr.fish
# This provides better organization and Fish-native approaches

# Additional environment variables for modern tools
if command -q rg
    set -gx RIPGREP_CONFIG_PATH "$HOME/.ripgreprc"
end

if command -q fd
    set -gx FD_OPTIONS "--hidden --follow --exclude .git"
end

# =============================================================================
# Cleanup and Finalization
# =============================================================================

# Remove duplicate PATH entries (Fish handles this better natively)
# Note: fish_add_path automatically handles duplicates, so this is mostly for cleanup
if set -q PATH[1]
    # Use Fish's built-in path deduplication which is more reliable
    set -l cleaned_path
    for path_entry in $PATH
        if not contains "$path_entry" $cleaned_path
            set -a cleaned_path "$path_entry"
        end
    end
    set -gx PATH $cleaned_path
end

# Set umask for secure file creation
umask 022