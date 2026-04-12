package registry

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chaseddevelopment/dotfiles/installer/internal/executor"
	"github.com/chaseddevelopment/dotfiles/installer/internal/pkgmgr"
	"github.com/chaseddevelopment/dotfiles/installer/internal/platform"
)

// stubPkgMgr implements pkgmgr.PackageManager with a fixed name so
// AppliesTo works. All operations are no-ops — these tests only
// exercise the strategy-selection loop, not real installs.
type stubPkgMgr struct{ name string }

func (s *stubPkgMgr) Name() string                               { return s.name }
func (s *stubPkgMgr) IsInstalled(_ string) bool                  { return false }
func (s *stubPkgMgr) Install(_ context.Context, _ ...string) error { return nil }
func (s *stubPkgMgr) UpdateAll(_ context.Context) error          { return nil }
func (s *stubPkgMgr) MapName(_ string) []string                  { return nil }

var _ pkgmgr.PackageManager = (*stubPkgMgr)(nil)

func newTestCtx(t *testing.T) *InstallContext {
	t.Helper()
	logPath := filepath.Join(t.TempDir(), "test.log")
	log, err := executor.NewLogFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { log.Close() })
	return &InstallContext{
		Runner:   executor.NewRunner(log, true), // dry-run so no shelling
		PkgMgr:   &stubPkgMgr{name: "brew"},
		Platform: &platform.Platform{},
	}
}

// TestExecuteInstall_StrategyFailureFallsThrough verifies the
// opposite direction: when the install step itself fails (no
// binary was placed), the loop moves on to the next strategy.
// This is the pre-existing behavior that the post-install fix
// must preserve.
func TestExecuteInstall_StrategyFailureFallsThrough(t *testing.T) {
	ic := newTestCtx(t)
	strategy2Called := false
	tool := &Tool{
		Name:    "fake-fallthrough",
		Command: "fake-fallthrough",
		Strategies: []InstallStrategy{
			{
				Method: MethodCustom,
				CustomFunc: func(_ context.Context, _ *InstallContext) error {
					return errors.New("brew-unavailable")
				},
			},
			{
				Method: MethodCustom,
				CustomFunc: func(_ context.Context, _ *InstallContext) error {
					strategy2Called = true
					return nil
				},
			},
		},
	}
	if err := ExecuteInstall(
		context.Background(), tool, ic, ic.Platform,
	); err != nil {
		t.Fatalf("expected success via fallthrough, got %v", err)
	}
	if !strategy2Called {
		t.Fatal("strategy 2 was not attempted after strategy 1 install failure")
	}
}

// TestExecuteScript_NoProfileModifyInjectsEnv confirms the
// opt-out env vars are exported to the child process when a
// ScriptConfig sets NoProfileModify. Regression guard for the
// "install script mutates symlinked zsh configs" bug — if this
// breaks, the upstream installer can freely append to the repo.
func TestExecuteScript_NoProfileModifyInjectsEnv(t *testing.T) {
	ic := newTestCtx(t)
	// Take the Runner out of dry-run so the probe actually executes.
	ic.Runner.DryRun = false

	tmp := t.TempDir()
	envOut := filepath.Join(tmp, "child-env.txt")
	// Write a script that dumps its env to a file whose path is
	// passed as arg $1. The installer wraps scripts with
	// `bash <scriptPath> <args...>` so $1 inside the script is
	// this envOut path.
	probeScript := filepath.Join(tmp, "probe.sh")
	scriptBody := "#!/usr/bin/env bash\nenv > \"$1\"\n"
	if err := os.WriteFile(
		probeScript, []byte(scriptBody), 0o755,
	); err != nil {
		t.Fatal(err)
	}

	// Stand up an http server that serves probeScript as the install
	// payload — executeScript always downloads from URL, so we need
	// a real endpoint. Using a local listener keeps the test offline.
	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte(scriptBody))
		},
	))
	defer srv.Close()

	strategy := &InstallStrategy{
		Method: MethodScript,
		Script: &ScriptConfig{
			URL:             srv.URL,
			Args:            []string{envOut},
			Shell:           "bash",
			NoProfileModify: true,
		},
	}
	if err := executeStrategy(
		context.Background(), strategy, ic, ic.Platform,
	); err != nil {
		t.Fatalf("executeStrategy: %v", err)
	}

	data, err := os.ReadFile(envOut)
	if err != nil {
		t.Fatalf("child env file missing: %v", err)
	}
	got := string(data)
	for _, needle := range []string{
		"PROFILE=/dev/null",
		"SHELL=/bin/sh",
		"INSTALLER_NO_MODIFY_PATH=1",
	} {
		if !strings.Contains(got, needle) {
			t.Errorf(
				"child env missing %q\nfull env:\n%s",
				needle, got,
			)
		}
	}
}

// TestExecuteInstall_NoApplicableStrategies surfaces a loud error
// when every strategy is filtered out by Managers. Pre-regression,
// the installer would silently succeed with a stale binary.
func TestExecuteInstall_NoApplicableStrategies(t *testing.T) {
	ic := newTestCtx(t)
	tool := &Tool{
		Name: "apt-only",
		Strategies: []InstallStrategy{
			{
				Method:   MethodPackageManager,
				Managers: []string{"apt"},
				Package:  "foo",
			},
		},
	}
	err := ExecuteInstall(context.Background(), tool, ic, ic.Platform)
	if err == nil {
		t.Fatal("expected error for no applicable strategies")
	}
	if !strings.Contains(err.Error(), "no applicable install strategies") {
		t.Fatalf("unexpected error: %v", err)
	}
}
