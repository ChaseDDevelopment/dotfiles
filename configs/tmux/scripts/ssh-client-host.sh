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

# Two-tone rounded pill mirroring powerkit's window-list entries.
# Left section (lighter #b096df) holds an `@` glyph so the pill
# reads as "user@host" SSH notation; right section (darker #9d7cd8)
# carries the hostname. Inner chevron (U+E0B6) transitions between
# the two sections — the same glyph powerkit uses for its
# number→name transition inside each window entry. Caps on both
# ends return to bar-bg.
#
# All fg/bg pairs lifted verbatim from powerkit's
# window-status-current-format so the pill reads as part of the
# window family instead of a lone annotation.
#
# Separator bytes via POSIX-portable `printf '%b'` + octal escapes.
# Hex `\xHH` in the format string isn't parsed by dash (Linux
# /bin/sh on Debian/Ubuntu); `%b` + `\0NNN` works in bash, dash,
# and busybox.
# U+E0B6 (left rounded / inner chevron) = 0xEE 0x82 0xB6 = \0356\0202\0266
# U+E0B4 (right rounded) = 0xEE 0x82 0xB4 = \0356\0202\0264
lcap=$(printf '%b' '\0356\0202\0266')
rcap=$(printf '%b' '\0356\0202\0264')
printf '#[fg=#b096df,bg=default]%s#[fg=#453760,bg=#b096df,bold] @ #[fg=#b096df,bg=#9d7cd8]%s#[fg=#453760,bg=#9d7cd8,bold] %s #[fg=#9d7cd8,bg=default,nobold]%s#[default]' "$lcap" "$lcap" "$host" "$rcap"
