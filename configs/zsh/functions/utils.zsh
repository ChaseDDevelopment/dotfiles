# Utility Functions
# =============================================================================

# Create directory and cd into it
mkcd() { mkdir -p "$1" && cd "$1" }

# cd to git repository root
cg() {
    local root
    root=$(git rev-parse --show-toplevel 2>/dev/null)
    [[ -n "$root" ]] && cd "$root" || echo "Not in a git repo"
}

# cd to new temp directory
cdtmp() { cd "$(mktemp -d)" && pwd }

# Clone repo and cd into it
take() {
    git clone "$1" && cd "$(basename "${1%.git}")"
}

# Universal archive extraction
extract() {
    [[ -f "$1" ]] || { echo "File not found: $1"; return 1 }
    case "$1" in
        *.tar.gz|*.tgz)     tar xzf "$1" ;;
        *.tar.bz2|*.tbz2)   tar xjf "$1" ;;
        *.tar.xz|*.txz)     tar xJf "$1" ;;
        *.tar)              tar xf "$1" ;;
        *.zip)              unzip "$1" ;;
        *.gz)               gunzip "$1" ;;
        *.bz2)              bunzip2 "$1" ;;
        *.xz)               unxz "$1" ;;
        *.7z)               7z x "$1" ;;
        *.rar)              unrar x "$1" ;;
        *)                  echo "Unknown format: $1" ;;
    esac
}
