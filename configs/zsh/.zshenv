# XDG Base Directories
export XDG_CONFIG_HOME="${HOME}/.config"
export XDG_DATA_HOME="${HOME}/.local/share"
export XDG_CACHE_HOME="${HOME}/.cache"
export XDG_STATE_HOME="${HOME}/.local/state"
export ZSH_CACHE_DIR="${XDG_CACHE_HOME}/ohmyzsh"

# Zsh config location
export ZDOTDIR="${XDG_CONFIG_HOME}/zsh"

# Default editor
export EDITOR="nvim"
export VISUAL="nvim"

# Language
export LANG="en_US.UTF-8"
export LC_ALL="en_US.UTF-8"

# Claude Code
export CLAUDE_CODE_NO_FLICKER=1

# User-local bin dirs live here (not .zprofile) so non-login shells
# — tmux panes, `ssh host cmd`, subshells spawned by the Go installer —
# also see them. path_helper on macOS only reorders system dirs in
# /etc/paths*, so $HOME-prefixed entries keep their prepended spot.
typeset -U path
path=(
    "${HOME}/.local/bin"
    "${HOME}/.cargo/bin"
    "${HOME}/.bun/bin"
    "${HOME}/go/bin"
    $path
)
export PATH

# Homebrew shellenv + macOS GUI-app paths stay in .zprofile — they
# must run AFTER /etc/zprofile's path_helper to keep /opt/homebrew
# ahead of /usr/bin (see commit e14ec37).

# Cross-platform clipboard detection for fzf
if [[ "$OSTYPE" == "darwin"* ]]; then
    FZF_CLIP_CMD="pbcopy"
elif command -v xclip &>/dev/null; then
    FZF_CLIP_CMD="xclip -selection clipboard"
elif command -v xsel &>/dev/null; then
    FZF_CLIP_CMD="xsel --clipboard --input"
elif command -v wl-copy &>/dev/null; then
    FZF_CLIP_CMD="wl-copy"
else
    FZF_CLIP_CMD="cat >/dev/null"  # Fallback: discard
fi

# fzf defaults with Catppuccin Mocha colors
export FZF_DEFAULT_OPTS="
  --height=40%
  --layout=reverse
  --border=rounded
  --info=inline
  --marker='*'
  --pointer='>'
  --preview-window=right:60%
  --bind='ctrl-y:execute-silent(echo -n {} | ${FZF_CLIP_CMD})'
  --color=bg+:#313244,bg:#1e1e2e,spinner:#f5e0dc,hl:#f38ba8
  --color=fg:#cdd6f4,header:#f38ba8,info:#cba6f7,pointer:#f5e0dc
  --color=marker:#b4befe,fg+:#cdd6f4,prompt:#cba6f7,hl+:#f38ba8
  --color=selected-bg:#45475a
"

# Use fd instead of find for fzf (respects .gitignore)
if command -v fd &>/dev/null; then
    export FZF_DEFAULT_COMMAND='fd --type f --hidden --exclude .git'
    export FZF_CTRL_T_COMMAND="$FZF_DEFAULT_COMMAND"
    export FZF_ALT_C_COMMAND='fd --type d --hidden --exclude .git'
fi
