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

// TestLoginShellFor verifies that loginShellFor parses the 7th
// colon-separated field of `getent passwd <user>` output. Using
// /etc/passwd (via getent) rather than $SHELL matters because
// $SHELL reflects the *current* shell, not what /bin/login will
// exec at the next session — and every "zsh broke my Proxmox
// login" forum thread roots at that distinction.
func TestLoginShellFor(t *testing.T) {
	fakebin := t.TempDir()
	write := func(script string) {
		t.Helper()
		p := filepath.Join(fakebin, "getent")
		if err := os.WriteFile(p, []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", fakebin+string(os.PathListSeparator)+os.Getenv("PATH"))

	cases := []struct {
		name, user, script, want string
	}{
		{
			name:   "zsh user",
			user:   "alice",
			script: "#!/bin/sh\nprintf 'alice:x:1000:1000:Alice,,,:/home/alice:/usr/bin/zsh\\n'\n",
			want:   "/usr/bin/zsh",
		},
		{
			name:   "bash user with trailing newline",
			user:   "bob",
			script: "#!/bin/sh\nprintf 'bob:x:1001:1001::/home/bob:/bin/bash\\n'\n",
			want:   "/bin/bash",
		},
		{
			name:   "empty user returns empty",
			user:   "",
			script: "#!/bin/sh\nexit 0\n",
			want:   "",
		},
		{
			name:   "getent failure returns empty",
			user:   "ghost",
			script: "#!/bin/sh\nexit 2\n",
			want:   "",
		},
		{
			name:   "malformed passwd line returns empty",
			user:   "weird",
			script: "#!/bin/sh\nprintf 'weird:x:1002:1002\\n'\n",
			want:   "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			write(tc.script)
			got := loginShellFor(tc.user)
			if got != tc.want {
				t.Fatalf("loginShellFor(%q) = %q, want %q",
					tc.user, got, tc.want)
			}
		})
	}
}

// newShellSwitchContext builds a minimal SetupContext for the
// EnsureLoginShellIsZsh tests. It redirects PATH to a temp bin so
// tests can stub getent/zsh/sudo/chsh/su without touching the real
// system.
func newShellSwitchContext(t *testing.T) (*SetupContext, string, string) {
	t.Helper()
	home := t.TempDir()
	fakebin := filepath.Join(home, "bin")
	if err := os.MkdirAll(fakebin, 0o755); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(home, "install.log")
	log, err := executor.NewLogFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { log.Close() })
	runner := executor.NewRunner(log, false)
	t.Setenv("HOME", home)
	t.Setenv("USER", "testuser")
	t.Setenv("PATH", fakebin+string(os.PathListSeparator)+os.Getenv("PATH"))

	sc := &SetupContext{
		Runner:   runner,
		RootDir:  home,
		Backup:   backup.NewManager(false),
		Platform: &platform.Platform{OS: platform.Linux, Arch: platform.AMD64},
		Failures: NewTrackedFailures(),
	}
	return sc, fakebin, logPath
}

func writeStub(t *testing.T, dir, name, body string) {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o755); err != nil {
		t.Fatalf("write stub %s: %v", name, err)
	}
}

// TestEnsureLoginShellIsZsh_AlreadyZshNoop covers the idempotent
// path: when the user's login shell is already zsh, the function
// returns without attempting chsh (so reruns on healthy hosts
// don't re-chsh or re-verify).
func TestEnsureLoginShellIsZsh_AlreadyZshNoop(t *testing.T) {
	sc, fakebin, logPath := newShellSwitchContext(t)
	writeStub(t, fakebin, "getent",
		"#!/bin/sh\nprintf 'testuser:x:1000:1000::/home/testuser:/usr/bin/zsh\\n'\n")
	writeStub(t, fakebin, "zsh", "#!/bin/sh\nexit 0\n")
	// chsh stub that fails if invoked — the no-op path must not
	// reach it.
	writeStub(t, fakebin, "chsh",
		"#!/bin/sh\necho chsh-was-called >&2; exit 1\n")
	writeStub(t, fakebin, "sudo", "#!/bin/sh\nexec \"$@\"\n")

	if err := EnsureLoginShellIsZsh(context.Background(), sc); err != nil {
		t.Fatalf("EnsureLoginShellIsZsh: %v", err)
	}

	got, _ := os.ReadFile(logPath)
	if strings.Contains(string(got), "chsh") {
		t.Fatalf("log should not mention chsh when already-zsh:\n%s", string(got))
	}
	if len(sc.Failures.Snapshot()) > 0 {
		t.Fatalf("no failures expected on already-zsh path, got %d", len(sc.Failures.Snapshot()))
	}
}

// TestEnsureLoginShellIsZsh_NoZshBinaryNoop covers the defensive
// path: when zsh isn't on PATH (and therefore not installed),
// there's nothing to chsh to, and the function should no-op
// without recording a failure. This matters on macOS where zsh is
// always present but on a partial-install Linux the task could
// fire before the apt batch finishes installing zsh.
func TestEnsureLoginShellIsZsh_NoZshBinaryNoop(t *testing.T) {
	sc, fakebin, _ := newShellSwitchContext(t)
	writeStub(t, fakebin, "getent",
		"#!/bin/sh\nprintf 'testuser:x:1000:1000::/home/testuser:/bin/bash\\n'\n")
	// Intentionally no zsh stub. Override PATH to JUST fakebin so
	// the real system zsh (present on every dev machine) can't be
	// found via fallback lookup.
	t.Setenv("PATH", fakebin)

	if err := EnsureLoginShellIsZsh(context.Background(), sc); err != nil {
		t.Fatalf("EnsureLoginShellIsZsh: %v", err)
	}
	if len(sc.Failures.Snapshot()) > 0 {
		t.Fatalf("no failures expected when zsh missing, got %d",
			len(sc.Failures.Snapshot()))
	}
}

// TestEnsureLoginShellIsZsh_SudoUnavailableRecordsFailure covers
// the failure-reporting contract: when chsh can't run (sudo not
// cached in this case), the function records the failure to
// Failures and logs a "run chsh manually" hint, but does NOT
// return an error — chsh hiccups must never abort the install.
func TestEnsureLoginShellIsZsh_SudoUnavailableRecordsFailure(t *testing.T) {
	sc, fakebin, logPath := newShellSwitchContext(t)
	writeStub(t, fakebin, "getent",
		"#!/bin/sh\nprintf 'testuser:x:1000:1000::/home/testuser:/bin/bash\\n'\n")
	writeStub(t, fakebin, "zsh", "#!/bin/sh\nexit 0\n")
	// sudo stub that fails both the -v probe (NeedsSudo=true) and
	// any real invocation. setDefaultShellZsh sees NeedsSudo=true
	// and returns early with "sudo credentials not cached".
	writeStub(t, fakebin, "sudo",
		"#!/bin/sh\nexit 1\n")

	if err := EnsureLoginShellIsZsh(context.Background(), sc); err != nil {
		t.Fatalf("EnsureLoginShellIsZsh should never return err: %v", err)
	}
	if len(sc.Failures.Snapshot()) == 0 {
		t.Fatal("expected chsh failure to be recorded in Failures")
	}

	got, _ := os.ReadFile(logPath)
	if !strings.Contains(string(got), "chsh to /") {
		t.Fatalf("log should carry the manual-chsh hint:\n%s", string(got))
	}
}
