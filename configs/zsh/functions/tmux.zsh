# Intentional tmux sessions. Ordinary shells stay ordinary until `t` is run.
function t() {
    if (( $# > 1 )); then
        echo "Usage: t [session-name]"
        return 2
    fi
    if ! (( $+commands[tmux] )); then
        echo "t: tmux is not installed"
        return 127
    fi

    local session root
    if [[ -n "${1:-}" ]]; then
        session="$1"
    else
        root=$(git -C "$PWD" rev-parse --show-toplevel 2>/dev/null) || root="$PWD"
        session="${root:t}"
    fi
    session="${session//[^A-Za-z0-9_-]/-}"

    if [[ -n "${TMUX:-}" ]]; then
        if ! tmux has-session -t "=$session" 2>/dev/null; then
            tmux new-session -d -s "$session" -c "$PWD" || return $?
        fi
        tmux switch-client -t "=$session"
    else
        tmux new-session -A -s "$session" -c "$PWD"
    fi
}

function ts() {
    if ! (( $+commands[tmux] )); then
        echo "ts: tmux is not installed"
        return 127
    fi
    if ! tmux list-sessions >/dev/null 2>&1; then
        echo "ts: no tmux sessions"
        return 1
    fi

    if [[ -n "${TMUX:-}" ]]; then
        tmux choose-tree -s
        return $?
    fi
    if ! (( $+commands[fzf] )); then
        echo "ts: fzf is required outside tmux"
        return 127
    fi

    local session
    session=$(tmux list-sessions -F '#S' | fzf --prompt='tmux> ') || return $?
    [[ -n "$session" ]] || return 1
    tmux attach-session -t "=$session"
}

function _tmux_window_session() {
    if [[ -n "${TMUX:-}" ]]; then
        tmux display-message -p '#{session_name}'
    else
        tmux display-message -p -t '=Main:' '#{session_name}'
    fi
}

function tw() {
    if (( $# != 1 )) || [[ ! "${1:-}" =~ "^[A-Za-z0-9._-]+$" ]]; then
        echo "Usage: tw <window-name>"
        return 2
    fi
    local session
    session=$(_tmux_window_session 2>/dev/null) || {
        echo "tw: no outer tmux session; start Main first"
        return 2
    }
    tmux select-window -t "=$session:=$1" 2>/dev/null && return 0
    tmux new-window -t "=$session:" -n "$1" -c "$PWD"
}

function hw() {
    if (( $# != 1 )) || [[ ! "${1:-}" =~ "^[A-Za-z0-9][A-Za-z0-9._-]*$" ]]; then
        echo "Usage: hw <host>"
        return 2
    fi
    local host="$1" session
    session=$(_tmux_window_session 2>/dev/null) || {
        echo "hw: no outer tmux session; start Main first"
        return 2
    }
    tmux select-window -t "=$session:=$host" 2>/dev/null && return 0
    tmux new-window -t "=$session:" -n "$host" -c "$PWD" \
        "/bin/zsh -lc 'herdr --remote $host; exec /bin/zsh -l'"
}

compdef hw=ssh
