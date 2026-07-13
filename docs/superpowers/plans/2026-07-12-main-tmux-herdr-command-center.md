# Main Tmux and Herdr Command Center Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use
> `superpowers:subagent-driven-development` to implement this plan task by task.
> Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Restore the local tmux `Main` entry point as a fast machine switcher,
with one Herdr client per named tmux window and Herdr owning projects, tabs,
panes, agents, and durable work.

**Architecture:** Ghostty's first surface runs a tiny `tmux-main` bootstrap that
attaches or creates local session `Main`; a newly created `macbook` window starts
local Herdr and falls back to a shell when Herdr exits. `tw NAME` creates or
selects a stable named shell window, while `hw HOST` creates or selects a named
window running `herdr --remote HOST`. Tmux uses `Ctrl+B` and a native top status
bar; Herdr keeps `Ctrl+Space` and its own tab bar beneath tmux.

**Tech Stack:** Ghostty, tmux 2.9-3.7b, Herdr 0.7.3, zsh, TPM, and the existing
Go installer.

## Global Constraints

- Do not add or update dependencies or lockfiles.
- Do not commit, push, or rewrite Git history.
- Preserve all existing Herdr worktree behavior and tests.
- Preserve `ssh HOST`, `ssht HOST`, and `ssht -s NAME HOST` semantics.
- Keep `.zprofile` free of tmux and Herdr auto-attach logic.
- Do not contact or modify remote hosts.
- Keep TPM synchronous; remove only Powerkit and its now-dead installer workaround.
- Use tests before every behavior-bearing shell or Go change.
- Use native tmux status formatting; do not add another status plugin.

---

### Task 1: Bootstrap `Main` and Add Named-Window Helpers

**Files:**
- Create: `configs/tmux/scripts/tmux-main`
- Modify: `configs/ghostty/config`
- Modify: `configs/zsh/functions/tmux.zsh`
- Modify: `tests/session-workflow.zsh`

**Interfaces:**
- Produces executable `~/.config/tmux/scripts/tmux-main`.
- Produces `tw NAME` and `hw HOST`; both use the current outer session or local
  `Main` when called from a server-owned Herdr pane without `$TMUX`.
- `hw` reuses SSH completion and starts `herdr --remote HOST` in a separate
  outer tmux window, never inside the current Herdr pane.

- [x] **Step 1: Add failing workflow assertions**

  Extend `tests/session-workflow.zsh` to invoke `tmux-main` through the fake
  tmux binary and require `new-session -A -s Main -n macbook` plus a command
  that starts Herdr and returns to a login shell. Add assertions that:

  - `tw logs` rejects use when neither the current tmux session nor local
    `Main` is available;
  - `tw logs` and `hw hydra` resolve local `Main` from Herdr pane shells that do
    not inherit `$TMUX`;
  - inside tmux, `tw logs` selects `=Main:=logs` when present and otherwise
    creates a named window in `Main` at the current directory;
  - `hw hydra` selects `=Main:=hydra` when present and otherwise creates a
    named window running `herdr --remote hydra` with a shell fallback;
  - unsafe window/host names are rejected before invoking tmux;
  - `hw=ssh` completion is registered; and
  - Ghostty's `initial-command` invokes `tmux-main` rather than Herdr directly.

- [x] **Step 2: Run the workflow test and confirm RED**

  Run `zsh tests/session-workflow.zsh`.

  Expected: the initial implementation fails because `tmux-main`, `tw`, and
  `hw` do not exist and Ghostty still launches Herdr directly. The later Herdr
  pane regression fails until `tw` and `hw` can resolve local `Main` without
  `$TMUX`.

- [x] **Step 3: Implement the minimal bootstrap**

  Create `configs/tmux/scripts/tmux-main` as an executable zsh script. When
  tmux exists it executes:

  ```zsh
  tmux new-session -A -s Main -n macbook \
    "/bin/zsh -lc 'command -v herdr >/dev/null 2>&1 && herdr; exec /bin/zsh -l'"
  ```

  If tmux is unavailable, execute Herdr when present and otherwise a login
  shell. Change Ghostty `initial-command` to execute this script; do not add an
  all-surface `command` setting.

- [x] **Step 4: Implement `tw` and `hw`**

  In `configs/zsh/functions/tmux.zsh`, allow `[A-Za-z0-9._-]+` window names and
  require host names to begin alphanumerically, resolve the current tmux
  session when `$TMUX` is present, otherwise resolve exact local session `Main`,
  and first try the exact target `=<session>:=<name>`. `tw` creates a named
  shell window at `$PWD`; `hw` creates a named window whose command is
  `herdr --remote HOST` followed by a login-shell fallback. Register
  `compdef hw=ssh`.

- [x] **Step 5: Confirm GREEN**

  Run:

  ```sh
  zsh -n configs/tmux/scripts/tmux-main configs/zsh/functions/tmux.zsh
  zsh tests/session-workflow.zsh
  /Applications/Ghostty.app/Contents/MacOS/ghostty +validate-config \
    --config-file=configs/ghostty/config
  ```

  Expected: syntax/validation exit 0 and `PASS: session workflow`.

---

### Task 2: Separate Key Layers and Replace Powerkit

**Files:**
- Modify: `configs/tmux/tmux.conf`
- Modify: `configs/herdr/config.toml`
- Modify: `configs/tmux/scripts/tmux-cheatsheet.sh`
- Delete: `configs/tmux/scripts/prepend-ssh-host.sh`
- Delete: `configs/tmux/scripts/ssh-client-host.sh`
- Modify: `installer/internal/config/components.go`
- Modify: `installer/internal/config/components_setup_test.go`
- Modify: `tests/session-workflow.zsh`

**Interfaces:**
- Tmux prefix is `Ctrl+B`; Herdr prefix remains `Ctrl+Space`.
- `Alt+Shift+H/L` switches outer tmux windows only.
- Herdr previous/next tabs remain `Prefix+P/N`.
- The native tmux status bar is top-positioned and shows session, stable window
  names, current directory basename, and host without subprocess helpers.

- [x] **Step 1: Add failing configuration assertions**

  Require in `tests/session-workflow.zsh`:

  - tmux prefix `C-b`, `status-position top`, native TokyoNight status formats,
    and `herdr` in `@vim_navigator_pattern`;
  - no `tmux-powerkit`, `@powerkit_`, or `prepend-ssh-host` references;
  - Herdr prefix `ctrl+space` without direct `alt+shift+h/l` tab bindings; and
  - the cheatsheet documents `C-b`.

- [x] **Step 2: Run focused tests and confirm RED**

  Run:

  ```sh
  zsh tests/session-workflow.zsh
  cd installer && go test ./internal/config
  ```

  Expected: workflow assertions fail on the current prefix, Powerkit, bottom
  bar, and Herdr direct bindings.

- [x] **Step 3: Replace Powerkit with native tmux formats**

  Remove the Powerkit plugin and all `@powerkit_*` settings. Configure a native
  TokyoNight bar with `status-position top`, a green `Main` session pill, blue
  current-window pill, subdued inactive windows, and a mauve right pill using
  `#{b:pane_current_path}` and `#{host_short}`. Keep TPM synchronous and retain
  the existing non-status plugins.

  Remove the two Powerkit-specific SSH status scripts. The native host format
  replaces their subprocess and their styling dependency.

- [x] **Step 4: Separate tmux and Herdr input ownership**

  Explicitly reset tmux to `C-b` so a config reload updates existing servers,
  retain direct `M-H/M-L` outer-window navigation, add `herdr` to the navigator
  passthrough pattern, and remove Herdr's direct `alt+shift+h/l` bindings.
  Update the cheatsheet prefix text.

- [x] **Step 5: Remove the dead installer workaround**

  Delete `tmuxPluginSkipRecursive`, its pre-clone loop, and the Powerkit-only
  test. Ordinary missing/stale plugin installation remains unchanged; because
  Powerkit is no longer declared, the existing stale-plugin cleanup removes its
  on-disk clone during the next installer run.

- [x] **Step 6: Confirm GREEN and benchmark**

  Run:

  ```sh
  zsh tests/session-workflow.zsh
  cd installer && go test ./internal/config
  ```

  Start an isolated tmux server with the updated config and record real startup
  time. Expected: workflow/config tests pass and fresh tmux startup is below one
  second on this MacBook, versus the recorded 2.83-second baseline.

---

### Task 3: Reconcile Documentation and Verify the Full Working Tree

**Files:**
- Modify: `README.md`
- Modify: `docs/superpowers/specs/2026-07-11-herdr-command-center-worktrees-design.md`
- Preserve: all worktree helper/config/tests and the non-Herdr `ssht` fallback.

**Interfaces:**
- Documents `Main` as the local tmux entry, tmux windows as machines, and Herdr
  workspaces as projects on each machine.

- [x] **Step 1: Update the workflow documentation**

  Document the exact model:

  ```text
  Ghostty -> local tmux Main
    macbook -> herdr
    hydra   -> herdr --remote hydra
  ```

  Include `tw NAME`, `hw HOST`, `Ctrl+B`, `Ctrl+Space`, top-bar ownership,
  `ssh HOST`, `ssht HOST`, and the unchanged phone path `ssh HOST` then `herdr`.
  Preserve the complete approved worktree safety documentation.

- [x] **Step 2: Run full proportional verification**

  Run:

  ```sh
  zsh -n configs/tmux/scripts/tmux-main \
    configs/herdr/scripts/herdr-worktree-create \
    configs/zsh/.zprofile configs/zsh/functions/herdr.zsh \
    configs/zsh/functions/ssh.zsh configs/zsh/functions/tmux.zsh
  zsh tests/session-workflow.zsh
  zsh tests/herdr-worktree.zsh
  /Applications/Ghostty.app/Contents/MacOS/ghostty +validate-config \
    --config-file=configs/ghostty/config
  cd installer && gofmt -w internal/config/components.go \
    internal/config/components_setup_test.go
  cd installer && go test ./...
  cd installer && go vet ./...
  git diff --check
  ```

- [x] **Step 3: Validate live-safe pieces without killing sessions**

  Reload Herdr config if a server is running. Validate the tmux config in a
  disposable isolated server, but do not kill or restart the user's live tmux
  server; a fresh server is required to fully unload already-loaded Powerkit.

- [x] **Step 4: Review scope without committing**

  Inspect `git status --short`, `git diff --stat`, and the final relevant files.
  Confirm no remote host, lockfile, Mosh, cmux, or unrelated config changes.
