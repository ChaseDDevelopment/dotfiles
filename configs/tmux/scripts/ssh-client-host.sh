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
# Styling: single-tone blue pill chained into powerkit's window
# family via entry + exit chevrons (see styling block below). The
# chevrons track window 1's active/inactive state so both cell
# boundaries land on whatever bg powerkit's session→windows
# chevron produces — no dark slivers around the pill.
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

# first_win_bg tracks window 1's state via the same conditional
# powerkit uses in windows_get_first_bg
# (~/.tmux/plugins/tmux-powerkit/src/renderer/entities/windows.sh).
# It's the bg powerkit's session→windows chevron lands on AND the
# bg the first window's leading index cell starts on — so our
# entry chevron consumes that bg on its left, and our exit
# chevron produces that bg on its right. Active lighter #b096df
# or inactive lighter #626880.
first_win_bg='#{?#{==:#{active_window_index},#{base-index}},#b096df,#626880}'

# U+E0B4 (rounded right, POWERKIT_SEP_ROUND_RIGHT) — same glyph
# powerkit's window format uses for its right-facing chevrons.
# Emit via POSIX-portable `printf '%b'` + octal so dash renders
# the bytes; hex `\xHH` in the format string is unparsed by dash.
# U+E0B4 = 0xEE 0x82 0xB4 = \0356\0202\0264
sep=$(printf '%b' '\0356\0202\0264')

# Entry chevron (first_win_bg → blue) ◀ pill content ▶ exit
# chevron (blue → first_win_bg). The exit chevron IS the
# hostname→windows separator: powerkit skips its own leading
# separator for the first window, so the window's leading cell
# starts on first_win_bg — matching the right edge of our exit
# chevron.
printf '#[fg=%s,bg=%s]%s#[fg=%s,bg=%s,bold] @ %s #[fg=%s,bg=%s,nobold]%s#[default]' \
    "$first_win_bg" "$host_bg" "$sep" \
    "$host_fg" "$host_bg" "$host" \
    "$host_bg" "$first_win_bg" "$sep"
