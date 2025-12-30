# Modern ls replacement with maximum beauty/info
if (( $+commands[eza] )); then
    # Basic listing with icons
    alias ls='eza --icons --group-directories-first'

    # Long format with git status, headers, and size coloring
    alias ll='eza -la --icons --git --header --group-directories-first --color-scale'
    alias la='eza -a --icons --group-directories-first'

    # Tree views
    alias lt='eza --tree --level=2 --icons'
    alias lt2='eza --tree --level=2 --icons'
    alias lt3='eza --tree --level=3 --icons'

    # Git-focused tree (great for repos)
    alias ltg='eza --tree --level=2 --icons --git'
else
    # GNU ls fallback with colors
    alias ls='ls --color=auto'
    alias ll='ls -la --color=auto'
    alias la='ls -a --color=auto'
fi

# bat / batcat (Ubuntu/Debian names it batcat due to package conflict)
# -pp = plain (no line numbers) + no paging (copy-paste friendly)
if (( $+commands[bat] )); then
    alias cat='bat -pp'
elif (( $+commands[batcat] )); then
    alias cat='batcat -pp'
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

# tailspin for pretty log viewing (tspin is the binary)
if (( $+commands[tspin] )); then
    alias tails='tspin -f'      # Follow mode (like tail -f but pretty)
    alias tailspin='tspin'      # Full name alias
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
