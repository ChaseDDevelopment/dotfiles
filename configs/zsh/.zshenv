# XDG Base Directories
export XDG_CONFIG_HOME="${HOME}/.config"
export XDG_DATA_HOME="${HOME}/.local/share"
export XDG_CACHE_HOME="${HOME}/.cache"
export XDG_STATE_HOME="${HOME}/.local/state"
export ZSH_CACHE_DIR="${XDG_CACHE_HOME}/ohmyzsh"

# Go: keep module cache out of $HOME/go and drop binaries into
# ~/.local/bin alongside uv, oh-my-posh, etc. `go install` without
# these defaults creates ~/go (GOPATH) and ~/go/bin (GOBIN).
export GOPATH="${XDG_DATA_HOME}/go"
export GOBIN="${HOME}/.local/bin"

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

# fzf defaults with TokyoNight Night colors
export FZF_DEFAULT_OPTS="
  --height=40%
  --layout=reverse
  --border=rounded
  --info=inline
  --marker='*'
  --pointer='>'
  --preview-window=right:60%
  --bind='ctrl-y:execute-silent(echo -n {} | ${FZF_CLIP_CMD})'
  --color=bg+:#292e42,bg:#1a1b26,spinner:#7dcfff,hl:#f7768e
  --color=fg:#c0caf5,header:#f7768e,info:#bb9af7,pointer:#7dcfff
  --color=marker:#bb9af7,fg+:#c0caf5,prompt:#bb9af7,hl+:#f7768e
  --color=selected-bg:#414868
"

# Use fd instead of find for fzf (respects .gitignore)
if command -v fd &>/dev/null; then
    export FZF_DEFAULT_COMMAND='fd --type f --hidden --exclude .git'
    export FZF_CTRL_T_COMMAND="$FZF_DEFAULT_COMMAND"
    export FZF_ALT_C_COMMAND='fd --type d --hidden --exclude .git'
fi
