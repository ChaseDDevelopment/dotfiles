package config

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chaseddevelopment/dotfiles/installer/internal/backup"
	"github.com/chaseddevelopment/dotfiles/installer/internal/executor"
)

func newSetupContext(t *testing.T) (*SetupContext, string) {
	t.Helper()
	dir := t.TempDir()
	log, err := executor.NewLogFile(filepath.Join(dir, "test.log"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { log.Close() })
	runner := executor.NewRunner(log, false)
	return &SetupContext{
		Runner:   runner,
		RootDir:  dir,
		Backup:   backup.NewManager(false),
		Failures: NewTrackedFailures(),
	}, dir
}

func TestRunUserHookAndBestEffort(t *testing.T) {
	sc, dir := newSetupContext(t)
	hookDir := filepath.Join(dir, "configs", "zsh", "hooks")
	if err := os.MkdirAll(hookDir, 0o755); err != nil {
		t.Fatal(err)
	}
	hookOut := filepath.Join(dir, "hook.out")
	t.Setenv("HOOK_OUT", hookOut)
	if err := os.WriteFile(filepath.Join(hookDir, "post-install.sh"), []byte("#!/usr/bin/env bash\nprintf 'hooked' > \"$HOOK_OUT\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := runUserHook(context.Background(), "Zsh", sc); err != nil {
		t.Fatalf("runUserHook: %v", err)
	}
	data, err := os.ReadFile(hookOut)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hooked" {
		t.Fatalf("hook output = %q", data)
	}

	sc.Component = "Zsh"
	bestEffort(sc, "compile plugins", func() error { return errors.New("boom") })
	formatted := sc.Failures.Format()
	if !strings.Contains(formatted, "compile plugins") || !strings.Contains(formatted, "boom") {
		t.Fatalf("bestEffort did not record failure:\n%s", formatted)
	}
}

func TestRunPostInstallUnknownAndSetupComponentRequiredCmd(t *testing.T) {
	sc, _ := newSetupContext(t)
	if err := runPostInstall(context.Background(), "Unknown", sc); err != nil {
		t.Fatalf("unknown post-install should be nil, got %v", err)
	}

	t.Setenv("PATH", "")
	err := SetupComponent(context.Background(), Component{Name: "Missing", RequiredCmd: "definitely-not-on-path"}, sc)
	if err == nil || !strings.Contains(err.Error(), "requires definitely-not-on-path") {
		t.Fatalf("unexpected SetupComponent error: %v", err)
	}
}
