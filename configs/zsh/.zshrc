# ============================================================================
# ZSH Configuration
# ============================================================================

# ----------------------------------------------------------------------------
# Profiling (uncomment to debug slow startup)
# ----------------------------------------------------------------------------
# zmodload zsh/zprof

# ----------------------------------------------------------------------------
# History Configuration
# ----------------------------------------------------------------------------
HISTFILE="${XDG_STATE_HOME:-$HOME/.local/state}/zsh/history"
[[ -d "${HISTFILE:h}" ]] || mkdir -p "${HISTFILE:h}"
HISTSIZE=50000
SAVEHIST=50000

setopt EXTENDED_HISTORY          # Write timestamps to history
setopt HIST_EXPIRE_DUPS_FIRST    # Expire duplicates first
setopt HIST_IGNORE_DUPS          # Don't record duplicates
setopt HIST_IGNORE_ALL_DUPS      # Delete old recorded entry if new is duplicate
setopt HIST_FIND_NO_DUPS         # Don't display duplicates in search
setopt HIST_IGNORE_SPACE         # Don't record entries starting with space
setopt HIST_SAVE_NO_DUPS         # Don't write duplicates
setopt HIST_REDUCE_BLANKS        # Remove superfluous blanks
setopt HIST_VERIFY               # Don't execute immediately on history expansion
setopt SHARE_HISTORY             # Share history between sessions
setopt INC_APPEND_HISTORY        # Add commands immediately

# ----------------------------------------------------------------------------
# General Options
# ----------------------------------------------------------------------------
setopt AUTO_CD                   # cd by typing directory name
setopt AUTO_PUSHD                # Push directories onto stack
setopt PUSHD_IGNORE_DUPS         # Don't push duplicates
setopt PUSHD_SILENT              # Don't print directory stack
setopt CORRECT                   # Command correction
setopt INTERACTIVE_COMMENTS      # Allow comments in interactive shell
setopt NO_BEEP                   # No beeping

# ----------------------------------------------------------------------------
# Completion System
# ----------------------------------------------------------------------------
autoload -Uz compinit

# Only regenerate .zcompdump once per day
if [[ -n ${ZDOTDIR}/.zcompdump(#qN.mh+24) ]]; then
    compinit
else
    compinit -C
fi

# Completion styling
zstyle ':completion:*' completer _extensions _complete _approximate
zstyle ':completion:*' use-cache on
zstyle ':completion:*' cache-path "${XDG_CACHE_HOME:-$HOME/.cache}/zsh/zcompcache"
zstyle ':completion:*' menu select
zstyle ':completion:*' list-colors ${(s.:.)LS_COLORS}
zstyle ':completion:*' matcher-list 'm:{a-zA-Z}={A-Za-z}' 'r:|=*' 'l:|=* r:|=*'
zstyle ':completion:*:*:*:*:descriptions' format '%F{green}-- %d --%f'
zstyle ':completion:*:*:*:*:corrections' format '%F{yellow}!- %d (errors: %e) -!%f'
zstyle ':completion:*:messages' format ' %F{purple} -- %d --%f'
zstyle ':completion:*:warnings' format ' %F{red}-- no matches found --%f'
zstyle ':completion:*' group-name ''
zstyle ':completion:*:default' list-prompt '%S%M matches%s'

# ----------------------------------------------------------------------------
# ohmyzsh Compatibility (for plugins loaded via Antidote)
# ----------------------------------------------------------------------------
[[ -d "$ZSH_CACHE_DIR/completions" ]] || mkdir -p "$ZSH_CACHE_DIR/completions"

# ----------------------------------------------------------------------------
# Antidote Plugin Manager (cross-platform detection)
# ----------------------------------------------------------------------------
if [[ -f "/opt/homebrew/opt/antidote/share/antidote/antidote.zsh" ]]; then
    # macOS Apple Silicon (Homebrew)
    source /opt/homebrew/opt/antidote/share/antidote/antidote.zsh
elif [[ -f "/usr/local/opt/antidote/share/antidote/antidote.zsh" ]]; then
    # macOS Intel (Homebrew)
    source /usr/local/opt/antidote/share/antidote/antidote.zsh
elif [[ -f "/home/linuxbrew/.linuxbrew/opt/antidote/share/antidote/antidote.zsh" ]]; then
    # Linux (Linuxbrew)
    source /home/linuxbrew/.linuxbrew/opt/antidote/share/antidote/antidote.zsh
elif [[ -d "${ZDOTDIR}/.antidote" ]]; then
    # Git clone installation (fallback for any platform)
    source "${ZDOTDIR}/.antidote/antidote.zsh"
else
    echo "Warning: Antidote not found. Run the setup script to install."
fi

# Generate static plugin file if needed
if (( $+functions[antidote] )); then
    zsh_plugins="${ZDOTDIR}/plugins/.zsh_plugins.zsh"
    if [[ ! ${zsh_plugins} -nt ${zsh_plugins:r}.txt ]]; then
        antidote bundle < "${zsh_plugins:r}.txt" > "${zsh_plugins}"
    fi
    source "${zsh_plugins}"
fi

# ----------------------------------------------------------------------------
# fzf-tab Configuration
# ----------------------------------------------------------------------------
# Preview directory contents
zstyle ':fzf-tab:complete:cd:*' fzf-preview 'eza -1 --color=always --icons $realpath 2>/dev/null || ls -1 --color=always $realpath'
zstyle ':fzf-tab:complete:ls:*' fzf-preview 'eza -1 --color=always --icons $realpath 2>/dev/null || ls -1 --color=always $realpath'

# Preview file contents
zstyle ':fzf-tab:complete:cat:*' fzf-preview 'bat --color=always --style=numbers --line-range=:100 $realpath 2>/dev/null || cat $realpath'
zstyle ':fzf-tab:complete:bat:*' fzf-preview 'bat --color=always --style=numbers --line-range=:100 $realpath 2>/dev/null || cat $realpath'

# Preview environment variables
zstyle ':fzf-tab:complete:(-command-|-parameter-|-brace-parameter-|export|unset|expand):*' fzf-preview 'echo ${(P)word}'

# Switch groups with < and >
zstyle ':fzf-tab:*' switch-group '<' '>'

# Use regular fzf
zstyle ':fzf-tab:*' fzf-command fzf

# ----------------------------------------------------------------------------
# Tool Integrations (conditional loading)
# ----------------------------------------------------------------------------

# Zoxide (smart cd)
(( $+commands[zoxide] )) && eval "$(zoxide init zsh)"

# Atuin (better history)
(( $+commands[atuin] )) && eval "$(atuin init zsh --disable-up-arrow)"

# fzf
(( $+commands[fzf] )) && eval "$(fzf --zsh)"

# ----------------------------------------------------------------------------
# Autosuggestions Configuration
# ----------------------------------------------------------------------------
ZSH_AUTOSUGGEST_STRATEGY=(history completion)
ZSH_AUTOSUGGEST_USE_ASYNC=1
ZSH_AUTOSUGGEST_BUFFER_MAX_SIZE=20
ZSH_AUTOSUGGEST_HIGHLIGHT_STYLE="fg=#6c7086"

# ----------------------------------------------------------------------------
# Keybindings
# ----------------------------------------------------------------------------
# Edit current command line in $EDITOR (nvim)
autoload -Uz edit-command-line
zle -N edit-command-line
bindkey '^x^e' edit-command-line

# Better word navigation (Option+Arrow on macOS, Alt+Arrow on Linux)
bindkey '^[[1;3D' backward-word
bindkey '^[[1;3C' forward-word

# ----------------------------------------------------------------------------
# Source Additional Configs
# ----------------------------------------------------------------------------
for config_file in "${ZDOTDIR}"/aliases/*.zsh(N); do
    source "${config_file}"
done

for config_file in "${ZDOTDIR}"/functions/*.zsh(N); do
    source "${config_file}"
done

for config_file in "${ZDOTDIR}"/tools/*.zsh(N); do
    source "${config_file}"
done

# Local machine-specific config (not in git)
[[ -f "${ZDOTDIR}/local.zsh" ]] && source "${ZDOTDIR}/local.zsh"

# ----------------------------------------------------------------------------
# Starship Prompt (MUST be last to properly hook into prompt)
# ----------------------------------------------------------------------------
(( $+commands[starship] )) && eval "$(starship init zsh)"

# ----------------------------------------------------------------------------
# Terminal State Reset (fixes prompt hang after interactive programs)
# ----------------------------------------------------------------------------
# Force terminal reset on precmd to handle dirty state from nvim/ssh/tmux
function _reset_terminal_state() {
    # Ensure cursor is visible (some programs leave it hidden)
    echoti cnorm 2>/dev/null || echo -ne '\033[?25h'
}
precmd_functions+=(_reset_terminal_state)

# Fix prompt not appearing after Ctrl+C
TRAPINT() {
    zle && zle .reset-prompt
    return $(( 128 + $1 ))
}

# ----------------------------------------------------------------------------
# Profiling output (uncomment if debugging)
# ----------------------------------------------------------------------------
# zprof
