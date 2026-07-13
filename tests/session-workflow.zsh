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
if [[ "$1" == display-message ]]; then
    exit_status="${FAKE_TMUX_DISPLAY_STATUS:-0}"
    (( exit_status == 0 )) || exit "$exit_status"
    print -r -- "${FAKE_TMUX_SESSION:-Main}"
fi
if [[ "$1" == select-window ]]; then
    exit "${FAKE_TMUX_SELECT_STATUS:-1}"
fi' > "$tmp_dir/tmux"

print -r -- '#!/usr/bin/env zsh
print -r -- "${FAKE_FZF_SELECTION:-Beta}"' > "$tmp_dir/fzf"

print -r -- '#!/usr/bin/env zsh
print -r -- "$*" >> "$TEST_LOG"
exit "${FAKE_SSH_STATUS:-0}"' > "$tmp_dir/ssh"

print -r -- '#!/usr/bin/env zsh
print -r -- "$*" >> "$TEST_LOG"' > "$tmp_dir/herdr"

chmod +x "$tmp_dir"/{tmux,fzf,ssh,herdr}
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
"$tmux_main" >/dev/null || fail "tmux Main bootstrap failed"
log=$(<"$TEST_LOG")
assert_contains "$log" 'new-session -A -s Main -n macbook' \
    "tmux Main bootstrap must attach or create the named session and window"
assert_contains "$log" \
    "/bin/zsh -lc 'command -v herdr >/dev/null 2>&1 && herdr; exec /bin/zsh -l'" \
    "tmux Main bootstrap must start Herdr with a login-shell fallback"

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
export FAKE_TMUX_SELECT_STATUS=1
HERDR_ENV=1 hw hydra >/dev/null || fail "hw from a Herdr pane failed"
log=$(<"$TEST_LOG")
assert_contains "$log" 'display-message -p -t =Main: #{session_name}' \
    "hw must resolve local Main from a Herdr pane"
assert_contains "$log" 'new-window -t =Main: -n hydra' \
    "hw from a Herdr pane must create the outer Main host window"
unset HERDR_ENV

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
assert_contains "$log" 'display-message -p #{session_name}' \
    "tw must resolve the current outer session"
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

export FAKE_TMUX_SELECT_STATUS=0
: > "$TEST_LOG"
hw hydra >/dev/null || fail "hw existing window failed"
log=$(<"$TEST_LOG")
assert_contains "$log" 'select-window -t =Main:=hydra' \
    "hw must select the exact existing host window"
assert_not_contains "$log" 'new-window' \
    "hw must not recreate an existing host window"

export FAKE_TMUX_SELECT_STATUS=1
: > "$TEST_LOG"
hw hydra >/dev/null || fail "hw new window failed"
log=$(<"$TEST_LOG")
assert_contains "$log" 'select-window -t =Main:=hydra' \
    "hw must check for the exact host window before creating it"
assert_contains "$log" 'new-window -t =Main: -n hydra' \
    "hw must create a named window in Main"
assert_contains "$log" \
    "/bin/zsh -lc 'herdr --remote hydra; exec /bin/zsh -l'" \
    "hw must run remote Herdr with a login-shell fallback"

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
assert_contains "$tmux_conf" 'set -g prefix C-b' \
    "tmux must restore its outer prefix"
assert_contains "$tmux_conf" 'unbind C-Space' \
    "tmux must release Herdr's prefix"
assert_contains "$tmux_conf" 'bind C-b send-prefix' \
    "tmux must preserve the standard double-prefix passthrough"
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
assert_contains "$herdr_config" 'prefix = "ctrl+space"' \
    "Herdr must retain its own prefix"
assert_contains "$herdr_config" 'previous_tab = ["prefix+p"]' \
    "Herdr previous-tab navigation must remain prefix-owned"
assert_contains "$herdr_config" 'next_tab = ["prefix+n"]' \
    "Herdr next-tab navigation must remain prefix-owned"
assert_not_contains "$herdr_config" 'alt+shift+h' \
    "Herdr must not claim outer previous-window navigation"
assert_not_contains "$herdr_config" 'alt+shift+l' \
    "Herdr must not claim outer next-window navigation"

tmux_cheatsheet=$(<"$repo_root/configs/tmux/scripts/tmux-cheatsheet.sh")
assert_contains "$tmux_cheatsheet" 'prefix = ${GREEN}C-b' \
    "tmux cheatsheet must document the restored prefix"
assert_not_contains "$tmux_cheatsheet" 'C-Space' \
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
