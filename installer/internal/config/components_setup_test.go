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

	for _, name := range []string{"tmux", "pgrep", "cargo", "nvim", "starship", "ya", "git", "bash", "zsh", "brew"} {
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

	if err := setupStarship(context.Background(), sc); err != nil {
		t.Fatalf("setupStarship: %v", err)
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
		"starship preset catppuccin-powerline",
		"ya pkg install",
		"git config --global user.name",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("component log missing %q:\n%s", want, got)
		}
	}
}
