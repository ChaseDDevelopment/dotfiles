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

# bat / batcat (Ubuntu/Debian names it batcat due to package conflict)
if (( $+commands[bat] )); then
    alias cat='bat --paging=never'
elif (( $+commands[batcat] )); then
    alias cat='batcat --paging=never'
    alias bat='batcat'
fi

if (( $+commands[rg] )); then
    alias grep='rg'
fi

# fd / fdfind (Ubuntu/Debian names it fdfind due to package conflict)
if (( $+commands[fd] )); then
    alias find='fd'
elif (( $+commands[fdfind] )); then
    alias find='fdfind'
    alias fd='fdfind'
fi

# Safety
# rm -I: prompts once for >3 files or recursive (not per-file like -i)
if [[ "$OSTYPE" == darwin* ]]; then
    # macOS: use GNU rm from coreutils
    alias rm='grm -I'
else
    # Linux: GNU rm is default
    alias rm='rm -I'
fi
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
