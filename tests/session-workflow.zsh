#!/usr/bin/env zsh

set -u

repo_root=${0:A:h:h}
tmp_dir=$(mktemp -d)
trap 'rm -rf -- "$tmp_dir"' EXIT
export TEST_LOG="$tmp_dir/commands.log"
export COMPDEF_LOG="$tmp_dir/compdefs.log"
: > "$TEST_LOG"
: > "$COMPDEF_LOG"

fail() {
    print -u2 -- "FAIL: $*"
    exit 1
}

assert_contains() {
    [[ "$1" == *"$2"* ]] || fail "$3 (missing: $2)"
}

assert_not_contains() {
    [[ "$1" != *"$2"* ]] || fail "$3 (unexpected: $2)"
}

assert_eq() {
    [[ "$1" == "$2" ]] || fail "$3 (got: $1, want: $2)"
}

assert_order() {
    local rest="${1#*$2}"
    [[ "$rest" != "$1" && "$rest" == *"$3"* ]] || \
        fail "$4 (expected $2 before $3)"
}

print -r -- '#!/usr/bin/env zsh
print -r -- "$*" >> "$TEST_LOG"
if [[ "$1" == has-session ]]; then
    exit "${FAKE_TMUX_HAS_SESSION:-1}"
fi
if [[ "$1" == list-sessions ]]; then
    (( ${FAKE_TMUX_LIST_STATUS:-0} == 0 )) || exit "$FAKE_TMUX_LIST_STATUS"
    print -r -- Alpha
    print -r -- Beta
fi
if [[ "$1" == list-windows ]]; then
    (( ${FAKE_TMUX_WINDOWS_STATUS:-0} == 0 )) || exit "$FAKE_TMUX_WINDOWS_STATUS"
    print -r -- "${FAKE_TMUX_WINDOWS:-}"
fi
if [[ "$1" == display-message ]]; then
    exit_status="${FAKE_TMUX_DISPLAY_STATUS:-0}"
    (( exit_status == 0 )) || exit "$exit_status"
    [[ " $* " == *" -p "* ]] && print -r -- "${FAKE_TMUX_SESSION:-Main}"
fi
if [[ "$1" == select-window ]]; then
    exit "${FAKE_TMUX_SELECT_STATUS:-1}"
fi
if [[ "$1" == list-panes ]]; then
    target=""
    for (( i = 2; i <= $#; i++ )); do
        if [[ "${@[i - 1]}" == -t ]]; then
            target="${@[i]}"
            break
        fi
    done
    if [[ -n "${TMUX_PANE:-}" && "$target" == "$TMUX_PANE" ]]; then
        exit_status="${FAKE_TMUX_CURRENT_STATUS:-0}"
        panes="${FAKE_TMUX_CURRENT_PANES:-}"
    else
        exit_status="${FAKE_TMUX_TARGET_STATUS:-1}"
        panes="${FAKE_TMUX_TARGET_PANES:-}"
    fi
    (( exit_status == 0 )) || exit "$exit_status"
    print -r -- "$panes"
fi' > "$tmp_dir/tmux"

print -r -- '#!/usr/bin/env zsh
print -r -- "${FAKE_FZF_SELECTION:-Beta}"' > "$tmp_dir/fzf"

print -r -- '#!/usr/bin/env zsh
print -r -- "$*" >> "$TEST_LOG"
exit "${FAKE_SSH_STATUS:-0}"' > "$tmp_dir/ssh"

print -r -- '#!/usr/bin/env zsh
print -r -- "$*" >> "$TEST_LOG"' > "$tmp_dir/herdr"

print -r -- '#!/usr/bin/env zsh
(( ${FAKE_HOSTNAME_STATUS:-0} == 0 )) || exit "$FAKE_HOSTNAME_STATUS"
[[ "${1:-}" == -s ]] && print -r -- "${FAKE_HOSTNAME-Chases-MacBook-Pro}"' > "$tmp_dir/hostname"

print -r -- '#!/usr/bin/env zsh
print -r -- "pgrep $*" >> "$TEST_LOG"
if [[ -n "${FAKE_PGREP_STATUS:-}" ]]; then
    (( FAKE_PGREP_STATUS == 0 )) && print -rl -- ${=FAKE_CHILD_PIDS}
    exit "$FAKE_PGREP_STATUS"
fi
if [[ "$*" == "-P ${FAKE_CHILD_PARENT:-}" && -n "${FAKE_CHILD_PIDS:-}" ]]; then
    print -rl -- ${=FAKE_CHILD_PIDS}
    exit 0
fi
exit 1' > "$tmp_dir/pgrep"

print -r -- '#!/usr/bin/env zsh
print -r -- "ps $*" >> "$TEST_LOG"
print -r -- "${FAKE_CHILD_COMMAND:-}"' > "$tmp_dir/ps"

chmod +x "$tmp_dir"/{tmux,fzf,ssh,herdr,hostname,pgrep,ps}
export PATH="$tmp_dir:$PATH"
rehash

compdef() { print -r -- "$*" >> "$COMPDEF_LOG"; }

[[ -r "$repo_root/configs/zsh/functions/tmux.zsh" ]] || \
    fail "tmux session helpers are missing"
[[ -r "$repo_root/configs/zsh/functions/herdr.zsh" ]] || \
    fail "Herdr remote helper is missing"
tmux_main="$repo_root/configs/tmux/scripts/tmux-main"
[[ -x "$tmux_main" ]] || fail "tmux Main bootstrap is missing or not executable"

source "$repo_root/configs/zsh/functions/tmux.zsh"
source "$repo_root/configs/zsh/functions/ssh.zsh"
source "$repo_root/configs/zsh/functions/herdr.zsh"
cd "$repo_root" || fail "cannot enter repository"

: > "$TEST_LOG"
export FAKE_HOSTNAME=Chases-MacBook-Pro
"$tmux_main" >/dev/null || fail "tmux Main bootstrap failed"
log=$(<"$TEST_LOG")
assert_contains "$log" 'new-session -A -s Main -n Macbook' \
    "tmux Main bootstrap must attach or create the named session and window"
assert_contains "$log" \
    "/bin/zsh -lc 'command -v herdr >/dev/null 2>&1 && herdr; exec /bin/zsh -l'" \
    "tmux Main bootstrap must start Herdr with a login-shell fallback"
assert_not_contains "$log" '--remote' \
    "tmux Main bootstrap must leave remote windows to tmux-resurrect"

typeset -a startup_hosts startup_windows
startup_hosts=(Chases-Mac-mini devbox build-node '')
startup_windows=(Mac-Mini DevBox Build-node Local)
for (( i = 1; i <= $#startup_hosts; i++ )); do
    export FAKE_HOSTNAME="${startup_hosts[i]}"
    : > "$TEST_LOG"
    "$tmux_main" >/dev/null || fail "tmux Main bootstrap failed for ${startup_windows[i]}"
    log=$(<"$TEST_LOG")
    assert_contains "$log" "new-session -A -s Main -n ${startup_windows[i]}" \
        "tmux Main bootstrap must map ${startup_hosts[i]:-an empty hostname} to ${startup_windows[i]}"
    assert_not_contains "$log" '--remote' \
        "tmux Main bootstrap must not start remote Herdr on ${startup_windows[i]}"
done
unset FAKE_HOSTNAME

unset TMUX
export FAKE_TMUX_DISPLAY_STATUS=1
: > "$TEST_LOG"
tw logs >/dev/null 2>&1
rc=$?
assert_eq "$rc" 2 "tw must fail when local tmux Main is unavailable"
assert_contains "$(<"$TEST_LOG")" 'display-message -p -t =Main: #{session_name}' \
    "tw must look for local Main when its Herdr pane has no TMUX environment"

export FAKE_TMUX_DISPLAY_STATUS=0
export FAKE_TMUX_SELECT_STATUS=0
: > "$TEST_LOG"
HERDR_ENV=1 tw logs >/dev/null || fail "tw from a Herdr pane failed"
log=$(<"$TEST_LOG")
assert_contains "$log" 'display-message -p -t =Main: #{session_name}' \
    "tw must resolve local Main from a Herdr pane"
assert_contains "$log" 'select-window -t =Main:=logs' \
    "tw from a Herdr pane must select the outer Main window"

: > "$TEST_LOG"
t 'Project One' >/dev/null || fail "t explicit session failed"
log=$(<"$TEST_LOG")
assert_contains "$log" "new-session -A -s Project-One -c $repo_root" \
    "t must normalize and attach an explicit session"

: > "$TEST_LOG"
t >/dev/null || fail "t default session failed"
log=$(<"$TEST_LOG")
assert_contains "$log" 'new-session -A -s dotfiles' \
    "t must derive the Git worktree name"

export TMUX=/tmp/fake-tmux
export TMUX_PANE=%1
export FAKE_TMUX_HAS_SESSION=0
: > "$TEST_LOG"
t Inside >/dev/null || fail "t inside tmux failed"
log=$(<"$TEST_LOG")
assert_contains "$log" 'has-session -t =Inside' \
    "t must check the exact existing session"
assert_not_contains "$log" 'new-session' \
    "t must not create an existing session"
assert_contains "$log" 'switch-client -t =Inside' \
    "t must switch instead of nesting"

export FAKE_TMUX_SESSION=Main
export FAKE_TMUX_SELECT_STATUS=0
: > "$TEST_LOG"
tw logs >/dev/null || fail "tw existing window failed"
log=$(<"$TEST_LOG")
assert_contains "$log" 'display-message -p -t %1 #{session_name}' \
    "tw must resolve the current outer session from the invoking pane"
assert_contains "$log" 'select-window -t =Main:=logs' \
    "tw must select the exact existing named window"
assert_not_contains "$log" 'new-window' \
    "tw must not recreate an existing named window"

export FAKE_TMUX_SELECT_STATUS=1
: > "$TEST_LOG"
tw logs >/dev/null || fail "tw new window failed"
log=$(<"$TEST_LOG")
assert_contains "$log" 'select-window -t =Main:=logs' \
    "tw must check for the exact named window before creating it"
assert_contains "$log" "new-window -t =Main: -n logs -c $repo_root" \
    "tw must create a named window in Main at the current directory"

: > "$TEST_LOG"
tw 'bad;name' >/dev/null 2>&1
rc=$?
assert_eq "$rc" 2 "tw must reject unsafe window names"
[[ ! -s "$TEST_LOG" ]] || fail "tw must reject unsafe names before invoking tmux"

reset_hw_state() {
    export FAKE_TMUX_SESSION=Main
    export FAKE_TMUX_DISPLAY_STATUS=0
    export FAKE_TMUX_SELECT_STATUS=0
    export FAKE_TMUX_TARGET_STATUS=0
    export FAKE_TMUX_CURRENT_STATUS=0
    export FAKE_TMUX_WINDOWS_STATUS=0
    export FAKE_TMUX_WINDOWS=scratch
    export FAKE_TMUX_TARGET_PANES='%2|4200|zsh|0|0|1|0|Mac-Mini'
    export FAKE_TMUX_CURRENT_PANES='%1|4100|zsh|0|0|1|0|scratch'
    unset FAKE_CHILD_PARENT FAKE_CHILD_PIDS FAKE_CHILD_COMMAND FAKE_PGREP_STATUS HERDR_ENV
}

reset_hw_state
export FAKE_CHILD_PARENT=4200
export FAKE_CHILD_PIDS=5001
export FAKE_CHILD_COMMAND='/opt/homebrew/bin/herdr --remote hydra'
: > "$TEST_LOG"
hw hydra >/dev/null || fail "hw matching remote client failed"
log=$(<"$TEST_LOG")
assert_contains "$log" 'display-message -p -t %1 #{session_name}' \
    "hw must resolve the outer session from the explicit invoking pane"
assert_contains "$log" 'list-panes -t =Main:=Mac-Mini' \
    "hw hydra must inspect the exact Mac-Mini target"
assert_contains "$log" 'pgrep -P 4200' \
    "hw must inspect direct children of the target pane PID"
assert_contains "$log" 'ps -p 5001 -o command=' \
    "hw must inspect the direct child's full command line"
assert_contains "$log" 'select-window -t =Main:=Mac-Mini' \
    "hw must select an existing matching remote client"
assert_not_contains "$log" 'send-keys' \
    "hw must not send keys to a matching remote client"
assert_not_contains "$log" 'new-window' \
    "hw must not duplicate a matching remote client"
assert_not_contains "$log" '--remote hydra' \
    "hw must not launch a second matching remote client"

typeset -a near_miss_names near_miss_commands
near_miss_names=(wrong-host wrong-binary extra-arguments)
near_miss_commands=(
    '/opt/homebrew/bin/herdr --remote atlas'
    '/opt/homebrew/bin/not-herdr --remote hydra'
    '/opt/homebrew/bin/herdr --remote hydra --verbose'
)
for (( i = 1; i <= $#near_miss_names; i++ )); do
    reset_hw_state
    export FAKE_CHILD_PARENT=4200
    export FAKE_CHILD_PIDS=5001
    export FAKE_CHILD_COMMAND="${near_miss_commands[i]}"
    : > "$TEST_LOG"
    busy=$(hw hydra 2>&1)
    rc=$?
    assert_eq "$rc" 1 "hw must reject ${near_miss_names[i]} Herdr commands"
    log=$(<"$TEST_LOG")
    assert_not_contains "$log" 'send-keys' \
        "hw must not launch over ${near_miss_names[i]} Herdr commands"
    assert_not_contains "$log" 'new-window' \
        "hw must not duplicate ${near_miss_names[i]} Herdr commands"
done

reset_hw_state
export FAKE_PGREP_STATUS=2
: > "$TEST_LOG"
busy=$(hw hydra 2>&1)
rc=$?
assert_eq "$rc" 1 "hw must treat child-process inspection errors as busy"
log=$(<"$TEST_LOG")
assert_contains "$log" 'display-message -t %2 hw: Mac-Mini is busy' \
    "hw must report a target whose child processes cannot be inspected"
assert_not_contains "$log" 'send-keys' \
    "hw must not launch when child-process inspection fails"
assert_not_contains "$log" 'new-window' \
    "hw must not create over an uninspectable target"

reset_hw_state
export FAKE_TMUX_TARGET_STATUS=2
export FAKE_TMUX_WINDOWS=$'scratch\nMac-Mini'
: > "$TEST_LOG"
busy=$(hw hydra 2>&1)
rc=$?
assert_eq "$rc" 1 "hw must preserve an existing target when pane inspection fails"
log=$(<"$TEST_LOG")
assert_contains "$log" 'list-windows -t =Main:' \
    "hw must distinguish an uninspectable target from an absent target"
assert_contains "$log" 'select-window -t =Main:=Mac-Mini' \
    "hw must focus an existing uninspectable target"
assert_not_contains "$log" 'send-keys' \
    "hw must not send keys to an uninspectable target"
assert_not_contains "$log" 'new-window' \
    "hw must not duplicate an uninspectable target"
assert_not_contains "$log" 'rename-window' \
    "hw must not rename another window over an uninspectable target"

reset_hw_state
: > "$TEST_LOG"
hw hydra >/dev/null || fail "hw idle other-pane launch failed"
log=$(<"$TEST_LOG")
assert_contains "$log" 'select-window -t =Main:=Mac-Mini' \
    "hw must select the idle restored target"
assert_contains "$log" 'send-keys -t %2 C-c' \
    "hw must clear the idle restored shell before launching"
assert_contains "$log" 'send-keys -t %2 -l herdr --remote hydra' \
    "hw must send the remote command literally"
assert_contains "$log" 'send-keys -t %2 Enter' \
    "hw must submit the remote command"
assert_order "$log" 'send-keys -t %2 C-c' \
    'send-keys -t %2 -l herdr --remote hydra' \
    "hw must clear before sending the remote command"
assert_order "$log" 'send-keys -t %2 -l herdr --remote hydra' \
    'send-keys -t %2 Enter' \
    "hw must send the remote command before Enter"

reset_hw_state
export FAKE_TMUX_TARGET_PANES='%1|4100|zsh|0|0|1|0|Mac-Mini'
: > "$TEST_LOG"
hw hydra >/dev/null || fail "hw current target direct launch failed"
log=$(<"$TEST_LOG")
assert_contains "$log" 'select-window -t =Main:=Mac-Mini' \
    "hw must select the current target before launching"
assert_contains "$log" '--remote hydra' \
    "hw must invoke Herdr directly in the current target pane"
assert_not_contains "$log" 'send-keys' \
    "hw must not send keys to its invoking pane"
assert_not_contains "$log" 'new-window' \
    "hw must not recreate the current target"

reset_hw_state
export FAKE_TMUX_TARGET_PANES='%2|4200|herdr|0|0|1|0|Mac-Mini'
export FAKE_CHILD_PARENT=4200
export FAKE_CHILD_PIDS=5001
export FAKE_CHILD_COMMAND='/bin/sleep 99'
: > "$TEST_LOG"
busy=$(hw hydra 2>&1)
rc=$?
assert_eq "$rc" 1 "hw must reject a target with a non-Herdr child"
log=$(<"$TEST_LOG")
assert_contains "$log" 'select-window -t =Main:=Mac-Mini' \
    "hw must focus a busy target for inspection"
assert_contains "$log" 'display-message -t %2 hw: Mac-Mini is busy' \
    "hw must report a concise busy message"
assert_not_contains "$log" 'send-keys' \
    "hw must not send keys to a busy target"
assert_not_contains "$log" 'new-window' \
    "hw must not create over a busy target"
assert_not_contains "$log" 'kill-' \
    "hw must never kill a busy target"
assert_not_contains "$log" 'respawn-' \
    "hw must never respawn a busy target"

typeset -a unsafe_names unsafe_panes
unsafe_names=(multiple-panes copy-mode dead linked non-zsh)
unsafe_panes=(
    $'%2|4200|zsh|0|0|2|0|Mac-Mini\n%3|4300|zsh|0|0|2|0|Mac-Mini'
    '%2|4200|zsh|0|1|1|0|Mac-Mini'
    '%2|4200|zsh|1|0|1|0|Mac-Mini'
    '%2|4200|zsh|0|0|1|1|Mac-Mini'
    '%2|4200|vim|0|0|1|0|Mac-Mini'
)
for (( i = 1; i <= $#unsafe_names; i++ )); do
    reset_hw_state
    export FAKE_TMUX_TARGET_PANES="${unsafe_panes[i]}"
    : > "$TEST_LOG"
    busy=$(hw hydra 2>&1)
    rc=$?
    assert_eq "$rc" 1 "hw must reject ${unsafe_names[i]} targets"
    log=$(<"$TEST_LOG")
    assert_contains "$log" 'select-window -t =Main:=Mac-Mini' \
        "hw must focus ${unsafe_names[i]} targets"
    assert_contains "$log" 'display-message -t %2 hw: Mac-Mini is busy' \
        "hw must report ${unsafe_names[i]} targets as busy"
    assert_not_contains "$log" 'send-keys' \
        "hw must not send keys to ${unsafe_names[i]} targets"
    assert_not_contains "$log" 'new-window' \
        "hw must not create over ${unsafe_names[i]} targets"
    assert_not_contains "$log" 'kill-' \
        "hw must not kill ${unsafe_names[i]} targets"
    assert_not_contains "$log" 'respawn-' \
        "hw must not respawn ${unsafe_names[i]} targets"
done

reset_hw_state
export FAKE_TMUX_TARGET_STATUS=1
: > "$TEST_LOG"
hw hydra >/dev/null || fail "hw safe invoking-window reuse failed"
log=$(<"$TEST_LOG")
assert_contains "$log" 'list-panes -t %1' \
    "hw must inspect the explicit invoking pane before reuse"
assert_contains "$log" 'rename-window -t %1 Mac-Mini' \
    "hw must rename a safe invoking window to Mac-Mini"
assert_contains "$log" '--remote hydra' \
    "hw must invoke Herdr directly after safe reuse"
assert_not_contains "$log" 'new-window' \
    "hw must not create a window when safe reuse is available"

reset_hw_state
export FAKE_TMUX_TARGET_STATUS=1
export FAKE_PGREP_STATUS=2
: > "$TEST_LOG"
hw hydra >/dev/null || fail "hw pgrep-error fallback failed"
log=$(<"$TEST_LOG")
assert_not_contains "$log" 'rename-window' \
    "hw must not reuse a pane whose child processes cannot be inspected"
assert_contains "$log" 'new-window -t =Main: -n Mac-Mini' \
    "hw must create a separate target when invoking-pane inspection fails"

reset_hw_state
export FAKE_TMUX_TARGET_STATUS=1
export FAKE_TMUX_CURRENT_PANES='%1|4100|zsh|0|0|1|0|macbook'
: > "$TEST_LOG"
hw hydra >/dev/null || fail "hw protected macbook path failed"
log=$(<"$TEST_LOG")
assert_not_contains "$log" 'rename-window' \
    "hw must not reuse the protected macbook window"
assert_contains "$log" 'new-window -t =Main: -n Mac-Mini' \
    "hw must create Mac-Mini when the invoking window is protected"
assert_contains "$log" \
    "/bin/zsh -lc 'herdr --remote hydra; exec /bin/zsh -l'" \
    "hw must retain a login-shell fallback in new windows"

reset_hw_state
export FAKE_TMUX_TARGET_STATUS=1
export HERDR_ENV=1
: > "$TEST_LOG"
hw hydra >/dev/null || fail "hw Herdr environment path failed"
log=$(<"$TEST_LOG")
assert_not_contains "$log" 'rename-window' \
    "hw must not reuse a Herdr-managed invoking window"
assert_contains "$log" 'new-window -t =Main: -n Mac-Mini' \
    "hw must create Mac-Mini from a Herdr-managed pane"

reset_hw_state
export FAKE_TMUX_TARGET_STATUS=1
export FAKE_TMUX_CURRENT_PANES='%1|4100|zsh|0|0|1|0|macbook'
: > "$TEST_LOG"
hw atlas >/dev/null || fail "hw other-host naming failed"
log=$(<"$TEST_LOG")
assert_contains "$log" 'list-panes -t =Main:=atlas' \
    "hw must use the host string for non-hydra targets"
assert_contains "$log" 'new-window -t =Main: -n atlas' \
    "hw must name non-hydra windows after the host"
assert_contains "$log" \
    "/bin/zsh -lc 'herdr --remote atlas; exec /bin/zsh -l'" \
    "hw must launch the requested non-hydra host"
assert_not_contains "$log" 'Mac-Mini' \
    "hw must map only hydra to Mac-Mini"

reset_hw_state
export FAKE_TMUX_TARGET_STATUS=1
export FAKE_TMUX_CURRENT_PANES='%1|4100|zsh|0|0|1|0|Macbook'
: > "$TEST_LOG"
hw devbox >/dev/null || fail "hw DevBox mapping failed"
log=$(<"$TEST_LOG")
assert_contains "$log" 'list-panes -t =Main:=DevBox' \
    "hw devbox must inspect the friendly DevBox target"
assert_not_contains "$log" 'rename-window' \
    "hw must not reuse the protected Macbook window"
assert_contains "$log" 'new-window -t =Main: -n DevBox' \
    "hw devbox must create the friendly DevBox window"
assert_contains "$log" \
    "/bin/zsh -lc 'herdr --remote devbox; exec /bin/zsh -l'" \
    "hw devbox must launch the devbox remote"

reset_hw_state
export FAKE_TMUX_TARGET_STATUS=1
unset TMUX_PANE
: > "$TEST_LOG"
hw hydra >/dev/null || fail "hw implicit-pane fallback failed"
log=$(<"$TEST_LOG")
assert_contains "$log" 'display-message -p -t =Main: #{session_name}' \
    "hw must target Main when the invoking pane is not explicit"
assert_not_contains "$log" 'display-message -p #{session_name}' \
    "hw must never resolve a session through implicit client focus"
assert_not_contains "$log" 'rename-window' \
    "hw must not reuse a window without an explicit pane identity"
assert_contains "$log" 'new-window -t =Main: -n Mac-Mini' \
    "hw must create the target without an explicit pane identity"
export TMUX_PANE=%1

mv "$tmp_dir/herdr" "$tmp_dir/herdr.off"
old_path=$PATH
PATH="$tmp_dir:/usr/bin:/bin"
rehash
reset_hw_state
export FAKE_CHILD_PARENT=4200
export FAKE_CHILD_PIDS=5001
export FAKE_CHILD_COMMAND='/usr/local/bin/herdr --remote hydra'
: > "$TEST_LOG"
hw hydra >/dev/null || fail "hw must select a running client without a local Herdr binary"
assert_not_contains "$(<"$TEST_LOG")" 'new-window' \
    "hw must not require Herdr when no launch is needed"

reset_hw_state
: > "$TEST_LOG"
missing=$(hw hydra 2>&1)
rc=$?
assert_eq "$rc" 127 "hw must report a missing Herdr binary when launch is needed"
assert_contains "$missing" 'hw: herdr is not installed' \
    "hw must explain when Herdr is unavailable"
log=$(<"$TEST_LOG")
assert_not_contains "$log" 'send-keys' \
    "hw must not send a missing command"
assert_not_contains "$log" 'new-window' \
    "hw must not create a window for a missing command"
assert_not_contains "$log" 'rename-window' \
    "hw must not rename a window for a missing command"
PATH=$old_path
rehash
mv "$tmp_dir/herdr.off" "$tmp_dir/herdr"
rehash

: > "$TEST_LOG"
usage=$(hw 2>&1)
rc=$?
assert_eq "$rc" 2 "hw without a host must return usage status"
assert_contains "$usage" 'Usage: hw <host>' \
    "hw without a host must explain its usage"
[[ ! -s "$TEST_LOG" ]] || fail "hw must validate arity before invoking tmux"

: > "$TEST_LOG"
hw hydra extra >/dev/null 2>&1
rc=$?
assert_eq "$rc" 2 "hw must reject more than one host"
[[ ! -s "$TEST_LOG" ]] || fail "hw must reject extra arguments before invoking tmux"

: > "$TEST_LOG"
hw 'bad host' >/dev/null 2>&1
rc=$?
assert_eq "$rc" 2 "hw must reject unsafe host names"
[[ ! -s "$TEST_LOG" ]] || fail "hw must reject unsafe names before invoking tmux"

: > "$TEST_LOG"
hw --session >/dev/null 2>&1
rc=$?
assert_eq "$rc" 2 "hw must reject option-looking host names"
[[ ! -s "$TEST_LOG" ]] || fail "hw must reject option-looking names before invoking tmux"
assert_contains "$(<"$COMPDEF_LOG")" 'hw=ssh' \
    "hw must reuse SSH host completion"

: > "$TEST_LOG"
ts >/dev/null || fail "ts inside tmux failed"
assert_contains "$(<"$TEST_LOG")" 'choose-tree -s' \
    "ts must use choose-tree inside tmux"

unset TMUX
unset FAKE_TMUX_HAS_SESSION
export FAKE_FZF_SELECTION=Beta
: > "$TEST_LOG"
ts >/dev/null || fail "ts outside tmux failed"
assert_contains "$(<"$TEST_LOG")" 'attach-session -t =Beta' \
    "ts must attach the selected exact session"

alias cmt >/dev/null 2>&1 && fail "obsolete cmt alias must be removed"

usage=$(hr 2>&1)
rc=$?
assert_eq "$rc" 2 "hr without a host must return usage status"
assert_contains "$usage" 'Usage: hr <host>' \
    "hr without a host must explain its usage"

: > "$TEST_LOG"
hr hydra --session agents >/dev/null || fail "remote Herdr attach failed"
assert_contains "$(<"$TEST_LOG")" '--remote hydra --session agents' \
    "hr must pass remote arguments through to Herdr"
assert_contains "$(<"$COMPDEF_LOG")" 'hr=ssh' \
    "hr must reuse SSH host completion"

: > "$TEST_LOG"
HERDR_ENV=1 hr hydra >/dev/null 2>&1
rc=$?
assert_eq "$rc" 2 "hr must reject a nested launch"
[[ ! -s "$TEST_LOG" ]] || fail "nested hr must not launch Herdr"
unset HERDR_ENV

mv "$tmp_dir/herdr" "$tmp_dir/herdr.off"
old_path=$PATH
PATH=$tmp_dir
rehash
missing=$(hr hydra 2>&1)
rc=$?
assert_eq "$rc" 127 "hr must report a missing Herdr binary"
assert_contains "$missing" 'hr: herdr is not installed' \
    "hr must explain when Herdr is unavailable"
PATH=$old_path
rehash
mv "$tmp_dir/herdr.off" "$tmp_dir/herdr"
rehash

export FAKE_SSH_STATUS=0
: > "$TEST_LOG"
TERM= ssht hydra >/dev/null || fail "default ssht failed"
log=$(<"$TEST_LOG")
assert_contains "$log" 'tmux-session Main' \
    "ssht must default to Main"
assert_contains "$log" 'new-session -A -s Main' \
    "ssht fallback must default to Main"

: > "$TEST_LOG"
TERM= ssht -s SandyClam hydra >/dev/null || fail "named ssht failed"
log=$(<"$TEST_LOG")
assert_contains "$log" 'tmux-session SandyClam' \
    "ssht must pass the named wrapper session"
assert_contains "$log" 'new-session -A -s SandyClam' \
    "ssht fallback must use the named session"

: > "$TEST_LOG"
TERM= ssht -s 'bad;name' hydra >/dev/null 2>&1
rc=$?
[[ $rc -ne 0 ]] || fail "ssht must reject an unsafe session name"
[[ ! -s "$TEST_LOG" ]] || fail "ssht must reject unsafe names before SSH"

export FAKE_SSH_STATUS=23
TERM= ssht hydra >/dev/null
rc=$?
assert_eq "$rc" 23 "ssht must preserve the SSH exit status"

zprofile=$(<"$repo_root/configs/zsh/.zprofile")
assert_not_contains "$zprofile" 'DOTFILES_TMUX_AUTOSTART' \
    "zprofile must not retain the auto-attach override"
assert_not_contains "$zprofile" 'tmux has-session -t Main' \
    "zprofile must not auto-attach tmux"
assert_not_contains "$zprofile" 'exec herdr' \
    "zprofile must not auto-attach Herdr"
[[ ! -e "$repo_root/configs/zsh/local.zprofile.example" ]] || \
    fail "obsolete local.zprofile example still exists"

tmux_conf=$(<"$repo_root/configs/tmux/tmux.conf")
assert_contains "$tmux_conf" 'setw -g window-size largest' \
    "tmux must preserve the largest-client workspace on tmux 2.9 and newer"
assert_contains "$tmux_conf" 'set -g prefix C-Space' \
    "tmux must restore its historical prefix"
assert_contains "$tmux_conf" 'unbind C-b' \
    "tmux must release Herdr's native prefix"
assert_contains "$tmux_conf" 'bind C-Space send-prefix' \
    "tmux must preserve double-prefix passthrough"
assert_contains "$tmux_conf" 'set -g status-position top' \
    "tmux must place the native status bar at the top"
assert_contains "$tmux_conf" 'set -g status on' \
    "tmux reloads must restore a single status row"
assert_not_contains "$tmux_conf" 'set -g status 2' \
    "tmux must not reserve a full spacer row"
assert_not_contains "$tmux_conf" 'status-format[1]' \
    "tmux must not retain the retired spacer-row format"
assert_contains "$tmux_conf" "set -g status-style 'bg=#1a1b26,fg=#a9b1d6'" \
    "tmux must use the TokyoNight status background"
assert_contains "$tmux_conf" "#[fg=#9ece6a,bg=#1a1b26,nobold] '" \
    "tmux must leave a cell after the session pill"
assert_contains "$tmux_conf" '#[fg=#1a1b26,bg=#9ece6a,bold] #S ' \
    "tmux must show the session in a green native pill"
assert_contains "$tmux_conf" '#[fg=#1a1b26,bg=#7aa2f7,bold] #I:#W ' \
    "tmux must show the current stable window name in a blue native pill"
assert_contains "$tmux_conf" \
    "set -g status-right '#[fg=#bb9af7,bg=#1a1b26]#[fg=#1a1b26,bg=#bb9af7,bold] #{b:pane_current_path} @ #{host_short} '" \
    "tmux status must show the current directory and host in a mauve native pill"
assert_contains "$tmux_conf" '|herdr|' \
    "tmux navigator must pass pane movement through Herdr"
assert_contains "$tmux_conf" 'bind -n M-H previous-window' \
    "tmux must retain direct outer previous-window navigation"
assert_contains "$tmux_conf" 'bind -n M-L next-window' \
    "tmux must retain direct outer next-window navigation"
assert_contains "$tmux_conf" "run '~/.tmux/plugins/tpm/tpm'" \
    "tmux must keep synchronous TPM initialization"
assert_not_contains "$tmux_conf" 'tmux-powerkit' \
    "tmux must not declare Powerkit"
assert_not_contains "$tmux_conf" '@powerkit_' \
    "tmux must not retain Powerkit configuration"
assert_not_contains "$tmux_conf" 'prepend-ssh-host' \
    "tmux must not invoke the Powerkit-specific status helper"
assert_not_contains "$tmux_conf" 'ssh-client-host' \
    "tmux must not invoke the retired SSH status subprocess"
[[ ! -e "$repo_root/configs/tmux/scripts/prepend-ssh-host.sh" ]] || \
    fail "Powerkit status prepend helper still exists"
[[ ! -e "$repo_root/configs/tmux/scripts/ssh-client-host.sh" ]] || \
    fail "Powerkit SSH host status helper still exists"

herdr_config=$(<"$repo_root/configs/herdr/config.toml")
assert_contains "$herdr_config" 'prefix = "ctrl+b"' \
    "Herdr must use its native prefix"
assert_contains "$herdr_config" 'previous_tab = ["prefix+p"]' \
    "Herdr previous-tab navigation must remain prefix-owned"
assert_contains "$herdr_config" 'next_tab = ["prefix+n"]' \
    "Herdr next-tab navigation must remain prefix-owned"
assert_not_contains "$herdr_config" 'alt+shift+h' \
    "Herdr must not claim outer previous-window navigation"
assert_not_contains "$herdr_config" 'alt+shift+l' \
    "Herdr must not claim outer next-window navigation"

tmux_cheatsheet=$(<"$repo_root/configs/tmux/scripts/tmux-cheatsheet.sh")
assert_contains "$tmux_cheatsheet" 'prefix = ${GREEN}C-Space' \
    "tmux cheatsheet must document the historical prefix"
assert_not_contains "$tmux_cheatsheet" 'prefix = ${GREEN}C-b' \
    "tmux cheatsheet must not claim Herdr's prefix"

ghostty_config="$repo_root/configs/ghostty/config"
ghostty=$(<"$ghostty_config")
assert_contains "$ghostty" 'adjust-cell-height = 1' \
    "Ghostty must add one pixel of vertical cell padding"
assert_contains "$ghostty" \
    "initial-command = /bin/zsh -lc 'exec ~/.config/tmux/scripts/tmux-main'" \
    "Ghostty's first surface must invoke the tmux Main bootstrap"
grep -Eq '^[[:space:]]*command[[:space:]]*=' "$ghostty_config" \
    && fail "Ghostty command would start Herdr in every surface"

readme=$(<"$repo_root/README.md")
assert_contains "$readme" 'first fresh Ghostty surface' \
    "README must explain first-surface Herdr startup"
assert_contains "$readme" '`Cmd+T` opens a plain shell' \
    "README must preserve plain later Ghostty tabs"
assert_contains "$readme" 'DevBox -> herdr --remote devbox' \
    "README must document the restored DevBox window"
assert_contains "$readme" '`herdr` | Create or attach' \
    "README must document local Herdr attach"
assert_contains "$readme" '`hr hydra`' \
    "README must document remote Herdr attach"
assert_contains "$readme" '`ssh hydra`, then `herdr`' \
    "README must document generic remote Herdr attach"
assert_contains "$readme" '`ssht hydra`' \
    "README must retain the tmux fallback"
assert_contains "$readme" '`Prefix+Shift+G`' \
    "README must document Herdr worktree creation"
assert_contains "$readme" '`.worktree-copy`' \
    "README must document the worktree copy manifest"
assert_contains "${readme:l}" 'branches are retained' \
    "README must document branch retention"
assert_not_contains "${readme:l}" 'cmux' \
    "README must not restore the obsolete cmux workflow"
grep -Eiq '(^|[^[:alnum:]_])cmt([^[:alnum:]_]|$)' "$repo_root/README.md" \
    && fail "README must not restore the obsolete cmt command"

print -- 'PASS: session workflow'
