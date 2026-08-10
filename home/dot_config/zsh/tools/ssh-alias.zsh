# The connecting ssh client sends the host alias it used (ssh config:
# `SetEnv LC_SSH_ALIAS=%n`); the prompt shows it as this machine's label.
# Tmux server environments and later local shells lose the original ssh
# environment, so persist the alias to a state file and restore it there.

_ssh_alias_state="${XDG_STATE_HOME:-$HOME/.local/state}/ssh-alias"
if [[ -n ${LC_SSH_ALIAS:-} ]]; then
    if [[ ! -r "$_ssh_alias_state" || "$(<"$_ssh_alias_state")" != "$LC_SSH_ALIAS" ]]; then
        mkdir -p "${_ssh_alias_state:h}" 2>/dev/null &&
            print -r -- "$LC_SSH_ALIAS" > "$_ssh_alias_state"
    fi
elif [[ -r "$_ssh_alias_state" ]]; then
    export LC_SSH_ALIAS="$(<"$_ssh_alias_state")"
fi
unset _ssh_alias_state
