# Bun JavaScript runtime
export BUN_INSTALL="$HOME/.bun"

# bun completions (conditional)
[ -s "$BUN_INSTALL/_bun" ] && source "$BUN_INSTALL/_bun"

# PATH is set in .zshenv, no need to duplicate here
