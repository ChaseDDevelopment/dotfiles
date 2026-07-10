# Auto-list directory contents on every `cd` (zsh chpwd hook).
# `la` is expanded to the eza command at parse time — aliases/*.zsh is
# sourced before functions/*.zsh in .zshrc, so the alias already exists.
autoload -Uz add-zsh-hook

_auto_ls() { la }
add-zsh-hook chpwd _auto_ls
