# Note: the `sudo apt`->`sudo nala` routing now lives in the unified `sudo()`
# wrapper in functions/sudo.zsh (which also routes `sudo nvim <file>` to
# sudoedit). Keep only the `apt` alias here to avoid two `sudo()` definitions.
if command -v nala &>/dev/null; then
  apt() { command nala "$@" }
fi
