#!/usr/bin/env zsh

set -eu

repo_root=${0:A:h:h}
bootstrap="$repo_root/bootstrap.sh"
[[ -x "$bootstrap" ]] || {
    print -u2 -- "FAIL: bootstrap.sh is missing or not executable"
    exit 1
}

tmp_dir=$(mktemp -d)
trap 'rm -rf -- "$tmp_dir"' EXIT
mkdir -p "$tmp_dir/bin" "$tmp_dir/home"
log_file="$tmp_dir/commands.log"
: > "$log_file"

print -r -- '#!/bin/sh
printf "%s\n" "$*" >> "$TEST_COMMAND_LOG"' > "$tmp_dir/bin/chezmoi"
chmod +x "$tmp_dir/bin/chezmoi"

output=$(
    HOME="$tmp_dir/home" \
    PATH="$tmp_dir/bin:/usr/bin:/bin" \
    TEST_COMMAND_LOG="$log_file" \
    /bin/sh "$bootstrap"
)

log=$(<"$log_file")
[[ "$log" == "-S $repo_root init" ]] || {
    print -u2 -- "FAIL: bootstrap must invoke existing chezmoi exactly once with source and init"
    exit 1
}
[[ "$output" == *"chezmoi diff"* ]] || {
    print -u2 -- "FAIL: bootstrap did not print the diff review step"
    exit 1
}
[[ "$output" == *"apply --dry-run --verbose"* ]] || {
    print -u2 -- "FAIL: bootstrap did not print the dry-run review step"
    exit 1
}

download_bin="$tmp_dir/download-bin"
mkdir "$download_bin"
print -r -- '#!/bin/sh
printf "curl %s\n" "$*" >> "$TEST_COMMAND_LOG"
printf "%s\n" \
    "#!/bin/sh" \
    "install_dir=" \
    "while [ \"\$#\" -gt 0 ]; do" \
    "    case \"\$1\" in" \
    "        -b) install_dir=\$2; shift 2 ;;" \
    "        *) shift ;;" \
    "    esac" \
    "done" \
    "mkdir -p \"\$install_dir\"" \
    "printf '\''%s\\n'\'' '\''#!/bin/sh'\'' '\''printf \"chezmoi %s\\\\n\" \"\$*\" >> \"\$TEST_COMMAND_LOG\"'\'' > \"\$install_dir/chezmoi\"" \
    "chmod +x \"\$install_dir/chezmoi\""' > "$download_bin/curl"
chmod +x "$download_bin/curl"

: > "$log_file"
HOME="$tmp_dir/download-home" \
    PATH="$download_bin:/usr/bin:/bin" \
    TEST_COMMAND_LOG="$log_file" \
    /bin/sh "$bootstrap" >/dev/null
download_log=$(<"$log_file")
expected_download_log=$'curl --proto =https --proto-redir =https -fsLS https://get.chezmoi.io\n'"chezmoi -S $repo_root init"
[[ "$download_log" == "$expected_download_log" ]] || {
    print -u2 -- "FAIL: bootstrap must download over HTTPS and invoke downloaded chezmoi exactly once with source and init"
    exit 1
}

mkdir -p "$tmp_dir/no-curl"
cp "$tmp_dir/bin/chezmoi" "$tmp_dir/no-curl/chezmoi"
if HOME="$tmp_dir/home" \
    PATH="$tmp_dir/no-curl" \
    TEST_COMMAND_LOG="$log_file" \
    /bin/sh "$bootstrap" >/dev/null 2>&1; then
    print -u2 -- "FAIL: bootstrap succeeded without curl"
    exit 1
fi

print -- "PASS: bootstrap safety"
