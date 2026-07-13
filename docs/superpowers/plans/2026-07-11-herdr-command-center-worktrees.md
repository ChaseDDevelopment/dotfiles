# Herdr Command Center and Worktrees Implementation Plan

> **Partially superseded:** The startup and topology steps in this plan are
> replaced by the authoritative
> [Main tmux and Herdr command-center plan](./2026-07-12-main-tmux-herdr-command-center.md).
> Its `Ghostty -> tmux Main -> Herdr` model replaces the direct Ghostty-to-Herdr
> startup described below; the installer and worktree-safety tasks remain valid.

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the first Ghostty surface attach to a managed Herdr command center and add a safe Herdr-native worktree workflow that carries untracked work and allowlisted secrets.

**Architecture:** Ghostty uses initial-command only for its first surface, leaving later tabs as ordinary shells for remote Herdr clients. Dotfiles manage Herdr's config file plus one executable helper; Herdr continues to own runtime state, checkout creation, grouping, and removal. The helper adds fetch, validation, copy bootstrap, and focus-after-success behavior.

**Tech Stack:** Ghostty 1.3, Herdr 0.7.3 TOML/CLI, zsh, Git, and the existing Go installer.

## Global Constraints

- Do not add or update dependencies or lockfiles.
- Do not commit, push, or rewrite Git history.
- Preserve unrelated dirty-worktree changes; only the unfinished cmux workflow is explicitly superseded.
- Keep the carried-forward explicit tmux recovery slice in scope: `t`/`ts`,
  default and named `ssht` sessions, login-shell auto-attach removal, and
  recovery-client window sizing. Do not expand it beyond the existing diff.
- Never symlink the whole ~/.config/herdr directory.
- Do not enable nested Herdr, timed deletion, automatic or unconfirmed force,
  or branch deletion. Herdr may offer force only after a second explicit confirmation
  when normal safe removal refuses a dirty checkout.
- Never overwrite copied destinations or print secret contents.
- Do not contact or modify any remote host.
- Use tests before each behavior-bearing shell or Go change.

---

### Task 1: Register Herdr and Its File-Only Configuration

**Files:**
- Create: configs/herdr/config.toml
- Modify: installer/internal/registry/cli_tools.go
- Modify: installer/internal/registry/catalog_test.go
- Modify: installer/internal/config/components.go
- Modify: installer/internal/config/symlinks.go
- Modify: installer/internal/config/symlinks_test.go

**Interfaces:**
- Produces tool command herdr, component Herdr, config target $HOME/.config/herdr/config.toml, and helper target $HOME/.local/bin/herdr-worktree-create.
- Consumes existing Tool, InstallStrategy, Component, and SymlinkEntry types.

- [ ] **Step 1: Add failing registry assertions**

Add to TestCliToolsCatalog:

    herdr := toolByCommand(t, tools, "herdr")
    if len(herdr.Strategies) != 2 {
        t.Fatalf("herdr strategies = %#v", herdr.Strategies)
    }
    if got := herdr.Strategies[0]; got.Method != MethodPackageManager || got.Package != "herdr" {
        t.Fatalf("herdr brew strategy = %#v", got)
    }
    if got := herdr.Strategies[1]; got.Method != MethodScript || got.Script == nil ||
        got.Script.URL != "https://herdr.dev/install.sh" {
        t.Fatalf("herdr script strategy = %#v", got)
    }

- [ ] **Step 2: Add a failing file-boundary assertion**

Import reflect in symlinks_test.go and add:

    func TestHerdrComponentOwnsOnlyConfigAndHelper(t *testing.T) {
        want := map[string]string{
            "herdr/config.toml": "$HOME/.config/herdr/config.toml",
            "herdr/scripts/herdr-worktree-create": "$HOME/.local/bin/herdr-worktree-create",
        }
        got := map[string]string{}
        for _, entry := range AllSymlinks() {
            if entry.Component == "Herdr" {
                got[entry.Source] = entry.Target
                if entry.IsDir {
                    t.Fatalf("Herdr must not symlink a directory: %#v", entry)
                }
            }
        }
        if !reflect.DeepEqual(got, want) {
            t.Fatalf("Herdr symlinks = %#v, want %#v", got, want)
        }
    }

- [ ] **Step 3: Run the focused tests and confirm RED**

Run:

    cd installer
    go test ./internal/registry ./internal/config

Expected: the registry test cannot find herdr and the symlink map is empty.

- [ ] **Step 4: Add minimal installer entries**

Add this tool to cliTools():

    {
        Name: "herdr", Command: "herdr",
        Description: "Terminal workspace manager for AI coding agents",
        Strategies: []InstallStrategy{
            {Managers: []string{"brew"}, Method: MethodPackageManager, Package: "herdr"},
            {Method: MethodScript, Script: &ScriptConfig{
                URL: "https://herdr.dev/install.sh", Shell: "sh", NoProfileModify: true,
            }},
        },
    },

Add {Name: "Herdr", Icon: "H", RequiredCmd: "herdr"} to AllComponents(), and add two non-directory symlinks:

    {Source: "herdr/config.toml", Target: "$HOME/.config/herdr/config.toml", IsDir: false, Component: "Herdr"},
    {Source: "herdr/scripts/herdr-worktree-create", Target: "$HOME/.local/bin/herdr-worktree-create", IsDir: false, Component: "Herdr"},

- [ ] **Step 5: Create the approved Herdr config**

Create configs/herdr/config.toml with the existing Tokyo Night/system-toast settings, then:

    [keys]
    prefix = "ctrl+space"
    detach = ["prefix+q", "prefix+d"]
    previous_tab = ["prefix+p", "alt+shift+h"]
    next_tab = ["prefix+n", "alt+shift+l"]
    new_worktree = ""
    open_worktree = "prefix+shift+o"
    remove_worktree = "prefix+alt+d"

    [[keys.command]]
    key = "prefix+shift+g"
    type = "pane"
    command = "herdr-worktree-create"
    description = "create worktree with local files"

    [worktrees]
    directory = "~/.herdr/worktrees"

    [remote]
    manage_ssh_config = true

    [experimental]
    pane_history = false
    allow_nested = false

- [ ] **Step 6: Format and confirm GREEN**

Run:

    gofmt -w installer/internal/registry/cli_tools.go installer/internal/registry/catalog_test.go
    gofmt -w installer/internal/config/components.go installer/internal/config/symlinks.go installer/internal/config/symlinks_test.go
    cd installer && go test ./internal/registry ./internal/config

Expected: PASS.

---

### Task 2: Build the Worktree Helper Test-First

**Files:**
- Create: tests/herdr-worktree.zsh
- Create: configs/herdr/scripts/herdr-worktree-create

**Interfaces:**
- Consumes HERDR_ACTIVE_WORKSPACE_ID, HERDR_ACTIVE_PANE_CWD, herdr worktree create, and herdr worktree open.
- Produces an executable helper and optional tracked .worktree-copy manifests
  with one root-anchored Git glob pathspec per line.

- [ ] **Step 1: Write a failing isolated behavior test**

The test creates a temporary source repository and bare origin, stubs herdr on PATH, and pipes branch codex/test plus base HEAD to the helper. The fake herdr parses --branch, --base, and --path, runs a real git worktree add, and logs the later open/focus call.

Assert:

    [[ -f "$target/notes.txt" ]]
    [[ "$(<"$target/.env.local")" == secret ]]
    [[ -f "$target/certs/dev.pem" ]]
    [[ ! -e "$target/node_modules/pkg/index.js" ]]
    assert_contains "$(<"$TEST_LOG")" "worktree open" "focus after copy"

Add cases proving invalid branch, invalid base, manifest parent traversal,
existing target, fetch failure, and destination collision all return non-zero
without overwriting data. Cover root-anchored manifest matching, refusal to
traverse symlink directories, leaf-symlink preservation, and private target and
copy-created directory modes.

- [ ] **Step 2: Run and confirm RED**

Run:

    zsh tests/herdr-worktree.zsh

Expected: FAIL because configs/herdr/scripts/herdr-worktree-create is missing.

- [ ] **Step 3: Implement safe input and Git mutation boundaries**

Start the helper with:

    #!/usr/bin/env zsh
    emulate -L zsh
    setopt pipe_fail no_unset

    fail() {
        print -u2 -- "herdr-worktree-create: $*"
        if [[ -t 0 && -t 1 ]]; then
            read -r "?Press Enter to close..."
        fi
        return 1
    }

    workspace_id=${HERDR_ACTIVE_WORKSPACE_ID:-}
    start_dir=${HERDR_ACTIVE_PANE_CWD:-$PWD}
    [[ -n "$workspace_id" ]] || fail "run this from a Herdr workspace" || exit $?
    source_root=$(git -C "$start_dir" rev-parse --show-toplevel 2>/dev/null) ||
        { fail "active workspace is not a Git repository"; exit $?; }

Fetch origin when present, prompt branch/base, validate with git check-ref-format --branch and git rev-parse --verify, slug slash as dash, and reject an existing target beneath ${HERDR_WORKTREE_ROOT:-$HOME/.herdr/worktrees}.

- [ ] **Step 4: Implement copy collection and validation**

Use:
- git ls-files --others --exclude-standard -z for all untracked non-ignored files;
- root .env, .env.*, and .envrc candidates;
- tracked .worktree-copy lines after rejecting absolute paths, parent
  components, and .git;
- root-anchored Git glob pathspec matching for ignored manifest candidates, so
  ordinary `*` does not cross `/`;
- recursive matching only for explicit ignored directory entries, without
  traversing symlink directories;
- an associative array for de-duplication;
- Git tracking checks to skip tracked files.

Preflight every destination before copying. Preserve symlink objects with
cp -P and regular-file source modes with cp -p. Use a private umask for new
parents and remove group/other access from Herdr's target root so the copied
worktree stays private.

- [ ] **Step 5: Create without focus, copy, then focus**

Use:

    herdr worktree create         --workspace "$workspace_id"         --branch "$branch"         --base "$base"         --path "$target"         --label "$branch"         --no-focus >/dev/null

After successful copies:

    herdr worktree open         --workspace "$workspace_id"         --path "$target"         --focus >/dev/null

On a copy failure leave the checkout in place, keep the source focused, print only paths/errors, and return non-zero.

- [ ] **Step 6: Confirm GREEN**

Run:

    zsh -n configs/herdr/scripts/herdr-worktree-create
    zsh tests/herdr-worktree.zsh

Expected: PASS: Herdr worktree workflow.

---

### Task 3: Add Ghostty Startup and Carry Forward Explicit Tmux Recovery

**Files:**
- Create: configs/zsh/functions/herdr.zsh
- Create: configs/zsh/functions/tmux.zsh
- Modify: configs/ghostty/config
- Modify: configs/zsh/.zprofile
- Modify: configs/zsh/functions/ssh.zsh
- Delete: configs/zsh/local.zprofile.example
- Modify: configs/tmux/tmux.conf
- Modify: tests/session-workflow.zsh

**Interfaces:**
- Produces hr HOST [HERDR_OPTIONS...].
- Produces `t [session-name]` and `ts` as explicit local tmux fallbacks.
- Expands `ssht [-c|--caffeinate] [-s|--session NAME] HOST` while preserving
  the default `Main` remote session.
- Leaves login shells outside tmux until one of those helpers is called, and
  keeps tmux 2.9+ recovery windows sized to the largest attached client.

- [ ] **Step 1: Replace cmux assertions with failing Herdr assertions**

Remove the fake cmux binary and cmt assertion. Add a fake herdr, source herdr.zsh, and verify:

    hr hydra --session agents
    assert_contains "$(<"$TEST_LOG")" "--remote hydra --session agents" "remote attach"

    HERDR_ENV=1 hr hydra >/dev/null 2>&1
    [[ $? -ne 0 ]] || fail "hr must reject a nested launch"
    unset HERDR_ENV

Also assert the carried-forward recovery behavior already present in the
working tree:

- `t` derives a default name from the Git worktree/current directory,
  normalizes explicit names, attaches outside tmux, and switches clients rather
  than nesting inside tmux;
- `ts` uses `choose-tree` inside tmux and `fzf` outside it;
- `ssht` defaults to `Main`, supports validated named sessions, preserves
  caffeination, and returns the real SSH status after terminal cleanup;
- `.zprofile` contains no tmux or Herdr auto-attach and the obsolete
  `local.zprofile.example` is gone;
- tmux's `window-size largest` recovery setting is guarded for tmux 2.9+.

Also require Ghostty initial-command, reject an all-surface command setting,
and require README documentation for hr hydra instead of cmt hydra.

- [ ] **Step 2: Run and confirm RED**

Run:

    zsh tests/session-workflow.zsh

Expected: FAIL because configs/zsh/functions/herdr.zsh is missing.

- [ ] **Step 3: Implement hr and retain explicit tmux recovery**

Create:

    function hr() {
        if (( $# == 0 )); then
            echo "Usage: hr <host> [herdr-remote-options...]"
            return 2
        fi
        if ! (( $+commands[herdr] )); then
            echo "hr: herdr is not installed"
            return 127
        fi
        if [[ "${HERDR_ENV:-}" == "1" ]]; then
            echo "hr: open a plain Ghostty tab first; nested Herdr is intentionally disabled"
            return 2
        fi
        command herdr --remote "$@"
    }
    compdef hr=ssh

Delete the cmt alias block from ssh.zsh and retain the carried-forward explicit
tmux recovery workflow:

- `tmux.zsh` provides `t [session-name]` and `ts` without automatic attachment;
- `ssh.zsh` parses `-c` and `-s NAME`, validates session names before SSH,
  defaults to `Main`, uses the remote `tmux-session` helper with a direct tmux
  fallback, and preserves SSH's exit status after `stty sane`;
- `.zprofile` removes the login-shell auto-attach block and its machine-local
  override hook, so delete `local.zprofile.example` as obsolete;
- `tmux.conf` applies `window-size largest` only on tmux 2.9 and newer, retaining
  larger desktop sizing when a smaller recovery client attaches.

- [ ] **Step 4: Configure first-surface-only startup**

Add to Ghostty:

    initial-command = /bin/zsh -lc 'command -v herdr >/dev/null 2>&1 && exec herdr; exec /bin/zsh -l'

Do not set Ghostty command, so subsequent tabs keep the default shell.

- [ ] **Step 5: Confirm GREEN**

Run:

    zsh -n configs/zsh/.zprofile configs/zsh/functions/herdr.zsh configs/zsh/functions/ssh.zsh configs/zsh/functions/tmux.zsh
    zsh tests/session-workflow.zsh
    /Applications/Ghostty.app/Contents/MacOS/ghostty +validate-config --config-file=configs/ghostty/config

Expected: session workflow PASS and Ghostty validation exit 0.

---

### Task 4: Replace Obsolete Documentation and Verify Everything

**Files:**
- Modify: README.md
- Delete: docs/superpowers/specs/2026-07-10-remote-first-cmux-tmux-design.md
- Delete: docs/superpowers/plans/2026-07-11-remote-first-cmux-tmux.md
- Preserve: the approved Herdr spec and this plan.

**Interfaces:**
- Documents local startup, hr, ssh then herdr, tmux fallback, .worktree-copy, and safe removal.

- [ ] **Step 1: Rewrite the README workflow**

Document:
- first fresh Ghostty surface attaches local Herdr;
- later Cmd+T tabs are plain shells;
- herdr, hr hydra, ssh hydra then herdr, and ssht hydra;
- Ctrl+Space and the compatible keymap;
- Prefix+Shift+G worktree creation;
- automatic untracked copy plus .env*/.envrc/.worktree-copy;
- manual safe-by-default removal with branch retention, including that any
  force offered after a failed safe removal requires Herdr's separate explicit
  confirmation and is never automatic;
- root-anchored, non-symlink-following ignored manifest matching and private
  target/copy-created directory permissions;
- explicit `t`/`ts` and default/named `ssht` recovery paths, with no login-shell
  auto-attach and largest-client tmux recovery sizing.

Remove every cmux/cmt workflow claim while retaining a short tmux fallback section.

- [ ] **Step 2: Delete superseded cmux documents**

Use apply_patch deletions and keep the new Herdr documents.

- [ ] **Step 3: Run proportional verification**

Run:

    zsh -n configs/herdr/scripts/herdr-worktree-create configs/zsh/.zprofile configs/zsh/functions/herdr.zsh configs/zsh/functions/ssh.zsh configs/zsh/functions/tmux.zsh
    zsh tests/herdr-worktree.zsh
    zsh tests/session-workflow.zsh
    /Applications/Ghostty.app/Contents/MacOS/ghostty +validate-config --config-file=configs/ghostty/config
    cd installer && gofmt -w internal/registry/cli_tools.go internal/registry/catalog_test.go internal/config/components.go internal/config/symlinks.go internal/config/symlinks_test.go
    cd installer && go test ./...
    cd installer && go vet ./...
    git diff --check

Expected: all commands exit 0 and both zsh behavior tests print PASS.

- [ ] **Step 4: Apply and verify the local config safely**

Back up the current regular ~/.config/herdr/config.toml, install the two file symlinks through the existing installer path or safe explicit linking, and run:

    herdr server reload-config

Expected: exit 0 and Prefix+? displays Ctrl+Space. Do not start a second local Herdr client inside the active pane.

- [ ] **Step 5: Review without committing**

Run git status --short and git diff --stat. Confirm no lockfile, remote-host, or unrelated config changes. Leave the working tree for user review.
