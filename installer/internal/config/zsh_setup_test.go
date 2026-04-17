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

func TestSetupZsh(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("SHELL", "/bin/bash")

	log, err := executor.NewLogFile(filepath.Join(home, "test.log"))
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	runner := executor.NewRunner(log, false)

	fakebin := filepath.Join(home, "bin")
	if err := os.MkdirAll(fakebin, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fakebin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("ZSH_LOG", filepath.Join(home, "zsh.log"))

	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(fakebin, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"brew", "git", "zsh"} {
		write(name, `#!/bin/sh
printf '%s %s\n' "`+name+`" "$*" >> "$ZSH_LOG"
exit 0
`)
	}

	pluginsDir := filepath.Join(home, ".config", "zsh", "plugins")
	if err := os.MkdirAll(pluginsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pluginsDir, ".zsh_plugins.txt"), []byte("plugin\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cacheDir := filepath.Join(home, ".cache", "zsh")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cacheDir, "old.zsh"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(home, ".zshrc")
	if err := os.WriteFile(stale, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	antidotePath := filepath.Join(home, ".config", "zsh", ".antidote", "antidote.zsh")
	if err := os.MkdirAll(filepath.Dir(antidotePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(antidotePath, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	sc := &SetupContext{
		Runner:   runner,
		RootDir:  home,
		Backup:   backup.NewManager(false),
		Platform: &platform.Platform{OS: platform.Linux, Arch: platform.AMD64},
		Failures: NewTrackedFailures(),
	}
	if err := setupZsh(context.Background(), sc); err != nil {
		t.Fatalf("setupZsh: %v", err)
	}

	for _, dir := range []string{
		filepath.Join(home, ".config"),
		filepath.Join(home, ".local", "share"),
		filepath.Join(home, ".cache", "ohmyzsh", "completions"),
	} {
		if _, err := os.Stat(dir); err != nil {
			t.Fatalf("expected directory %s: %v", dir, err)
		}
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatalf("expected stale .zshrc removal, err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(cacheDir, "old.zsh")); !os.IsNotExist(err) {
		t.Fatalf("expected zsh cache cleared, err=%v", err)
	}
	if !sc.Backup.Exists() {
		t.Fatal("expected backup for stale .zshrc")
	}
	data, err := os.ReadFile(filepath.Join(home, "zsh.log"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	if !strings.Contains(got, "zsh -c") {
		t.Fatalf("expected plugin compile attempt in log:\n%s", got)
	}
}
