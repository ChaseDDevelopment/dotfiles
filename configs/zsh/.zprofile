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

# .NET SDK — point DOTNET_ROOT at whichever install layout is present.
# Brew installs the formula at $HOMEBREW_PREFIX/opt/dotnet/libexec (caveat
# output explicitly tells callers to set DOTNET_ROOT there). The script
# installer (dotnet-install.sh) lands under ~/.dotnet. Prefer brew when both
# exist so `brew upgrade dotnet` keeps control of the active toolchain.
if [[ -n "${HOMEBREW_PREFIX:-}" && -d "${HOMEBREW_PREFIX}/opt/dotnet/libexec" ]]; then
    export DOTNET_ROOT="${HOMEBREW_PREFIX}/opt/dotnet/libexec"
elif [[ -d "${HOME}/.dotnet" ]]; then
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

# Tmux auto-attach — runs in the login shell BEFORE .zshrc so the outer
# process hands off to tmux without paying the antidote / compinit /
# plugin-source tax in .zshrc. Inner zsh (spawned by tmux as default-
# command) is NOT a login shell, so it skips this block and loads .zshrc
# normally. The $TMUX / $SSH_TTY / $VSCODE_PID guards prevent launching
# tmux in nested or non-interactive contexts.
if [[ -z "$TMUX" && -z "$SSH_TTY" && -z "$VSCODE_PID" ]] \
   && [[ "$TERM_PROGRAM" != "vscode" && "$TERM_PROGRAM" != "cursor" ]] \
   && [[ -t 0 ]] \
   && (( $+commands[tmux] )); then
    if ! tmux has-session -t Main 2>/dev/null; then
        exec tmux new-session -s Main
    elif [[ "$(tmux display-message -t Main -p '#{session_attached}')" == "0" ]]; then
        exec tmux attach -t Main
    else
        exec tmux new-session -s "term-$$" \; \
            set-option destroy-unattached on
    fi
fi
