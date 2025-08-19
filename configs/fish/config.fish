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
    
    # Welcome message (optional)
    if test "$TERM_PROGRAM" != "WarpTerminal"
        # Only show welcome in non-Warp terminals
        if not set -q FISH_WELCOME_SHOWN
            echo -e "\033[0;34m🐟 Fish shell loaded with custom configuration\033[0m"
            set -g FISH_WELCOME_SHOWN 1
        end
    end
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
    set -gx BAT_THEME "Catppuccin-mocha"
    set -gx BAT_STYLE "numbers,changes,header"
end

# =============================================================================
# Aliases and Functions
# =============================================================================

# Modern replacements (only if commands are available)
if command -q eza
    alias ls "eza -lag --header --icons=always"
    alias ll "eza -la --header --icons=always"
    alias tree "eza --tree"
end

if command -q bat
    alias cat "bat"
end

if command -q fd
    alias find "fd"
end

if command -q rg
    alias grep "rg"
end

# Git shortcuts
if command -q git
    alias g "git"
    alias gs "git status"
    alias ga "git add"
    alias gc "git commit"
    alias gp "git push"
    alias gl "git pull"
    alias gd "git diff"
    alias gb "git branch"
    alias gco "git checkout"
end

# =============================================================================
# Cleanup and Finalization
# =============================================================================

# Remove duplicate PATH entries
set -gx PATH (printf "%s\n" $PATH | awk '!seen[$0]++')

# Set umask for secure file creation
umask 022