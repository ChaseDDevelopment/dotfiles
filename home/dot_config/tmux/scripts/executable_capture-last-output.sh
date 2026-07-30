#!/usr/bin/env zsh
# Capture the last command's output and copy to clipboard via OSC 52.
# Finds prompt boundaries by grepping for ❯ in capture-pane output.

tmp=$(mktemp)
trap "rm -f $tmp" EXIT
tmux capture-pane -p -S - > "$tmp"

# Find line numbers of all prompt lines containing ❯
markers=(${(f)"$(grep -n '❯' "$tmp" | cut -d: -f1)"})

if (( ${#markers} < 2 )); then
    tmux display-message "Need 2+ prompts to capture"
    exit 0
fi

# Last two prompts bracket the most recent command output
prev=${markers[-2]}
curr=${markers[-1]}

# Output starts after the previous ❯ line
# Output ends before the current prompt's status bar (1 line above ❯)
start=$(( prev + 1 ))
end=$(( curr - 2 ))

if (( start > end )); then
    tmux display-message "Last command had no output"
    exit 0
fi

output=$(sed -n "${start},${end}p" "$tmp")

if [[ -n "$output" && -n "${output//[[:space:]]/}" ]]; then
    printf '%s' "$output" | tmux load-buffer -w -
    tmux display-message "Copied $(( end - start + 1 )) lines!"
else
    tmux display-message "Last command had no output"
fi
