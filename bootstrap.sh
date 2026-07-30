#!/bin/sh

set -eu

command -v curl >/dev/null 2>&1 || {
    printf '%s\n' 'bootstrap: curl is required' >&2
    exit 1
}

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)

if command -v chezmoi >/dev/null 2>&1; then
    chezmoi_bin=$(command -v chezmoi)
else
    install_dir="${HOME}/.local/bin"
    mkdir -p "$install_dir"
    sh -c "$(curl --proto '=https' --proto-redir '=https' -fsLS https://get.chezmoi.io)" -- -b "$install_dir"
    chezmoi_bin="$install_dir/chezmoi"
fi

"$chezmoi_bin" -S "$repo_root" init

printf '%s\n' \
    'Source initialized. Review before changing the home directory:' \
    '  chezmoi diff' \
    '  chezmoi apply --dry-run --verbose'
