fpath+=( "$HOME/Library/Caches/antidote/https-COLON--SLASH--SLASH-github.com-SLASH-zsh-users-SLASH-zsh-completions" )
source "$HOME/Library/Caches/antidote/https-COLON--SLASH--SLASH-github.com-SLASH-zsh-users-SLASH-zsh-completions/zsh-completions.plugin.zsh"
source "$HOME/Library/Caches/antidote/https-COLON--SLASH--SLASH-github.com-SLASH-ohmyzsh-SLASH-ohmyzsh/lib/completion.zsh"
source "$HOME/Library/Caches/antidote/https-COLON--SLASH--SLASH-github.com-SLASH-ohmyzsh-SLASH-ohmyzsh/lib/history.zsh"
source "$HOME/Library/Caches/antidote/https-COLON--SLASH--SLASH-github.com-SLASH-ohmyzsh-SLASH-ohmyzsh/lib/key-bindings.zsh"
fpath+=( "$HOME/Library/Caches/antidote/https-COLON--SLASH--SLASH-github.com-SLASH-ohmyzsh-SLASH-ohmyzsh/plugins/git" )
source "$HOME/Library/Caches/antidote/https-COLON--SLASH--SLASH-github.com-SLASH-ohmyzsh-SLASH-ohmyzsh/plugins/git/git.plugin.zsh"
fpath+=( "$HOME/Library/Caches/antidote/https-COLON--SLASH--SLASH-github.com-SLASH-ohmyzsh-SLASH-ohmyzsh/plugins/docker" )
source "$HOME/Library/Caches/antidote/https-COLON--SLASH--SLASH-github.com-SLASH-ohmyzsh-SLASH-ohmyzsh/plugins/docker/docker.plugin.zsh"
fpath+=( "$HOME/Library/Caches/antidote/https-COLON--SLASH--SLASH-github.com-SLASH-ohmyzsh-SLASH-ohmyzsh/plugins/kubectl" )
source "$HOME/Library/Caches/antidote/https-COLON--SLASH--SLASH-github.com-SLASH-ohmyzsh-SLASH-ohmyzsh/plugins/kubectl/kubectl.plugin.zsh"
fpath+=( "$HOME/Library/Caches/antidote/https-COLON--SLASH--SLASH-github.com-SLASH-Aloxaf-SLASH-fzf-tab" )
source "$HOME/Library/Caches/antidote/https-COLON--SLASH--SLASH-github.com-SLASH-Aloxaf-SLASH-fzf-tab/fzf-tab.plugin.zsh"
fpath+=( "$HOME/Library/Caches/antidote/https-COLON--SLASH--SLASH-github.com-SLASH-zsh-users-SLASH-zsh-autosuggestions" )
source "$HOME/Library/Caches/antidote/https-COLON--SLASH--SLASH-github.com-SLASH-zsh-users-SLASH-zsh-autosuggestions/zsh-autosuggestions.plugin.zsh"
fpath+=( "$HOME/Library/Caches/antidote/https-COLON--SLASH--SLASH-github.com-SLASH-zsh-users-SLASH-zsh-history-substring-search" )
source "$HOME/Library/Caches/antidote/https-COLON--SLASH--SLASH-github.com-SLASH-zsh-users-SLASH-zsh-history-substring-search/zsh-history-substring-search.plugin.zsh"
fpath+=( "$HOME/Library/Caches/antidote/https-COLON--SLASH--SLASH-github.com-SLASH-wfxr-SLASH-forgit" )
source "$HOME/Library/Caches/antidote/https-COLON--SLASH--SLASH-github.com-SLASH-wfxr-SLASH-forgit/forgit.plugin.zsh"
if ! (( $+functions[zsh-defer] )); then
  fpath+=( "$HOME/Library/Caches/antidote/https-COLON--SLASH--SLASH-github.com-SLASH-romkatv-SLASH-zsh-defer" )
  source "$HOME/Library/Caches/antidote/https-COLON--SLASH--SLASH-github.com-SLASH-romkatv-SLASH-zsh-defer/zsh-defer.plugin.zsh"
fi
fpath+=( "$HOME/Library/Caches/antidote/https-COLON--SLASH--SLASH-github.com-SLASH-olets-SLASH-zsh-abbr" )
zsh-defer source "$HOME/Library/Caches/antidote/https-COLON--SLASH--SLASH-github.com-SLASH-olets-SLASH-zsh-abbr/zsh-abbr.plugin.zsh"
fpath+=( "$HOME/Library/Caches/antidote/https-COLON--SLASH--SLASH-github.com-SLASH-MichaelAquilina-SLASH-zsh-you-should-use" )
source "$HOME/Library/Caches/antidote/https-COLON--SLASH--SLASH-github.com-SLASH-MichaelAquilina-SLASH-zsh-you-should-use/you-should-use.plugin.zsh"
source "$HOME/Library/Caches/antidote/https-COLON--SLASH--SLASH-github.com-SLASH-MichaelAquilina-SLASH-zsh-you-should-use/zsh-you-should-use.plugin.zsh"
fpath+=( "$HOME/Library/Caches/antidote/https-COLON--SLASH--SLASH-github.com-SLASH-hlissner-SLASH-zsh-autopair" )
source "$HOME/Library/Caches/antidote/https-COLON--SLASH--SLASH-github.com-SLASH-hlissner-SLASH-zsh-autopair/autopair.plugin.zsh"
source "$HOME/Library/Caches/antidote/https-COLON--SLASH--SLASH-github.com-SLASH-hlissner-SLASH-zsh-autopair/zsh-autopair.plugin.zsh"
fpath+=( "$HOME/Library/Caches/antidote/https-COLON--SLASH--SLASH-github.com-SLASH-zdharma-continuum-SLASH-fast-syntax-highlighting" )
zsh-defer source "$HOME/Library/Caches/antidote/https-COLON--SLASH--SLASH-github.com-SLASH-zdharma-continuum-SLASH-fast-syntax-highlighting/fast-syntax-highlighting.plugin.zsh"
