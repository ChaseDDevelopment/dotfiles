#!/usr/bin/env zsh

set -eu

repo_root=${0:A:h:h}
data="$repo_root/home/.chezmoidata/packages.yaml"
installer="$repo_root/home/run_onchange_before_10-install-packages.sh.tmpl"
homebrew="$repo_root/home/run_once_before_05-install-homebrew.sh.tmpl"
config="$repo_root/home/.chezmoi.toml.tmpl"
ignore="$repo_root/home/.chezmoiignore.tmpl"
bat_cache_hook="$repo_root/home/run_onchange_after_30-bat-cache.sh.tmpl"
zsh_hook="$repo_root/home/run_onchange_after_20-zsh-plugins.sh.tmpl"
tmux_hook="$repo_root/home/run_onchange_after_40-tmux.sh.tmpl"
neovim_fallback="$repo_root/home/run_onchange_before_15-install-neovim-apt.sh.tmpl"
neovim_hook="$repo_root/home/run_onchange_after_60-neovim.sh.tmpl"
git_config="$repo_root/home/dot_config/git/config.tmpl"
bat_shim="$repo_root/home/dot_local/bin/symlink_bat.tmpl"
fd_shim="$repo_root/home/dot_local/bin/symlink_fd.tmpl"

for required_path in \
    "$data" "$installer" "$homebrew" "$config" "$ignore" \
    "$zsh_hook" "$bat_cache_hook" "$tmux_hook" \
    "$neovim_fallback" "$neovim_hook" "$git_config" "$bat_shim" "$fd_shim"; do
    [[ -f "$required_path" ]] || {
        print -u2 -- "FAIL: missing ${required_path#$repo_root/}"
        exit 1
    }
done

[[ $(<"$bat_shim") == /usr/bin/batcat && $(<"$fd_shim") == /usr/bin/fdfind ]] || {
    print -u2 -- "FAIL: Debian command shims must target batcat and fdfind"
    exit 1
}

data_text=$(<"$data")
installer_text=$(<"$installer")
homebrew_text=$(<"$homebrew")

for key in brew cask pacman apt; do
    [[ "$data_text" == *"$key:"* ]] || {
        print -u2 -- "FAIL: package data missing $key list"
        exit 1
    }
done
[[ "$data_text" != *"aur:"* ]] || {
    print -u2 -- "FAIL: package data must not add speculative AUR support"
    exit 1
}
[[ "$data_text" == *"base:"* && "$data_text" == *"dev:"* ]] || {
    print -u2 -- "FAIL: APT packages must be split into base and dev lists"
    exit 1
}

config_text=$(<"$config")
[[ "$config_text" == *"promptChoiceOnce"* &&
    "$config_text" == *'"workstation"'* &&
    "$config_text" == *'"dev"'* &&
    "$config_text" == *'"server"'* &&
    "$config_text" == *'chezmoi.os "darwin"'* &&
    "$config_text" == *'osRelease.id "debian"'* &&
    "$config_text" == *'$profileDefault = "workstation"'* &&
    "$config_text" == *'$profileDefault = "server"'* &&
    "$config_text" == *'$profileDefault := "dev"'* &&
    "$config_text" == *"profile ="* ]] || {
    print -u2 -- "FAIL: machine config must persist platform-defaulted profile choice"
    exit 1
}

[[ "$installer_text" == *"brew bundle"* ]] || {
    print -u2 -- "FAIL: Darwin package path must use brew bundle"
    exit 1
}
[[ "$installer_text" == *"HOMEBREW_BUNDLE_NO_UPGRADE=1"* ]] || {
    print -u2 -- "FAIL: Brew bundle must not upgrade existing packages"
    exit 1
}
[[ "$installer_text" == *"pacman -Syu --needed --noconfirm"* ]] || {
    print -u2 -- "FAIL: Arch package path must use pacman -Syu --needed"
    exit 1
}
[[ "$installer_text" == *'osRelease.id "ubuntu"'* &&
    "$installer_text" == *'osRelease.id "debian"'* &&
    "$installer_text" == *"dpkg --audit"* &&
    "$installer_text" == *"DPkg::Lock::Timeout=60"* &&
    "$installer_text" == *"--no-upgrade --no-remove"* &&
    "$installer_text" == *"apt-get update"* ]] || {
    print -u2 -- "FAIL: Debian-family package path is incomplete"
    exit 1
}
for forbidden in \
    'brew upgrade' 'pacman -R' 'apt-get upgrade' \
    'apt-get remove' 'apt-get purge' 'apt-get autoremove' \
    'paru ' 'yay ' '--apply'; do
    [[ "$installer_text" != *"$forbidden"* ]] || {
        print -u2 -- "FAIL: package installer contains forbidden action: $forbidden"
        exit 1
    }
done
! grep -Eq \
    '^[[:space:]]*(sudo[[:space:]]+|run_root[[:space:]]+)?dpkg[[:space:]]+--configure' \
    "$installer" || {
    print -u2 -- "FAIL: package installer must never auto-repair dpkg state"
    exit 1
}
[[ "$homebrew_text" == *'chezmoi.os "darwin"'* ]] || {
    print -u2 -- "FAIL: Homebrew bootstrap is not Darwin-gated"
    exit 1
}

grep -Fq '{{ include "dot_config/zsh/plugins/dot_zsh_plugins.txt" | sha256sum }}' "$zsh_hook" &&
    grep -Fq '{{ include "dot_config/bat/config" | sha256sum }}' "$bat_cache_hook" &&
    grep -Fq '{{ include "dot_config/bat/themes/tokyonight_night.tmTheme" | sha256sum }}' "$bat_cache_hook" &&
    grep -Fq '{{ include "dot_config/tmux/tmux.conf" | sha256sum }}' "$tmux_hook" &&
    grep -Fq '{{ include "dot_config/tmux/scripts/executable_tmux-boot" | sha256sum }}' "$tmux_hook" &&
    grep -Fq '{{ include "Library/LaunchAgents/com.orion.tmux-server.plist.tmpl" | sha256sum }}' "$tmux_hook" &&
    grep -Fq '{{ include "dot_config/nvim/init.lua" | sha256sum }}' "$neovim_hook" &&
    grep -Fq '{{ include "dot_config/nvim/lua/core/pack.lua" | sha256sum }}' "$neovim_hook" || {
    print -u2 -- "FAIL: run_onchange hooks must hash every managed input"
    exit 1
}

fallback_path_line=$(
    grep -nF 'PATH="$HOME/.local/bin:$PATH"' "$neovim_fallback" |
        cut -d: -f1
)
fallback_probe_line=$(
    grep -nF 'if command -v nvim' "$neovim_fallback" |
        cut -d: -f1
)
[[ -n "$fallback_path_line" && -n "$fallback_probe_line" &&
    "$fallback_path_line" -lt "$fallback_probe_line" ]] || {
    print -u2 -- "FAIL: Neovim fallback must prefer ~/.local/bin before version validation"
    exit 1
}

chezmoi_bin=${CHEZMOI_BIN:-}
if [[ -z "$chezmoi_bin" ]] && (( $+commands[chezmoi] )); then
    chezmoi_bin=${commands[chezmoi]}
fi
[[ -n "$chezmoi_bin" ]] || {
    print -u2 -- "FAIL: CHEZMOI_BIN or chezmoi command is required for render tests"
    exit 1
}

tmp_dir=$(mktemp -d)
trap 'rm -rf -- "$tmp_dir"' EXIT
touch "$tmp_dir/empty.toml"

render() {
    local os=$1
    local distro=$2
    local profile=$3
    local template=$4
    local override
    override=$(
        printf \
            '{"chezmoi":{"os":"%s","osRelease":{"id":"%s"}},"profile":"%s"}' \
            "$os" "$distro" "$profile"
    )
    "$chezmoi_bin" \
        --config "$tmp_dir/empty.toml" \
        --source "$repo_root" \
        --override-data "$override" \
        execute-template < "$template"
}

render_neovim_fallback() {
    local os=$1
    local distro=$2
    local arch=$3
    local override
    override=$(printf \
        '{"chezmoi":{"os":"%s","osRelease":{"id":"%s"},"arch":"%s"}}' \
        "$os" "$distro" "$arch")
    "$chezmoi_bin" \
        --config "$tmp_dir/empty.toml" \
        --source "$repo_root" \
        --override-data "$override" \
        execute-template < "$neovim_fallback"
}

source_hash() {
    if (( $+commands[sha256sum] )); then
        sha256sum "$repo_root/home/$1" | awk '{ print $1 }'
    elif (( $+commands[shasum] )); then
        shasum -a 256 "$repo_root/home/$1" | awk '{ print $1 }'
    else
        print -u2 -- "FAIL: sha256sum or shasum is required for render tests"
        return 1
    fi
}

zsh_manifest_hash=$(source_hash dot_config/zsh/plugins/dot_zsh_plugins.txt)
bat_config_hash=$(source_hash dot_config/bat/config)
bat_theme_hash=$(source_hash dot_config/bat/themes/tokyonight_night.tmTheme)
tmux_config_hash=$(source_hash dot_config/tmux/tmux.conf)
tmux_boot_hash=$(source_hash dot_config/tmux/scripts/executable_tmux-boot)
tmux_plist_hash=$(
    source_hash Library/LaunchAgents/com.orion.tmux-server.plist.tmpl
)
neovim_init_hash=$(source_hash dot_config/nvim/init.lua)
neovim_pack_hash=$(source_hash dot_config/nvim/lua/core/pack.lua)

zsh_setup=$(render linux ubuntu dev "$zsh_hook")
darwin_tmux_setup=$(render darwin "" workstation "$tmux_hook")
linux_tmux_setup=$(render linux ubuntu dev "$tmux_hook")
[[ "$zsh_setup" == *"$zsh_manifest_hash"* &&
    "$darwin_tmux_setup" == *"$tmux_config_hash"* &&
    "$darwin_tmux_setup" == *"$tmux_boot_hash"* &&
    "$darwin_tmux_setup" == *"$tmux_plist_hash"* ]] || {
    print -u2 -- "FAIL: rendered zsh/Darwin tmux hooks must contain their source hashes"
    exit 1
}
[[ "$linux_tmux_setup" == *"$tmux_config_hash"* &&
    "$linux_tmux_setup" != *"$tmux_boot_hash"* &&
    "$linux_tmux_setup" != *"$tmux_plist_hash"* ]] || {
    print -u2 -- "FAIL: Linux tmux hook must ignore macOS-only source hashes"
    exit 1
}
for rendered_hook in "$zsh_setup" "$darwin_tmux_setup" "$linux_tmux_setup"; do
    print -r -- "$rendered_hook" | sh -n || {
        print -u2 -- "FAIL: rendered zsh/tmux hook must be POSIX shell syntax"
        exit 1
    }
done

server_render=$(render linux debian server "$installer")
dev_render=$(render linux ubuntu dev "$installer")
server_packages='bat btop build-essential ca-certificates curl direnv fd-find fzf git jq libxml2-dev libxslt1-dev nala ripgrep tmux unzip wget zoxide zsh'
dev_packages='cargo ffmpeg golang-go hyperfine imagemagick libtree-sitter-dev p7zip-full poppler-utils python3 python3-venv rustc wl-clipboard xclip'
for rendered_script in "$server_render" "$dev_render"; do
    print -r -- "$rendered_script" | sh -n || {
        print -u2 -- "FAIL: rendered package installer must be POSIX shell syntax"
        exit 1
    }
    [[ $(print -r -- "$rendered_script" | grep -c '^run_root apt-get update$') == 1 &&
        $(print -r -- "$rendered_script" | grep -c '^run_root apt-get install ') == 1 ]] || {
        print -u2 -- "FAIL: rendered APT script must contain exactly one update and install"
        exit 1
    }
done
[[ $(print -r -- "$server_render" | grep '^run_root apt-get install ') == \
    "run_root apt-get install -y --no-install-recommends --no-upgrade --no-remove -o DPkg::Lock::Timeout=60 $server_packages" &&
    $(print -r -- "$dev_render" | grep '^run_root apt-get install ') == \
    "run_root apt-get install -y --no-install-recommends --no-upgrade --no-remove -o DPkg::Lock::Timeout=60 $server_packages $dev_packages" ]] || {
    print -u2 -- "FAIL: rendered APT package order or flags changed"
    exit 1
}

stub_dir="$tmp_dir/stubs"
mkdir "$stub_dir"
calls="$tmp_dir/calls"
for stub in id dpkg apt-get sudo; do
    print -r -- '#!/bin/sh' > "$stub_dir/$stub"
done
print -r -- 'printf "%s\\n" "$TEST_UID"' >> "$stub_dir/id"
print -rl -- 'printf "dpkg %s\\n" "$*" >> "$CALLS"' \
    'case ${DPKG_MODE:-ok} in output|fail) printf "%s\\n" "audit output";; esac' \
    'test "${DPKG_MODE:-ok}" != fail' >> "$stub_dir/dpkg"
print -r -- 'printf "apt-get %s\\n" "$*" >> "$CALLS"' >> "$stub_dir/apt-get"
print -rl -- 'printf "sudo %s\\n" "$*" >> "$CALLS"' '"$@"' >> "$stub_dir/sudo"
chmod +x "$stub_dir"/*
print -r -- "$server_render" > "$tmp_dir/server.sh"

run_server() {
    TEST_UID=$1 DPKG_MODE=$2 CALLS="$calls" PATH="$stub_dir:$PATH" sh "$tmp_dir/server.sh"
}

: > "$calls"
run_server 0 ok
calls_text=$(<"$calls")
expected_root_calls=$'dpkg --audit\napt-get update\n'"apt-get install -y --no-install-recommends --no-upgrade --no-remove -o DPkg::Lock::Timeout=60 $server_packages"
[[ "$calls_text" == "$expected_root_calls" ]] || {
    print -u2 -- "FAIL: root APT path must run only the expected audit, update, and install"
    exit 1
}

: > "$calls"
run_server 1000 ok
calls_text=$(<"$calls")
expected_user_calls=$'sudo dpkg --audit\ndpkg --audit\nsudo apt-get update\napt-get update\n'"sudo apt-get install -y --no-install-recommends --no-upgrade --no-remove -o DPkg::Lock::Timeout=60 $server_packages"$'\n'"apt-get install -y --no-install-recommends --no-upgrade --no-remove -o DPkg::Lock::Timeout=60 $server_packages"
[[ "$calls_text" == "$expected_user_calls" ]] || {
    print -u2 -- "FAIL: non-root APT path must run only the expected sudo audit, update, and install"
    exit 1
}

for audit_mode in output fail; do
    : > "$calls"
    if audit_output=$(run_server 0 "$audit_mode" 2>&1); then
        print -u2 -- "FAIL: dpkg audit $audit_mode must stop package installation"
        exit 1
    fi
    [[ "$audit_output" == *'audit output'* &&
        "$audit_output" == *'sudo dpkg --configure -a'* &&
        $(<"$calls") != *'apt-get '* ]] || {
        print -u2 -- "FAIL: dpkg audit $audit_mode must preserve output and block APT"
        exit 1
    }
done

amd64_neovim=$(render_neovim_fallback linux debian amd64)
arm64_neovim=$(render_neovim_fallback linux ubuntu arm64)
for rendered_neovim in "$amd64_neovim" "$arm64_neovim"; do
    [[ "$rendered_neovim" == '#!/bin/sh'* ]] || {
        print -u2 -- "FAIL: rendered Neovim fallback must start with a POSIX shebang"
        exit 1
    }
    print -r -- "$rendered_neovim" | sh -n || {
        print -u2 -- "FAIL: rendered Neovim fallback must be POSIX shell syntax"
        exit 1
    }
done
[[ "$amd64_neovim" == *'nvim-linux-x86_64.tar.gz'* &&
    "$amd64_neovim" == *'nvim-linux-x86_64'* &&
    "$amd64_neovim" == *'012bf3fcac5ade43914df3f174668bf64d05e049a4f032a388c027b1ebd78628'* &&
    "$amd64_neovim" == *'.local/opt/nvim-v0.12.4'* &&
    "$amd64_neovim" == *'.local/bin/nvim'* ]] || {
    print -u2 -- "FAIL: amd64 Neovim fallback must pin the expected asset, checksum, and paths"
    exit 1
}
[[ "$arm64_neovim" == *'nvim-linux-arm64.tar.gz'* &&
    "$arm64_neovim" == *'nvim-linux-arm64'* &&
    "$arm64_neovim" == *'ceb7e88c6b681f0515d135dcdfad54f5eb4373b25ce6172197cd9a69c758063f'* ]] || {
    print -u2 -- "FAIL: arm64 Neovim fallback must pin the expected asset and checksum"
    exit 1
}
sha_line=$(print -r -- "$amd64_neovim" | nl -ba | awk '/sha256sum -c/{ print $1; exit }')
extract_line=$(print -r -- "$amd64_neovim" | nl -ba | awk '/tar -xzf/{ print $1; exit }')
[[ -n "$sha_line" && -n "$extract_line" && "$sha_line" -lt "$extract_line" ]] || {
    print -u2 -- "FAIL: Neovim fallback must verify SHA256 before extraction"
    exit 1
}
[[ "$amd64_neovim" == *'[ -e "$link_path" ] && [ ! -L "$link_path" ]'* ]] || {
    print -u2 -- "FAIL: Neovim fallback must refuse a non-symlink nvim collision"
    exit 1
}

non_apt_neovim=$(render_neovim_fallback linux arch amd64)
[[ -z "$non_apt_neovim" ]] || {
    print -u2 -- "FAIL: Neovim fallback must not render outside Debian-family Linux"
    exit 1
}
unsupported_neovim=$(render_neovim_fallback linux debian riscv64)
[[ "$unsupported_neovim" == *'Unsupported Neovim fallback architecture: riscv64'* &&
    "$unsupported_neovim" != *'https://github.com/neovim/'* ]] || {
    print -u2 -- "FAIL: unsupported Neovim architectures must stop before constructing a download URL"
    exit 1
}

neovim_stub_dir="$tmp_dir/neovim-stubs"
mkdir "$neovim_stub_dir"
neovim_calls="$tmp_dir/neovim-calls"
print -rl -- '#!/bin/sh' 'printf "NVIM v0.12.0\\n"' > "$neovim_stub_dir/nvim"
print -rl -- '#!/bin/sh' 'printf "curl %s\\n" "$*" >> "$NEOVIM_CALLS"' > "$neovim_stub_dir/curl"
print -rl -- '#!/bin/sh' 'printf "ln %s\\n" "$*" >> "$NEOVIM_CALLS"' > "$neovim_stub_dir/ln"
chmod +x "$neovim_stub_dir"/*
neovim_home="$tmp_dir/neovim-home"
mkdir -p "$neovim_home/.local/bin"
print -r -- sentinel > "$neovim_home/.local/bin/nvim"
: > "$neovim_calls"
NEOVIM_CALLS="$neovim_calls" HOME="$neovim_home" PATH="$neovim_stub_dir:$PATH" \
    sh -c "$amd64_neovim"
[[ ! -s "$neovim_calls" && $(<"$neovim_home/.local/bin/nvim") == sentinel ]] || {
    print -u2 -- "FAIL: Neovim >=0.12 must exit before curl or link changes"
    exit 1
}

print -rl -- '#!/bin/sh' 'printf "NVIM v0.12.4\\n"' > "$neovim_stub_dir/nvim"
print -rl -- '#!/bin/sh' 'printf "NVIM v0.11.5\\n"' > \
    "$neovim_home/.local/bin/nvim"
chmod +x "$neovim_home/.local/bin/nvim"
: > "$neovim_calls"
if collision_output=$(
    NEOVIM_CALLS="$neovim_calls" \
        HOME="$neovim_home" \
        PATH="$neovim_stub_dir:$PATH" \
        sh -c "$amd64_neovim" 2>&1
); then
    print -u2 -- "FAIL: stale local Neovim must not be masked by a newer system binary"
    exit 1
fi
[[ "$collision_output" == *'refuses to overwrite non-symlink'* &&
    ! -s "$neovim_calls" ]] || {
    print -u2 -- "FAIL: stale local Neovim collision must stop before curl or link changes"
    exit 1
}

neovim_hook_text=$(render linux ubuntu dev "$neovim_hook")
[[ "$neovim_hook_text" == *'PATH="$HOME/.local/bin:$HOME/.cargo/bin:$PATH"'* ]] || {
    print -u2 -- "FAIL: Neovim hook must prefer the local fallback binary"
    exit 1
}
[[ "$neovim_hook_text" == *"$neovim_init_hash"* &&
    "$neovim_hook_text" == *"$neovim_pack_hash"* ]] || {
    print -u2 -- "FAIL: rendered Neovim hook must contain its source hashes"
    exit 1
}
print -r -- "$neovim_hook_text" | sh -n || {
    print -u2 -- "FAIL: Neovim hook must be POSIX shell syntax"
    exit 1
}

fedora_render=$(render linux fedora server "$installer")
[[ "$fedora_render" != *"apt-get"* &&
    "$fedora_render" == *"not defined for this platform"* ]] || {
    print -u2 -- "FAIL: unsupported Linux must fail without guessing a manager"
    exit 1
}

ignore_render=$(render linux debian server "$ignore")
[[ "$ignore_render" == *".config/ghostty/"* &&
    "$ignore_render" != *".local/bin/bat"* &&
    "$ignore_render" != *".local/bin/fd"* ]] || {
    print -u2 -- "FAIL: server profile must exclude workstation-only config"
    exit 1
}

ubuntu_ignore=$(render linux ubuntu dev "$ignore")
[[ "$ubuntu_ignore" != *".local/bin/bat"* &&
    "$ubuntu_ignore" != *".local/bin/fd"* ]] || {
    print -u2 -- "FAIL: Ubuntu must manage Debian command shims"
    exit 1
}

darwin_ignore=$(render darwin "" workstation "$ignore")
[[ "$darwin_ignore" == *".local/bin/bat"* &&
    "$darwin_ignore" == *".local/bin/fd"* ]] || {
    print -u2 -- "FAIL: non-Debian platforms must ignore Debian command shims"
    exit 1
}

debian_git=$(render linux debian server "$git_config")
ubuntu_git=$(render linux ubuntu dev "$git_config")
arch_git=$(render linux arch dev "$git_config")
darwin_git=$(render darwin "" workstation "$git_config")
[[ "$debian_git" != *"pager = delta"* &&
    "$debian_git" != *"diffFilter = delta --color-only"* &&
    "$debian_git" != *"[delta]"* ]] || {
    print -u2 -- "FAIL: Debian Git config must not reference delta"
    exit 1
}
[[ "$ubuntu_git" == "$debian_git" ]] || {
    print -u2 -- "FAIL: Ubuntu Git config must omit delta like Debian"
    exit 1
}
[[ "$arch_git" == *"pager = delta"* &&
    "$arch_git" == *"diffFilter = delta --color-only"* &&
    "$arch_git" == *"[delta]"* &&
    "$arch_git" == *"navigate = true"* &&
    "$arch_git" == *"dark = true"* &&
    "$arch_git" == *"line-numbers = true"* &&
    "$arch_git" == *"hyperlinks = true"* &&
    "$arch_git" == *"syntax-theme = tokyonight_night"* &&
    "$darwin_git" == "$arch_git" ]] || {
    print -u2 -- "FAIL: Darwin and Arch Git config must preserve delta settings"
    exit 1
}

bat_cache=$(render linux debian server "$bat_cache_hook")
[[ "$bat_cache" == '#!'* &&
    "$bat_cache" == *"$bat_config_hash"* &&
    "$bat_cache" == *"$bat_theme_hash"* ]] || {
    print -u2 -- "FAIL: rendered bat cache hook must start with a shebang"
    exit 1
}
print -r -- "$bat_cache" | sh -n || {
    print -u2 -- "FAIL: rendered bat cache hook must be POSIX shell syntax"
    exit 1
}
cache_stub_dir="$tmp_dir/batcat-stub"
mkdir "$cache_stub_dir"
bat_calls="$tmp_dir/bat-calls"
print -rl -- '#!/bin/sh' 'printf "%s\\n" "$*" > "$BAT_CALLS"' > "$cache_stub_dir/batcat"
chmod +x "$cache_stub_dir/batcat"
print -r -- "$bat_cache" > "$tmp_dir/bat-cache.sh"
BAT_CALLS="$bat_calls" PATH="$cache_stub_dir" /bin/sh "$tmp_dir/bat-cache.sh"
[[ $(<"$bat_calls") == "cache --build" ]] || {
    print -u2 -- "FAIL: bat cache hook must fall back to batcat cache --build"
    exit 1
}

print -- "PASS: package render contract"
