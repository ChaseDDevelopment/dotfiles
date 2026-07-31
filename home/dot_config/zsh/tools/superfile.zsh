# Superfile file manager - cd-on-quit wrapper (adapted from upstream
# cd_on_quit/cd_on_quit.sh): quitting spf leaves the shell in the last
# visited directory. Requires cd_on_quit = true in the managed config.
# Upstream's macOS branch reads Application Support, but zshenv exports the
# XDG vars, which superfile honors on every OS - so lastdir is always under
# XDG_STATE_HOME.
if (( $+commands[spf] )); then
    function spf() {
        local last_dir="${XDG_STATE_HOME:-$HOME/.local/state}/superfile/lastdir"

        command spf "$@"

        [ ! -f "$last_dir" ] || {
            . "$last_dir"
            rm -f -- "$last_dir" >/dev/null
        }
    }

    alias y='spf'
fi
