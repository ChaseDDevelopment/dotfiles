# Git aliases (curated set - no conflicts with coreutils)
# Note: forgit plugin provides fzf-enhanced versions (ga, glo, gd, etc.)
# Note: ohmyzsh git plugin removed due to grm='git rm' conflicting with GNU rm

alias g='git'
alias gs='git status'
alias gss='git status -s'

# Add
alias ga='git add'
alias gaa='git add --all'
alias gap='git add --patch'

# Commit
alias gc='git commit -v'
alias gca='git commit -va'
alias gcm='git commit -m'
alias 'gc!'='git commit -v --amend'
alias 'gca!'='git commit -va --amend'

# Branch
alias gb='git branch'
alias gba='git branch -a'
alias gbd='git branch -d'
alias gbD='git branch -D'
alias gco='git checkout'
alias gcb='git checkout -b'
alias gsw='git switch'
alias gswc='git switch -c'

# Diff
alias gd='git diff'
alias gdc='git diff --cached'
alias gds='git diff --staged'

# Fetch/Pull/Push
alias gf='git fetch'
alias gfa='git fetch --all --prune'
alias gl='git pull'
alias glr='git pull --rebase'
alias gp='git push'
alias gpf='git push --force-with-lease'
alias gpsup='git push --set-upstream origin $(git branch --show-current)'

# Rebase
alias grb='git rebase'
alias grbi='git rebase -i'
alias grbc='git rebase --continue'
alias grba='git rebase --abort'

# Reset
alias grh='git reset HEAD'
alias grhh='git reset HEAD --hard'

# Log (basic - forgit provides better interactive versions)
alias glog='git log --oneline --graph --decorate'
alias gloga='git log --oneline --graph --decorate --all'

# Stash
alias gsta='git stash push'
alias gstp='git stash pop'
alias gstl='git stash list'
alias gstd='git stash drop'

# Misc
alias gcp='git cherry-pick'
alias gm='git merge'
alias grs='git restore'
alias grss='git restore --staged'
