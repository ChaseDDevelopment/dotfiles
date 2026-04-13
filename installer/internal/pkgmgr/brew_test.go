package pkgmgr

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chaseddevelopment/dotfiles/installer/internal/executor"
)

func newPkgRunner(t *testing.T) (*executor.Runner, string) {
	t.Helper()
	dir := t.TempDir()
	log, err := executor.NewLogFile(filepath.Join(dir, "test.log"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { log.Close() })
	return executor.NewRunner(log, false), dir
}

func TestBrewInstallAndIsInstalled(t *testing.T) {
	runner, dir := newPkgRunner(t)
	fakebin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(fakebin, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fakebin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("BREW_LOG", filepath.Join(dir, "brew.log"))
	if err := os.WriteFile(filepath.Join(fakebin, "brew"), []byte(`#!/usr/bin/env bash
printf '%s %s\n' "$1" "$*" >> "$BREW_LOG"
if [ "$1" = "list" ]; then
  if [ "$2" = "node" ]; then
    exit 0
  fi
  exit 1
fi
`), 0o755); err != nil {
		t.Fatal(err)
	}

	b := &Brew{runner: runner}
	if err := b.Install(context.Background(), "nodejs", "build-essential", "nodejs"); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if !b.IsInstalled("nodejs") {
		t.Fatal("expected nodejs to be installed via brew list")
	}
	if b.IsInstalled("bat") {
		t.Fatal("expected bat to be absent")
	}

	data, err := os.ReadFile(filepath.Join(dir, "brew.log"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	if !strings.Contains(got, "install install node") {
		t.Fatalf("unexpected brew install log:\n%s", got)
	}
}
