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
# Styling: two-tone pill chained into powerkit's window family
# (see styling block below). Colors track window 1's active/
# inactive state so both cell boundaries (session→hostname and
# hostname→windows) land on matching backgrounds — no dark
# slivers around the pill.
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

# Two-tone pill chained into powerkit's window family. No explicit
# end caps: the pill's opening cell starts on the same bg that
# powerkit's session→windows chevron lands on, and the closing
# chevron transitions to the bg that the first window's leading
# cell starts on — so every cell boundary is color-matched and
# no dark slivers appear on either side of the pill.
#
# Colors track window 1's active/inactive state via the same
# conditional powerkit uses in windows_get_first_bg (see
# ~/.tmux/plugins/tmux-powerkit/src/renderer/entities/windows.sh).
# Active: lighter #b096df / base #9d7cd8 / contrast #453760.
# Inactive: lighter #626880 / base #3b4261 / contrast #d9dae0.
idx_bg='#{?#{==:#{active_window_index},#{base-index}},#b096df,#626880}'
idx_fg='#{?#{==:#{active_window_index},#{base-index}},#453760,#d9dae0}'
content_bg='#{?#{==:#{active_window_index},#{base-index}},#9d7cd8,#3b4261}'

# U+E0B4 (rounded right, POWERKIT_SEP_ROUND_RIGHT) — same glyph
# powerkit's window format uses for its index→content chevron.
# Emit via POSIX-portable `printf '%b'` + octal so dash renders
# the bytes; hex `\xHH` in the format string is unparsed by dash.
# U+E0B4 = 0xEE 0x82 0xB4 = \0356\0202\0264
sep=$(printf '%b' '\0356\0202\0264')

# Index ( @ ) → inner chevron → content (hostname) → exit chevron
# back to idx_bg. The exit chevron IS the hostname→windows
# separator: powerkit skips its own leading separator for the
# first window, and the first window's leading cell starts on
# idx_bg, so the two sides of that boundary match.
printf '#[fg=%s,bg=%s,bold] @ #[fg=%s,bg=%s]%s#[fg=%s,bg=%s,bold] %s #[fg=%s,bg=%s,nobold]%s#[default]' \
    "$idx_fg" "$idx_bg" \
    "$idx_bg" "$content_bg" "$sep" \
    "$idx_fg" "$content_bg" "$host" \
    "$content_bg" "$idx_bg" "$sep"
