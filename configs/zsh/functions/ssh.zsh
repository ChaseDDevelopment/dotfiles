# SSH with auto-attach to tmux session on remote server
# Always attaches to "main" session (creates if doesn't exist)
#
# Usage:
#   ssht server1              # Attach to "main" session
#   ssht user@192.168.1.10    # Works with full SSH syntax
#   ssht -c server1           # Keep local Mac awake during session
#
function ssht() {
    local caffeinate_mode=0

    case "$1" in
        -c|--caffeinate)
            caffeinate_mode=1
            shift
            ;;
    esac

    if [[ -z "$1" ]]; then
        echo "Usage: ssht [-c|--caffeinate] <host> [ssh-options...]"
        echo "Connects via SSH and attaches to tmux 'main' session"
        echo "  -c, --caffeinate  prevent local idle sleep during session"
        return 1
    fi

    if (( caffeinate_mode )) \
        && ! (( $+commands[caffeinate] || $+functions[caffeinate] )); then
        echo "ssht: caffeinate requested, but caffeinate was not found"
        return 1
    fi

    local host="$1"
    local remote_cmd
    shift

    # -t forces TTY allocation (required for tmux).
    #
    # Try the tmux-session wrapper (installed to ~/.local/bin by the
    # dotfiles installer — findable via .zshenv's PATH prepend even in
    # non-interactive sshd shells where brew PATH is otherwise absent).
    # Fall back to an inline PATH prefix covering common brew install
    # locations for hosts where the installer hasn't run yet.
    #
    # Single-quoted so the local shell leaves $HOME / PATH alone;
    # expansion happens on the remote. `command -v` is the right signal
    # (cheap PATH existence check) — guards against cases where the
    # wrapper exists but errors on source, which would otherwise leak
    # stderr noise into the fallback path.
    remote_cmd='command -v tmux-session >/dev/null 2>&1'
    remote_cmd+=' && exec tmux-session Main'
    remote_cmd+=' || PATH="$HOME/.local/bin:/home/linuxbrew/.linuxbrew/bin'
    remote_cmd+=':/opt/homebrew/bin:/usr/local/bin:$PATH"'
    remote_cmd+=' exec tmux new-session -A -s Main'

    local -a ssh_cmd=(ssh -t "$@" "$host" "$remote_cmd")

    if (( caffeinate_mode )); then
        caffeinate -i "${ssh_cmd[@]}"
    else
        "${ssh_cmd[@]}"
    fi

    # Reset terminal state after SSH exits
    # Fixes terminal corruption when connection drops unexpectedly
    stty sane 2>/dev/null
    echo
}

# Completion: reuse ssh completions for ssht
compdef ssht=ssh
