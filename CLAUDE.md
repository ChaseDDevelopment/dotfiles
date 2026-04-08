# Dotfiles

## Architecture

- `configs/` — Dotfile configs (zsh, tmux, nvim, starship, atuin, ghostty, yazi, git, lazygit)
- `installer/` — Go TUI installer (Bubble Tea + Lipgloss, Catppuccin Mocha theme)
- `install.sh` — Bash fallback installer (uses gum for TUI)
- `scripts/` — Per-tool setup scripts sourced by the bash installer

## Build & Run

### Go TUI installer
- `cd installer && go build -o dotsetup .` — build
- `./dotsetup` or `./installer/bootstrap.sh` — run
- `go vet ./...` — static analysis
- `go test ./...` — run tests (only registry package has tests)

### Bash installer
- `bash install.sh` — run directly (no build step)

## Go TUI: Lipgloss Styling Gotchas

- Lipgloss `\x1b[0m` (SGR 0) resets ALL attributes. Container `Background()` does NOT propagate through child styled text — every inline style needs its own `Background()` to prevent transparency leaks
- `lipgloss.Place()` + `WithWhitespaceBackground` only fills whitespace AROUND the content block, not WITHIN it (e.g., JoinVertical centering padding). Use `Style.Render()` with `Background()` + `Width()` + `Height()` to fill inner whitespace too
- When using `JoinVertical`, all elements must match the widest element's rendered width. Panel total width = `contentWidth + panelStyle.GetHorizontalBorderSize()`. Use `panelOuterWidth()` helper
- Catppuccin Mocha layering: `catBase` (#1e1e2e) for full-screen bg, `catSurface0` (#313244) for panel interiors
- Test transparency fixes in a terminal with wallpaper/transparency enabled

## Adding a New Tool

1. Add config files under `configs/<toolname>/`
2. Register in `installer/internal/registry/` (see existing `*_tools.go` files)
3. Add component setup in `installer/internal/config/components.go`
4. Add bash setup script in `scripts/setup-<toolname>.sh`
