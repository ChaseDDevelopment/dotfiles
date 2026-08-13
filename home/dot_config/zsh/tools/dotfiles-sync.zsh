# Auto-sync dotfiles on interactive shells (Ghostty surfaces, ssh, tmux).
# Pulls and applies the public source — and the AI source when configured —
# at most once per interval, then stays out of the way. Deliberately
# conservative: fast-forward pulls only, never touches a dirty or diverged
# checkout, never prompts (drift is reported for a manual apply), and every
# failure is a one-line warning, never a broken shell.
#
# Escape hatches: DOTFILES_SYNC=0 disables entirely; DOTFILES_SYNC_FORCE=1
# ignores the throttle for one shell.

_dotfiles_sync() {
    [[ "${DOTFILES_SYNC:-1}" == 0 ]] && return 0
    (( $+commands[chezmoi] )) || return 0
    (( $+commands[git] )) || return 0

    # 15 minutes: cheap (ff-only pull, apply only if HEAD moved) and short
    # enough that a push-then-ssh the same evening still picks up the AI
    # source. Override with DOTFILES_SYNC_FORCE=1.
    local interval=$(( 15 * 60 ))
    local state_dir="${XDG_CACHE_HOME:-$HOME/.cache}/dotfiles-sync"
    local stamp="$state_dir/last-sync"
    mkdir -p "$state_dir" 2>/dev/null || return 0

    if [[ "${DOTFILES_SYNC_FORCE:-0}" != 1 && -f "$stamp" ]]; then
        local last=0
        last=$(zstat +mtime "$stamp" 2>/dev/null) || last=0
        (( EPOCHSECONDS - last < interval )) && return 0
    fi

    # One sync at a time across concurrent shells; drop stale locks.
    local lock="$state_dir/lock"
    if ! mkdir "$lock" 2>/dev/null; then
        local lock_age=0
        lock_age=$(zstat +mtime "$lock" 2>/dev/null) || lock_age=0
        (( EPOCHSECONDS - lock_age < 600 )) && return 0
        rmdir "$lock" 2>/dev/null || return 0
        mkdir "$lock" 2>/dev/null || return 0
    fi
    {
        touch "$stamp"
        print -- "dotfiles-sync: checking for updates..."
        _dotfiles_sync_one "dotfiles" "chezmoi"
        if [[ -f "${XDG_CONFIG_HOME:-$HOME/.config}/chezmoi-ai/chezmoi.toml" ]] &&
            (( $+commands[chezmoi-ai] )); then
            _dotfiles_sync_one "ai-dotfiles" "chezmoi-ai"
        fi
    } always {
        rmdir "$lock" 2>/dev/null
    }
}

# $1 label, $2 chezmoi command. Pull the source repo (ff-only, clean tree
# required), and apply when the pull advanced HEAD.
_dotfiles_sync_one() {
    local label=$1 cz=$2 src repo before after
    src=$("$cz" source-path 2>/dev/null) || return 0
    repo=${src:h}
    [[ -d "$repo/.git" ]] || repo=$src
    [[ -d "$repo/.git" ]] || return 0

    if [[ -n "$(git -C "$repo" status --porcelain 2>/dev/null)" ]]; then
        print -u2 -- "dotfiles-sync: $label checkout is dirty; skipping auto-sync"
        return 0
    fi

    before=$(git -C "$repo" rev-parse HEAD 2>/dev/null) || return 0
    if ! git -C "$repo" pull --ff-only --quiet 2>/dev/null; then
        print -u2 -- "dotfiles-sync: $label pull failed (offline or diverged); skipping"
        return 0
    fi
    after=$(git -C "$repo" rev-parse HEAD 2>/dev/null) || return 0
    [[ "$before" == "$after" ]] && return 0

    print -- "dotfiles-sync: applying new $label ($(git -C "$repo" rev-list --count "$before..$after" 2>/dev/null) commits)"
    if ! "$cz" apply < /dev/null; then
        print -u2 -- "dotfiles-sync: $label apply needs attention; run '$cz apply' manually"
        return 0
    fi

    # New agent skills ride along with the AI source.
    if [[ "$cz" == chezmoi-ai ]] && (( $+commands[skillshare] )); then
        skillshare sync < /dev/null ||
            print -u2 -- "dotfiles-sync: skillshare sync failed; run it manually"
    fi
}

zmodload zsh/stat 2>/dev/null
zmodload zsh/datetime 2>/dev/null
_dotfiles_sync
