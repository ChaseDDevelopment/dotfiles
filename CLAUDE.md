# Dotfiles

## Architecture

- `home/` is the public chezmoi source for machine configuration.
- `.chezmoiroot` selects `home/`.
- `bootstrap.sh` requires `curl`, initializes this source, and never applies.
- `tests/` contains shell contract tests.
- AI tool configuration belongs in the separate private `ai-chezmoi` source.
- Profiles default to `workstation` on macOS, `dev` on Arch/Ubuntu, and
  `server` on Debian/Proxmox; callers may override them with chezmoi data.
  Server base-only behavior is APT-only; macOS/Arch retain full lists.
- Native package managers are Homebrew Bundle, `pacman --needed`, and APT.

## Workflow

- Preview with `chezmoi diff` and `chezmoi apply --dry-run --verbose`.
- Apply only after reviewing the rendered change.
- Run tests with
  `for test_file in tests/*.zsh; do zsh "$test_file" || exit 1; done`.
- Render tests require `chezmoi` and sibling `../ai-chezmoi`; set
  `AI_CHEZMOI_REPO=/path/to/ai-chezmoi` to override.

## Adding a tool

1. Add its configuration under the matching chezmoi path in `home/`.
2. Add native package intent to `home/.chezmoidata/packages.yaml` when needed.
3. Add a post-apply script only when chezmoi cannot manage the result directly.
4. Update the target manifest and the smallest relevant contract test.

## Package safety

- Package and setup hooks run only when rendered script/input changes, not on
  package drift or every apply; bootstrap never applies.
- APT audits `dpkg` first and stops for manual repair; it never upgrades,
  removes, auto-repairs, retries, or changes Proxmox repositories. Root runs
  directly; non-root uses `sudo`.
- Arch uses `pacman -Syu --needed --noconfirm`: package installs refresh the
  sync DB and perform a full system upgrade (stale DBs 404 on mirrors, and
  refresh-without-upgrade is unsupported partial-upgrade territory). Homebrew
  Bundle and APT never upgrade.
- Homebrew installs Herdr on macOS. Linux hosts must install it separately
  using <https://herdr.dev/docs/install/>; chezmoi only syncs its configuration.
- The only upstream Neovim binary fallback is checksum-pinned v0.12.4 on APT
  amd64/arm64 hosts when Neovim is missing or below 0.12.

## Safety

This is a public repository. Never add credentials, private keys,
authentication state, histories, sessions, caches, or secret environment files.
