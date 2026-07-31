# Superfile file manager - upstream cd-on-quit wrapper (yorukot/superfile
# cd_on_quit/cd_on_quit.sh): quitting spf leaves the shell in the last
# visited directory.
if (( $+commands[spf] )); then
    function spf() {
        if [[ "$(uname -s)" == "Darwin" ]]; then
            export SPF_LAST_DIR="$HOME/Library/Application Support/superfile/lastdir"
        else
            export SPF_LAST_DIR="${XDG_STATE_HOME:-$HOME/.local/state}/superfile/lastdir"
        fi

        command spf "$@"

        [ ! -f "$SPF_LAST_DIR" ] || {
            . "$SPF_LAST_DIR"
            rm -f -- "$SPF_LAST_DIR" >/dev/null
        }
    }

    alias y='spf'
fi
