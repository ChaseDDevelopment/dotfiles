package config

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chaseddevelopment/dotfiles/installer/internal/backup"
	"github.com/chaseddevelopment/dotfiles/installer/internal/executor"
	"github.com/chaseddevelopment/dotfiles/installer/internal/platform"
)

func newComponentSetup(t *testing.T) (*SetupContext, string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	log, err := executor.NewLogFile(filepath.Join(home, "test.log"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { log.Close() })
	sc := &SetupContext{
		Runner:   executor.NewRunner(log, false),
		RootDir:  home,
		Backup:   backup.NewManager(false),
		Platform: &platform.Platform{OS: platform.Linux, Arch: platform.AMD64},
		Failures: NewTrackedFailures(),
	}
	return sc, home
}

func writeTool(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestComponentSetupHelpers(t *testing.T) {
	sc, home := newComponentSetup(t)
	fakebin := filepath.Join(home, "bin")
	if err := os.MkdirAll(fakebin, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fakebin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("COMPONENT_LOG", filepath.Join(home, "commands.log"))

	for _, name := range []string{"tmux", "pgrep", "cargo", "nvim", "ya", "git", "bash", "zsh", "brew"} {
		body := `#!/bin/sh
printf '%s %s\n' "` + name + `" "$*" >> "$COMPONENT_LOG"
if [ "` + name + `" = "pgrep" ]; then
  exit 0
fi
if [ "` + name + `" = "git" ] && [ "$1" = "config" ]; then
  exit 0
fi
exit 0
`
		writeTool(t, fakebin, name, body)
	}

	cacheDir := filepath.Join(home, ".cache", "zsh")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"a.zsh", "b.zsh", "keep.txt"} {
		if err := os.WriteFile(filepath.Join(cacheDir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := clearZshInitCaches(home, sc.Runner); err != nil {
		t.Fatalf("clearZshInitCaches: %v", err)
	}
	if _, err := os.Stat(filepath.Join(cacheDir, "a.zsh")); !os.IsNotExist(err) {
		t.Fatal("expected .zsh cache file removal")
	}
	if _, err := os.Stat(filepath.Join(cacheDir, "keep.txt")); err != nil {
		t.Fatalf("expected non-zsh file to remain: %v", err)
	}

	tpmScript := filepath.Join(home, ".tmux", "plugins", "tpm", "scripts", "install_plugins.sh")
	if err := os.MkdirAll(filepath.Dir(tpmScript), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tpmScript, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := setupTmux(context.Background(), sc); err != nil {
		t.Fatalf("setupTmux: %v", err)
	}

	blinkDir := filepath.Join(home, ".local", "share", "nvim", "site", "pack", "core", "opt", "blink.cmp")
	if err := os.MkdirAll(blinkDir, 0o755); err != nil {
		t.Fatal(err)
	}
	lazyDir := filepath.Join(home, ".local", "share", "nvim", "lazy")
	if err := os.MkdirAll(lazyDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := setupNeovim(context.Background(), sc); err != nil {
		t.Fatalf("setupNeovim: %v", err)
	}
	if _, err := os.Stat(lazyDir); !os.IsNotExist(err) {
		t.Fatal("expected stale lazy dir to be removed")
	}

	if err := setupYazi(context.Background(), sc); err != nil {
		t.Fatalf("setupYazi: %v", err)
	}
	if err := setupGit(context.Background(), sc); err != nil {
		t.Fatalf("setupGit: %v", err)
	}

	sc.Platform = &platform.Platform{OS: platform.Linux, Arch: platform.AMD64}
	if err := setupGhostty(context.Background(), sc); err != nil {
		t.Fatalf("setupGhostty: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(home, "commands.log"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	for _, want := range []string{
		"tmux source-file",
		"cargo build --release",
		"nvim --headless",
		"ya pkg install",
		"git config --global user.name",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("component log missing %q:\n%s", want, got)
		}
	}
}

// TestMaintainTmuxPluginsInstallsMissing verifies the fresh-install
// healing path: when tmux.conf declares plugins that aren't on disk
// AND TPM is on disk, MaintainTmuxPlugins must start the tmux server,
// source the config (so TPM can read TMUX_PLUGIN_MANAGER_PATH from
// the running server), and run install_plugins.sh — in that order.
func TestMaintainTmuxPluginsInstallsMissing(t *testing.T) {
	sc, home := newComponentSetup(t)

	fakebin := filepath.Join(home, "bin")
	if err := os.MkdirAll(fakebin, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fakebin+string(os.PathListSeparator)+os.Getenv("PATH"))
	logPath := filepath.Join(home, "commands.log")
	t.Setenv("COMPONENT_LOG", logPath)
	for _, name := range []string{"tmux", "chmod"} {
		writeTool(t, fakebin, name, `#!/bin/sh
printf '%s %s\n' "`+name+`" "$*" >> "$COMPONENT_LOG"
exit 0
`)
	}

	tmuxConfDir := filepath.Join(home, ".config", "tmux")
	if err := os.MkdirAll(tmuxConfDir, 0o755); err != nil {
		t.Fatal(err)
	}
	tmuxConf := filepath.Join(tmuxConfDir, "tmux.conf")
	if err := os.WriteFile(tmuxConf, []byte(sampleTmuxConf), 0o644); err != nil {
		t.Fatal(err)
	}

	tpmScript := filepath.Join(
		home, ".tmux", "plugins", "tpm", "scripts", "install_plugins.sh",
	)
	if err := os.MkdirAll(filepath.Dir(tpmScript), 0o755); err != nil {
		t.Fatal(err)
	}
	// Script writes to the shared command log so the test can assert
	// install_plugins.sh actually ran.
	if err := os.WriteFile(tpmScript, []byte(`#!/bin/sh
printf 'install_plugins.sh ran\n' >> "$COMPONENT_LOG"
exit 0
`), 0o755); err != nil {
		t.Fatal(err)
	}
	tpmDir := filepath.Join(home, ".tmux", "plugins", "tpm")
	if err := os.MkdirAll(tpmDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := MaintainTmuxPlugins(context.Background(), sc); err != nil {
		t.Fatalf("MaintainTmuxPlugins: %v", err)
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	logStr := string(data)
	for _, want := range []string{
		"tmux start-server",
		"tmux source-file",
		"chmod +x",
		"install_plugins.sh ran",
	} {
		if !strings.Contains(logStr, want) {
			t.Fatalf("expected %q in command log, got:\n%s", want, logStr)
		}
	}
}

// TestMaintainTmuxPluginsSkipsWhenTpmAbsent — defensive check: if the
// tpm dep regresses and maintain-tmux fires before TPM is cloned, we
// must NOT invoke install_plugins.sh (it doesn't exist) and must log
// the skip so the regression is observable.
func TestMaintainTmuxPluginsSkipsWhenTpmAbsent(t *testing.T) {
	sc, home := newComponentSetup(t)

	fakebin := filepath.Join(home, "bin")
	if err := os.MkdirAll(fakebin, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fakebin+string(os.PathListSeparator)+os.Getenv("PATH"))
	logPath := filepath.Join(home, "commands.log")
	t.Setenv("COMPONENT_LOG", logPath)
	writeTool(t, fakebin, "tmux", `#!/bin/sh
printf 'tmux %s\n' "$*" >> "$COMPONENT_LOG"
exit 0
`)

	tmuxConfDir := filepath.Join(home, ".config", "tmux")
	if err := os.MkdirAll(tmuxConfDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(tmuxConfDir, "tmux.conf"),
		[]byte(sampleTmuxConf), 0o644,
	); err != nil {
		t.Fatal(err)
	}
	// Note: TPM directory is intentionally NOT created.

	if err := MaintainTmuxPlugins(context.Background(), sc); err != nil {
		t.Fatalf("MaintainTmuxPlugins: %v", err)
	}

	if data, err := os.ReadFile(logPath); err == nil {
		if strings.Contains(string(data), "tmux start-server") {
			t.Fatalf("expected no tmux start-server when TPM absent, got:\n%s",
				string(data))
		}
	}
	if snap := sc.Failures.Snapshot(); len(snap) != 0 {
		t.Fatalf("expected no failures recorded, got %v", snap)
	}
}

// TestMaintainTmuxPluginsNoOpWhenAllPresent — the steady-state case:
// nothing missing, so we don't waste cycles starting a tmux server
// just to invoke install_plugins.sh against a no-op plugin set.
func TestMaintainTmuxPluginsNoOpWhenAllPresent(t *testing.T) {
	sc, home := newComponentSetup(t)

	fakebin := filepath.Join(home, "bin")
	if err := os.MkdirAll(fakebin, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fakebin+string(os.PathListSeparator)+os.Getenv("PATH"))
	logPath := filepath.Join(home, "commands.log")
	t.Setenv("COMPONENT_LOG", logPath)
	writeTool(t, fakebin, "tmux", `#!/bin/sh
printf 'tmux %s\n' "$*" >> "$COMPONENT_LOG"
exit 0
`)

	tmuxConfDir := filepath.Join(home, ".config", "tmux")
	if err := os.MkdirAll(tmuxConfDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(tmuxConfDir, "tmux.conf"),
		[]byte(sampleTmuxConf), 0o644,
	); err != nil {
		t.Fatal(err)
	}
	pluginsDir := filepath.Join(home, ".tmux", "plugins")
	for _, name := range []string{
		"tpm", "tmux-sensible", "tmux", "vim-tmux-navigator",
	} {
		if err := os.MkdirAll(filepath.Join(pluginsDir, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	if err := MaintainTmuxPlugins(context.Background(), sc); err != nil {
		t.Fatalf("MaintainTmuxPlugins: %v", err)
	}

	if data, err := os.ReadFile(logPath); err == nil {
		if strings.Contains(string(data), "tmux") {
			t.Fatalf("expected zero tmux invocations when all plugins present, got:\n%s",
				string(data))
		}
	}
}
