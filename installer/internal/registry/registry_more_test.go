package registry

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chaseddevelopment/dotfiles/installer/internal/executor"
	"github.com/chaseddevelopment/dotfiles/installer/internal/pkgmgr"
	"github.com/chaseddevelopment/dotfiles/installer/internal/platform"
)

type recordingPkgMgr struct {
	name       string
	installed  []string
	installErr error
	repaired   bool
}

func (r *recordingPkgMgr) Name() string              { return r.name }
func (r *recordingPkgMgr) IsInstalled(_ string) bool { return false }
func (r *recordingPkgMgr) Install(_ context.Context, names ...string) error {
	r.installed = append(r.installed, names...)
	return r.installErr
}
func (r *recordingPkgMgr) UpdateAll(context.Context) error { return nil }
func (r *recordingPkgMgr) MapName(name string) []string    { return []string{name} }
func (r *recordingPkgMgr) RunDpkgConfigureAll(context.Context) error {
	r.repaired = true
	return nil
}

var _ pkgmgr.PackageManager = (*recordingPkgMgr)(nil)

func newExecCtx(t *testing.T) (*InstallContext, string) {
	t.Helper()
	dir := t.TempDir()
	log, err := executor.NewLogFile(filepath.Join(dir, "test.log"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { log.Close() })
	return &InstallContext{
		Runner:   executor.NewRunner(log, false),
		PkgMgr:   &recordingPkgMgr{name: "brew"},
		Platform: &platform.Platform{},
	}, dir
}

func TestExecuteStrategyCommandMethods(t *testing.T) {
	ic, dir := newExecCtx(t)
	fakebin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(fakebin, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fakebin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("REGISTRY_LOG", filepath.Join(dir, "commands.log"))
	for _, name := range []string{"cargo", "git", "curl"} {
		if err := os.WriteFile(filepath.Join(fakebin, name), []byte(`#!/usr/bin/env bash
printf '%s %s\n' "`+name+`" "$*" >> "$REGISTRY_LOG"
if [ "`+name+`" = "curl" ]; then
  dest=""
  while [ "$#" -gt 0 ]; do
    if [ "$1" = "-o" ]; then
      dest="$2"
      shift 2
      continue
    fi
    shift
  done
  cat > "$dest" <<'EOF'
#!/usr/bin/env bash
echo script-ran >> "$REGISTRY_LOG"
EOF
fi
`), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	if err := executeStrategy(context.Background(), &InstallStrategy{
		Method: MethodCargo, Crate: "mycrate",
	}, ic, ic.Platform); err != nil {
		t.Fatalf("cargo strategy: %v", err)
	}
	if err := executeStrategy(context.Background(), &InstallStrategy{
		Method:   MethodGitClone,
		GitClone: &GitCloneConfig{Repo: "https://example.invalid/repo.git", Dest: "$HOME/repo", Depth: 1},
	}, ic, ic.Platform); err != nil {
		t.Fatalf("git clone strategy: %v", err)
	}
	if err := executeStrategy(context.Background(), &InstallStrategy{
		Method: MethodScript,
		Script: &ScriptConfig{URL: "https://example.invalid/install.sh", Shell: "bash", NoProfileModify: true},
	}, ic, ic.Platform); err != nil {
		t.Fatalf("script strategy: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "commands.log"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	for _, want := range []string{
		"cargo install mycrate",
		"git clone --depth=1 https://example.invalid/repo.git",
		"curl -fsSL https://example.invalid/install.sh",
		"script-ran",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("strategy log missing %q:\n%s", want, got)
		}
	}
}

func TestExecuteInstallAndPostActions(t *testing.T) {
	ic, dir := newExecCtx(t)
	fakebin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(fakebin, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fakebin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("POST_LOG", filepath.Join(dir, "post.log"))
	if err := os.WriteFile(filepath.Join(fakebin, "sudo"), []byte(`#!/usr/bin/env bash
printf '%s\n' "$*" >> "$POST_LOG"
exec "$@"
`), 0o755); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(dir, "src")
	tgt := filepath.Join(dir, "target")
	if err := os.WriteFile(src, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	pm := &recordingPkgMgr{name: "brew"}
	ic.PkgMgr = pm
	tool := &Tool{
		Name: "pkg-tool",
		Strategies: []InstallStrategy{{
			Method:  MethodPackageManager,
			Package: "fd-find",
			PostInstall: []PostAction{
				{Type: PostAddToPath, Target: filepath.Join(dir, "bin-extra")},
				{Type: PostSymlink, Source: src, Target: tgt},
			},
		}},
	}
	if err := ExecuteInstall(context.Background(), tool, ic, ic.Platform); err != nil {
		t.Fatalf("ExecuteInstall: %v", err)
	}
	if len(pm.installed) != 1 || pm.installed[0] != "fd-find" {
		t.Fatalf("unexpected installed packages: %#v", pm.installed)
	}
	if _, err := os.Lstat(tgt); err != nil {
		t.Fatalf("expected symlink target to exist: %v", err)
	}
	if !strings.Contains(strings.Join(ic.Runner.Env, " "), filepath.Join(dir, "bin-extra")) {
		t.Fatalf("PATH env not updated: %#v", ic.Runner.Env)
	}
}

func TestExecuteInstallSkippingPkgMgrAndRecovery(t *testing.T) {
	ic, _ := newExecCtx(t)
	pm := &recordingPkgMgr{name: "apt"}
	ic.PkgMgr = pm
	called := false
	tool := &Tool{
		Name: "recoverable",
		Strategies: []InstallStrategy{
			{Method: MethodPackageManager, Package: "one"},
			{Method: MethodCustom, CustomFunc: func(_ context.Context, _ *InstallContext) error {
				called = true
				return nil
			}},
		},
	}
	if err := ExecuteInstallSkippingPkgMgr(context.Background(), tool, ic, ic.Platform); err != nil {
		t.Fatalf("ExecuteInstallSkippingPkgMgr: %v", err)
	}
	if !called {
		t.Fatal("expected non-pkgmgr strategy to run")
	}

	attempts := 0
	tool = &Tool{
		Name: "healed",
		Strategies: []InstallStrategy{{
			Method: MethodCustom,
			CustomFunc: func(_ context.Context, _ *InstallContext) error {
				attempts++
				if attempts == 1 {
					return fmt.Errorf("wrapped: %w", pkgmgr.ErrDpkgInterrupted)
				}
				return nil
			},
		}},
	}
	if err := ExecuteInstall(context.Background(), tool, ic, ic.Platform); err != nil {
		t.Fatalf("ExecuteInstall recoverable: %v", err)
	}
	if !pm.repaired || attempts != 2 {
		t.Fatalf("expected one repair and one retry, repaired=%v attempts=%d", pm.repaired, attempts)
	}
}

func TestUtilityHelpers(t *testing.T) {
	if !ShouldInstall(&Tool{}, &platform.Platform{}) {
		t.Fatal("empty OS filter should install")
	}
	if FirstPkgMgrStrategy(&Tool{
		Strategies: []InstallStrategy{{Method: MethodPackageManager, Managers: []string{"brew"}}},
	}, "brew") == nil {
		t.Fatal("expected first package manager strategy")
	}
	err := executeStrategy(context.Background(), &InstallStrategy{Method: MethodCustom}, &InstallContext{}, &platform.Platform{})
	if err == nil || !strings.Contains(err.Error(), "missing custom function") {
		t.Fatalf("unexpected custom strategy error: %v", err)
	}
	if !isRecoverableDpkg(fmt.Errorf("wrapped: %w", pkgmgr.ErrDpkgLocked)) {
		t.Fatal("expected dpkg locked to be recoverable")
	}
	if isRecoverableDpkg(errors.New("other")) {
		t.Fatal("unexpected recoverable classification")
	}
}

func TestExecuteInstallFatalAndPostInstallErrors(t *testing.T) {
	ic, dir := newExecCtx(t)
	pm := &recordingPkgMgr{name: "apt"}
	ic.PkgMgr = pm

	fallbackRan := false
	tool := &Tool{
		Name: "fatal-apt",
		Strategies: []InstallStrategy{
			{
				Method:  MethodPackageManager,
				Package: "broken",
			},
			{
				Method: MethodCustom,
				CustomFunc: func(context.Context, *InstallContext) error {
					fallbackRan = true
					return nil
				},
			},
		},
	}
	pm.installErr = fmt.Errorf("wrapped: %w", pkgmgr.ErrAptFatal)
	err := ExecuteInstall(context.Background(), tool, ic, ic.Platform)
	if err == nil || !strings.Contains(err.Error(), "not attempting fallback strategies") {
		t.Fatalf("expected apt fatal terminal error, got %v", err)
	}
	if fallbackRan {
		t.Fatal("fallback strategy should not run after apt fatal error")
	}

	src := filepath.Join(dir, "source")
	if err := os.WriteFile(src, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	postTool := &Tool{
		Name: "post-failure",
		Strategies: []InstallStrategy{{
			Method:     MethodCustom,
			CustomFunc: func(context.Context, *InstallContext) error { return nil },
			PostInstall: []PostAction{
				{Type: PostSymlink, Source: src, Target: filepath.Join(dir, "target")},
			},
		}},
	}
	ic.PkgMgr = &recordingPkgMgr{name: "brew"}
	if err := ExecuteInstall(context.Background(), postTool, ic, ic.Platform); err == nil || !strings.Contains(err.Error(), "post-install after strategy") {
		t.Fatalf("expected post-install failure, got %v", err)
	}
}

func TestExecuteInstallNoApplicableStrategyAndScriptEnv(t *testing.T) {
	ic, dir := newExecCtx(t)
	tool := &Tool{
		Name: "linux-only",
		Strategies: []InstallStrategy{{
			Method:   MethodPackageManager,
			Package:  "pkg",
			Managers: []string{"apt"},
		}},
	}
	if err := ExecuteInstall(context.Background(), tool, ic, ic.Platform); err == nil || !strings.Contains(err.Error(), "no applicable install strategies") {
		t.Fatalf("expected no applicable strategy error, got %v", err)
	}

	fakebin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(fakebin, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fakebin+string(os.PathListSeparator)+os.Getenv("PATH"))
	envLog := filepath.Join(dir, "env.log")
	t.Setenv("SCRIPT_ENV_LOG", envLog)
	if err := os.WriteFile(filepath.Join(fakebin, "bash"), []byte(`#!/bin/sh
printf '%s|%s|%s|%s\n' "$1" "$PROFILE" "$SHELL" "$INSTALLER_NO_MODIFY_PATH" > "$SCRIPT_ENV_LOG"
`), 0o755); err != nil {
		t.Fatal(err)
	}

	scriptPath := filepath.Join(dir, "installer.sh")
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := &ScriptConfig{
		Shell:           "bash",
		Args:            []string{"$HOME/bin"},
		NoProfileModify: true,
	}
	t.Setenv("HOME", "/home/tester")
	if err := executeScriptFile(context.Background(), cfg, scriptPath, ic); err != nil {
		t.Fatalf("executeScriptFile: %v", err)
	}
	data, err := os.ReadFile(envLog)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	for _, want := range []string{scriptPath, "/dev/null", "/bin/sh", "1"} {
		if !strings.Contains(got, want) {
			t.Fatalf("script env log missing %q: %s", want, got)
		}
	}
}
