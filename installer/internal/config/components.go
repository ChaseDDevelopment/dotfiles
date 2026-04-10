package config

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/chaseddevelopment/dotfiles/installer/internal/backup"
	"github.com/chaseddevelopment/dotfiles/installer/internal/executor"
	"github.com/chaseddevelopment/dotfiles/installer/internal/platform"
)

// Component describes a configurable dotfiles component.
type Component struct {
	Name        string
	Icon        string // Nerd Font icon for TUI display
	RequiredCmd string // binary that must exist before setup
}

// AllComponents returns the ordered list of components.
func AllComponents() []Component {
	return []Component{
		{Name: "Zsh", Icon: " ", RequiredCmd: "zsh"},
		{Name: "Tmux", Icon: " ", RequiredCmd: "tmux"},
		{Name: "Neovim", Icon: " ", RequiredCmd: "nvim"},
		{Name: "Starship", Icon: " ", RequiredCmd: "starship"},
		{Name: "Atuin", Icon: " ", RequiredCmd: "atuin"},
		{Name: "Ghostty", Icon: "󰊠"},
		{Name: "Yazi", Icon: " ", RequiredCmd: "yazi"},
		{Name: "Git", Icon: " ", RequiredCmd: "git"},
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
			return fmt.Errorf(
				"%s setup requires %s, but it was not found in PATH",
				comp.Name, comp.RequiredCmd,
			)
		}
	}

	// Apply symlinks for this component.
	sc.Runner.EmitVerbose("Configuring symlinks for " + comp.Name)
	if err := ApplyAllSymlinks(
		comp.Name, sc.RootDir, sc.Backup, sc.DryRun, sc.Runner,
	); err != nil {
		return fmt.Errorf("symlinks for %s: %w", comp.Name, err)
	}

	// Run post-install hook.
	if sc.DryRun {
		return nil
	}
	if err := runPostInstall(ctx, comp.Name, sc); err != nil {
		// Rollback symlinks on hook failure to avoid half-configured state.
		rollbackSymlinks(comp.Name, sc.RootDir, sc.Runner)
		return err
	}

	// Run user-defined hook script if present.
	if err := runUserHook(ctx, comp.Name, sc); err != nil {
		rollbackSymlinks(comp.Name, sc.RootDir, sc.Runner)
		return err
	}
	return nil
}

// rollbackSymlinks removes symlinks that were just applied for a
// component, restoring a clean state after a hook failure.
func rollbackSymlinks(component, rootDir string, runner *executor.Runner) {
	for _, entry := range AllSymlinks() {
		if entry.Component != component {
			continue
		}
		target := os.ExpandEnv(entry.Target)
		// Only remove if it's a symlink pointing to our source.
		if link, err := os.Readlink(target); err == nil {
			source := resolveSource(rootDir, entry.Source)
			canonSource, _ := filepath.Abs(source)
			canonLink, _ := filepath.Abs(link)
			if canonSource == canonLink {
				os.Remove(target)
				if runner != nil {
					runner.EmitVerbose("Rolled back " + target)
				}
			}
		}
	}
}

// runUserHook executes an optional user-defined shell script at
// configs/<component>/hooks/post-install.sh. This allows users to
// extend setup without modifying Go source.
func runUserHook(ctx context.Context, name string, sc *SetupContext) error {
	hookPath := filepath.Join(
		sc.RootDir, "configs",
		strings.ToLower(name), "hooks", "post-install.sh",
	)
	if _, err := os.Stat(hookPath); os.IsNotExist(err) {
		return nil
	}
	sc.Runner.EmitVerbose("Running user hook: " + hookPath)
	return sc.Runner.Run(ctx, "bash", hookPath)
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
	sc.Runner.EmitVerbose("Creating XDG directories")
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
		if err := os.MkdirAll(d, 0o755); err != nil {
			return fmt.Errorf("create dir %s: %w", d, err)
		}
	}

	// Remove stale ~/.zshrc (ZDOTDIR handles it now).
	// Back it up first if it exists and is not already a symlink.
	staleZshrc := filepath.Join(home, ".zshrc")
	if info, err := os.Lstat(staleZshrc); err == nil {
		if info.Mode()&os.ModeSymlink == 0 {
			_ = sc.Backup.BackupFile(staleZshrc)
		}
		os.Remove(staleZshrc)
	}

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
		sc.Runner.EmitVerbose("Installing Antidote plugin manager")
		if platform.HasCommand("brew") {
			if err := sc.Runner.Run(ctx, "brew", "install", "antidote"); err != nil {
				return fmt.Errorf("install antidote: %w", err)
			}
		} else {
			dest := filepath.Join(home, ".config", "zsh", ".antidote")
			if err := sc.Runner.Run(ctx, "git", "clone", "--depth=1",
				"https://github.com/mattmc3/antidote.git", dest); err != nil {
				return fmt.Errorf("clone antidote: %w", err)
			}
		}
	}

	// Compile Antidote plugins (best-effort — log warning on failure).
	pluginsTxt := filepath.Join(home, ".config", "zsh", "plugins", ".zsh_plugins.txt")
	pluginsZsh := filepath.Join(home, ".config", "zsh", "plugins", ".zsh_plugins.zsh")
	if _, err := os.Stat(pluginsTxt); err == nil {
		script := fmt.Sprintf(
			`for p in /opt/homebrew/opt/antidote/share/antidote /usr/local/opt/antidote/share/antidote %s/.config/zsh/.antidote; do [ -f "$p/antidote.zsh" ] && source "$p/antidote.zsh" && antidote bundle < "%s" > "%s" && break; done`,
			home, pluginsTxt, pluginsZsh,
		)
		if err := sc.Runner.Run(ctx, "zsh", "-c", script); err != nil {
			sc.Runner.Log.Write(fmt.Sprintf("WARNING: antidote plugin compilation failed: %v", err))
		}
	}

	// Set zsh as default shell.
	zshPath, err := exec.LookPath("zsh")
	if err == nil {
		currentShell := os.Getenv("SHELL")
		if currentShell != zshPath {
			if err := sc.Runner.Run(ctx, "chsh", "-s", zshPath); err != nil {
				return fmt.Errorf("chsh to zsh: %w", err)
			}
		}
	}

	return nil
}

func setupTmux(ctx context.Context, sc *SetupContext) error {
	// Install TPM plugins if TPM exists.
	sc.Runner.EmitVerbose("Checking for TPM plugins")
	tpmScript := filepath.Join(os.Getenv("HOME"), ".tmux", "plugins", "tpm", "scripts", "install_plugins.sh")
	if _, err := os.Stat(tpmScript); err == nil {
		// Start tmux server and source config for TPM env (best-effort).
		tmuxConf := filepath.Join(os.Getenv("HOME"), ".config", "tmux", "tmux.conf")
		if err := sc.Runner.RunShell(ctx,
			fmt.Sprintf(`tmux start-server \; source-file "%s" 2>/dev/null || true`, tmuxConf)); err != nil {
			sc.Runner.Log.Write(fmt.Sprintf("WARNING: tmux server start failed: %v", err))
		}
		if err := sc.Runner.Run(ctx, "chmod", "+x", tpmScript); err != nil {
			sc.Runner.Log.Write(fmt.Sprintf("WARNING: chmod tpm script failed: %v", err))
		}
		if err := sc.Runner.Run(ctx, tpmScript); err != nil {
			sc.Runner.Log.Write(fmt.Sprintf("WARNING: TPM plugin install failed: %v", err))
		}
	}

	// Reload tmux config if running (best-effort).
	if err := sc.Runner.Run(ctx, "pgrep", "-x", "tmux"); err == nil {
		tmuxConf := filepath.Join(os.Getenv("HOME"), ".config", "tmux", "tmux.conf")
		if err := sc.Runner.Run(ctx, "tmux", "source-file", tmuxConf); err != nil {
			sc.Runner.Log.Write(fmt.Sprintf("WARNING: tmux config reload failed: %v", err))
		}
	}

	return nil
}

func setupNeovim(ctx context.Context, sc *SetupContext) error {
	home := os.Getenv("HOME")
	sc.Runner.EmitVerbose("Creating Neovim directories")
	for _, d := range []string{
		filepath.Join(home, ".local", "share", "nvim"),
		filepath.Join(home, ".local", "state", "nvim"),
		filepath.Join(home, ".cache", "nvim"),
	} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return fmt.Errorf("create nvim dir %s: %w", d, err)
		}
	}

	// Build blink.cmp fuzzy matcher if available.
	blinkDir := filepath.Join(home, ".local", "share", "nvim", "site", "pack", "core", "opt", "blink.cmp")
	if _, err := os.Stat(blinkDir); err == nil && platform.HasCommand("cargo") {
		sc.Runner.EmitVerbose("Building blink.cmp fuzzy matcher")
		if err := sc.Runner.RunInDir(ctx, blinkDir, "cargo", "build", "--release"); err != nil {
			sc.Runner.Log.Write(fmt.Sprintf("WARNING: blink.cmp cargo build failed: %v", err))
		}
	}

	// Pre-install plugins headlessly so first launch is fast.
	if platform.HasCommand("nvim") {
		sc.Runner.EmitVerbose("Syncing Neovim plugins (headless)")
		if err := sc.Runner.Run(
			ctx, "nvim", "--headless", "+q",
		); err != nil {
			sc.Runner.Log.Write(fmt.Sprintf(
				"WARNING: headless nvim plugin sync failed: %v", err,
			))
		}
	}

	return nil
}

func setupStarship(ctx context.Context, sc *SetupContext) error {
	configFile := os.ExpandEnv("$HOME/.config/starship.toml")
	customConfig := filepath.Join(sc.RootDir, "configs", "starship", "starship.toml")

	// If no custom config was symlinked, generate catppuccin preset.
	if _, err := os.Stat(customConfig); os.IsNotExist(err) {
		if platform.HasCommand("starship") {
			if err := sc.Runner.Run(ctx, "starship", "preset", "catppuccin-powerline", "-o", configFile); err != nil {
				sc.Runner.Log.Write(fmt.Sprintf("WARNING: starship preset failed: %v", err))
			}
		}
	}
	return nil
}

func setupYazi(ctx context.Context, sc *SetupContext) error {
	if platform.HasCommand("ya") {
		if err := sc.Runner.Run(ctx, "ya", "pkg", "install"); err != nil {
			sc.Runner.Log.Write(fmt.Sprintf("WARNING: yazi package install failed: %v", err))
		}
	}
	return nil
}

func setupGhostty(sc *SetupContext) error {
	if !sc.Platform.IsDesktopEnvironment() {
		sc.Runner.EmitVerbose(
			"Skipping Ghostty: no desktop environment",
		)
		sc.Runner.Log.Write(
			"Skipping Ghostty: no desktop environment detected",
		)
		return nil
	}
	// Ghostty config is handled by symlinks — no extra setup needed.
	return nil
}

func setupGit(ctx context.Context, sc *SetupContext) error {
	// Ensure ~/.config/git/ exists before the file symlink.
	sc.Runner.EmitVerbose("Creating ~/.config/git directory")
	gitDir := os.ExpandEnv("$HOME/.config/git")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		return err
	}

	// Warn if git identity is not configured.
	if platform.HasCommand("git") {
		name, _ := sc.Runner.RunWithOutput(
			ctx, "git", "config", "--global", "user.name",
		)
		email, _ := sc.Runner.RunWithOutput(
			ctx, "git", "config", "--global", "user.email",
		)
		if strings.TrimSpace(name) == "" || strings.TrimSpace(email) == "" {
			sc.Runner.Log.Write(
				"WARNING: git user.name or user.email not set — " +
					"run: git config --global user.name 'Your Name' && " +
					"git config --global user.email 'you@example.com'",
			)
			sc.Runner.EmitVerbose(
				"⚠ git identity not configured (user.name/user.email)",
			)
		}
	}
	return nil
}
