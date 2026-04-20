#!/bin/sh
# Emit a styled tmux status segment when the pane has an ssh
# descendant, or nothing when it doesn't. Used in status-right so
# the LOCAL tmux surfaces the remote hostname as a pill that
# visually matches powerkit's cpu/memory/battery segments; nested
# tmuxes on panes not running ssh emit nothing and keep their bars
# clean.
#
# Styling uses TokyoNight Night palette purple (icon-bg #c3a6f8,
# data-bg #bb9af7 == tmux-powerkit magenta) with a rounded
# left-separator (Nerd Font powerline extras U+E0B6). Dark base
# #1a1b26 for text keeps contrast consistent with powerkit.
pane_pid="$1"
[ -n "$pane_pid" ] || exit 0

# Fast path: direct descendants. Skip arg-parse entirely if no
# ssh is in the pane's tree so status refreshes stay cheap.
ssh_pid=""
for pid in $(pgrep -P "$pane_pid" 2>/dev/null); do
    if [ "$(ps -o comm= -p "$pid" 2>/dev/null)" = "ssh" ]; then
        ssh_pid="$pid"
        break
    fi
done
[ -n "$ssh_pid" ] || exit 0

# Walk the ssh argv, skipping flags and their values, and capture
# the first positional argument — the host. Strip user@ and :port.
host=$(ps -o args= -p "$ssh_pid" 2>/dev/null | awk '{
    for (i = 2; i <= NF; i++) {
        if (skip) { skip = 0; continue }
        a = $i
        if (a == "-o" || a == "-i" || a == "-l" || a == "-p" ||
            a == "-F" || a == "-L" || a == "-R" || a == "-D" ||
            a == "-J" || a == "-w" || a == "-b" || a == "-c" ||
            a == "-e" || a == "-m" || a == "-S" || a == "-W" ||
            a == "-Q" || a == "-E" || a == "-I") { skip = 1; continue }
        if (substr(a, 1, 1) == "-") continue
        sub(/^.*@/, "", a)
        sub(/:.*/, "", a)
        if (a != "") { print a; exit }
    }
}')
[ -n "$host" ] || exit 0

# Styled pill: left-rounded cap (U+E0B6) → icon pill (server glyph)
# → inner-rounded sep → data pill (host name) → right-rounded cap
# (U+E0B4) → reset. Caps on both ends yield a fully rounded pill.
#
# The opening separator uses bg=#7dcfff — powerkit's battery
# data-bg — so its flat left edge visually fuses with battery's
# right edge and there's no bar-bg gap between the two pills.
# Assumes battery is the last powerkit segment; on a host without
# a battery (desktop mini) the preceding segment is memory
# (data-bg #394b70) and this one cell will appear light-blue —
# acceptable for an outbound-ssh indicator that's primarily meant
# for the MacBook/laptop flow.
printf '#[fg=#c3a6f8,bg=#7dcfff]\xee\x82\xb6#[fg=#1a1b26,bg=#c3a6f8] \xef\x88\xb3 #[fg=#bb9af7,bg=#c3a6f8]\xee\x82\xb6#[fg=#1a1b26,bg=#bb9af7] %s #[fg=#bb9af7,bg=default]\xee\x82\xb4#[default]' "$host"
