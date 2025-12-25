# Bun JavaScript runtime
export BUN_INSTALL="$HOME/.bun"

# bun completions (conditional)
[ -s "$BUN_INSTALL/_bun" ] && source "$BUN_INSTALL/_bun"

# Add bun to path if installed
if [[ -d "$BUN_INSTALL/bin" ]]; then
    export PATH="$BUN_INSTALL/bin:$PATH"
fi
