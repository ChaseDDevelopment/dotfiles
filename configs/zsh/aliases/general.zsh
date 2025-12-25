# Modern replacements (conditional - only if tools are installed)
if (( $+commands[eza] )); then
    alias ls='eza --icons --group-directories-first'
    alias ll='eza -la --icons --group-directories-first'
    alias la='eza -a --icons --group-directories-first'
    alias lt='eza --tree --level=2 --icons'
    alias lt2='eza --tree --level=2 --icons'
    alias lt3='eza --tree --level=3 --icons'
else
    alias ll='ls -la'
    alias la='ls -a'
fi

if (( $+commands[bat] )); then
    alias cat='bat --paging=never'
fi

if (( $+commands[rg] )); then
    alias grep='rg'
fi

if (( $+commands[fd] )); then
    alias find='fd'
fi

# Safety
alias rm='rm -i'
alias cp='cp -i'
alias mv='mv -i'

# Convenience
alias c='clear'
alias h='history'
alias j='jobs -l'
alias path='echo -e ${PATH//:/\\n}'
alias now='date +"%Y-%m-%d %H:%M:%S"'

# Quick edits
alias zshrc='${EDITOR} ${ZDOTDIR}/.zshrc'
alias reload='source ${ZDOTDIR}/.zshrc'

# Networking
alias myip='curl -s ifconfig.me'
alias ports='ss -tulanp'

# Quick parent directory navigation
alias ..='cd ..'
alias ...='cd ../..'
alias ....='cd ../../..'
alias .....='cd ../../../..'

# Docker compose shorthand
alias dc='docker compose'
