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
	// Failures collects best-effort post-install warnings that used
	// to vanish into install.log. May be nil in tests — all recording
	// goes through TrackedFailures.Record which tolerates nil.
	Failures *TrackedFailures
	// Component is the name of the component currently being set up.
	// Used by bestEffort to attribute failures for the summary screen.
	Component string
}

// SetupComponent applies symlinks and runs post-install hooks.
func SetupComponent(ctx context.Context, comp Component, sc *SetupContext) error {
	// Tag failures with the component name so the summary screen
	// shows "Tmux — TPM plugin install: ..." instead of a bare step.
	sc.Component = comp.Name

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

// bestEffort runs fn and records any failure against the current
// component so it reaches the summary screen. Failures are also
// logged to install.log for post-mortem. The setup continues
// regardless — that's the "best-effort" contract.
func bestEffort(sc *SetupContext, msg string, fn func() error) {
	if err := fn(); err != nil {
		sc.Runner.Log.Write(fmt.Sprintf("WARNING: %s: %v", msg, err))
		sc.Failures.Record(sc.Component, msg, err)
	}
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
		return setupGhostty(ctx, sc)
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
	// Back it up first if it exists and is not already a symlink —
	// refuse to delete if the backup step fails.
	staleZshrc := filepath.Join(home, ".zshrc")
	if info, err := os.Lstat(staleZshrc); err == nil {
		if info.Mode()&os.ModeSymlink == 0 {
			if err := sc.Backup.BackupFile(staleZshrc); err != nil {
				return fmt.Errorf(
					"backup %s before removal: %w",
					staleZshrc, err,
				)
			}
		}
		if err := os.Remove(staleZshrc); err != nil {
			return fmt.Errorf("remove stale %s: %w", staleZshrc, err)
		}
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
		bestEffort(sc, "antidote plugin compilation failed", func() error {
			return sc.Runner.Run(ctx, "zsh", "-c", script)
		})
	}

	// Set zsh as the default shell. `chsh` itself would prompt for
	// the user's password via PAM and deadlock the TUI, but
	// `sudo -n chsh` reuses the sudo credentials we pre-authed at
	// startup (see executor/sudo.go), so the change happens without
	// another prompt. Failures here degrade to the old "log a
	// reminder" behavior — a chsh hiccup shouldn't abort a run.
	currentShell := os.Getenv("SHELL")
	if !strings.HasSuffix(currentShell, "/zsh") {
		zshPath, lerr := exec.LookPath("zsh")
		if lerr == nil {
			if err := setDefaultShellZsh(ctx, sc, currentShell, zshPath); err != nil {
				sc.Runner.Log.Write(fmt.Sprintf(
					"chsh to %s failed (%v) — run "+
						"'sudo chsh -s %s %s' manually to make zsh permanent",
					zshPath, err, zshPath, os.Getenv("USER"),
				))
			}
		}
	}

	// Nuke cached shell init output so the next shell start
	// regenerates with the current .zshrc flags. _cached_init only
	// invalidates on binary-mtime changes; flag-only changes (e.g.
	// `zoxide init zsh` → `zoxide init zsh --cmd cd`) otherwise
	// reuse stale cache forever.
	bestEffort(sc, "clear zsh init caches", func() error {
		return clearZshInitCaches(home, sc.Runner)
	})

	return nil
}

// clearZshInitCaches removes every *.zsh file in ~/.cache/zsh so
// the next shell start regenerates cached init output with the
// current .zshrc. Runs at the end of setupZsh — the one place
// that owns both the .zshrc symlink and the cache directory.
// setDefaultShellZsh changes the current user's login shell to
// zshPath via `sudo -n chsh`, reusing cached sudo credentials so
// the TUI doesn't have to prompt. On Linux, zshPath must appear in
// /etc/shells before chsh will accept it; ensureShellListed adds
// it there when missing (also via sudo).
func setDefaultShellZsh(
	ctx context.Context,
	sc *SetupContext,
	currentShell, zshPath string,
) error {
	if !executor.HasSudo() {
		return fmt.Errorf("sudo not available")
	}
	if executor.NeedsSudo() {
		// Credentials weren't preauthed / cache expired. Running
		// `sudo chsh` would prompt inside the TUI and hang.
		return fmt.Errorf("sudo credentials not cached — skipping chsh")
	}
	if err := ensureShellListed(ctx, sc, zshPath); err != nil {
		return fmt.Errorf("ensure %s in /etc/shells: %w", zshPath, err)
	}
	user := os.Getenv("USER")
	if user == "" {
		return fmt.Errorf("USER env unset")
	}
	if err := sc.Runner.Run(
		ctx, "sudo", "-n", "chsh", "-s", zshPath, user,
	); err != nil {
		return err
	}
	sc.Runner.Log.Write(fmt.Sprintf(
		"chsh: default shell %s → %s (via sudo -n, cached creds)",
		currentShell, zshPath,
	))
	return nil
}

// ensureShellListed adds zshPath to /etc/shells when it isn't
// already there — required on most Linux distros before chsh will
// accept the path. No-op on macOS, where /etc/shells ships with
// the stock zsh entries and our Homebrew zsh is auto-accepted by
// Directory Services.
func ensureShellListed(
	ctx context.Context,
	sc *SetupContext,
	zshPath string,
) error {
	const shellsFile = "/etc/shells"
	data, err := os.ReadFile(shellsFile)
	if err != nil {
		if os.IsNotExist(err) {
			// No /etc/shells — chsh on this host either doesn't
			// check or we'll fail later with a clearer error.
			return nil
		}
		return err
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == zshPath {
			return nil
		}
	}
	// Append via `sudo tee -a` so we keep the same cached-sudo
	// path as the actual chsh call.
	line := zshPath + "\n"
	cmd := exec.CommandContext(
		ctx, "sudo", "-n", "tee", "-a", shellsFile,
	)
	cmd.Stdin = strings.NewReader(line)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("sudo tee %s: %w (%s)", shellsFile, err, strings.TrimSpace(string(out)))
	}
	sc.Runner.Log.Write(fmt.Sprintf(
		"Added %s to %s", zshPath, shellsFile,
	))
	return nil
}

func clearZshInitCaches(home string, runner *executor.Runner) error {
	cacheDir := filepath.Join(home, ".cache", "zsh")
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read %s: %w", cacheDir, err)
	}
	var cleared []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".zsh") {
			continue
		}
		path := filepath.Join(cacheDir, e.Name())
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("remove %s: %w", path, err)
		}
		cleared = append(cleared, e.Name())
	}
	if len(cleared) > 0 && runner != nil && runner.Log != nil {
		runner.Log.Write(fmt.Sprintf(
			"zsh: cleared %d cached init file(s): %s",
			len(cleared), strings.Join(cleared, ", "),
		))
	}
	return nil
}

func setupTmux(ctx context.Context, sc *SetupContext) error {
	home := os.Getenv("HOME")
	tmuxConf := filepath.Join(home, ".config", "tmux", "tmux.conf")
	tmuxPluginsDir := filepath.Join(home, ".tmux", "plugins")

	// Wipe any tmux-resurrect / tmux-continuum save state. The
	// plugin dirs are handled by staleTmuxPlugins below, but
	// session JSONs under XDG_DATA_HOME (and the legacy
	// ~/.tmux/resurrect/) persist and would be silently replayed
	// if the user ever reinstalls either plugin. Idempotent: skips
	// when the dirs don't exist.
	xdgData := os.Getenv("XDG_DATA_HOME")
	if xdgData == "" {
		xdgData = filepath.Join(home, ".local", "share")
	}
	for _, dir := range []string{
		filepath.Join(xdgData, "tmux", "resurrect"),
		filepath.Join(home, ".tmux", "resurrect"),
	} {
		if _, err := os.Stat(dir); err != nil {
			continue
		}
		sc.Runner.EmitVerbose("Removing stale tmux-resurrect saves: " + dir)
		saveDir := dir
		bestEffort(sc, "remove tmux-resurrect saves", func() error {
			return os.RemoveAll(saveDir)
		})
	}

	// Prune plugin dirs that no longer appear in tmux.conf. TPM
	// installs but never cleans (its `clean_plugins` script is only
	// bound to a key), so a plugin removed from `set -g @plugin`
	// lingers on disk with its bindings still active in any running
	// tmux server — that's how tmux-menus kept clobbering `|`.
	if stale, err := staleTmuxPlugins(tmuxConf, tmuxPluginsDir); err != nil {
		sc.Runner.Log.Write(fmt.Sprintf(
			"tmux plugin prune skipped: %v", err,
		))
	} else {
		for _, dir := range stale {
			sc.Runner.EmitVerbose("Removing stale tmux plugin: " + dir)
			bestEffort(sc, "remove stale tmux plugin "+filepath.Base(dir), func() error {
				return os.RemoveAll(dir)
			})
		}
	}

	// Install TPM plugins if TPM exists.
	sc.Runner.EmitVerbose("Checking for TPM plugins")
	tpmScript := filepath.Join(os.Getenv("HOME"), ".tmux", "plugins", "tpm", "scripts", "install_plugins.sh")
	if _, err := os.Stat(tpmScript); err == nil {
		// Start tmux server and source config for TPM env (best-effort).
		bestEffort(sc, "tmux server start failed", func() error {
			return sc.Runner.RunShell(ctx,
				fmt.Sprintf(`tmux start-server \; source-file "%s" 2>/dev/null || true`, tmuxConf))
		})
		bestEffort(sc, "chmod tpm script failed", func() error {
			return sc.Runner.Run(ctx, "chmod", "+x", tpmScript)
		})
		bestEffort(sc, "TPM plugin install failed", func() error {
			return sc.Runner.Run(ctx, tpmScript)
		})
	}

	// Reload tmux config if running (best-effort).
	if _, err := sc.Runner.RunProbe(ctx, "pgrep", "-x", "tmux"); err == nil {
		bestEffort(sc, "tmux config reload failed", func() error {
			return sc.Runner.Run(ctx, "tmux", "source-file", tmuxConf)
		})
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

	// Remove the legacy lazy.nvim plugin directory from a prior
	// nvim setup. The current config uses vim.pack (native to 0.12)
	// which clones into site/pack/core/opt/, so the lazy/ tree is
	// unused cruft that can shadow the active plugins depending on
	// runtimepath order.
	lazyDir := filepath.Join(home, ".local", "share", "nvim", "lazy")
	if _, err := os.Stat(lazyDir); err == nil {
		sc.Runner.EmitVerbose("Removing stale lazy.nvim plugin dir")
		bestEffort(sc, "remove stale ~/.local/share/nvim/lazy", func() error {
			return os.RemoveAll(lazyDir)
		})
	}

	// Reconcile plugin clones against nvim-pack-lock.json. Clones
	// whose HEAD doesn't match the locked rev get wiped so the next
	// headless `vim.pack.update` (or `vim.pack.add` from init.lua)
	// re-clones cleanly. Fixes the hydra case where harpoon was
	// stuck on master after the version spec switched to "harpoon2" —
	// `vim.pack.update` fetches new refs but won't switch a detached
	// HEAD across branches.
	lockPath := filepath.Join(sc.RootDir, "configs", "nvim", "nvim-pack-lock.json")
	if drifted, err := nvimDriftedClones(lockPath, home); err != nil {
		sc.Runner.Log.Write(fmt.Sprintf(
			"nvim drift scan skipped: %v", err,
		))
	} else {
		for _, dir := range drifted {
			sc.Runner.EmitVerbose("Removing drifted nvim plugin clone: " + dir)
			bestEffort(sc, "remove drifted plugin "+filepath.Base(dir), func() error {
				return os.RemoveAll(dir)
			})
		}
	}

	// Build blink.cmp fuzzy matcher if available.
	blinkDir := filepath.Join(home, ".local", "share", "nvim", "site", "pack", "core", "opt", "blink.cmp")
	if _, err := os.Stat(blinkDir); err == nil && platform.HasCommand("cargo") {
		sc.Runner.EmitVerbose("Building blink.cmp fuzzy matcher")
		bestEffort(sc, "blink.cmp cargo build failed", func() error {
			return sc.Runner.RunInDir(ctx, blinkDir, "cargo", "build", "--release")
		})
	}

	// Install missing plugins + pull updates to tracked branch tips.
	// vim.pack.add (called from init.lua) only clones what's missing
	// — it never updates. Without this explicit vim.pack.update call
	// every re-install silently no-ops on the plugin set, leaving
	// pinned versions stale indefinitely. force=true suppresses the
	// confirmation prompt that would otherwise hang headless mode.
	if platform.HasCommand("nvim") {
		sc.Runner.EmitVerbose("Syncing Neovim plugins (headless update)")
		bestEffort(sc, "headless nvim plugin update failed", func() error {
			return sc.Runner.Run(ctx, "nvim", "--headless",
				"+lua vim.pack.update(nil, {force = true})",
				"+q",
			)
		})
	}

	return nil
}

func setupStarship(ctx context.Context, sc *SetupContext) error {
	configFile := os.ExpandEnv("$HOME/.config/starship.toml")
	customConfig := filepath.Join(sc.RootDir, "configs", "starship", "starship.toml")

	// If no custom config was symlinked, generate catppuccin preset.
	if _, err := os.Stat(customConfig); os.IsNotExist(err) {
		if platform.HasCommand("starship") {
			bestEffort(sc, "starship preset failed", func() error {
				return sc.Runner.Run(ctx, "starship", "preset", "catppuccin-powerline", "-o", configFile)
			})
		}
	}
	return nil
}

func setupYazi(ctx context.Context, sc *SetupContext) error {
	if platform.HasCommand("ya") {
		bestEffort(sc, "yazi package install failed", func() error {
			return sc.Runner.Run(ctx, "ya", "pkg", "install")
		})
	}
	return nil
}

func setupGhostty(_ context.Context, sc *SetupContext) error {
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
		name, _ := sc.Runner.RunProbe(
			ctx, "git", "config", "--global", "user.name",
		)
		email, _ := sc.Runner.RunProbe(
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
