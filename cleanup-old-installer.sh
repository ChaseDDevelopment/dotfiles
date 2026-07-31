#!/bin/sh
# One-time cleanup of pre-chezmoi installer remnants on a migrated host.
# Rescues machine-local zsh state out of the old configs/ tree, lists
# everything it would delete, and asks once before deleting. Safe to re-run.

set -eu

repo_root=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)

# Rescue lands in the chezmoi-managed zsh dir; require the migration to have
# actually applied so state is never copied into a dangling symlink.
zsh_dir="${XDG_CONFIG_HOME:-$HOME/.config}/zsh"
if [ -L "$zsh_dir" ] || [ ! -d "$zsh_dir" ]; then
    printf '%s\n' 'cleanup: ~/.config/zsh is not a real directory yet; run chezmoi apply first' >&2
    exit 1
fi

# Machine-local files the old symlink layout kept inside the repo. Never
# overwrite state already living in the new location.
old_zsh="$repo_root/configs/zsh"
if [ -d "$old_zsh" ]; then
    for state_file in local.zsh .zsh_history; do
        src="$old_zsh/$state_file"
        dst="$zsh_dir/$state_file"
        if [ -f "$src" ] && [ ! -e "$dst" ]; then
            cp -p "$src" "$dst"
            printf 'rescued: %s\n' "$dst"
        fi
    done
fi

set --
for candidate in \
    "$repo_root/configs" \
    "$repo_root/installer" \
    "$repo_root/install.log" \
    "$HOME/.local/share/dotsetup"; do
    [ -e "$candidate" ] && set -- "$@" "$candidate"
done
for backup in "$HOME"/.dotfiles-backup-*; do
    [ -e "$backup" ] && set -- "$@" "$backup"
done

if [ "$#" -eq 0 ]; then
    printf '%s\n' 'cleanup: no old-installer remnants found'
    exit 0
fi

printf '%s\n' 'cleanup: will delete:'
printf '  %s\n' "$@"
printf '%s' 'Proceed? [y/N] '
read -r answer
case "$answer" in
    y | Y | yes | YES) ;;
    *)
        printf '%s\n' 'cleanup: aborted; nothing deleted'
        exit 1
        ;;
esac

rm -rf -- "$@"
printf '%s\n' 'cleanup: done'
