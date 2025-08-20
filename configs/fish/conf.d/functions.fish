# =============================================================================
# Fish Shell Functions
# =============================================================================
# Functions that replace or enhance standard commands
# Fish functions are preferred over aliases for command replacements
# =============================================================================

# Only set up functions in interactive mode
if status is-interactive
    
    # =============================================================================
    # Safety Functions - Always use interactive mode
    # =============================================================================
    
    function rm --description 'Remove with confirmation'
        command rm -i $argv
    end
    
    function cp --description 'Copy with confirmation'
        command cp -i $argv
    end
    
    function mv --description 'Move with confirmation'
        command mv -i $argv
    end
    
    # =============================================================================
    # Enhanced Commands
    # =============================================================================
    
    function mkdir --description 'Create directories with parents'
        command mkdir -p $argv
    end
    
    function which --description 'Show all command locations'
        type -a $argv
    end
    
    # =============================================================================
    # Modern Tool Replacements
    # =============================================================================
    
    # EZA - Modern ls replacement
    if command -q eza
        function ls --description 'List with eza'
            eza -lag --header --icons=always $argv
        end
        
        function ll --description 'Detailed list with eza'
            eza -la --header --icons=always --group $argv
        end
        
        function la --description 'List all with eza'
            eza -la --header --icons=always $argv
        end
        
        function lt --description 'Tree view with eza'
            eza --tree $argv
        end
        
        function l --description 'Simple list with eza'
            eza -1 $argv
        end
    else
        # Fallback to enhanced ls if eza is not available
        function ls --description 'List with colors'
            command ls --color=auto -la $argv
        end
        
        function ll --description 'Detailed list'
            command ls --color=auto -la $argv
        end
        
        function la --description 'List all'
            command ls --color=auto -la $argv
        end
    end
    
    # BAT - Better cat with syntax highlighting
    if command -q bat
        function cat --description 'View files with bat'
            bat $argv
        end
    end
    
    # FD - Fast and user-friendly find replacement
    if command -q fd
        function find --description 'Find with fd'
            fd $argv
        end
    end
    
    # RIPGREP - Fast grep replacement
    if command -q rg
        function grep --description 'Search with ripgrep'
            rg $argv
        end
    end
    
    # =============================================================================
    # Package Manager Functions (Bun as default)
    # =============================================================================
    
    function npm --description 'Use bun instead of npm'
        bun $argv
    end
    
    function npx --description 'Use bunx instead of npx'
        bunx $argv
    end
    
    # =============================================================================
    # Git Shortcuts (Functions for Complex Commands)
    # =============================================================================
    
    function glog --description 'Beautiful git log'
        git log --graph --pretty=format:'%Cred%h%Creset -%C(yellow)%d%Creset %s %Cgreen(%cr) %C(bold blue)<%an>%Creset' --abbrev-commit $argv
    end
    
    function gstatus --description 'Detailed git status'
        git status --short --branch $argv
    end
    
    # =============================================================================
    # Development Shortcuts
    # =============================================================================
    
    function serve --description 'Quick HTTP server'
        if command -q python3
            python3 -m http.server $argv
        else if command -q python
            python -m SimpleHTTPServer $argv
        else
            echo "Python not found for serving files"
            return 1
        end
    end
    
    function mkcd --description 'Create directory and cd into it'
        mkdir -p $argv[1]; and cd $argv[1]
    end
    
    # =============================================================================
    # System Information Functions
    # =============================================================================
    
    function sysinfo --description 'Show system information'
        echo "=== System Information ==="
        echo "OS: "(uname -s)" "(uname -r)
        echo "Hostname: "(hostname)
        echo "Uptime: "(uptime | awk '{print $3,$4}' | sed 's/,//')
        
        if command -q free
            echo "Memory: "(free -h | awk 'NR==2{printf "%.1f/%.1f GB (%.2f%%)", $3/1024/1024/1024, $2/1024/1024/1024, $3*100/$2}')
        end
        
        if command -q df
            echo "Disk: "(df -h / | awk 'NR==2{printf "%s/%s (%s)", $3, $2, $5}')
        end
        
        echo "Shell: "$SHELL
        echo "Fish version: "(fish --version)
    end
    
    # =============================================================================
    # Utility Functions
    # =============================================================================
    
    function extract --description 'Extract various archive formats'
        if test (count $argv) -ne 1
            echo "Usage: extract <archive>"
            return 1
        end
        
        set file $argv[1]
        
        if not test -f "$file"
            echo "Error: '$file' is not a valid file"
            return 1
        end
        
        switch $file
            case '*.tar.bz2'
                tar xjf "$file"
            case '*.tar.gz'
                tar xzf "$file"
            case '*.bz2'
                bunzip2 "$file"
            case '*.rar'
                command -q unrar; and unrar x "$file"; or echo "unrar not installed"
            case '*.gz'
                gunzip "$file"
            case '*.tar'
                tar xf "$file"
            case '*.tbz2'
                tar xjf "$file"
            case '*.tgz'
                tar xzf "$file"
            case '*.zip'
                unzip "$file"
            case '*.Z'
                uncompress "$file"
            case '*.7z'
                command -q 7z; and 7z x "$file"; or echo "7z not installed"
            case '*'
                echo "Error: '$file' cannot be extracted via extract()"
                return 1
        end
    end
    
    function backup --description 'Create a backup of a file or directory'
        if test (count $argv) -ne 1
            echo "Usage: backup <file_or_directory>"
            return 1
        end
        
        set target $argv[1]
        set backup_name "$target.backup."(date +%Y%m%d_%H%M%S)
        
        if test -f "$target"
            cp "$target" "$backup_name"
            echo "File backed up as: $backup_name"
        else if test -d "$target"
            cp -r "$target" "$backup_name"
            echo "Directory backed up as: $backup_name"
        else
            echo "Error: '$target' does not exist"
            return 1
        end
    end
    
    # =============================================================================
    # Network Functions
    # =============================================================================
    
    function myip --description 'Show external IP address'
        if command -q curl
            curl -s https://ipinfo.io/ip
        else if command -q wget
            wget -qO- https://ipinfo.io/ip
        else
            echo "curl or wget required"
            return 1
        end
    end
    
    function ports --description 'Show listening ports'
        if command -q ss
            ss -tuln
        else if command -q netstat
            netstat -tuln
        else
            echo "ss or netstat required"
            return 1
        end
    end
    
end