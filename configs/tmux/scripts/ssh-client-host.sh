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
# Styling: left-rounded cap on bar-bg → magenta data pill with bold
# hostname → right-rounded cap on bar-bg → reset. Matches the
# TokyoNight Night palette and powerkit's pill aesthetic.
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

# Separator bytes via POSIX-portable printf. `\xHH` hex escapes in
# the format string aren't universally supported — dash (Linux
# /bin/sh on Debian/Ubuntu) emits them literally. `%b` + octal
# escapes IS in POSIX and works in dash, bash, and busybox.
# U+E0B6 (left rounded) = 0xEE 0x82 0xB6 = \0356\0202\0266
# U+E0B4 (right rounded) = 0xEE 0x82 0xB4 = \0356\0202\0264
lcap=$(printf '%b' '\0356\0202\0266')
rcap=$(printf '%b' '\0356\0202\0264')

printf '#[fg=#bb9af7,bg=default]%s#[fg=#1a1b26,bg=#bb9af7,bold] %s #[fg=#bb9af7,bg=default,nobold]%s#[default]' "$lcap" "$host" "$rcap"
