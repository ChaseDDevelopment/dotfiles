# Tmux auto-attach on terminal launch
# - First terminal: attaches to (or creates) "Main" session
# - Additional terminals: create unique "term-<pid>" sessions
# - Non-Main sessions auto-cleanup on detach/close

# Guard: skip if already inside tmux
[[ -n "$TMUX" ]] && return

# Guard: skip for SSH sessions
[[ -n "$SSH_TTY" ]] && return

# Guard: skip for IDE terminals (VS Code, Cursor)
[[ "$TERM_PROGRAM" == "vscode" || "$TERM_PROGRAM" == "cursor" ]] && return
[[ -n "$VSCODE_PID" ]] && return

# Guard: skip if tmux is not installed
(( ! $+commands[tmux] )) && return

# Guard: skip if not a real terminal
[[ ! -t 0 ]] && return

if ! tmux has-session -t Main 2>/dev/null; then
    # No Main session exists — create and attach
    exec tmux new-session -s Main
elif [[ "$(tmux display-message -t Main -p '#{session_attached}')" == "0" ]]; then
    # Main exists but no clients attached — reattach
    exec tmux attach -t Main
else
    # Main is in use — create a new disposable session
    exec tmux new-session -s "term-$$" \; \
        set-option destroy-unattached on
fi
