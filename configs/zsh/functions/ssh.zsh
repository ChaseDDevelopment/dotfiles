# SSH with auto-attach to tmux session on remote server
# Always attaches to "main" session (creates if doesn't exist)
#
# Usage:
#   ssht server1              # Attach to "main" session
#   ssht user@192.168.1.10    # Works with full SSH syntax
#
function ssht() {
    if [[ -z "$1" ]]; then
        echo "Usage: ssht <host> [ssh-options...]"
        echo "Connects via SSH and attaches to tmux 'main' session"
        return 1
    fi

    local host="$1"
    shift

    # -t forces TTY allocation (required for tmux)
    # new-session -A: attach if exists, create if not
    # -s main: session name
    ssh -t "$@" "$host" "tmux new-session -A -s main"

    # Reset terminal state after SSH exits
    # Fixes terminal corruption when connection drops unexpectedly
    stty sane 2>/dev/null
    echo
}

# Completion: reuse ssh completions for ssht
compdef ssht=ssh
