# Make muscle-memory `sudo nvim <file>` do the same secure thing as `snvim`
# (sudoedit): the editor runs as YOU (full plugins + LSP), the result is written
# back as root. Directories, nvim flags, or no file fall through to the real
# `sudo nvim` (sudoedit can't open dirs/flags) — directory browsing as root just
# gets raw config, which we accept; use `nvim <dir>` without sudo for a
# full-config read-only browse of world-readable trees. Non-editor commands pass
# straight through, with the existing `apt`->`nala` convenience preserved on
# Debian hosts.
sudo() {
  emulate -L zsh
  local -a editors=(nvim vim vi)
  if (( ${editors[(Ie)$1]} )); then
    shift
    (( $# )) || { command sudo nvim; return; }                  # no file
    local a divert=1
    for a in "$@"; do
      [[ "$a" == [-+]* || -d "$a" ]] && { divert=0; break; }     # flag or directory
    done
    if (( divert )); then sudoedit -- "$@"; else command sudo nvim "$@"; fi
    return
  fi
  if [[ "$1" == apt ]] && command -v nala >/dev/null 2>&1; then
    shift; command sudo nala "$@"; return
  fi
  command sudo "$@"
}
