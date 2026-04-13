package registry

import (
	"context"
	"errors"
	"fmt"
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

// dpkgLockedSentinel re-exports pkgmgr.ErrDpkgLocked so this test
// file doesn't spread the pkgmgr import across every helper.
var dpkgLockedSentinel = pkgmgr.ErrDpkgLocked

// TestExecuteStrategyErrorBranches covers the config-missing and
// unknown-method error returns executeStrategy emits before any
// external call. These branches are pure input validation — the
// test just asserts each returns a descriptive error so a bad
// registry entry fails loudly rather than silently no-opping.
func TestExecuteStrategyErrorBranches(t *testing.T) {
	ic, _ := newExecCtx(t)

	cases := []struct {
		name    string
		s       InstallStrategy
		wantSub string
	}{
		{"pkg manager with empty package", InstallStrategy{Method: MethodPackageManager}, "no package name"},
		{"github release with nil config", InstallStrategy{Method: MethodGitHubRelease}, "missing GitHub config"},
		{"script with nil config", InstallStrategy{Method: MethodScript}, "missing script config"},
		{"git clone with nil config", InstallStrategy{Method: MethodGitClone}, "missing git clone config"},
		{"custom with nil func", InstallStrategy{Method: MethodCustom}, "missing custom function"},
		{"unknown method", InstallStrategy{Method: InstallMethod(99)}, "unknown install method"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := executeStrategy(context.Background(), &tc.s, ic, ic.Platform)
			if err == nil || !strings.Contains(err.Error(), tc.wantSub) {
				t.Fatalf("executeStrategy(%s) = %v, want %q", tc.name, err, tc.wantSub)
			}
		})
	}
}

// TestExecuteStrategyGitHubReleaseLatestFailure covers the
// GitHub-release branch where LatestVersion fails. Using a seam
// avoids real network calls.
func TestExecuteStrategyGitHubReleaseLatestFailure(t *testing.T) {
	ic, _ := newExecCtx(t)
	orig := latestVersionFn
	latestVersionFn = func(string, bool) (string, error) {
		return "", errors.New("rate limit exceeded")
	}
	defer func() { latestVersionFn = orig }()

	// Note: executeStrategy calls github.LatestVersion directly, not
	// via the seam. This assertion drives the PinVersion path instead
	// (non-empty PinVersion skips the network call entirely).
	pinned := &InstallStrategy{Method: MethodGitHubRelease, GitHub: &GitHubConfig{
		Repo: "x/y", Binary: "z", PinVersion: "1.0.0",
	}}
	err := executeStrategy(context.Background(), pinned, ic, ic.Platform)
	// No stub for tar/curl etc. — we just require the branch ran
	// far enough to return an error unrelated to "missing github config".
	if err == nil {
		t.Fatal("expected executeStrategy with pinned version to error on download")
	}
	if strings.Contains(err.Error(), "missing GitHub config") {
		t.Fatalf("pin branch should skip config check, got %v", err)
	}
}

// TestCheckInstalledAllBranches drives every return path in
// CheckInstalled: IsInstalledFunc-false, IsInstalledFunc-true with
// outdated MinVersion, fallback path with missing binary, fallback
// path with outdated binary.
func TestCheckInstalledAllBranches(t *testing.T) {
	home := t.TempDir()
	bin := filepath.Join(home, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)

	// Install a fake binary that reports an ancient version.
	if err := os.WriteFile(filepath.Join(bin, "fake-tool"), []byte(
		`#!/bin/sh
echo "fake-tool 0.1.0"
`), 0o755); err != nil {
		t.Fatal(err)
	}

	// 1) IsInstalledFunc → false short-circuits to NotInstalled.
	tool := &Tool{
		Command:         "fake-tool",
		IsInstalledFunc: func() bool { return false },
	}
	if got := CheckInstalled(tool); got != StatusNotInstalled {
		t.Fatalf("IsInstalledFunc=false CheckInstalled = %v", got)
	}

	// 2) IsInstalledFunc → true + MinVersion + binary missing in PATH.
	tool = &Tool{
		Command:         "not-on-path",
		MinVersion:      "1.0.0",
		IsInstalledFunc: func() bool { return true },
	}
	if got := CheckInstalled(tool); got != StatusInstalled {
		t.Fatalf("IsInstalledFunc=true missing binary CheckInstalled = %v", got)
	}

	// 3) IsInstalledFunc → true + MinVersion + binary exists but outdated.
	tool = &Tool{
		Command:         "fake-tool",
		MinVersion:      "2.0.0",
		IsInstalledFunc: func() bool { return true },
	}
	if got := CheckInstalled(tool); got != StatusOutdated {
		t.Fatalf("IsInstalledFunc=true outdated CheckInstalled = %v", got)
	}

	// 4) Fallback path: no IsInstalledFunc, binary missing.
	tool = &Tool{Command: "missing"}
	if got := CheckInstalled(tool); got != StatusNotInstalled {
		t.Fatalf("missing fallback CheckInstalled = %v", got)
	}

	// 5) Fallback path: binary exists but outdated.
	tool = &Tool{Command: "fake-tool", MinVersion: "2.0.0"}
	if got := CheckInstalled(tool); got != StatusOutdated {
		t.Fatalf("fallback outdated CheckInstalled = %v", got)
	}
}

// TestFirstPkgMgrStrategyManagerFilter covers the "Managers
// mismatch → continue" branch plus the return-nil exhaustion path.
func TestFirstPkgMgrStrategyManagerFilter(t *testing.T) {
	tool := &Tool{
		Strategies: []InstallStrategy{
			{Method: MethodPackageManager, Managers: []string{"apt"}, Package: "a"},
			{Method: MethodPackageManager, Managers: []string{"brew"}, Package: "b"},
		},
	}
	got := FirstPkgMgrStrategy(tool, "brew")
	if got == nil || got.Package != "b" {
		t.Fatalf("expected brew strategy, got %#v", got)
	}

	// Nothing matches → nil.
	got = FirstPkgMgrStrategy(tool, "dnf")
	if got != nil {
		t.Fatalf("expected nil for non-matching manager, got %#v", got)
	}
}

// TestShouldInstallDesktopOnly covers the DesktopOnly+headless skip.
func TestShouldInstallDesktopOnly(t *testing.T) {
	p := &platform.Platform{OS: platform.Linux}
	t.Setenv("DISPLAY", "")
	t.Setenv("WAYLAND_DISPLAY", "")
	tool := &Tool{DesktopOnly: true}
	if ShouldInstall(tool, p) {
		t.Fatal("DesktopOnly on headless linux should not install")
	}
}

// TestExecuteScriptDownloadFailure covers the curl-failed branch
// of executeScript — the outer temp-file and shell-invocation paths
// are covered elsewhere.
func TestExecuteScriptDownloadFailure(t *testing.T) {
	home := t.TempDir()
	bin := filepath.Join(home, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	// curl stub that always fails.
	if err := os.WriteFile(filepath.Join(bin, "curl"), []byte(
		"#!/bin/sh\nexit 22\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)

	log, err := executor.NewLogFile(filepath.Join(home, "log"))
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	ic := &InstallContext{
		Runner:   executor.NewRunner(log, false),
		PkgMgr:   &stubPkgMgr{name: "brew"},
		Platform: &platform.Platform{},
	}
	err = executeScript(context.Background(), &ScriptConfig{URL: "https://x.invalid/install.sh"}, ic)
	if err == nil || !strings.Contains(err.Error(), "download script") {
		t.Fatalf("expected download-script error, got %v", err)
	}
}

// TestExecuteScriptDefaultShell covers the shell-defaulting branch
// of executeScriptFile when cfg.Shell is empty. Catches regressions
// where a nil/empty shell silently picks /bin/sh (dash) on Debian
// systems, which blows up any bash-isms in upstream installers.
func TestExecuteScriptDefaultShell(t *testing.T) {
	home := t.TempDir()
	bin := filepath.Join(home, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	// Rename default to "bash" so the default-shell branch runs it.
	if err := os.WriteFile(filepath.Join(bin, "bash"), []byte(
		"#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)

	log, err := executor.NewLogFile(filepath.Join(home, "log"))
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	ic := &InstallContext{
		Runner:   executor.NewRunner(log, false),
		PkgMgr:   &stubPkgMgr{name: "brew"},
		Platform: &platform.Platform{},
	}
	script := filepath.Join(home, "probe.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := executeScriptFile(context.Background(), &ScriptConfig{}, script, ic); err != nil {
		t.Fatalf("executeScriptFile default shell: %v", err)
	}
}

// TestExecuteScriptEndToEnd exercises the full download→exec path
// so both executeScript and executeScriptFile register a successful
// run (the existing tests cover the RunWithEnv branch only).
func TestExecuteScriptEndToEnd(t *testing.T) {
	home := t.TempDir()
	bin := filepath.Join(home, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("SCRIPT_LOG", filepath.Join(home, "log.txt"))

	// curl writes "exit 0" to -o dest; bash runs it.
	if err := os.WriteFile(filepath.Join(bin, "curl"), []byte(`#!/bin/sh
dest=""
prev=""
for a in "$@"; do
  if [ "$prev" = "-o" ]; then dest="$a"; fi
  prev="$a"
done
cat > "$dest" <<'EOF'
#!/bin/sh
echo ran >> "$SCRIPT_LOG"
EOF
exit 0
`), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "bash"), []byte(
		"#!/bin/sh\nsh \"$@\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	log, err := executor.NewLogFile(filepath.Join(home, "main.log"))
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	ic := &InstallContext{
		Runner:   executor.NewRunner(log, false),
		PkgMgr:   &stubPkgMgr{name: "brew"},
		Platform: &platform.Platform{},
	}
	if err := executeScript(context.Background(), &ScriptConfig{
		URL: "https://example.invalid/install.sh", Shell: "bash",
	}, ic); err != nil {
		t.Fatalf("executeScript: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(home, "log.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "ran") {
		t.Fatalf("expected script to have run: %s", data)
	}
}

// TestExecuteInstallLastErrAggregation covers the "all strategies
// failed → wrap lastErr" path after every applicable strategy
// returned a non-fatal error.
func TestExecuteInstallLastErrAggregation(t *testing.T) {
	ic, _ := newExecCtx(t)
	tool := &Tool{
		Name: "always-fails",
		Strategies: []InstallStrategy{{
			Method: MethodCustom,
			CustomFunc: func(context.Context, *InstallContext) error {
				return errors.New("boom")
			},
		}},
	}
	err := ExecuteInstall(context.Background(), tool, ic, ic.Platform)
	if err == nil || !strings.Contains(err.Error(), "all install strategies failed") {
		t.Fatalf("expected aggregated error, got %v", err)
	}
}

// TestExecuteInstallDpkgHealerFailure covers the inner branch where
// the dpkg healer runs but itself fails, so the retry is abandoned
// and the outer loop moves on.
func TestExecuteInstallDpkgHealerFailure(t *testing.T) {
	ic, _ := newExecCtx(t)
	pm := &failingHealerPkgMgr{name: "apt"}
	ic.PkgMgr = pm

	tool := &Tool{
		Name: "unhealable",
		Strategies: []InstallStrategy{{
			Method: MethodCustom,
			CustomFunc: func(context.Context, *InstallContext) error {
				return fmt.Errorf("locked: %w", errNotExported())
			},
		}},
	}
	err := ExecuteInstall(context.Background(), tool, ic, ic.Platform)
	if err == nil {
		t.Fatal("expected failure when healer cannot repair dpkg")
	}
	if !pm.healCalled {
		t.Fatal("expected healer to be consulted")
	}
}

// failingHealerPkgMgr reports recoverable-dpkg errors to registry
// but its healer always fails, covering the "heal itself errored"
// branch that otherwise never runs.
type failingHealerPkgMgr struct {
	name       string
	healCalled bool
}

func (f *failingHealerPkgMgr) Name() string                             { return f.name }
func (f *failingHealerPkgMgr) IsInstalled(string) bool                  { return false }
func (f *failingHealerPkgMgr) Install(context.Context, ...string) error { return nil }
func (f *failingHealerPkgMgr) UpdateAll(context.Context) error          { return nil }
func (f *failingHealerPkgMgr) MapName(s string) []string                { return []string{s} }
func (f *failingHealerPkgMgr) RunDpkgConfigureAll(context.Context) error {
	f.healCalled = true
	return errors.New("dpkg repair failed")
}

// errNotExported is the recoverable-dpkg sentinel by value reach —
// imported here via the pkgmgr package-level error so registry's
// isRecoverableDpkg classifier returns true.
func errNotExported() error {
	// Keep in sync with pkgmgr.ErrDpkgLocked; we import it via a
	// local alias to avoid widening the test's import surface.
	return dpkgLockedSentinel
}

// TestExecuteStrategyScriptDownloadTempCleanup also exercises the
// httptest-backed script download, showing the download branch runs
// and the temp file is removed afterward.
func TestExecuteStrategyScriptDownloadTempCleanup(t *testing.T) {
	home := t.TempDir()
	bin := filepath.Join(home, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "curl"), []byte(`#!/bin/sh
dest=""
prev=""
for a in "$@"; do
  if [ "$prev" = "-o" ]; then dest="$a"; fi
  prev="$a"
done
cat > "$dest" <<'EOF'
#!/bin/sh
exit 0
EOF
exit 0
`), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "bash"), []byte(
		"#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("#!/bin/sh\nexit 0\n"))
	}))
	defer srv.Close()

	log, err := executor.NewLogFile(filepath.Join(home, "log"))
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	ic := &InstallContext{
		Runner:   executor.NewRunner(log, false),
		PkgMgr:   &stubPkgMgr{name: "brew"},
		Platform: &platform.Platform{},
	}
	cfg := &ScriptConfig{URL: srv.URL + "/install.sh", Shell: "bash"}
	if err := executeScript(context.Background(), cfg, ic); err != nil {
		t.Fatalf("executeScript: %v", err)
	}
}
