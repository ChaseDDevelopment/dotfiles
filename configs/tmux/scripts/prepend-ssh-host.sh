#!/bin/sh
# Prepend the SSH hostname pill invocation to tmux's status-left
# after powerkit has set it. Idempotent — safe on config reload.
#
# Why prepend instead of append: powerkit emits a session→windows
# chevron at the end of status-left, and that chevron has bg =
# first_window_index_bg (lavender). Inserting the hostname pill
# between session and windows puts it right next to that lavender
# cell, leaving a visible purple sliver on the pill's left side
# regardless of how we style our own entry chevron. Leftmost
# placement sidesteps the problem — the pill gets a clean LCAP on
# bar-bg and exits into the session pill's own bg.
#
# Detects both the current (prepended) and legacy (appended)
# layouts so a user transitioning from `set -ag` won't end up
# with duplicate pills after a source-file.
pfx='#(~/.config/tmux/scripts/ssh-client-host.sh #{client_pid})'

sl=$(tmux show-option -gv status-left 2>/dev/null || printf '')

# Strip any existing instance (legacy append or prior prepend)
case "$sl" in
    "$pfx"*) sl=${sl#"$pfx"} ;;
esac
case "$sl" in
    *"$pfx") sl=${sl%"$pfx"} ;;
esac

tmux set-option -g status-left "$pfx$sl"
