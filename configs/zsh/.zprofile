# PATH setup — runs AFTER /etc/zprofile so macOS path_helper doesn't
# clobber homebrew by shoving /usr/bin ahead of /opt/homebrew/bin.

# Homebrew (cross-platform detection with cached shellenv)
_brew_bin=""
if [[ -f "/opt/homebrew/bin/brew" ]]; then
    _brew_bin="/opt/homebrew/bin/brew"
elif [[ -f "/usr/local/bin/brew" ]]; then
    _brew_bin="/usr/local/bin/brew"
elif [[ -f "/home/linuxbrew/.linuxbrew/bin/brew" ]]; then
    _brew_bin="/home/linuxbrew/.linuxbrew/bin/brew"
fi
if [[ -n "$_brew_bin" ]]; then
    _brew_cache="${XDG_CACHE_HOME:-$HOME/.cache}/zsh/brew_shellenv.zsh"
    [[ -d "${_brew_cache:h}" ]] || mkdir -p "${_brew_cache:h}"
    if [[ ! -f "$_brew_cache" ]] || [[ "$_brew_bin" -nt "$_brew_cache" ]]; then
        "$_brew_bin" shellenv > "$_brew_cache"
    fi
    source "$_brew_cache"
fi
unset _brew_bin _brew_cache

# User-local bin dirs (cargo/bun/go/local) are prepended in .zshenv
# so non-login shells see them too. Keep `typeset -U path` here so
# DOTNET_ROOT and the GUI-app prepends below dedupe cleanly.
typeset -U path

# .NET SDK (installed via dotnet-install.sh to ~/.dotnet)
if [[ -d "${HOME}/.dotnet" ]]; then
    export DOTNET_ROOT="${HOME}/.dotnet"
    export PATH="${DOTNET_ROOT}:${PATH}"
fi

# JetBrains Toolbox
if [[ -d "${HOME}/Library/Application Support/JetBrains/Toolbox/scripts" ]]; then
    path+=("${HOME}/Library/Application Support/JetBrains/Toolbox/scripts")
fi

# Obsidian CLI
if [[ -d "/Applications/Obsidian.app/Contents/MacOS" ]]; then
    path+=("/Applications/Obsidian.app/Contents/MacOS")
fi
