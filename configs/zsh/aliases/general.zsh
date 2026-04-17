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

# Privileged editing — sudoedit runs nvim as YOUR user with full config,
# then writes the result back as root. Avoids permission/ownership issues.
alias snvim='sudoedit'

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

# Modern du replacement + storage-sleuthing shortcuts
if (( $+commands[dust] )); then
    alias du='dust'
    alias bigdirs='dust -n 30 -d 3'     # top 30 dirs, max depth 3
    alias bigfiles='dust -n 30 -F'      # top 30 files (skip dirs)
    alias biggest='dust -n 20 -x /'     # whole-root, one filesystem
fi

# Docker compose shorthand
alias dc='docker compose'

# Suffix aliases — type a filename as a command to open it.
#   `report.json`  → jless report.json
#   `./notes.md`   → bat notes.md
# Only fires when the filename is the first word; `cat file.json` is unaffected.
if (( $+commands[bat] )); then
    _view_cmd='bat'
elif (( $+commands[batcat] )); then
    _view_cmd='batcat'
else
    _view_cmd='less'
fi
for _ext in txt log md yml yaml toml ini conf; do
    alias -s "$_ext"="$_view_cmd"
done
unset _ext

if (( $+commands[jless] )); then
    alias -s json='jless'
    alias -s ndjson='jless'
else
    alias -s json="$_view_cmd"
    alias -s ndjson="$_view_cmd"
fi
unset _view_cmd
