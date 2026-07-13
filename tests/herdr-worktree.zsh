#!/usr/bin/env zsh

emulate -L zsh
setopt no_unset pipe_fail
zmodload zsh/stat

repo_root=${0:A:h:h}
helper="$repo_root/configs/herdr/scripts/herdr-worktree-create"
tmp_dir=$(mktemp -d)
trap 'rm -rf -- "$tmp_dir"' EXIT

export TEST_LOG="$tmp_dir/herdr.log"
fake_bin="$tmp_dir/fake bin"
mkdir -p -- "$fake_bin"
: > "$TEST_LOG"

fail() {
    print -u2 -- "FAIL: $*"
    exit 1
}

assert_contains() {
    [[ "$1" == *"$2"* ]] || fail "$3 (missing: $2)"
}

assert_not_contains() {
    [[ "$1" != *"$2"* ]] || fail "$3 (unexpected: $2)"
}

real_git=$(command -v git) || fail "git is missing"
real_cp=$(command -v cp) || fail "cp is missing"

print -r -- '#!/usr/bin/env zsh
emulate -L zsh
setopt no_unset

for argument in "$@"; do
    if [[ "$argument" == fetch && -n "${FAKE_GIT_FETCH_STATUS:-}" ]]; then
        print -u2 -r -- "${FAKE_GIT_FETCH_STDERR:-fake fetch failure}"
        exit "$FAKE_GIT_FETCH_STATUS"
    fi
done
exec "$REAL_GIT" "$@"' > "$fake_bin/git"

print -r -- '#!/usr/bin/env zsh
emulate -L zsh
setopt no_unset

if [[ -n "${FAKE_CP_STATUS:-}" ]]; then
    print -u2 -r -- "${FAKE_CP_STDERR:-fake copy failure}"
    exit "$FAKE_CP_STATUS"
fi
exec "$REAL_CP" "$@"' > "$fake_bin/cp"

print -r -- '#!/usr/bin/env zsh
emulate -L zsh
setopt no_unset

print -r -- "$*" >> "$TEST_LOG"

case "${1:-} ${2:-}" in
    "worktree create")
        shift 2
        workspace= branch= base= checkout_path= label=
        no_focus=0
        while (( $# )); do
            case "$1" in
                --workspace)
                    (( $# >= 2 )) || exit 90
                    workspace=$2
                    shift 2
                    ;;
                --branch)
                    (( $# >= 2 )) || exit 90
                    branch=$2
                    shift 2
                    ;;
                --base)
                    (( $# >= 2 )) || exit 90
                    base=$2
                    shift 2
                    ;;
                --path)
                    (( $# >= 2 )) || exit 90
                    checkout_path=$2
                    shift 2
                    ;;
                --label)
                    (( $# >= 2 )) || exit 90
                    label=$2
                    shift 2
                    ;;
                --no-focus)
                    no_focus=1
                    shift
                    ;;
                *)
                    exit 90
                    ;;
            esac
        done

        [[ "$workspace" == "$EXPECTED_WORKSPACE_ID" ]] || exit 91
        [[ "$checkout_path" == "$EXPECTED_TARGET" ]] || exit 92
        [[ "$label" == "$branch" ]] || exit 93
        (( no_focus )) || exit 94
        if [[ -n "${FAKE_HERDR_CREATE_STATUS:-}" ]]; then
            print -u2 -r -- "${FAKE_HERDR_CREATE_STDERR:-fake create failure}"
            exit "$FAKE_HERDR_CREATE_STATUS"
        fi
        git -C "$HERDR_SOURCE_ROOT" worktree add -q -b "$branch" "$checkout_path" "$base" || exit $?
        chmod 0755 "$checkout_path" || exit $?

        if [[ -n "${FAKE_HERDR_COLLISION_PATH:-}" ]]; then
            collision="$checkout_path/$FAKE_HERDR_COLLISION_PATH"
            mkdir -p -- "${collision:h}"
            print -r -- "${FAKE_HERDR_COLLISION_CONTENT:-collision}" > "$collision"
        fi
        ;;
    "worktree open")
        shift 2
        workspace= checkout_path=
        focus=0
        while (( $# )); do
            case "$1" in
                --workspace)
                    (( $# >= 2 )) || exit 95
                    workspace=$2
                    shift 2
                    ;;
                --path)
                    (( $# >= 2 )) || exit 95
                    checkout_path=$2
                    shift 2
                    ;;
                --focus)
                    focus=1
                    shift
                    ;;
                *)
                    exit 95
                    ;;
            esac
        done

        [[ "$workspace" == "$EXPECTED_WORKSPACE_ID" ]] || exit 96
        [[ "$checkout_path" == "$EXPECTED_TARGET" ]] || exit 97
        (( focus )) || exit 98
        if [[ -n "${FAKE_HERDR_EXPECT_COPY:-}" ]]; then
            [[ -f "$checkout_path/$FAKE_HERDR_EXPECT_COPY" ]] || exit 99
        fi
        if [[ -n "${FAKE_HERDR_OPEN_STATUS:-}" ]]; then
            print -u2 -r -- "${FAKE_HERDR_OPEN_STDERR:-fake open failure}"
            exit "$FAKE_HERDR_OPEN_STATUS"
        fi
        ;;
    *)
        exit 89
        ;;
esac' > "$fake_bin/herdr"
chmod +x "$fake_bin"/{git,cp,herdr}

setup_repo() {
    local name=$1
    local manifest=${2:-}
    local track_manifest=${3:-yes}

    CASE_DIR="$tmp_dir/$name case"
    SOURCE_ROOT="$CASE_DIR/source repo"
    ORIGIN="$CASE_DIR/origin.git"
    WORKTREE_ROOT="$CASE_DIR/worktree root"
    RUN_OUTPUT="$CASE_DIR/helper.out"
    EXPECTED_WORKSPACE_ID='workspace one'

    mkdir -p -- "$CASE_DIR" "$WORKTREE_ROOT"
    git init -q --bare "$ORIGIN" || fail "$name: cannot initialize origin"
    git init -q -b main "$SOURCE_ROOT" || fail "$name: cannot initialize source"
    git -C "$SOURCE_ROOT" config user.name Test
    git -C "$SOURCE_ROOT" config user.email test@example.com

    print -r -- base > "$SOURCE_ROOT/README.md"
    print -r -- '.env
.env.*
.envrc
.environment
certs/
private/
node_modules/
dist/' > "$SOURCE_ROOT/.gitignore"
    git -C "$SOURCE_ROOT" add README.md .gitignore
    if [[ "$track_manifest" == yes ]]; then
        print -r -- "$manifest" > "$SOURCE_ROOT/.worktree-copy"
        git -C "$SOURCE_ROOT" add .worktree-copy
    fi
    git -C "$SOURCE_ROOT" commit -q -m base || fail "$name: cannot create base commit"
    git -C "$SOURCE_ROOT" remote add origin "$ORIGIN"
    git -C "$SOURCE_ROOT" push -q -u origin main || fail "$name: cannot seed origin"
    if [[ "$track_manifest" != yes ]]; then
        print -r -- "$manifest" > "$SOURCE_ROOT/.worktree-copy"
    fi

    unset FAKE_HERDR_COLLISION_PATH FAKE_HERDR_COLLISION_CONTENT \
        FAKE_HERDR_EXPECT_COPY FAKE_GIT_FETCH_STATUS FAKE_GIT_FETCH_STDERR \
        FAKE_CP_STATUS FAKE_CP_STDERR FAKE_HERDR_CREATE_STATUS \
        FAKE_HERDR_CREATE_STDERR FAKE_HERDR_OPEN_STATUS \
        FAKE_HERDR_OPEN_STDERR 2>/dev/null || true
}

target_for_branch() {
    local branch=$1
    print -r -- "$WORKTREE_ROOT/${SOURCE_ROOT:t}/${branch//\//-}"
}

run_helper() {
    local branch=$1
    local base=$2

    EXPECTED_TARGET=$(target_for_branch "$branch")
    : > "$TEST_LOG"
    : > "$RUN_OUTPUT"
    printf '%s\n%s\n' "$branch" "$base" |
        env \
            PATH="$fake_bin:$PATH" \
            REAL_GIT="$real_git" \
            REAL_CP="$real_cp" \
            TEST_LOG="$TEST_LOG" \
            HERDR_ACTIVE_WORKSPACE_ID="$EXPECTED_WORKSPACE_ID" \
            HERDR_ACTIVE_PANE_CWD="$SOURCE_ROOT" \
            HERDR_WORKTREE_ROOT="$WORKTREE_ROOT" \
            HERDR_SOURCE_ROOT="$SOURCE_ROOT" \
            EXPECTED_WORKSPACE_ID="$EXPECTED_WORKSPACE_ID" \
            EXPECTED_TARGET="$EXPECTED_TARGET" \
            FAKE_HERDR_COLLISION_PATH="${FAKE_HERDR_COLLISION_PATH:-}" \
            FAKE_HERDR_COLLISION_CONTENT="${FAKE_HERDR_COLLISION_CONTENT:-}" \
            FAKE_HERDR_EXPECT_COPY="${FAKE_HERDR_EXPECT_COPY:-}" \
            FAKE_GIT_FETCH_STATUS="${FAKE_GIT_FETCH_STATUS:-}" \
            FAKE_GIT_FETCH_STDERR="${FAKE_GIT_FETCH_STDERR:-}" \
            FAKE_CP_STATUS="${FAKE_CP_STATUS:-}" \
            FAKE_CP_STDERR="${FAKE_CP_STDERR:-}" \
            FAKE_HERDR_CREATE_STATUS="${FAKE_HERDR_CREATE_STATUS:-}" \
            FAKE_HERDR_CREATE_STDERR="${FAKE_HERDR_CREATE_STDERR:-}" \
            FAKE_HERDR_OPEN_STATUS="${FAKE_HERDR_OPEN_STATUS:-}" \
            FAKE_HERDR_OPEN_STDERR="${FAKE_HERDR_OPEN_STDERR:-}" \
            "$helper" > "$RUN_OUTPUT" 2>&1
}

setup_repo happy $'certs/\nprivate/*.txt'
mkdir -p -- "$SOURCE_ROOT/certs" "$SOURCE_ROOT/private" \
    "$SOURCE_ROOT/node_modules/pkg" "$CASE_DIR/outside directory"
print -r -- notes > "$SOURCE_ROOT/notes.txt"
weird_name=$'line\nbreak.txt'
print -r -- odd > "$SOURCE_ROOT/$weird_name"
print -r -- env-secret > "$SOURCE_ROOT/.env.local"
print -r -- envrc-secret > "$SOURCE_ROOT/.envrc"
print -r -- not-an-env-file > "$SOURCE_ROOT/.environment"
print -r -- cert-secret > "$SOURCE_ROOT/certs/dev.pem"
ln -s -- '../../outside directory' "$SOURCE_ROOT/certs/outside-link"
print -r -- selected > "$SOURCE_ROOT/private/tool.txt"
print -r -- excluded > "$SOURCE_ROOT/private/tool.bin"
print -r -- dependency > "$SOURCE_ROOT/node_modules/pkg/index.js"
print -r -- modified > "$SOURCE_ROOT/README.md"

[[ -x "$helper" ]] || fail "worktree helper is missing: $helper"

FAKE_HERDR_EXPECT_COPY=notes.txt
run_helper codex/test HEAD || fail "happy path failed: $(<"$RUN_OUTPUT")"
target=$EXPECTED_TARGET
[[ -f "$target/notes.txt" ]] || fail "untracked file was not copied"
[[ -f "$target/$weird_name" ]] || fail "newline-bearing untracked path was not copied"
[[ "$(<"$target/.env.local")" == env-secret ]] || fail ".env.local was not copied"
[[ "$(<"$target/.envrc")" == envrc-secret ]] || fail ".envrc was not copied"
[[ -f "$target/certs/dev.pem" ]] || fail "manifest directory file was not copied"
[[ -L "$target/certs/outside-link" ]] || fail "manifest symlink was not preserved"
[[ "$(readlink "$target/certs/outside-link")" == '../../outside directory' ]] || \
    fail "manifest symlink target changed"
[[ -f "$target/private/tool.txt" ]] || fail "manifest glob file was not copied"
[[ ! -e "$target/private/tool.bin" ]] || fail "unmatched ignored file was copied"
[[ ! -e "$target/node_modules/pkg/index.js" ]] || fail "ignored dependency was copied"
[[ ! -e "$target/.environment" ]] || fail "non-matching .environment file was copied"
[[ "$(<"$target/README.md")" == base ]] || fail "modified tracked file replaced base content"
log=$(<"$TEST_LOG")
assert_contains "$log" 'worktree create' "Herdr create was not called"
assert_contains "$log" '--no-focus' "Herdr create must not focus"
assert_contains "$log" 'worktree open' "Herdr open was not called"
assert_contains "$log" '--focus' "Herdr open must focus after copying"
[[ "$log" == *'worktree create'*'worktree open'* ]] || fail "Herdr focused before creating"
output=$(<"$RUN_OUTPUT")
assert_not_contains "$output" env-secret "helper output leaked an environment secret"
assert_not_contains "$output" cert-secret "helper output leaked a manifest secret"

setup_repo symlink-glob 'link/*'
outside="$CASE_DIR/outside"
mkdir -p -- "$outside"
print -r -- outside-secret > "$outside/secret.txt"
ln -s -- "$outside" "$SOURCE_ROOT/link"
print -r -- link >> "$SOURCE_ROOT/.git/info/exclude"
run_helper codex/symlink-glob HEAD ||
    fail "ignored symlink glob followed its target: $(<"$RUN_OUTPUT")"
[[ ! -e "$EXPECTED_TARGET/link" && ! -L "$EXPECTED_TARGET/link" ]] ||
    fail "manifest glob copied through an ignored symlink directory"

setup_repo restrictive-permissions 'private/'
mkdir -p -- "$SOURCE_ROOT/private"
print -r -- secret > "$SOURCE_ROOT/private/secret.txt"
chmod 700 "$SOURCE_ROOT/private"
chmod 600 "$SOURCE_ROOT/private/secret.txt"
original_umask=$(umask)
umask 000
run_helper codex/restrictive-permissions HEAD
permission_status=$?
umask "$original_umask"
(( permission_status == 0 )) ||
    fail "restrictive permissions case failed: $(<"$RUN_OUTPUT")"
target_mode=$(zstat +mode "$EXPECTED_TARGET")
private_mode=$(zstat +mode "$EXPECTED_TARGET/private")
(( (target_mode & 8#77) == 0 )) ||
    fail "worktree root exposes group/other permissions"
(( (private_mode & 8#777) == 8#700 )) ||
    fail "copied 0700 directory mode changed (got: $(( private_mode & 8#777 )))"

setup_repo no-origin-default-base
git -C "$SOURCE_ROOT" remote remove origin
FAKE_GIT_FETCH_STATUS=42
run_helper codex/no-origin-default-base '' ||
    fail "no-origin default-base case failed: $(<"$RUN_OUTPUT")"
assert_contains "$(<"$TEST_LOG")" '--base HEAD' "blank base did not default to HEAD"

setup_repo invalid-branch
run_helper 'bad branch' HEAD
(( $? != 0 )) || fail "invalid branch was accepted"
assert_not_contains "$(<"$TEST_LOG")" 'worktree create' "invalid branch reached Herdr"

setup_repo invalid-base
pwned="$CASE_DIR/pwned"
run_helper codex/invalid-base "HEAD; touch $pwned; #"
(( $? != 0 )) || fail "invalid base was accepted"
[[ ! -e "$pwned" ]] || fail "base prompt text was evaluated"
assert_not_contains "$(<"$TEST_LOG")" 'worktree create' "invalid base reached Herdr"

for invalid_manifest in '../outside.txt' '/tmp/outside.txt' '.git/config'; do
    case_name="manifest-${invalid_manifest//[^A-Za-z0-9]/-}"
    setup_repo "$case_name"
    print -r -- "$invalid_manifest" > "$SOURCE_ROOT/.worktree-copy"
    run_helper "codex/$case_name" HEAD
    (( $? != 0 )) || fail "invalid manifest entry was accepted: $invalid_manifest"
    [[ ! -e "$EXPECTED_TARGET" && ! -L "$EXPECTED_TARGET" ]] ||
        fail "invalid manifest entry created a target: $invalid_manifest"
    assert_not_contains "$(<"$TEST_LOG")" 'worktree create' \
        "invalid manifest entry reached Herdr create: $invalid_manifest"
    assert_not_contains "$(<"$TEST_LOG")" 'worktree open' \
        "invalid manifest entry focused a worktree: $invalid_manifest"
done

setup_repo untracked-manifest 'private/ignored.txt' no
mkdir -p -- "$SOURCE_ROOT/private"
print -r -- ignored > "$SOURCE_ROOT/private/ignored.txt"
run_helper codex/untracked-manifest HEAD ||
    fail "untracked manifest case failed: $(<"$RUN_OUTPUT")"
[[ -f "$EXPECTED_TARGET/.worktree-copy" ]] ||
    fail "untracked manifest was not copied as an ordinary untracked file"
[[ ! -e "$EXPECTED_TARGET/private/ignored.txt" ]] ||
    fail "untracked manifest authorized an ignored file"

setup_repo existing-target
target=$(target_for_branch codex/existing)
mkdir -p -- "$target"
print -r -- keep > "$target/sentinel"
run_helper codex/existing HEAD
(( $? != 0 )) || fail "existing target was accepted"
[[ "$(<"$target/sentinel")" == keep ]] || fail "existing target data was overwritten"
assert_not_contains "$(<"$TEST_LOG")" 'worktree create' "existing target reached Herdr"

setup_repo fetch-failure
FAKE_GIT_FETCH_STATUS=42
FAKE_GIT_FETCH_STDERR=fetch-status-marker
run_helper codex/fetch-failure HEAD
helper_status=$?
(( helper_status == 42 )) || fail "fetch status changed (got: $helper_status, want: 42)"
assert_contains "$(<"$RUN_OUTPUT")" fetch-status-marker "fetch stderr was hidden"
[[ ! -e "$EXPECTED_TARGET" ]] || fail "fetch failure created a target"
assert_not_contains "$(<"$TEST_LOG")" 'worktree create' "fetch failure reached Herdr"

setup_repo create-failure
FAKE_HERDR_CREATE_STATUS=43
FAKE_HERDR_CREATE_STDERR=create-status-marker
run_helper codex/create-failure HEAD
helper_status=$?
(( helper_status == 43 )) || fail "create status changed (got: $helper_status, want: 43)"
assert_contains "$(<"$RUN_OUTPUT")" create-status-marker "create stderr was hidden"
[[ ! -e "$EXPECTED_TARGET" ]] || fail "failed Herdr create left a target"
assert_not_contains "$(<"$TEST_LOG")" 'worktree open' "failed Herdr create reached open"

setup_repo copy-failure
print -r -- local > "$SOURCE_ROOT/local.txt"
FAKE_CP_STATUS=44
FAKE_CP_STDERR=copy-status-marker
run_helper codex/copy-failure HEAD
helper_status=$?
(( helper_status == 44 )) || fail "copy status changed (got: $helper_status, want: 44)"
assert_contains "$(<"$RUN_OUTPUT")" copy-status-marker "copy stderr was hidden"
[[ -d "$EXPECTED_TARGET" ]] || fail "copy failure removed the created checkout"
assert_not_contains "$(<"$TEST_LOG")" 'worktree open' "copy failure reached open"

setup_repo open-failure
print -r -- local > "$SOURCE_ROOT/local.txt"
FAKE_HERDR_EXPECT_COPY=local.txt
FAKE_HERDR_OPEN_STATUS=45
FAKE_HERDR_OPEN_STDERR=open-status-marker
run_helper codex/open-failure HEAD
helper_status=$?
(( helper_status == 45 )) || fail "open status changed (got: $helper_status, want: 45)"
assert_contains "$(<"$RUN_OUTPUT")" open-status-marker "open stderr was hidden"
[[ -f "$EXPECTED_TARGET/local.txt" ]] || fail "open failure removed copied content"

setup_repo destination-collision
print -r -- first > "$SOURCE_ROOT/a-first.txt"
print -r -- source-secret > "$SOURCE_ROOT/z-collision.txt"
FAKE_HERDR_COLLISION_PATH=z-collision.txt
FAKE_HERDR_COLLISION_CONTENT=destination-data
run_helper codex/collision HEAD
(( $? != 0 )) || fail "destination collision was overwritten"
target=$EXPECTED_TARGET
[[ -d "$target" ]] || fail "failed bootstrap removed the created checkout"
[[ "$(<"$target/z-collision.txt")" == destination-data ]] || \
    fail "destination collision content was overwritten"
[[ ! -e "$target/a-first.txt" ]] || fail "copy began before collision preflight completed"
assert_not_contains "$(<"$TEST_LOG")" 'worktree open' \
    "failed bootstrap focused the new worktree"
assert_not_contains "$(<"$RUN_OUTPUT")" source-secret \
    "collision failure leaked source contents"

print -- 'PASS: Herdr worktree workflow'
