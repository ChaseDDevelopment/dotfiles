#!/bin/sh
# Print the short hostname if the tmux client PID ($1) has sshd anywhere
# in its process ancestry; print nothing otherwise. Used in tmux
# status-right so only SSH-attached clients see the hostname — local
# clients attached to the same session get a clean status bar.
pid="$1"
while [ -n "$pid" ] && [ "$pid" -gt 1 ]; do
    comm=$(ps -o comm= -p "$pid" 2>/dev/null) || break
    case "$comm" in *sshd*) hostname -s; exit 0 ;; esac
    pid=$(ps -o ppid= -p "$pid" 2>/dev/null | tr -d ' ')
done
