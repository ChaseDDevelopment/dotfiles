#!/usr/bin/env zsh

set -eu

repo_root=${0:A:h:h}

fail() {
    print -u2 -- "FAIL: $*"
    exit 1
}

require_file() {
    [[ -f "$repo_root/$1" ]] || fail "missing $1"
}

require_file .chezmoiroot
[[ "$(<"$repo_root/.chezmoiroot")" == home ]] || \
    fail ".chezmoiroot must select home"

cmp -s "$repo_root/AGENTS.md" "$repo_root/CLAUDE.md" || \
    fail "AGENTS.md and CLAUDE.md must remain byte-identical"
grep -Fq 'zsh "$test_file" || exit 1' "$repo_root/README.md" || \
    fail "README test loop must fail fast"
grep -Fq 'zsh "$test_file" || exit 1' "$repo_root/AGENTS.md" || \
    fail "agent test loop must fail fast"
grep -Fq 'https://herdr.dev/docs/install/' "$repo_root/README.md" &&
    grep -Fq 'not install its Linux binary' "$repo_root/README.md" || \
    fail "README must document Herdr as a separate Linux prerequisite"
grep -Fq 'https://herdr.dev/docs/install/' "$repo_root/AGENTS.md" &&
    grep -Fq 'Linux hosts must install it separately' "$repo_root/AGENTS.md" || \
    fail "agent guidance must document the Herdr Linux prerequisite"

required_paths=(
    home/.chezmoi.toml.tmpl
    home/.chezmoiignore.tmpl
    home/.chezmoidata/packages.yaml
    home/dot_config/aerospace/aerospace.toml
    home/dot_config/zsh/dot_zshenv
    home/symlink_dot_zshenv.tmpl
    home/dot_config/tmux/tmux.conf
    home/symlink_dot_tmux.conf.tmpl
    home/dot_config/tmux/scripts/executable_tmux-session
    home/dot_local/bin/symlink_tmux-session.tmpl
    home/dot_local/bin/symlink_bat.tmpl
    home/dot_local/bin/symlink_fd.tmpl
    home/dot_local/bin/executable_chezmoi-ai
    home/dot_local/bin/executable_dot-update
    home/dot_config/private_chezmoi-ai/private_chezmoi.toml.tmpl
    home/dot_config/git/config.tmpl
    home/private_dot_pi/agent/aliases.bash
    home/run_once_before_05-install-homebrew.sh.tmpl
    home/run_onchange_before_10-install-packages.sh.tmpl
    home/run_onchange_before_15-install-neovim-apt.sh.tmpl
    home/run_onchange_after_20-zsh-plugins.sh.tmpl
    home/run_onchange_after_30-bat-cache.sh.tmpl
    home/run_onchange_after_40-tmux.sh.tmpl
    home/run_onchange_after_50-yazi-packages.sh.tmpl
    home/run_onchange_after_60-neovim.sh.tmpl
)

for required_path in "${required_paths[@]}"; do
    require_file "$required_path"
done

grep -q 'persistentState' \
    "$repo_root/home/dot_config/private_chezmoi-ai/private_chezmoi.toml.tmpl" || \
    fail "AI chezmoi config must use independent persistent state"

expected_executables=$(
    print -rl -- \
        home/dot_config/tmux/scripts/executable_capture-last-output.sh \
        home/dot_config/tmux/scripts/executable_tmux-boot \
        home/dot_config/tmux/scripts/executable_tmux-cheatsheet.sh \
        home/dot_config/tmux/scripts/executable_tmux-main \
        home/dot_config/tmux/scripts/executable_tmux-session \
        home/dot_local/bin/executable_chezmoi-ai \
        home/dot_local/bin/executable_dot-update \
        home/dot_local/bin/executable_herdr-worktree-create |
        sort
)
actual_executables=$(
    find "$repo_root/home" -type f -name 'executable_*' |
        sed "s|$repo_root/||" |
        sort
)
[[ "$actual_executables" == "$expected_executables" ]] || {
    print -u2 -- "FAIL: executable target allowlist differs"
    diff -u <(print -r -- "$expected_executables") \
        <(print -r -- "$actual_executables") >&2 || true
    exit 1
}

print -- "PASS: chezmoi layout"
