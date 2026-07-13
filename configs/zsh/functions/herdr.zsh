function hr() {
    if (( $# == 0 )); then
        echo "Usage: hr <host> [herdr-remote-options...]"
        return 2
    fi
    if ! (( $+commands[herdr] )); then
        echo "hr: herdr is not installed"
        return 127
    fi
    if [[ "${HERDR_ENV:-}" == "1" ]]; then
        echo "hr: open a plain Ghostty tab first; nested Herdr is intentionally disabled"
        return 2
    fi
    command herdr --remote "$@"
}
compdef hr=ssh
