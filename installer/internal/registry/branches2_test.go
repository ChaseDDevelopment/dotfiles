package registry

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chaseddevelopment/dotfiles/installer/internal/executor"
	"github.com/chaseddevelopment/dotfiles/installer/internal/pkgmgr"
	"github.com/chaseddevelopment/dotfiles/installer/internal/platform"
)

// errPkgMgr is a minimal pkgmgr.PackageManager fake whose Install
// always returns a configurable error. Used to drive the dep-install
// failure branches of installYaziApt and installGhCLI without standing
// up the full setupClosureEnv stub PATH.
type errPkgMgr struct {
	name string
	err  error
}

var _ pkgmgr.PackageManager = (*errPkgMgr)(nil)

func (e *errPkgMgr) Name() string                                    { return e.name }
func (e *errPkgMgr) IsInstalled(_ string) bool                       { return false }
func (e *errPkgMgr) Install(_ context.Context, _ ...string) error   { return e.err }
func (e *errPkgMgr) UpdateAll(_ context.Context) error               { return nil }
func (e *errPkgMgr) MapName(g string) []string                      { return []string{g} }

// newErrCtx mirrors newTestCtx but wires an errPkgMgr so dep-install
// failure branches surface as the test wants. Returns the ic plus the
// underlying runner so callers can flip dry-run if needed.
func newErrCtx(t *testing.T, mgr *errPkgMgr) *InstallContext {
	t.Helper()
	dir := t.TempDir()
	lf, err := executor.NewLogFile(filepath.Join(dir, "test.log"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { lf.Close() })
	return &InstallContext{
		Runner:   executor.NewRunner(lf, true),
		PkgMgr:   mgr,
		Platform: &platform.Platform{OS: platform.Linux, Arch: platform.AMD64},
	}
}

// TestInstallYaziAptDepInstallFails covers dev_tools.go:294-296 — the
// branch where ic.PkgMgr.Install for the companion deps (ffmpeg,
// p7zip-full, jq, poppler-utils, imagemagick) fails. installYaziApt
// must wrap the error with "yazi companion deps:" and stop before
// touching the network for the .deb download.
//
// A regression that swallowed the error would let installYaziApt
// proceed to the curl + dpkg path under broken apt state, masking the
// classifier signal the orchestrator depends on.
func TestInstallYaziAptDepInstallFails(t *testing.T) {
	sentinel := errors.New("apt held back")
	ic := newErrCtx(t, &errPkgMgr{name: "apt", err: sentinel})

	err := installYaziApt(context.Background(), ic)
	if err == nil {
		t.Fatal("expected error when companion-dep install fails, got nil")
	}
	if !strings.Contains(err.Error(), "yazi companion deps") {
		t.Fatalf("err = %q, want 'yazi companion deps' wrap prefix", err)
	}
	if !errors.Is(err, sentinel) {
		t.Fatalf("err = %v, want wraps sentinel %v", err, sentinel)
	}
}

// TestInstallGhCLICurlMissingTriggersPkgMgrInstall covers
// cli_tools.go:361-364 — the branch entered when curl is missing from
// PATH and ic.PkgMgr.Install("curl") is invoked. We also assert the
// error wrap prefix when that install itself fails, so the test
// double-covers the rare-but-real "curl missing AND apt broken" path.
//
// Empty PATH guarantees exec.LookPath("curl") errors. The errPkgMgr
// stub then forces the inner Install to fail so we can assert the
// "gh: install curl:" wrap text.
func TestInstallGhCLICurlMissingTriggersPkgMgrInstall(t *testing.T) {
	t.Setenv("PATH", "")

	sentinel := errors.New("apt locked")
	ic := newErrCtx(t, &errPkgMgr{name: "apt", err: sentinel})

	err := installGhCLI(context.Background(), ic)
	if err == nil {
		t.Fatal("expected error when curl is missing AND " +
			"PkgMgr.Install fails, got nil")
	}
	if !strings.Contains(err.Error(), "gh: install curl") {
		t.Fatalf("err = %q, want 'gh: install curl' wrap prefix "+
			"from cli_tools.go:363", err)
	}
	if !errors.Is(err, sentinel) {
		t.Fatalf("err = %v, want wraps sentinel %v", err, sentinel)
	}
}

// TestInstallNerdFontLinuxLatestVersionFails covers cli_tools.go:461-
// 463 — the latestVersionFn error branch. Use the existing seam
// (latestVersionFn package-level var) and inject a returning error.
//
// Saves+restores the seam via defer so concurrent tests sharing the
// var aren't disturbed by the override.
func TestInstallNerdFontLinuxLatestVersionFails(t *testing.T) {
	orig := latestVersionFn
	defer func() { latestVersionFn = orig }()
	sentinel := errors.New("github API down")
	latestVersionFn = func(string, bool) (string, error) {
		return "", sentinel
	}

	dir := t.TempDir()
	lf, err := executor.NewLogFile(filepath.Join(dir, "test.log"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { lf.Close() })
	ic := &InstallContext{
		Runner: executor.NewRunner(lf, true),
		PkgMgr: &stubPkgMgr{name: "apt"},
		Platform: &platform.Platform{
			OS: platform.Linux, Arch: platform.AMD64,
		},
	}

	err = installNerdFontLinux(context.Background(), ic)
	if err == nil {
		t.Fatal("expected latestVersionFn error to surface, got nil")
	}
	if !strings.Contains(err.Error(), "resolve nerd-fonts latest version") {
		t.Fatalf("err = %q, want 'resolve nerd-fonts latest version' "+
			"wrap prefix", err)
	}
	if !errors.Is(err, sentinel) {
		t.Fatalf("err = %v, want wraps sentinel %v", err, sentinel)
	}
}

// TestTreeSitterLibIsInstalledFalse covers the
// "no pkg-config + no header anywhere" branch of the tree-sitter-lib
// IsInstalledFunc (dev_tools.go:59-74). With PATH cleared, pkg-config
// itself isn't on PATH so the exec.Command fails. The header probes
// hit absolute paths under /opt/homebrew, /usr/local, /usr — on a
// hermetic CI runner those should be absent, but the test guards
// against false-success by skipping if any header happens to exist on
// the host (a homebrew-equipped dev box).
func TestTreeSitterLibIsInstalledFalse(t *testing.T) {
	// Skip if any of the system paths actually has the header — we
	// can't stat-fake those without containerization. This keeps the
	// test honest about what it verifies on the test host.
	for _, p := range []string{
		"/opt/homebrew/include/tree_sitter/api.h",
		"/usr/local/include/tree_sitter/api.h",
		"/usr/include/tree_sitter/api.h",
	} {
		if _, err := os.Stat(p); err == nil {
			t.Skipf("tree_sitter header present at %s on host; "+
				"can't exercise the false branch", p)
		}
	}

	// Empty PATH guarantees exec.LookPath("pkg-config") fails inside
	// the IsInstalledFunc (and so does the exec.Command(...).Run()).
	t.Setenv("PATH", "")

	// Find the tree-sitter-lib tool by walking the catalog.
	var fn func() bool
	for _, tool := range AllTools() {
		if tool.Name == "tree-sitter-lib" && tool.IsInstalledFunc != nil {
			fn = tool.IsInstalledFunc
			break
		}
	}
	if fn == nil {
		t.Fatal("tree-sitter-lib missing IsInstalledFunc; catalog regressed")
	}
	if fn() {
		t.Fatal("IsInstalledFunc returned true with empty PATH and " +
			"no system headers — the false branch did not fire")
	}
}
