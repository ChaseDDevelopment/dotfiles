package config

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/chaseddevelopment/dotfiles/installer/internal/backup"
	"github.com/chaseddevelopment/dotfiles/installer/internal/executor"
	"github.com/chaseddevelopment/dotfiles/installer/internal/platform"
)

// Component describes a configurable dotfiles component.
type Component struct {
	Name        string
	RequiredCmd string // binary that must exist before setup
}

// AllComponents returns the ordered list of components.
func AllComponents() []Component {
	return []Component{
		{Name: "Zsh", RequiredCmd: "zsh"},
		{Name: "Tmux", RequiredCmd: "tmux"},
		{Name: "Neovim", RequiredCmd: "nvim"},
		{Name: "Starship", RequiredCmd: "starship"},
		{Name: "Atuin", RequiredCmd: "atuin"},
		{Name: "Ghostty"},
		{Name: "Yazi", RequiredCmd: "yazi"},
		{Name: "Git", RequiredCmd: "git"},
	}
}

// SetupContext provides shared state to component setup hooks.
type SetupContext struct {
	Runner   *executor.Runner
	RootDir  string
	Backup   *backup.Manager
	DryRun   bool
	Platform *platform.Platform
}

// SetupComponent applies symlinks and runs post-install hooks.
func SetupComponent(ctx context.Context, comp Component, sc *SetupContext) error {
	// Check required command.
	if comp.RequiredCmd != "" {
		if _, err := exec.LookPath(comp.RequiredCmd); err != nil {
			sc.Runner.Log.Write(fmt.Sprintf(
				"Skipping %s setup: %s not found", comp.Name, comp.RequiredCmd))
			return nil
		}
	}

	// Apply symlinks for this component.
	if err := ApplyAllSymlinks(comp.Name, sc.RootDir, sc.Backup, sc.DryRun); err != nil {
		return fmt.Errorf("symlinks for %s: %w", comp.Name, err)
	}

	// Run post-install hook.
	if sc.DryRun {
		return nil
	}
	return runPostInstall(ctx, comp.Name, sc)
}

func runPostInstall(ctx context.Context, name string, sc *SetupContext) error {
	switch name {
	case "Zsh":
		return setupZsh(ctx, sc)
	case "Tmux":
		return setupTmux(ctx, sc)
	case "Neovim":
		return setupNeovim(ctx, sc)
	case "Starship":
		return setupStarship(ctx, sc)
	case "Yazi":
		return setupYazi(ctx, sc)
	case "Ghostty":
		return setupGhostty(sc)
	case "Git":
		return setupGit(ctx, sc)
	}
	return nil
}

func setupZsh(ctx context.Context, sc *SetupContext) error {
	home := os.Getenv("HOME")

	// Create XDG directories.
	dirs := []string{
		filepath.Join(home, ".config"),
		filepath.Join(home, ".local", "share"),
		filepath.Join(home, ".local", "state"),
		filepath.Join(home, ".local", "state", "zsh"),
		filepath.Join(home, ".local", "bin"),
		filepath.Join(home, ".cache"),
		filepath.Join(home, ".cache", "zsh"),
		filepath.Join(home, ".cache", "ohmyzsh", "completions"),
	}
	for _, d := range dirs {
		os.MkdirAll(d, 0o755)
	}

	// Remove stale ~/.zshrc (ZDOTDIR handles it now).
	os.Remove(filepath.Join(home, ".zshrc"))

	// Install Antidote.
	antidotePaths := []string{
		"/opt/homebrew/opt/antidote/share/antidote/antidote.zsh",
		"/usr/local/opt/antidote/share/antidote/antidote.zsh",
		"/home/linuxbrew/.linuxbrew/opt/antidote/share/antidote/antidote.zsh",
		filepath.Join(home, ".config", "zsh", ".antidote", "antidote.zsh"),
	}
	antidoteFound := false
	for _, p := range antidotePaths {
		if _, err := os.Stat(p); err == nil {
			antidoteFound = true
			break
		}
	}
	if !antidoteFound {
		if platform.HasCommand("brew") {
			_ = sc.Runner.Run(ctx, "brew", "install", "antidote")
		} else {
			dest := filepath.Join(home, ".config", "zsh", ".antidote")
			_ = sc.Runner.Run(ctx, "git", "clone", "--depth=1",
				"https://github.com/mattmc3/antidote.git", dest)
		}
	}

	// Compile Antidote plugins.
	pluginsTxt := filepath.Join(home, ".config", "zsh", "plugins", ".zsh_plugins.txt")
	pluginsZsh := filepath.Join(home, ".config", "zsh", "plugins", ".zsh_plugins.zsh")
	if _, err := os.Stat(pluginsTxt); err == nil {
		script := fmt.Sprintf(
			`for p in /opt/homebrew/opt/antidote/share/antidote /usr/local/opt/antidote/share/antidote %s/.config/zsh/.antidote; do [ -f "$p/antidote.zsh" ] && source "$p/antidote.zsh" && antidote bundle < "%s" > "%s" && break; done`,
			home, pluginsTxt, pluginsZsh,
		)
		_ = sc.Runner.Run(ctx, "zsh", "-c", script)
	}

	// Set zsh as default shell.
	zshPath, err := exec.LookPath("zsh")
	if err == nil {
		currentShell := os.Getenv("SHELL")
		if currentShell != zshPath {
			_ = sc.Runner.Run(ctx, "chsh", "-s", zshPath)
		}
	}

	return nil
}

func setupTmux(ctx context.Context, sc *SetupContext) error {
	// Install TPM plugins if TPM exists.
	tpmScript := filepath.Join(os.Getenv("HOME"), ".tmux", "plugins", "tpm", "scripts", "install_plugins.sh")
	if _, err := os.Stat(tpmScript); err == nil {
		// Start tmux server and source config for TPM env.
		tmuxConf := filepath.Join(os.Getenv("HOME"), ".config", "tmux", "tmux.conf")
		_ = sc.Runner.RunShell(ctx,
			fmt.Sprintf(`tmux start-server \; source-file "%s" 2>/dev/null || true`, tmuxConf))
		_ = sc.Runner.Run(ctx, "chmod", "+x", tpmScript)
		_ = sc.Runner.Run(ctx, tpmScript)
	}

	// Reload tmux config if running.
	if err := exec.Command("pgrep", "-x", "tmux").Run(); err == nil {
		tmuxConf := filepath.Join(os.Getenv("HOME"), ".config", "tmux", "tmux.conf")
		_ = sc.Runner.Run(ctx, "tmux", "source-file", tmuxConf)
	}

	return nil
}

func setupNeovim(_ context.Context, sc *SetupContext) error {
	home := os.Getenv("HOME")
	for _, d := range []string{
		filepath.Join(home, ".local", "share", "nvim"),
		filepath.Join(home, ".local", "state", "nvim"),
		filepath.Join(home, ".cache", "nvim"),
	} {
		os.MkdirAll(d, 0o755)
	}

	// Build blink.cmp fuzzy matcher if available.
	blinkDir := filepath.Join(home, ".local", "share", "nvim", "site", "pack", "core", "opt", "blink.cmp")
	if _, err := os.Stat(blinkDir); err == nil && platform.HasCommand("cargo") {
		_ = sc.Runner.Run(context.Background(), "cargo", "build", "--release")
	}

	return nil
}

func setupStarship(ctx context.Context, sc *SetupContext) error {
	configFile := os.ExpandEnv("$HOME/.config/starship.toml")
	customConfig := filepath.Join(sc.RootDir, "configs", "starship", "starship.toml")

	// If no custom config was symlinked, generate catppuccin preset.
	if _, err := os.Stat(customConfig); os.IsNotExist(err) {
		if platform.HasCommand("starship") {
			_ = sc.Runner.Run(ctx, "starship", "preset", "catppuccin-powerline", "-o", configFile)
		}
	}
	return nil
}

func setupYazi(ctx context.Context, sc *SetupContext) error {
	if platform.HasCommand("ya") {
		_ = sc.Runner.Run(ctx, "ya", "pkg", "install")
	}
	return nil
}

func setupGhostty(sc *SetupContext) error {
	if !sc.Platform.IsDesktopEnvironment() {
		sc.Runner.Log.Write("Skipping Ghostty: no desktop environment detected")
	}
	return nil
}

func setupGit(_ context.Context, sc *SetupContext) error {
	// Ensure ~/.config/git/ exists before the file symlink.
	gitDir := os.ExpandEnv("$HOME/.config/git")
	return os.MkdirAll(gitDir, 0o755)
}
