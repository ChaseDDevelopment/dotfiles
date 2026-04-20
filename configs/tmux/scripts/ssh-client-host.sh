#!/bin/sh
# Emit a styled tmux status pill naming THIS machine's hostname,
# but only when the calling tmux client's process ancestry contains
# sshd — i.e. the client is viewing this tmux via SSH. Console-
# attached clients (no sshd in the ancestry) get a clean status bar.
#
# Invoked from status-left with #{client_pid} — tmux renders status
# formats independently per attached client, so each client's bar
# decision is made from its own PID walk.
#
# Styling: single-tone blue pill at the leftmost position of the
# bar (prepended to status-left by prepend-ssh-host.sh). LCAP on
# bar-bg, exit chevron tracks powerkit's session-bg conditional so
# the pill flows cleanly into the green Main pill on its right
# without colliding with the session→windows chevron.
client_pid="$1"
[ -n "$client_pid" ] || exit 0

pid="$client_pid"
is_ssh=""
while [ -n "$pid" ] && [ "$pid" -gt 1 ]; do
    comm=$(ps -o comm= -p "$pid" 2>/dev/null) || break
    case "$comm" in *sshd*) is_ssh=1; break ;; esac
    pid=$(ps -o ppid= -p "$pid" 2>/dev/null | tr -d ' ')
done
[ -n "$is_ssh" ] || exit 0

host=$(hostname -s 2>/dev/null)
[ -n "$host" ] || exit 0

# Pill bg/fg are static TokyoNight blue + bg-dark; the pill reads
# distinctly against powerkit's purple active-window and green
# session pills instead of mimicking one of them. High-contrast
# bold text mirrors the session pill's styling convention.
host_bg='#7aa2f7'
host_fg='#1a1b26'

# session_bg tracks powerkit's session pill color via the same
# conditional session_build_bg_condition uses
# (~/.tmux/plugins/tmux-powerkit/src/renderer/entities/session.sh).
# Our exit chevron lands on this bg so its right edge matches the
# session pill's leading cell. Prefix mode → #e0af68, copy mode
# → #7dcfff, normal → #9ece6a.
session_bg='#{?client_prefix,#e0af68,#{?pane_in_mode,#7dcfff,#9ece6a}}'

# U+E0B6 (rounded left, POWERKIT_SEP_ROUND_LEFT)  for the LCAP
# U+E0B4 (rounded right, POWERKIT_SEP_ROUND_RIGHT) for the exit
# chevron — same glyphs powerkit uses for bar-edge caps. Emit via
# POSIX-portable `printf '%b'` + octal so dash renders the bytes;
# hex `\xHH` in the format string is unparsed by dash.
# U+E0B6 = 0xEE 0x82 0xB6 = \0356\0202\0266
# U+E0B4 = 0xEE 0x82 0xB4 = \0356\0202\0264
lcap=$(printf '%b' '\0356\0202\0266')
rcap=$(printf '%b' '\0356\0202\0264')

# Pill renders at the LEFTMOST position of the bar (prepended to
# powerkit's status-left by prepend-ssh-host.sh). LCAP enters from
# bar-bg → blue, pill content on blue, exit chevron transitions
# blue → session_bg so the session pill's leading cell matches.
# The session pill handles its own transition to the window list
# via powerkit's native session→windows chevron.
printf '#[fg=%s,bg=default]%s#[fg=%s,bg=%s,bold] @ %s #[fg=%s,bg=%s,nobold]%s#[default]' \
    "$host_bg" "$lcap" \
    "$host_fg" "$host_bg" "$host" \
    "$host_bg" "$session_bg" "$rcap"
