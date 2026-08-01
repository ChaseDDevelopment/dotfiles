#!/usr/bin/env zsh

set -eu

repo_root=${0:A:h:h}
cd "$repo_root"

fail() {
    print -u2 -- "FAIL: $*"
    exit 1
}

sensitive_paths=(
    .env
    server.key
    encrypted_server.key.age
    encrypted_client.pem.age
    private_id_ed25519
    home/dot_ssh/id_ed25519
    home/dot_gnupg/private-keys-v1.d/key
    home/dot_aws/credentials
    home/dot_kube/config
    home/dot_docker/config.json
    home/dot_config/gh/hosts.yml
    home/dot_codex/auth.json
    home/dot_codex/history.jsonl
    home/dot_claude/history.jsonl
    home/dot_grok/auth.json
    home/private_dot_codex/private_auth.json
    home/private_dot_claude/private_history.jsonl
    home/private_dot_pi/agent/auth.json
    home/private_dot_pi/agent/private_auth.json
    home/private_dot_pi/agent/models-store.json
    home/private_dot_pi/agent/sessions/session.json
    home/private_dot_pi/agent/private_sessions/session.json
    home/private_dot_pi/agent/context7-cache/cache.json
    home/private_dot_pi/agent/private_context7-cache/cache.json
    home/private_dot_pi/agent/extensions/herdr-agent-state.ts
    home/private_dot_codex/settings.json
    home/private_dot_claude/settings.json
    home/private_dot_grok/settings.json
    home/private_dot_config/app/secrets.json
    home/private_dot_config/app/credentials.toml
    home/private_dot_config/app/private_credentials.json
    home/private_dot_config/app/.env
    home/private_dot_config/app/.env.local
    home/private_dot_config/app/private_dot_env
    home/dot_config/zsh/local.zsh
)

for candidate_path in "${sensitive_paths[@]}"; do
    git check-ignore --no-index -q -- "$candidate_path" || \
        fail "sensitive path is trackable: $candidate_path"
done

# Scan the filesystem directly so `git add -f` cannot bypass this boundary.
forbidden_dir=$(find home -type d \( \
    -name '.ssh' -o -name '*dot_ssh' -o \
    -name '.gnupg' -o -name '*dot_gnupg' -o \
    -name '.aws' -o -name '*dot_aws' -o \
    -name '.kube' -o -name '*dot_kube' -o \
    -name '.docker' -o -name '*dot_docker' -o \
    -name '.codex' -o -name '*dot_codex' -o \
    -name '.claude' -o -name '*dot_claude' -o \
    -name '.grok' -o -name '*dot_grok' -o \
    -path 'home/private_dot_pi/agent/*sessions*' -o \
    -path 'home/private_dot_pi/agent/*cache*' \
\) -print -quit)
[[ -z "$forbidden_dir" ]] || \
    fail "sensitive directory exists in public source: $forbidden_dir"

forbidden_path=$(find . -path './.git' -prune -o \
    \( -type f -o -type l \) \( \
        -name '.env' -o -name '.env.*' -o -name '*dot_env*' -o \
        -name '*.key*' -o -name '*.pem*' -o \
        -name '*id_rsa*' -o -name '*id_ed25519*' -o \
        -name '*id_ecdsa*' -o -name '*auth.json*' -o \
        -name '*credentials*' -o -name '*secrets*' -o \
        -name '*history.jsonl*' -o \
        -path './home/*dot_config/gh/hosts.yml' -o \
        -name '*models-store*' -o \
        -name '*herdr-agent-state*' \
    \) -print -quit)
[[ -z "$forbidden_path" ]] || \
    fail "sensitive file exists in public source: $forbidden_path"

safe_paths=(
    home/dot_config/atuin/config.toml
    home/dot_config/git/config.tmpl
    home/private_dot_pi/agent/aliases.bash
    home/dot_config/zsh/local.zsh.example
)
for candidate_path in "${safe_paths[@]}"; do
    if git check-ignore --no-index -q -- "$candidate_path"; then
        fail "approved config path is ignored: $candidate_path"
    fi
done

print -- "PASS: public secret boundary"
