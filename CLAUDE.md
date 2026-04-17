# Dotfiles

## Architecture

- `configs/` — Dotfile configs (zsh, tmux, nvim, oh-my-posh, atuin, ghostty, yazi, git, lazygit)
- `installer/` — Go TUI installer (Bubble Tea + Lipgloss, TokyoNight Night theme)
- `install.sh` — Bootstrap script (downloads Go binary from GitHub Releases, runs it)

## Build & Run

### Run
- `./install.sh` — downloads Go binary (if needed) and launches TUI
- `cd installer && go build -o dotsetup .` — build from source

### Dev
- `go vet ./...` — static analysis (run from `installer/`)
- `go test ./...` — run tests (run from `installer/`)

## Go TUI: Lipgloss Styling Gotchas

- Lipgloss `\x1b[0m` (SGR 0) resets ALL attributes. Container `Background()` does NOT propagate through child styled text — every inline style needs its own `Background()` to prevent transparency leaks
- `lipgloss.Place()` + `WithWhitespaceBackground` only fills whitespace AROUND the content block, not WITHIN it (e.g., JoinVertical centering padding). Use `Style.Render()` with `Background()` + `Width()` + `Height()` to fill inner whitespace too
- When using `JoinVertical`, all elements must match the widest element's rendered width. Panel total width = `contentWidth + panelStyle.GetHorizontalBorderSize()`. Use `panelOuterWidth()` helper
- TokyoNight Night layering: `catBase` (#1a1b26) for full-screen bg, `catSurface0` (#292e42) for panel interiors (the `cat*` identifier prefix is legacy — palette is TokyoNight)
- Test transparency fixes in a terminal with wallpaper/transparency enabled

## Adding a New Tool

1. Add config files under `configs/<toolname>/`
2. Register in `installer/internal/registry/` (see existing `*_tools.go` files)
3. Add component setup in `installer/internal/config/components.go`
