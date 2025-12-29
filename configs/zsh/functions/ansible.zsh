# Ansible wrapper with configurable per-inventory credentials
# Usage: ans <inventory> <playbook> [--user USER] [--key KEY] [ansible-args...]
#        ans config <inventory> [--user USER] [--key KEY]  # Configure defaults
#        ans config --list                                  # Show all configs

# Only load on workstations (not servers)
_is_workstation() {
    [[ "$OSTYPE" == darwin* ]] && return 0                                      # macOS
    [[ -f /etc/arch-release ]] && return 0                                      # Arch Linux
    [[ -f /proc/version ]] && grep -qi microsoft /proc/version && return 0     # WSL
    return 1
}
_is_workstation || return 0

ANSIBLE_DIR="${HOME}/ansible"
ANS_CONFIG_FILE="${XDG_CONFIG_HOME:-$HOME/.config}/ans/config"

# Ensure config directory exists
[[ -d "${ANS_CONFIG_FILE:h}" ]] || mkdir -p "${ANS_CONFIG_FILE:h}"

# Load config into associative arrays
typeset -A ANS_USERS ANS_KEYS
_ans_load_config() {
    ANS_USERS=() ANS_KEYS=()
    [[ -f "$ANS_CONFIG_FILE" ]] || return
    while IFS='=' read -r key value; do
        case "$key" in
            *_user) ANS_USERS[${key%_user}]="$value" ;;
            *_key)  ANS_KEYS[${key%_key}]="$value" ;;
        esac
    done < "$ANS_CONFIG_FILE"
}

# Save config
_ans_save_config() {
    local inv="$1" user="$2" key="$3"
    # Load existing, update, save
    _ans_load_config
    [[ -n "$user" ]] && ANS_USERS[$inv]="$user"
    [[ -n "$key" ]]  && ANS_KEYS[$inv]="$key"

    # Write all configs
    : > "$ANS_CONFIG_FILE"
    for inv in ${(k)ANS_USERS}; do
        echo "${inv}_user=${ANS_USERS[$inv]}" >> "$ANS_CONFIG_FILE"
    done
    for inv in ${(k)ANS_KEYS}; do
        echo "${inv}_key=${ANS_KEYS[$inv]}" >> "$ANS_CONFIG_FILE"
    done
}

ans() {
    _ans_load_config

    # Handle config subcommand
    if [[ "$1" == "config" ]]; then
        shift
        if [[ "$1" == "--list" ]]; then
            echo "Configured inventories:"
            for inv in ${(k)ANS_USERS}; do
                echo "  $inv: user=${ANS_USERS[$inv]:-<none>} key=${ANS_KEYS[$inv]:-<none>}"
            done
            return 0
        fi

        local inv="$1"; shift
        [[ -z "$inv" ]] && { echo "Usage: ans config <inventory> [--user USER] [--key KEY]"; return 1; }

        local user="" key=""
        while [[ $# -gt 0 ]]; do
            case "$1" in
                --user) user="$2"; shift 2 ;;
                --key)  key="${2/#\~/$HOME}"; shift 2 ;;  # Expand ~
                *) echo "Unknown option: $1"; return 1 ;;
            esac
        done

        _ans_save_config "$inv" "$user" "$key"
        echo "Configured $inv: user=${user:-${ANS_USERS[$inv]:-<unchanged>}} key=${key:-${ANS_KEYS[$inv]:-<unchanged>}}"
        return 0
    fi

    # Normal playbook execution
    local inventory="$1" playbook="$2"
    shift 2 2>/dev/null || { echo "Usage: ans <inventory> <playbook> [--user USER] [--key KEY] [args...]"; return 1; }

    # Parse optional overrides
    local user="" key="" args=()
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --user) user="$2"; shift 2 ;;
            --key)  key="$2"; shift 2 ;;
            *)      args+=("$1"); shift ;;
        esac
    done

    # Validate inventory exists
    local inv_file="${ANSIBLE_DIR}/inventory/${inventory}"
    [[ -f "$inv_file" ]] || { echo "Inventory not found: $inv_file"; return 1; }

    # Find playbook
    local pb_file="${ANSIBLE_DIR}/playbooks/${playbook}"
    [[ -f "$pb_file" ]] || pb_file="${ANSIBLE_DIR}/playbooks/${playbook}.yml"
    [[ -f "$pb_file" ]] || pb_file="${ANSIBLE_DIR}/playbooks/${playbook}.yaml"
    [[ -f "$pb_file" ]] || { echo "Playbook not found: ${playbook}"; return 1; }

    # Build command
    local cmd=(ansible-playbook "$pb_file" -i "$inv_file")

    # Use override > config > nothing
    local final_user="${user:-${ANS_USERS[$inventory]}}"
    local final_key="${key:-${ANS_KEYS[$inventory]}}"

    [[ -n "$final_user" ]] && cmd+=(--user "$final_user")
    [[ -n "$final_key" ]]  && cmd+=(--private-key "$final_key")

    cmd+=("${args[@]}")
    "${cmd[@]}"
}

# Tab completion
_ans() {
    local curcontext="$curcontext" state line
    typeset -A opt_args

    _arguments -C \
        '1:command:->command' \
        '*::arg:->args'

    case $state in
        command)
            local -a commands inventories
            commands=(config)
            inventories=(${ANSIBLE_DIR}/inventory/*(N:t))
            _describe -t commands 'command' commands
            _describe -t inventories 'inventory' inventories
            ;;
        args)
            case ${line[1]} in
                config)
                    _arguments -C \
                        '1:inventory:->inv' \
                        '--user[SSH user]:user:' \
                        '--key[SSH private key]:key:_files' \
                        '--list[List all configs]'
                    if [[ $state == inv ]]; then
                        local inventories=(${ANSIBLE_DIR}/inventory/*(N:t))
                        _describe 'inventory' inventories
                    fi
                    ;;
                *)
                    # inventory was given, complete playbook then options
                    _arguments -C \
                        '1:playbook:->pb' \
                        '--user[SSH user]:user:' \
                        '--key[SSH private key]:key:_files' \
                        '*:ansible args:'
                    if [[ $state == pb ]]; then
                        local playbooks=(${ANSIBLE_DIR}/playbooks/*.yml(N:t:r) ${ANSIBLE_DIR}/playbooks/*.yaml(N:t:r))
                        _describe 'playbook' playbooks
                    fi
                    ;;
            esac
            ;;
    esac
}

compdef _ans ans
