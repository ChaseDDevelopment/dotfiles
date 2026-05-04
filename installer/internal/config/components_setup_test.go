package config

import (
	"context"
	"encoding/json"
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

func TestSetupPiCreatesSettings(t *testing.T) {
	sc, home := newComponentSetup(t)

	if err := setupPi(sc); err != nil {
		t.Fatalf("setupPi: %v", err)
	}

	settingsPath := filepath.Join(home, ".pi", "agent", "settings.json")
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}

	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatal(err)
	}
	if settings["shellCommandPrefix"] != piShellCommandPrefix {
		t.Fatalf(
			"shellCommandPrefix = %q, want %q",
			settings["shellCommandPrefix"], piShellCommandPrefix,
		)
	}
}

func TestSetupPiPreservesExistingSettings(t *testing.T) {
	sc, home := newComponentSetup(t)
	agentDir := filepath.Join(home, ".pi", "agent")
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	settingsPath := filepath.Join(agentDir, "settings.json")
	existing := `{
  "defaultProvider": "openai-codex",
  "packages": ["npm:context-mode"],
  "shellCommandPrefix": "old",
  "terminal": {"showTerminalProgress": true}
}
`
	if err := os.WriteFile(settingsPath, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := setupPi(sc); err != nil {
		t.Fatalf("setupPi: %v", err)
	}

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatal(err)
	}
	if settings["defaultProvider"] != "openai-codex" {
		t.Fatalf("defaultProvider was not preserved: %#v", settings)
	}
	packages, ok := settings["packages"].([]any)
	if !ok || len(packages) != 1 || packages[0] != "npm:context-mode" {
		t.Fatalf("packages were not preserved: %#v", settings["packages"])
	}
	terminal, ok := settings["terminal"].(map[string]any)
	if !ok || terminal["showTerminalProgress"] != true {
		t.Fatalf("terminal settings were not preserved: %#v", settings["terminal"])
	}
	if settings["shellCommandPrefix"] != piShellCommandPrefix {
		t.Fatalf(
			"shellCommandPrefix = %q, want %q",
			settings["shellCommandPrefix"], piShellCommandPrefix,
		)
	}
}

func TestSetupPiInvalidSettingsFailsLoudly(t *testing.T) {
	sc, home := newComponentSetup(t)
	agentDir := filepath.Join(home, ".pi", "agent")
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	settingsPath := filepath.Join(agentDir, "settings.json")
	if err := os.WriteFile(settingsPath, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := setupPi(sc)
	if err == nil {
		t.Fatal("expected setupPi to reject invalid JSON")
	}
	if !strings.Contains(err.Error(), "parse Pi settings") {
		t.Fatalf("expected parse error, got %v", err)
	}
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

// TestMaintainTmuxPluginsPreClonesSkipRecursive — plugins listed in
// tmuxPluginSkipRecursive (e.g. tmux-powerkit, whose upstream ships a
// wiki submodule that breaks TPM's `git clone --recursive`) must be
// cloned by us shallow + --no-recurse-submodules BEFORE TPM runs, so
// TPM sees the dir on disk and skips its own broken attempt.
func TestMaintainTmuxPluginsPreClonesSkipRecursive(t *testing.T) {
	sc, home := newComponentSetup(t)

	fakebin := filepath.Join(home, "bin")
	if err := os.MkdirAll(fakebin, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fakebin+string(os.PathListSeparator)+os.Getenv("PATH"))
	logPath := filepath.Join(home, "commands.log")
	t.Setenv("COMPONENT_LOG", logPath)
	for _, name := range []string{"tmux", "chmod", "git"} {
		writeTool(t, fakebin, name, `#!/bin/sh
printf '`+name+` %s\n' "$*" >> "$COMPONENT_LOG"
exit 0
`)
	}

	tmuxConfDir := filepath.Join(home, ".config", "tmux")
	if err := os.MkdirAll(tmuxConfDir, 0o755); err != nil {
		t.Fatal(err)
	}
	tmuxConf := filepath.Join(tmuxConfDir, "tmux.conf")
	confBody := `set -g @plugin 'tmux-plugins/tpm'
set -g @plugin 'tmux-plugins/tmux-sensible'
set -g @plugin 'fabioluciano/tmux-powerkit'
`
	if err := os.WriteFile(tmuxConf, []byte(confBody), 0o644); err != nil {
		t.Fatal(err)
	}

	tpmScript := filepath.Join(
		home, ".tmux", "plugins", "tpm", "scripts", "install_plugins.sh",
	)
	if err := os.MkdirAll(filepath.Dir(tpmScript), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tpmScript, []byte(`#!/bin/sh
printf 'install_plugins.sh ran\n' >> "$COMPONENT_LOG"
exit 0
`), 0o755); err != nil {
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

	cloneIdx := strings.Index(logStr, "git clone --depth=1 --single-branch --no-recurse-submodules")
	if cloneIdx < 0 {
		t.Fatalf("expected pre-clone of tmux-powerkit before TPM, log:\n%s", logStr)
	}
	if !strings.Contains(logStr, "fabioluciano/tmux-powerkit") {
		t.Fatalf("pre-clone did not target tmux-powerkit, log:\n%s", logStr)
	}

	tpmIdx := strings.Index(logStr, "install_plugins.sh ran")
	if tpmIdx < 0 {
		t.Fatalf("install_plugins.sh did not run, log:\n%s", logStr)
	}
	if cloneIdx > tpmIdx {
		t.Fatalf("pre-clone must happen before TPM, got clone=%d tpm=%d log:\n%s",
			cloneIdx, tpmIdx, logStr)
	}

	// tmux-sensible is declared but NOT in skipRecursive — confirm we
	// didn't also pre-clone it (only entries in the map get pre-cloned).
	if strings.Contains(logStr, "tmux-sensible.git") {
		t.Fatalf("only skipRecursive plugins should pre-clone, log:\n%s", logStr)
	}
}
