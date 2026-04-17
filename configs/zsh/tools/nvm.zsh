# Node Version Manager (lazy-loaded for fast shell startup)
export NVM_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/nvm"

# Lazy-load NVM: define placeholder functions that load NVM on first use
_lazy_load_nvm() {
    unset -f nvm node npm npx yarn pnpm 2>/dev/null
    [ -s "$NVM_DIR/nvm.sh" ] && \. "$NVM_DIR/nvm.sh"
    [ -s "$NVM_DIR/bash_completion" ] && \. "$NVM_DIR/bash_completion"
}

nvm()  { _lazy_load_nvm; nvm "$@"; }
node() { _lazy_load_nvm; node "$@"; }
npm()  { _lazy_load_nvm; npm "$@"; }
npx()  { _lazy_load_nvm; npx "$@"; }
yarn() { _lazy_load_nvm; yarn "$@"; }
pnpm() { _lazy_load_nvm; pnpm "$@"; }
