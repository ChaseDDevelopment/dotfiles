# Tmux Main, Herdr, and Worktree Workflow Design

## Goal

Make Ghostty enter local tmux session `Main`, use named tmux windows for
machines, and use Herdr workspaces for projects on each machine. Preserve plain
SSH and remote-tmux paths alongside the unchanged phone path. Add a
Herdr-integrated worktree workflow that keeps the useful Supacode behavior:
fresh remote refs, explicit branch/base selection, grouped workspaces,
untracked work in the new checkout, and selected ignored secrets such as
`.env.local`.

Tmux is the permanent local outer layer, not a migration fallback. Herdr owns
the project layer inside each machine window. No Mosh or cmux route is added to
this workflow.

## Decisions

- Ghostty launches or attaches local tmux session `Main` only for its first
  surface. Later Ghostty tabs remain ordinary shells.
- The first `Main` window is named `macbook` and runs the local Herdr session,
  returning to a login shell when Herdr exits.
- `tw NAME` and `hw HOST` use the current outer tmux session when one is present.
  From a local Herdr pane, whose server-owned shell does not inherit `$TMUX`,
  they target local session `Main`. `hw` runs `herdr --remote HOST`.
- Tmux owns the native top bar, machine-window switching, and `Ctrl+B`. Herdr
  owns project workspaces, tabs, panes, agents, and `Ctrl+Space`.
- `Alt+Shift+H/L` switches outer tmux windows only. Herdr keeps
  `Prefix+P/N` for its previous/next tabs.
- A bare `herdr` remains the local attach-or-create operation. `hr HOST` remains
  a direct `herdr --remote HOST` wrapper for a plain shell. No shell login hook
  and no Homebrew service are added.
- Plain `ssh HOST` and `ssht HOST` remain non-Herdr paths. `ssht` attaches or
  creates a remote tmux session, defaulting to `Main`.
- `ssh HOST` followed by `herdr` remains the phone and generic-terminal path.
- Nested Herdr is not enabled. `hw` creates the remote Herdr client in its own
  outer tmux window instead of the current Herdr pane.
- Herdr uses `Ctrl+Space` as its prefix and adopts only compatible tmux muscle
  memory. Herdr-native resize and pane-swap modes are retained.
- Herdr's own worktree API remains the owner of checkout creation, workspace
  grouping, focus, opening, and safe removal.
- One small repository-owned helper augments creation with fetch, prompts, and
  file copying. It is invoked from Herdr at `Prefix`, then `Shift+G`.
- Worktree removal is manual and safe by default. If safe removal rejects a
  dirty checkout, Herdr may offer force only after a separate explicit
  confirmation; dotfiles never schedules or triggers automatic or unconfirmed
  force. There is no archive scheduler, timed deletion, or automatic branch
  deletion.

## Architecture

```text
Ghostty -> local tmux Main
  macbook -> herdr
    workspaces -> tabs -> panes -> agents
    Prefix Shift+G -> herdr-worktree-create
  hydra   -> herdr --remote hydra
    remote workspaces -> tabs -> panes -> agents

Later Ghostty tab -> plain shell
  ssh HOST  -> remote shell
  ssht HOST -> remote tmux Main
  hr HOST   -> direct remote Herdr client

iPhone / generic SSH client
  ssh HOST -> herdr -> same host-owned persistent session
```

The local tmux server owns `Main` and its machine windows. Each machine owns its
own Herdr server, workspaces, panes, agents, sockets, and session state. Remote
sessions do not merge into the MacBook's local Herdr sidebar. `herdr --remote`
renders the remote machine's full Herdr UI inside its named outer window and
preserves local keybindings and local clipboard-image bridging.

## Ghostty Startup

Ghostty's `initial-command` starts a login zsh and replaces it with
`~/.config/tmux/scripts/tmux-main`. When tmux is available, the helper executes
`tmux new-session -A -s Main -n macbook` with a window command that starts local
Herdr and returns to a login shell when Herdr exits. An existing `Main` session
is attached instead of duplicated.

If tmux is unavailable, the helper executes Herdr when present and otherwise a
login shell. If tmux exists but Herdr does not, the `macbook` window remains a
usable login shell.

`initial-command`, rather than Ghostty's all-surface `command` or a zsh
auto-attach hook, is intentional:

- the first surface enters the local `Main` command center automatically;
- `Cmd+T` still creates a plain shell for one-off commands and direct clients;
- Herdr-created login shells do not recursively attach another outer tmux;
- SSH, IDE terminals, scripts, and non-Ghostty shells remain ordinary.

The existing removal of tmux login-shell auto-attach remains in place.

## Main and Layer Ownership

`Main` is the normal local entry point. Its native TokyoNight status bar is at
the top and shows the session, stable named windows, current directory basename,
and host without per-refresh subprocesses. Tmux uses `Ctrl+B`; direct
`Alt+Shift+H/L` switches the outer machine windows. `tw NAME` selects an exact
existing name or creates a named shell window at the current directory.
`hw HOST` does the same for a window whose command is `herdr --remote HOST`,
with a login-shell fallback after the client exits.

Herdr is the inner layer. Its workspaces are projects on the machine represented
by the current tmux window. It uses `Ctrl+Space`, with `Prefix+P/N` for its own
tabs. Herdr does not bind direct `Alt+Shift+H/L`, so those keys always remain
owned by outer tmux.

## Herdr Configuration and State

Dotfiles manage only `~/.config/herdr/config.toml`. Herdr also stores sockets,
logs, `session.json`, and named-session state under `~/.config/herdr`; those
runtime files remain machine-local and must never be symlinked into Git.

The managed config preserves the current Tokyo Night theme, system toasts,
pane-border agent labels, and disabled pane-history persistence. It adds:

- `[worktrees].directory = "~/.herdr/worktrees"`;
- `keys.prefix = "ctrl+space"`;
- both `Prefix+Q` and tmux-style `Prefix+D` for detach;
- existing Herdr/tmux-compatible `c`, `n`, `p`, `1..9`, `h/j/k/l`, `x`, `z`,
  copy mode, help, and `-` bindings;
- `Prefix+P` and `Prefix+N` for previous/next tabs, without direct
  `Alt+Shift+H/L` bindings that would collide with outer tmux;
- `Prefix+Shift+O` for Herdr's native open-worktree action and
  `Prefix+Alt+D` for its confirmed native remove-worktree action;
- a custom `Prefix+Shift+G` command for the augmented creation helper.

The helper replaces only Herdr's default new-worktree binding. Herdr keeps
`Prefix+R` for resize mode and `Prefix+Shift+H/J/K/L` for pane swaps; tmux's
conflicting reload/resize bindings are not recreated. Tmux plugins, capture
last output, Floax, Extrakto, TPM bindings, and smart Vim/tmux navigation are
not ported unless the Herdr pilot demonstrates a real missing workflow.

## Remote Commands

`hw HOST` is the normal MacBook path to another machine's Herdr server. It
reuses SSH host completion, selects an existing exact-name outer window when
present, and otherwise creates that window with `herdr --remote HOST`.

`hr HOST [HERDR_OPTIONS...]` remains the direct wrapper for a plain shell. It
expands to `herdr --remote HOST` while preserving Herdr's option handling and
reusing SSH host completion. For example:

```text
hr hydra
hr hydra --session agents
```

Herdr uses the local config's keybindings for remote attach by default, so the
MacBook keeps the same inner-layer muscle memory even before the server receives
updated dotfiles. Once servers are synchronized, `ssh HOST` followed by `herdr`
also uses the same config; this remains the phone and generic-terminal workflow.

Plain `ssh HOST` opens a remote shell without Herdr or remote tmux. `ssht HOST`
opens remote tmux `Main`, and `ssht -s NAME HOST` opens a named remote tmux
session; both are deliberately non-Herdr paths. The existing `t`/`ts` helpers
remain available for additional local tmux sessions. Mosh and cmux are not part
of this command-center model.

## Worktree Creation

`herdr-worktree-create` runs as a temporary Herdr command pane and receives the
active workspace context from Herdr. It performs this sequence:

1. Resolve the active Git worktree root. Refuse to run outside a Git workspace.
2. If an `origin` remote exists, run `git fetch --prune origin`. A failed fetch
   aborts before creating anything. Repositories without `origin` continue.
3. Prompt for a required branch name and an optional base ref. The base defaults
   to `HEAD`. Validate the branch with `git check-ref-format --branch` and the
   base as an existing commit; never evaluate prompt text as shell code.
4. Derive a checkout path beneath
   `~/.herdr/worktrees/<repo>/<branch-slug>` and refuse an existing path.
5. Ask Herdr to create the worktree without focusing it, using its CLI/API with
   explicit, quoted arguments. Herdr remains responsible for the Git checkout
   and for grouping the new workspace under its source workspace.
6. Copy the approved source files into the new checkout and print a path/count
   summary without printing file contents.
7. Focus the new Herdr workspace only after its bootstrap succeeds.

If creation succeeds but a later copy fails, the helper leaves the safely
created worktree in place, keeps focus on the source workspace, reports the
exact failure, and returns non-zero. It does not force-remove a partially
prepared checkout or claim success.

## Worktree Copy Policy

The source is the worktree from which creation was requested, not necessarily
the repository's primary checkout.

Two classes of files are copied as independent snapshots:

1. Every untracked, non-ignored file reported by Git.
2. Ignored secret/local files matching the built-in root patterns `.env`,
   `.env.*`, and `.envrc`, plus entries in an optional tracked
   `.worktree-copy` manifest.

`.worktree-copy` contains one root-anchored Git glob pathspec per line. Blank
lines and lines beginning with `#` are ignored. The manifest records only paths,
never secret values. Absolute paths, parent traversal, and anything resolving
outside the source repository are rejected. Explicit directory entries copy
their contained files recursively; `.git` is never copied.

Ignored dependency trees, build products, and caches such as `node_modules`,
`.next`, `dist`, and cache directories are not copied unless the repository
explicitly names them in `.worktree-copy`. Manifest entries are root-anchored
Git glob pathspecs: `*` does not cross `/`, explicit ignored directories
recurse, and matching never traverses a symlink directory. A matching symlink
object may be copied, but its target is never followed. Existing destination
files are never overwritten. Regular files preserve their source mode, while
the target root and copy-created parent directories remove group/other access
so the copied worktree stays private. Secret contents are never logged.

This is one-time bootstrap, not synchronization. Later edits in either
worktree do not propagate to the other automatically. Modified tracked files
from the source worktree are also not copied; the new checkout receives tracked
content from the selected base commit.

## Worktree Removal

Herdr's native `Delete worktree checkout...` action owns removal. Normal
removal asks Git to remove safely; a dirty or untracked checkout is not silently
discarded. If that safe attempt is refused, Herdr can offer a force action only
behind a second explicit confirmation. Dotfiles never invokes force
automatically or without that confirmation. Closing a Herdr workspace removes
only UI state and does not delete its checkout.

The workflow does not reproduce Supacode's archive state, seven-day cleanup,
or delete-local-branch setting. Local and remote branches remain until the user
deletes them explicitly.

## Installer and Server Rollout

Herdr becomes a normal dotfiles tool and component:

- Homebrew uses the official `herdr` formula;
- other supported Linux/macOS installations use Herdr's official installer as
  the fallback strategy;
- the component symlinks only `config.toml` and the helper executable;
- no Herdr background service is installed because the client starts or
  attaches its server automatically.

This local implementation does not connect to Hydra or mutate any server.
After local verification, a separate runbook will synchronize the dotfiles and
install Herdr on Hydra and selected Linux workers. Remote `herdr --remote`
bootstrap may be used for a pilot, but managed installation remains the final
server state.

Agent-specific Herdr integrations are not installed automatically in this
slice. Current foreground-process/output detection is sufficient for the
pilot; semantic integrations can be added individually if the pilot exposes
missed or stale agent states.

## Files

- `configs/herdr/config.toml`: managed Herdr UI, key, remote, and worktree
  configuration.
- `configs/herdr/scripts/herdr-worktree-create`: augmented native worktree
  creation helper.
- `configs/tmux/scripts/tmux-main`: attach-or-create `Main`, start local Herdr
  in initial window `macbook`, and retain usable fallbacks.
- `configs/ghostty/config`: first-surface `tmux-main` startup.
- `configs/zsh/functions/herdr.zsh`: `hr` wrapper and completion.
- `configs/zsh/functions/ssh.zsh`: carry forward non-Herdr `ssht`
  default/named tmux sessions, safe session-name validation, caffeination, and
  SSH exit-status preservation.
- `configs/zsh/functions/tmux.zsh`: provide `tw NAME` and `hw HOST` for outer
  named windows, plus explicit `t [session-name]` and interactive `ts` helpers.
- `configs/zsh/.zprofile`: remove the login-shell tmux auto-attach handoff.
- `configs/zsh/local.zprofile.example`: delete the now-obsolete auto-attach
  override example.
- `configs/tmux/tmux.conf`: own the `Ctrl+B` outer layer, direct machine-window
  navigation, native top status bar, and tmux 2.9+ `window-size largest`
  setting.
- `installer/internal/registry/cli_tools.go`: register Herdr installation.
- `installer/internal/config/components.go`: register the Herdr component.
- `installer/internal/config/symlinks.go`: link only the config file and helper.
- focused installer tests: cover the registry, component, and symlink entries.
- `tests/herdr-worktree.zsh`: exercise creation/copy safety in temporary local
  Git repositories.
- `tests/session-workflow.zsh`: cover `Main`, named windows, separate key
  layers, direct remote Herdr, and non-Herdr tmux paths.
- `README.md`: document the outer tmux and inner Herdr model, key ownership,
  remote choices, and worktrees.

## Verification

- Format touched Go and shell/config files with their existing project tools.
- Run zsh syntax checks for the helper and shell functions.
- Run the focused worktree behavior script against temporary repositories and
  a local bare `origin`; it must cover fetch failure, branch/base validation,
  untracked copying, default `.env*` copying, manifest paths, excluded build
  directories, path traversal rejection, and no-overwrite behavior.
- Validate Ghostty configuration with Ghostty's config validator.
- Reload the running Herdr server configuration and confirm it accepts the
  managed keymap; reattach clients because remote keybindings are snapshotted
  at attach time.
- Load the complete tmux config on a fresh disposable `tmux -L` server so
  already-loaded plugins from a live server cannot mask the result. Do not
  restart, kill, or reload the user's live tmux server.
- Benchmark startup on that isolated server against the recorded 2.83-second
  Powerkit baseline.
- Run installer `go test ./...` and `go vet ./...` from `installer/`.
- Review `git diff --check` and the final working-tree scope.

## Acceptance Criteria

1. Starting a fresh Ghostty process opens or attaches local tmux session `Main`.
2. The initial `macbook` window runs local Herdr and returns to a usable login
   shell if Herdr is missing or exits; later `Cmd+T` surfaces remain plain.
3. `tw NAME` selects or creates an exact-name outer shell window, and `hw HOST`
   selects or creates an exact-name outer window running `herdr --remote HOST`;
   both resolve local `Main` when called from a local Herdr pane without `$TMUX`.
4. The native tmux top bar owns machine-window context. Outer tmux uses
   `Ctrl+B` and direct `Alt+Shift+H/L`; inner Herdr uses `Ctrl+Space` and
   `Prefix+P/N` without binding collisions.
5. Tmux windows represent machines, while Herdr workspaces represent projects
   on the current machine.
6. `ssh HOST` and `ssht HOST` remain non-Herdr paths. A phone reaches Herdr with
   `ssh HOST` followed by `herdr`; no Mosh or cmux route is introduced.
7. `Prefix+Shift+G` fetches `origin`, validates branch/base input, and creates a
   focused native Herdr worktree group under the configured directory.
8. All untracked non-ignored files copy from the requesting worktree.
9. `.env*` and valid root-anchored `.worktree-copy` Git pathspecs copy without
   traversing symlink directories or copying unrelated ignored dependency/build
   trees.
10. Copying never escapes the repository, overwrites checkout files, exposes
    the private target through group/other directory permissions, or prints
    secret contents.
11. Manual Herdr removal is safe by default; any offered force action requires
    Herdr's separate explicit confirmation and does not delete the branch.
12. Herdr runtime sockets, logs, and session state remain outside Git.
13. No remote host is changed during this local implementation.
