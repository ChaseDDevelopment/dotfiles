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
    if [[ -n "${TMUX:-}" && -n "${TMUX_PANE:-}" ]]; then
        tmux display-message -p -t "$TMUX_PANE" '#{session_name}'
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
    local host="$1" window="$1" session
    [[ "$host" == hydra ]] && window=Mac-Mini
    [[ "$host" == devbox ]] && window=DevBox
    session=$(_tmux_window_session 2>/dev/null) || {
        echo "hw: no outer tmux session; start Main first"
        return 2
    }

    local pane_format='#{pane_id}|#{pane_pid}|#{pane_current_command}|#{pane_dead}|#{pane_in_mode}|#{window_panes}|#{window_linked}|#{window_name}'
    local panes line child_output child_command target_pane target_pid window_names
    local pane_command pane_dead pane_in_mode pane_count window_linked pgrep_status
    local child_count=0 matching=0 inspection_failed=0
    local -a pane_lines pane_fields child_pids child_args

    if panes=$(tmux list-panes -t "=$session:=$window" -F "$pane_format" 2>/dev/null); then
        pane_lines=("${(@f)panes}")
        for line in "${pane_lines[@]}"; do
            pane_fields=("${(@s:|:)line}")
            (( $#pane_fields >= 2 )) || continue
            [[ -n "$target_pane" ]] || target_pane="${pane_fields[1]}"
            child_output=$(pgrep -P "${pane_fields[2]}" 2>/dev/null)
            pgrep_status=$?
            if (( pgrep_status > 1 )); then
                inspection_failed=1
                continue
            fi
            child_pids=("${(@f)child_output}")
            for target_pid in "${child_pids[@]}"; do
                [[ -n "$target_pid" ]] || continue
                (( child_count++ ))
                child_command=$(ps -p "$target_pid" -o command= 2>/dev/null) || continue
                child_args=(${=child_command})
                if (( $#child_args == 3 )) && \
                    [[ "${child_args[1]:t}" == herdr && \
                    "${child_args[2]}" == --remote && "${child_args[3]}" == "$host" ]]; then
                    matching=1
                fi
            done
        done

        if (( matching )); then
            tmux select-window -t "=$session:=$window" || return $?
            return 0
        fi

        if (( $#pane_lines == 1 )); then
            pane_fields=("${(@s:|:)pane_lines[1]}")
        else
            pane_fields=()
        fi
        if (( $#pane_fields >= 8 )); then
            target_pane="${pane_fields[1]}"
            pane_command="${pane_fields[3]}"
            pane_dead="${pane_fields[4]}"
            pane_in_mode="${pane_fields[5]}"
            pane_count="${pane_fields[6]}"
            window_linked="${pane_fields[7]}"
        fi
        if (( $#pane_fields < 8 || child_count != 0 || inspection_failed )) || \
            [[ "${pane_command:t}" != zsh || "$pane_dead" != 0 || \
            "$pane_in_mode" != 0 || "$pane_count" != 1 || "$window_linked" != 0 ]]; then
            tmux select-window -t "=$session:=$window" 2>/dev/null
            [[ -n "$target_pane" ]] || target_pane="=$session:=$window"
            tmux display-message -t "$target_pane" "hw: $window is busy" 2>/dev/null
            return 1
        fi

        tmux select-window -t "=$session:=$window" || return $?
        if ! (( $+commands[herdr] )); then
            echo "hw: herdr is not installed"
            return 127
        fi
        if [[ -n "${TMUX_PANE:-}" && "$target_pane" == "$TMUX_PANE" ]]; then
            command herdr --remote "$host"
            return $?
        fi
        tmux send-keys -t "$target_pane" C-c || return $?
        tmux send-keys -t "$target_pane" -l "herdr --remote $host" || return $?
        tmux send-keys -t "$target_pane" Enter
        return $?
    fi

    window_names=$(tmux list-windows -t "=$session:" -F '#{window_name}' 2>/dev/null) || {
        echo "hw: cannot inspect tmux session $session"
        return 1
    }
    for line in ${(f)window_names}; do
        if [[ "$line" == "$window" ]]; then
            tmux select-window -t "=$session:=$window" 2>/dev/null
            tmux display-message -t "=$session:=$window" "hw: $window is busy" 2>/dev/null
            return 1
        fi
    done

    if ! (( $+commands[herdr] )); then
        echo "hw: herdr is not installed"
        return 127
    fi

    if [[ -n "${TMUX:-}" && -n "${TMUX_PANE:-}" && "${HERDR_ENV:-}" != 1 ]] && \
        panes=$(tmux list-panes -t "$TMUX_PANE" -F "$pane_format" 2>/dev/null); then
        pane_lines=("${(@f)panes}")
        if (( $#pane_lines == 1 )); then
            pane_fields=("${(@s:|:)pane_lines[1]}")
            if (( $#pane_fields >= 8 )); then
                child_output=$(pgrep -P "${pane_fields[2]}" 2>/dev/null)
                pgrep_status=$?
                if (( pgrep_status <= 1 )) && [[ -z "$child_output" && \
                    "${pane_fields[1]}" == "$TMUX_PANE" && \
                    "${pane_fields[3]:t}" == zsh && "${pane_fields[4]}" == 0 && \
                    "${pane_fields[5]}" == 0 && "${pane_fields[6]}" == 1 && \
                    "${pane_fields[7]}" == 0 && "${pane_fields[8]}" != macbook && \
                    "${pane_fields[8]}" != Macbook ]]; then
                    tmux rename-window -t "$TMUX_PANE" "$window" || return $?
                    command herdr --remote "$host"
                    return $?
                fi
            fi
        fi
    fi

    tmux new-window -t "=$session:" -n "$window" -c "$PWD" \
        "/bin/zsh -lc 'herdr --remote $host; exec /bin/zsh -l'"
}

compdef hw=ssh
