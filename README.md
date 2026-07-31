# Dotfiles

Public, cross-platform machine configuration managed by
[chezmoi](https://www.chezmoi.io/).

This repository contains machine configuration, package intent, and post-apply
setup hooks only. Credentials, private keys, authentication state, histories,
sessions, and caches do not belong here.

## Layout

- `.chezmoiroot` selects `home/` as the chezmoi source root.
- `home/` contains machine configuration and chezmoi scripts.
- `home/.chezmoidata/packages.yaml` lists native packages for macOS, Arch,
  Ubuntu, Debian, and Proxmox.
- `bootstrap.sh` installs chezmoi when necessary and initializes this existing
  source without applying it.
- `tests/` contains the repository contract checks.

AI tool configuration is managed separately in the private `ai-chezmoi`
repository. Its configuration and state are independent; the current disjoint
target contract is enforced by `tests/target-overlap-test.zsh`.

## Profiles and packages

Chezmoi asks once for a machine profile: macOS defaults to `workstation`, Arch
and Ubuntu to `dev`, and Debian/Proxmox to `server`. Override the profile when
needed with chezmoi data. On APT hosts, `server` installs the base list and
omits Ghostty and Yazi configuration; `dev` and `workstation` install the base
and dev lists. macOS and Arch retain their full native package lists even when
the profile is overridden.

Native package managers are Homebrew Bundle (macOS),
`pacman -S --needed --noconfirm` (Arch), and APT (Ubuntu/Debian/Proxmox). Arch
may upgrade outdated listed packages; Homebrew Bundle and APT do not. Before
APT runs, it audits `dpkg`; any pending configuration stops with manual
`sudo dpkg --configure -a` guidance. It never auto-repairs, upgrades, removes,
retries, or changes Proxmox repositories. Root runs it directly; non-root
requires `sudo`.

Ubuntu and Debian provide `bat`/`fd` compatibility shims and disable Git Delta,
which is not in their native package list. If Neovim is missing or below 0.12,
a fallback downloads only checksum-verified upstream Neovim v0.12.4 for amd64
or arm64. Yazi's optional package hook simply skips when Yazi is absent; it
does not recreate an upstream installer.

## Bootstrap

The checkout can live anywhere; bootstrap records its location as the
chezmoi source directory. `~/Documents/GitHub/dotfiles` is the usual spot.

```sh
git clone https://github.com/ChaseDDevelopment/dotfiles.git \
  ~/Documents/GitHub/dotfiles
cd ~/Documents/GitHub/dotfiles
./bootstrap.sh
```

Bootstrap unconditionally requires `curl`; network access is needed only when
it must install chezmoi. It initializes the source but never applies it. Review
both views before changing the home directory:

```sh
chezmoi diff
chezmoi apply --dry-run --verbose
```

Apply only after reviewing the output:

```sh
chezmoi apply
```

Package installation uses Homebrew Bundle on macOS,
`pacman -S --needed --noconfirm` on Arch, and APT on Ubuntu, Debian, and
Proxmox. Package and post-apply setup hooks are `run_onchange` scripts: they
run when their rendered script/input changes, not for package drift or every
apply. Bootstrap never applies changes.

## Daily use

Edit the source under `home/`, then review the rendered change:

```sh
chezmoi diff
chezmoi apply --dry-run --verbose
```

For the private AI source:

```sh
chezmoi-ai diff
chezmoi-ai apply --dry-run --verbose
```

## Updating

Setup hooks never upgrade packages. Run `dot-update` (installed to
`~/.local/bin`) for an explicit full system update: native packages, tool
updaters (rustup, uv tools, bun, atuin, oh-my-posh), and tmux/zsh/yazi/Neovim
plugins. Absent tools are skipped visibly; failed steps are summarized and
make the command exit non-zero. Finish with `chezmoi diff` for config drift.

## Secret boundary

Before committing, inspect the complete staged diff and verify that it contains
no credentials or machine-local runtime data:

```sh
git diff --cached
```

The public source intentionally excludes SSH/GPG keys, cloud and container
credentials, GitHub CLI authentication, AI authentication, histories, sessions,
and environment-secret files.

## Tmux and Herdr

Homebrew installs Herdr on macOS. On Linux hosts, including DevBox, Ubuntu,
Debian, and Proxmox, install it separately using the
[official Herdr instructions](https://herdr.dev/docs/install/) before using
the workflow below. Chezmoi syncs Herdr configuration and workflows; it does
not install its Linux binary.

The first fresh Ghostty surface starts or attaches local tmux session `Main`;
`Cmd+T` opens a plain shell. Machine windows run Herdr locally or remotely:

```text
Macbook -> herdr
Mac-Mini -> herdr --remote hydra
DevBox -> herdr --remote devbox
```

| Command | Action |
| --- | --- |
| `herdr` | Create or attach the current machine's Herdr session |
| `hr hydra` | Attach directly to Hydra's Herdr session |
| `ssht hydra` | Attach to Hydra's fallback tmux session |

Before a Hydra launch, `hw hydra` prepares the native Chase key in the stable
`~/.ssh/agent.sock`, allowing one passphrase prompt per reboot and silent later
launches without a GUI login. After a reboot, macOS may first request the local
account's FileVault password; `hw` restores the terminal, waits for normal SSH,
and continues automatically. Canceling either prompt leaves tmux unchanged.

From a generic SSH client, use `ssh hydra`, then `herdr`. Tmux-resurrect
restores `herdr` and `ssh` processes. On macOS, `tmux-boot` starts the server
headlessly at login; `@continuum-boot` stays disabled because it launches GUI
terminals rather than Ghostty.

Inside Herdr, `Prefix+Shift+G` creates a native worktree. An optional tracked
`.worktree-copy` manifest lists ignored files or directories that should be
copied into a new worktree. Dirty worktrees are never silently discarded, and
branches are retained until explicitly deleted.

## Validation

Run the shell contract tests from the repository root. The suite requires the
sibling private `../ai-chezmoi`; override that location with
`AI_CHEZMOI_REPO=/path/to/ai-chezmoi`.

```sh
for test_file in tests/*.zsh; do
  zsh "$test_file" || exit 1
done
```

`tests/target-overlap-test.zsh` enforces the disjoint target contract and runs
an additional rendered-target check when `chezmoi` is available. Full
validation requires `chezmoi` for the render tests.
