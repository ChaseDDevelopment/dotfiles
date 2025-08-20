# =============================================================================
# Fish Shell Abbreviations
# =============================================================================
# Custom abbreviations for commonly used commands
# Abbreviations expand when you press space or enter, unlike aliases
# =============================================================================

# Only set up abbreviations in interactive mode
if status is-interactive
    
    # =============================================================================
    # Safety Abbreviations (Interactive Mode)
    # =============================================================================
    
    # NOTE: rm, cp, mv, mkdir, which, ls commands are now handled by functions.fish
    # This provides better safety and consistent behavior
    
    # =============================================================================
    # Quick Shortcuts
    # =============================================================================
    
    # Better history command
    abbr h "history"
    
    # =============================================================================
    # Git Abbreviations
    # =============================================================================
    
    if command -q git
        abbr g "git"
        abbr gs "git status"
        abbr ga "git add"
        abbr gaa "git add --all"
        abbr gc "git commit"
        abbr gcm "git commit -m"
        abbr gca "git commit --amend"
        abbr gp "git push"
        abbr gpl "git pull"
        abbr gf "git fetch"
        abbr gd "git diff"
        abbr gds "git diff --staged"
        abbr gl "git log --oneline"
        abbr gll "git log --graph --pretty=format:'%Cred%h%Creset -%C(yellow)%d%Creset %s %Cgreen(%cr) %C(bold blue)<%an>%Creset' --abbrev-commit"
        abbr gb "git branch"
        abbr gba "git branch -a"
        abbr gco "git checkout"
        abbr gcb "git checkout -b"
        abbr gm "git merge"
        abbr gr "git rebase"
        abbr gst "git stash"
        abbr gstp "git stash pop"
        abbr gstl "git stash list"
        abbr grs "git reset"
        abbr grsh "git reset --hard"
        abbr gclone "git clone"
    end
    
    # =============================================================================
    # Development Abbreviations
    # =============================================================================
    
    # Node.js/npm
    if command -q npm
        abbr ni "npm install"
        abbr nid "npm install --save-dev"
        abbr nig "npm install -g"
        abbr nr "npm run"
        abbr ns "npm start"
        abbr nt "npm test"
        abbr nb "npm run build"
        abbr nrd "npm run dev"
        abbr nu "npm update"
        abbr nci "npm ci"
    end
    
    # Yarn (if available)
    if command -q yarn
        abbr y "yarn"
        abbr ya "yarn add"
        abbr yad "yarn add --dev"
        abbr yr "yarn run"
        abbr ys "yarn start"
        abbr yt "yarn test"
        abbr yb "yarn build"
        abbr yd "yarn dev"
        abbr yu "yarn upgrade"
    end
    
    # Bun (if available)
    if command -q bun
        abbr bi "bun install"
        abbr ba "bun add"
        abbr bad "bun add --dev"
        abbr br "bun run"
        abbr bs "bun start"
        abbr bt "bun test"
        abbr bb "bun run build"
        abbr bd "bun run dev"
    end
    
    # Python
    if command -q python3
        abbr py "python3"
        abbr pip "pip3"
        abbr venv "python3 -m venv"
        abbr activate "source venv/bin/activate"
    end
    
    # =============================================================================
    # System Abbreviations
    # =============================================================================
    
    # Process management
    abbr ps "ps aux"
    abbr psg "ps aux | grep"
    abbr k "kill"
    abbr k9 "kill -9"
    
    # Disk usage
    abbr df "df -h"
    abbr du "du -h"
    abbr dus "du -sh"
    
    # Network
    abbr ports "netstat -tuln"
    abbr ping "ping -c 5"
    
    # System info
    abbr sys "uname -a"
    abbr mem "free -h"
    
    # =============================================================================
    # Editor Abbreviations
    # =============================================================================
    
    # Neovim shortcuts
    if command -q nvim
        abbr v "nvim"
        abbr vi "nvim"
        abbr vim "nvim"
        abbr nv "nvim"
    end
    
    # =============================================================================
    # Tmux Abbreviations
    # =============================================================================
    
    if command -q tmux
        abbr t "tmux"
        abbr ta "tmux attach"
        abbr tn "tmux new-session"
        abbr tl "tmux list-sessions"
        abbr tk "tmux kill-session"
        abbr td "tmux detach"
    end
    
    # =============================================================================
    # Python/UV Abbreviations
    # =============================================================================
    
    if command -q uv
        abbr uvx "uv tool run"
        abbr uvs "uv sync"
        abbr uva "uv add"
        abbr uvi "uv pip install"
        abbr uvt "uv tool"
        abbr uvl "uv tool list"
    end
    
    if command -q ruff
        abbr ruffmt "ruff format"
        abbr rufflint "ruff check"
        abbr rufffix "ruff check --fix"
        abbr ruffwatch "ruff check --watch"
    end
    
    # =============================================================================
    # Docker Abbreviations (if available)
    # =============================================================================
    
    if command -q docker
        abbr d "docker"
        abbr dc "docker-compose"
        abbr dps "docker ps"
        abbr dpa "docker ps -a"
        abbr di "docker images"
        abbr drm "docker rm"
        abbr drmi "docker rmi"
        abbr dstop "docker stop"
        abbr dstart "docker start"
        abbr dexec "docker exec -it"
        abbr dlogs "docker logs"
        abbr dbuild "docker build"
        abbr dpull "docker pull"
        abbr dpush "docker push"
    end
    
    # =============================================================================
    # Modern CLI Tool Abbreviations
    # =============================================================================
    
    # Note: cat, find, grep are now functions but add some convenience abbreviations
    if command -q bat
        abbr batl "bat --list-themes"
        abbr batp "bat --plain"
    end
    
    if command -q fd
        abbr fdt "fd --type f"
        abbr fdd "fd --type d"
    end
    
    if command -q rg
        abbr rgt "rg --type"
        abbr rgf "rg --files"
        abbr rgi "rg --ignore-case"
    end
    
    # =============================================================================
    # Utility Abbreviations
    # =============================================================================
    
    # Clear screen
    abbr c "clear"
    abbr cls "clear"
    
    # Quick directory navigation
    abbr .. "cd .."
    abbr ... "cd ../.."
    abbr .... "cd ../../.."
    abbr ..... "cd ../../../.."
    
    # Home directory
    abbr ~ "cd ~"
    
    # Reload Fish configuration
    abbr reload "source ~/.config/fish/config.fish"
    
    # Quick edit configurations
    abbr fishconfig "nvim ~/.config/fish/config.fish"
    abbr tmuxconfig "nvim ~/.tmux.conf"
    abbr nvimconfig "nvim ~/.config/nvim"
    
end