# Bash-compatible aliases loaded by Pi's shellCommandPrefix.
# Keep this file free of zsh-only syntax; Pi runs non-interactive bash.

if command -v eza >/dev/null 2>&1; then
    alias ls='eza --icons --group-directories-first'
    alias ll='eza -la --icons --git --header --group-directories-first --color-scale'
    alias la='eza -a --icons --group-directories-first'
    alias lt='eza --tree --level=2 --icons'
    alias lt2='eza --tree --level=2 --icons'
    alias lt3='eza --tree --level=3 --icons'
    alias ltg='eza --tree --level=2 --icons --git'
else
    if ls --color=auto /dev/null >/dev/null 2>&1; then
        alias ls='ls --color=auto'
        alias ll='ls -la --color=auto'
        alias la='ls -a --color=auto'
    else
        alias ll='ls -la'
        alias la='ls -a'
    fi
fi

if command -v batcat >/dev/null 2>&1 && ! command -v bat >/dev/null 2>&1; then
    alias bat='batcat'
fi

if command -v rg >/dev/null 2>&1; then
    alias grep='rg'
fi

if command -v fd >/dev/null 2>&1; then
    alias find='fd'
elif command -v fdfind >/dev/null 2>&1; then
    alias find='fdfind'
    alias fd='fdfind'
fi

if command -v tspin >/dev/null 2>&1; then
    alias tails='tspin -f'
    alias tailspin='tspin'
fi

alias c='clear'
alias h='history'
alias j='jobs -l'
alias path='tr ":" "\n" <<< "$PATH"'
alias now='date +"%Y-%m-%d %H:%M:%S"'
alias snvim='sudoedit'
alias zshrc='${EDITOR:-nvim} "${ZDOTDIR:-$HOME/.config/zsh}/.zshrc"'
alias myip='curl -s ifconfig.me'
alias ports='ss -tulanp'

alias ..='cd ..'
alias ...='cd ../..'
alias ....='cd ../../..'
alias .....='cd ../../../..'

if command -v dust >/dev/null 2>&1; then
    alias du='dust'
    alias bigdirs='dust -n 30 -d 3'
    alias bigfiles='dust -n 30 -F'
    alias biggest='dust -n 20 -x /'
fi

alias dc='docker compose'

alias g='git'
alias gs='git status'
alias gss='git status -s'
alias ga='git add'
alias gaa='git add --all'
alias gc='git commit -v'
alias gcm='git commit -m'
alias gb='git branch'
alias gba='git branch -a'
alias gbd='git branch -d'
alias gbD='git branch -D'
alias gco='git checkout'
alias gcb='git checkout -b'
alias gsw='git switch'
alias gswc='git switch -c'
alias gd='git diff'
alias gdc='git diff --cached'
alias gds='git diff --staged'
alias gf='git fetch'
alias gfa='git fetch --all --prune'
alias gl='git pull'
alias glr='git pull --rebase'
alias gp='git push'
alias gpsup='git push --set-upstream origin $(git branch --show-current)'
alias grb='git rebase'
alias grbc='git rebase --continue'
alias grba='git rebase --abort'
alias grh='git reset HEAD'
alias glog='git log --oneline --graph --decorate'
alias gloga='git log --oneline --graph --decorate --all'
alias gsta='git stash push'
alias gstp='git stash pop'
alias gstl='git stash list'
alias gstd='git stash drop'
alias lg='lazygit'
alias gcp='git cherry-pick'
alias gm='git merge'
alias grs='git restore'
alias grss='git restore --staged'
