package pkgmgr

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chaseddevelopment/dotfiles/installer/internal/executor"
)

func newAptRunner(t *testing.T) (*executor.Runner, string) {
	t.Helper()
	dir := t.TempDir()
	log, err := executor.NewLogFile(filepath.Join(dir, "test.log"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { log.Close() })
	return executor.NewRunner(log, false), dir
}

func writeExec(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestAptInstallIsInstalledAndUpdate(t *testing.T) {
	runner, dir := newAptRunner(t)
	fakebin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(fakebin, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fakebin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("APT_LOG", filepath.Join(dir, "apt.log"))
	writeExec(t, fakebin, "sudo", `#!/bin/sh
printf 'sudo %s\n' "$*" >> "$APT_LOG"
exec "$@"
`)
	writeExec(t, fakebin, "nala", `#!/bin/sh
printf 'nala %s\n' "$*" >> "$APT_LOG"
exit 0
`)
	writeExec(t, fakebin, "dpkg-query", `#!/bin/sh
printf 'install ok installed'
`)

	a := NewApt(runner, true)
	if a.Name() != "apt" {
		t.Fatalf("Name() = %q", a.Name())
	}
	if got := aptEnv(); len(got) != 4 {
		t.Fatalf("aptEnv len = %d", len(got))
	}
	if got := a.MapName("nodejs"); strings.Join(got, ",") != "nodejs,npm" {
		t.Fatalf("MapName(nodejs) = %#v", got)
	}
	if err := a.Install(context.Background(), "nodejs", "fd"); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if !a.didUpdate {
		t.Fatal("expected ensureUpdated to mark didUpdate")
	}
	if !a.IsInstalled("nodejs") {
		t.Fatal("expected nodejs to be installed")
	}
	if err := a.UpdateAll(context.Background()); err != nil {
		t.Fatalf("UpdateAll: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "apt.log"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	for _, want := range []string{
		"sudo nala update",
		"sudo nala install -o DPkg::Lock::Timeout=60 -y nodejs npm fd-find",
		"sudo nala upgrade -y",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("apt log missing %q:\n%s", want, got)
		}
	}
}

func TestAptHealthAndRepair(t *testing.T) {
	runner, dir := newAptRunner(t)
	fakebin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(fakebin, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fakebin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("APT_LOG", filepath.Join(dir, "apt.log"))
	stateFile := filepath.Join(dir, "audit-state")
	writeExec(t, fakebin, "dpkg", `#!/bin/sh
if [ "$1" = "--audit" ]; then
  if [ -f "`+stateFile+`" ]; then
    exit 0
  fi
  printf 'packages are broken'
  exit 1
fi
exit 0
`)
	writeExec(t, fakebin, "sudo", `#!/bin/sh
printf 'sudo %s\n' "$*" >> "$APT_LOG"
if [ "$1" = "dpkg" ] && [ "$2" = "--configure" ] && [ "$3" = "-a" ]; then
  : > "`+stateFile+`"
  exit 0
fi
exec "$@"
`)

	a := NewApt(runner, false)
	// NewApt now defaults UserApprovedRepair=false; this branch of
	// the test exercises the post-consent repair path, so opt in
	// explicitly as the TUI would after a user presses "r".
	a.UserApprovedRepair = true
	state, err := a.DetectDpkgHealth(context.Background())
	if err != nil {
		t.Fatalf("DetectDpkgHealth: %v", err)
	}
	if state.Healthy {
		t.Fatal("expected unhealthy state before repair")
	}
	if err := a.EnsureHealthy(context.Background()); err != nil {
		t.Fatalf("EnsureHealthy: %v", err)
	}
	if !a.repaired {
		t.Fatal("expected repair to run")
	}

	b := NewApt(runner, false)
	b.UserApprovedRepair = false
	if _, err := os.Stat(stateFile); err == nil {
		if err := os.Remove(stateFile); err != nil {
			t.Fatal(err)
		}
	}
	err = b.EnsureHealthy(context.Background())
	if err == nil || !strings.Contains(err.Error(), "repair was not approved") {
		t.Fatalf("unexpected unapproved repair error: %v", err)
	}
}
