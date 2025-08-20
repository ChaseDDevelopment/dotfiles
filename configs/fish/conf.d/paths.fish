# =============================================================================
# Fish Shell Path Configuration
# =============================================================================
# Manages PATH and environment variables for various tools and languages
# =============================================================================

# Only run in interactive mode
if status is-interactive
    
    # =============================================================================
    # Priority Path Overrides (must come first)
    # =============================================================================
    
    # Force Homebrew paths to front of PATH for modern tool versions
    if test (uname) = "Darwin"
        if test -d "/opt/homebrew/bin"
            # Completely rebuild PATH with Homebrew first
            set -l new_path "/opt/homebrew/bin" "/opt/homebrew/sbin"
            # Add remaining paths, avoiding duplicates
            for path_entry in $PATH
                if not contains "$path_entry" $new_path
                    set -a new_path "$path_entry"
                end
            end
            set -gx PATH $new_path
        else if test -d "/usr/local/bin"
            # Intel Mac equivalent
            set -l new_path "/usr/local/bin" "/usr/local/sbin"
            # Add remaining paths, avoiding duplicates
            for path_entry in $PATH
                if not contains "$path_entry" $new_path
                    set -a new_path "$path_entry"
                end
            end
            set -gx PATH $new_path
        end
    end
    
    # =============================================================================
    # Core Path Management
    # =============================================================================
    
    # User local bin (highest priority)
    if test -d "$HOME/.local/bin"
        fish_add_path --prepend "$HOME/.local/bin"
    end
    
    # User bin directory
    if test -d "$HOME/bin"
        fish_add_path --prepend "$HOME/bin"
    end
    
    # =============================================================================
    # Development Tools
    # =============================================================================
    
    # Bun Package Manager
    if test -d "$HOME/.bun"
        set -gx BUN_INSTALL "$HOME/.bun"
        fish_add_path "$BUN_INSTALL/bin"
    end
    
    # Rust/Cargo
    if test -d "$HOME/.cargo"
        set -gx CARGO_HOME "$HOME/.cargo"
        fish_add_path "$HOME/.cargo/bin"
    end
    
    # Go Programming Language
    if command -q go
        # Set GOPATH if not already set
        if not set -q GOPATH
            set -gx GOPATH "$HOME/go"
        end
        
        # Add Go bin directories to PATH
        fish_add_path "$GOPATH/bin"
        
        # Add system Go bin if it exists
        if test -d "/usr/local/go/bin"
            fish_add_path "/usr/local/go/bin"
        end
    end
    
    # Python/pip user packages
    if command -q python3
        # Python user base
        set -l python_user_base (python3 -m site --user-base 2>/dev/null)
        if test -n "$python_user_base" -a -d "$python_user_base/bin"
            fish_add_path "$python_user_base/bin"
        end
        
        # Common Python user bin locations
        if test -d "$HOME/.local/bin"
            fish_add_path "$HOME/.local/bin"
        end
    end
    
    # Ruby Gems (if Ruby is installed)
    if command -q ruby
        set -l gem_bin (ruby -e 'puts Gem.user_dir' 2>/dev/null)
        if test -n "$gem_bin" -a -d "$gem_bin/bin"
            fish_add_path "$gem_bin/bin"
        end
    end
    
    # =============================================================================
    # Node.js and Package Managers
    # =============================================================================
    
    # NPM Global packages
    if command -q npm
        # Set npm global prefix to avoid permission issues
        if not set -q NPM_CONFIG_PREFIX
            set -gx NPM_CONFIG_PREFIX "$HOME/.npm-global"
        end
        
        # Create directory if it doesn't exist
        if not test -d "$NPM_CONFIG_PREFIX"
            mkdir -p "$NPM_CONFIG_PREFIX"
        end
        
        # Add to PATH
        fish_add_path "$NPM_CONFIG_PREFIX/bin"
    end
    
    # Yarn global packages
    if command -q yarn
        set -l yarn_global_bin (yarn global bin 2>/dev/null)
        if test -n "$yarn_global_bin" -a -d "$yarn_global_bin"
            fish_add_path "$yarn_global_bin"
        end
    end
    
    # pnpm global packages
    if command -q pnpm
        set -gx PNPM_HOME "$HOME/.local/share/pnpm"
        fish_add_path "$PNPM_HOME"
    end
    
    # =============================================================================
    # Platform-Specific Paths
    # =============================================================================
    
    # macOS specific paths
    if test (uname) = "Darwin"
        # Homebrew paths - prioritize over system paths for modern tools
        if test -d "/opt/homebrew/bin"
            # Apple Silicon Macs - use --prepend to ensure Homebrew takes precedence
            fish_add_path --prepend "/opt/homebrew/bin"
            fish_add_path --prepend "/opt/homebrew/sbin"
        else if test -d "/usr/local/bin"
            # Intel Macs - use --prepend to ensure Homebrew takes precedence
            fish_add_path --prepend "/usr/local/bin"
            fish_add_path --prepend "/usr/local/sbin"
        end
        
        # macOS system paths
        fish_add_path "/usr/bin"
        fish_add_path "/bin"
        fish_add_path "/usr/sbin"
        fish_add_path "/sbin"
    end
    
    # Linux specific paths
    if test (uname) = "Linux"
        # Snap packages
        if test -d "/snap/bin"
            fish_add_path "/snap/bin"
        end
        
        # Flatpak
        if test -d "/var/lib/flatpak/exports/bin"
            fish_add_path "/var/lib/flatpak/exports/bin"
        end
        
        # AppImage directory
        if test -d "$HOME/Applications"
            fish_add_path "$HOME/Applications"
        end
    end
    
    # =============================================================================
    # Editor and Tool Paths
    # =============================================================================
    
    # Neovim Mason binaries
    if test -d "$HOME/.local/share/nvim/mason/bin"
        fish_add_path "$HOME/.local/share/nvim/mason/bin"
    end
    
    # VS Code extensions (if VS Code is installed)
    if command -q code
        if test -d "$HOME/.vscode/extensions"
            for ext_dir in "$HOME/.vscode/extensions"/*/bin
                if test -d "$ext_dir"
                    fish_add_path "$ext_dir"
                end
            end
        end
    end
    
    # =============================================================================
    # Container and Virtualization Tools
    # =============================================================================
    
    # Docker Desktop (macOS)
    if test -d "/Applications/Docker.app/Contents/Resources/bin"
        fish_add_path "/Applications/Docker.app/Contents/Resources/bin"
    end
    
    # Podman
    if test -d "$HOME/.local/podman/bin"
        fish_add_path "$HOME/.local/podman/bin"
    end
    
    # =============================================================================
    # Cloud and DevOps Tools
    # =============================================================================
    
    # kubectl krew plugins
    if test -d "$HOME/.krew/bin"
        fish_add_path "$HOME/.krew/bin"
    end
    
    # Terraform
    if test -d "$HOME/.terraform.d/bin"
        fish_add_path "$HOME/.terraform.d/bin"
    end
    
    # AWS CLI
    if test -d "$HOME/.aws/bin"
        fish_add_path "$HOME/.aws/bin"
    end
    
    # =============================================================================
    # Path Cleanup and Validation
    # =============================================================================
    
    # Remove duplicate entries from PATH
    set -gx PATH (printf "%s\n" $PATH | awk '!seen[$0]++')
    
    # Remove non-existent directories from PATH
    set -l clean_path
    for path_entry in $PATH
        if test -d "$path_entry"
            set -a clean_path "$path_entry"
        end
    end
    set -gx PATH $clean_path
    
end