package pkgmgr

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chaseddevelopment/dotfiles/installer/internal/executor"
	"github.com/chaseddevelopment/dotfiles/installer/internal/platform"
)

// TestManagerNamesMatchConsumerSwitches pins the exact string each
// manager returns from Name(). These strings are the case-keys in
// update/updater.go and orchestrator/orchestrator.go switches — a
// typo here would silently route past every case and skip work.
func TestManagerNamesMatchConsumerSwitches(t *testing.T) {
	cases := []struct {
		want string
		mgr  PackageManager
	}{
		{"apt", &Apt{}},
		{"brew", &Brew{}},
		{"pacman", newPacman(nil)},
		{"dnf", newDnf(nil)},
		{"yum", newYum(nil)},
		{"zypper", newZypper(nil)},
	}
	for _, tc := range cases {
		if got := tc.mgr.Name(); got != tc.want {
			t.Errorf("Name() = %q, want %q (case-key in updater/orchestrator switches)", got, tc.want)
		}
	}
}

// TestBatchFailureError formats the BatchFailure message so a
// regression in the error text (e.g. dropping FailedNames) is caught.
func TestBatchFailureError(t *testing.T) {
	bf := &BatchFailure{
		FailedNames: []string{"a", "b"},
		Wrapped:     errors.New("underlying"),
	}
	msg := bf.Error()
	if !strings.Contains(msg, "a, b") || !strings.Contains(msg, "underlying") {
		t.Fatalf("unexpected BatchFailure.Error: %q", msg)
	}
}

// TestBrewUpdateAll covers the brew update && brew upgrade shell.
func TestBrewUpdateAll(t *testing.T) {
	runner, dir := newPkgRunner(t)
	bin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	if err := os.WriteFile(filepath.Join(bin, "brew"),
		[]byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	b := &Brew{runner: runner}
	if err := b.UpdateAll(context.Background()); err != nil {
		t.Fatalf("brew UpdateAll: %v", err)
	}
}

// TestBrewInstallNilBecauseEveryNameMapsEmpty covers the "every
// generic mapped to empty slice" early return.
func TestBrewInstallNilBecauseEveryNameMapsEmpty(t *testing.T) {
	runner, _ := newPkgRunner(t)
	b := &Brew{runner: runner}
	if err := b.Install(context.Background(), "build-essential"); err != nil {
		t.Fatalf("Install build-essential on brew should no-op: %v", err)
	}
	if err := b.Install(context.Background()); err != nil {
		t.Fatalf("Install with no args should no-op: %v", err)
	}
	if !b.IsInstalled("build-essential") {
		t.Fatal("build-essential should report installed on brew (non-applicable)")
	}
}

// TestGenericInstallEarlyReturns covers the two early-return branches
// in genericMgr.Install (no names, all-mapped-empty).
func TestGenericInstallEarlyReturns(t *testing.T) {
	runner, _ := newPkgRunner(t)
	pm := newPacman(runner)
	if err := pm.Install(context.Background()); err != nil {
		t.Fatalf("empty install should no-op: %v", err)
	}
	// Pacman nameMap doesn't define any "maps to empty" entry, so use
	// a crafted generic mgr to cover that branch too.
	empty := &genericMgr{
		runner: runner,
		name:   "empty",
		installFn: func(context.Context, *executor.Runner, []string) error {
			t.Fatal("installFn should not be called when every name maps empty")
			return nil
		},
		checkFn:  func(*executor.Runner, string) bool { return true },
		updateFn: func(context.Context, *executor.Runner) error { return nil },
		nameMap:  map[string][]string{"skip": {}},
	}
	if err := empty.Install(context.Background(), "skip"); err != nil {
		t.Fatalf("all-empty install: %v", err)
	}
}

// TestGenericIsInstalledEmpty covers the no-mapped-names branch.
func TestGenericIsInstalledEmpty(t *testing.T) {
	runner, _ := newPkgRunner(t)
	empty := &genericMgr{
		runner:  runner,
		name:    "empty",
		checkFn: func(*executor.Runner, string) bool { return true },
		nameMap: map[string][]string{"skip": {}},
	}
	if empty.IsInstalled("skip") {
		t.Fatal("empty-mapping IsInstalled must return false")
	}
}

// TestGenericInstallPropagatesError covers the installFn-fails branch
// plus the attribute wrap — checkFn reports failures so the classified
// error is returned either wrapped or bare.
func TestGenericInstallPropagatesError(t *testing.T) {
	runner, _ := newPkgRunner(t)
	shellErr := errors.New("boom")
	pm := &genericMgr{
		runner: runner,
		name:   "broken",
		installFn: func(context.Context, *executor.Runner, []string) error {
			return shellErr
		},
		checkFn:  func(*executor.Runner, string) bool { return false },
		updateFn: func(context.Context, *executor.Runner) error { return nil },
		nameMap:  map[string][]string{},
	}
	err := pm.Install(context.Background(), "pkg")
	if err == nil || !strings.Contains(err.Error(), "broken install") {
		t.Fatalf("expected wrapped install error, got %v", err)
	}
	if !errors.Is(err, shellErr) {
		t.Fatalf("error should unwrap to inner shell err, got %v", err)
	}
}

// TestNewUnsupported covers the "no supported manager" branch.
func TestNewUnsupported(t *testing.T) {
	runner, _ := newPkgRunner(t)
	_, err := New(&platform.Platform{PackageManager: platform.PkgNone}, runner)
	if err == nil || !strings.Contains(err.Error(), "no supported package manager") {
		t.Fatalf("expected no-pm error, got %v", err)
	}
}

// TestAptClassifyAptErr covers each classification branch of
// classifyAptErr — the function is internal but critical for the
// orchestrator's recoverable-vs-fatal distinction.
func TestAptClassifyAptErr(t *testing.T) {
	base := errors.New("exit 100")
	cases := []struct {
		name   string
		output string
		want   error
	}{
		{"interrupted", "dpkg was interrupted, run --configure", ErrDpkgInterrupted},
		{"locked", "Could not get lock /var/lib/dpkg/lock-frontend", ErrDpkgLocked},
		{"locked-prefix-only", "/var/lib/dpkg/lock-frontend is busy", ErrDpkgLocked},
		{"unmet deps", "The following packages have unmet dependencies:", ErrAptFatal},
		{"held", "held packages prevent upgrade", ErrAptFatal},
		{"hash mismatch", "Hash Sum mismatch on http://...", ErrAptFatal},
		{"release file", "Release file for ... signed", ErrAptFatal},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := classifyAptErr(base, tc.output)
			if !errors.Is(got, tc.want) {
				t.Fatalf("classifyAptErr(%q) = %v, want Is=%v", tc.output, got, tc.want)
			}
		})
	}

	// Unclassified passes through unchanged.
	if got := classifyAptErr(base, "some other output"); !errors.Is(got, base) {
		t.Fatalf("expected base error passthrough, got %v", got)
	}
	// Nil in → nil out.
	if got := classifyAptErr(nil, ""); got != nil {
		t.Fatalf("expected nil passthrough, got %v", got)
	}
}

// TestAptContainsInstalled covers dpkg-query status parsing.
func TestAptContainsInstalled(t *testing.T) {
	if !containsInstalled("install ok installed") {
		t.Fatal("expected true for installed status")
	}
	if containsInstalled("deinstall ok config-files") {
		t.Fatal("expected false for rc state")
	}
	if containsInstalled("install") {
		t.Fatal("expected false for truncated status")
	}
}

// TestAptMapNameOverrides covers the mapping entries.
func TestAptMapNameOverrides(t *testing.T) {
	a := NewApt(nil, false)
	for generic, want := range map[string][]string{
		"nodejs":          {"nodejs", "npm"},
		"build-essential": {"build-essential"},
		"fd":              {"fd-find"},
		"bat":             {"bat"},
		"unknown":         {"unknown"},
	} {
		got := a.MapName(generic)
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Fatalf("MapName(%q) = %v, want %v", generic, got, want)
		}
	}
}

// TestAptProbeDpkgAuditFails covers the non-zero audit-exit branch.
func TestAptProbeDpkgAuditFails(t *testing.T) {
	runner, dir := newPkgRunner(t)
	bin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	if err := os.WriteFile(filepath.Join(bin, "dpkg"),
		[]byte("#!/bin/sh\necho 'dpkg audit failure' ; exit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	a := NewApt(runner, false)
	state, err := a.probeDpkg(context.Background())
	if err != nil {
		t.Fatalf("probeDpkg with audit failure should return state, got %v", err)
	}
	if state.Healthy {
		t.Fatal("expected unhealthy state when audit fails")
	}
	if !strings.Contains(state.Reason, "reported errors") {
		t.Fatalf("unexpected reason: %q", state.Reason)
	}
}

// TestAptProbeDpkgAuditDirty covers the "audit exits 0 but output
// non-empty" branch — some dpkg versions report inconsistencies that
// way.
func TestAptProbeDpkgAuditDirty(t *testing.T) {
	runner, dir := newPkgRunner(t)
	bin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	if err := os.WriteFile(filepath.Join(bin, "dpkg"),
		[]byte("#!/bin/sh\necho 'package X is broken' ; exit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	a := NewApt(runner, false)
	state, err := a.probeDpkg(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if state.Healthy || !strings.Contains(state.Reason, "inconsistencies") {
		t.Fatalf("unexpected state: %#v", state)
	}
}

// TestAptEnsureHealthyPaths drives every return path of EnsureHealthy.
func TestAptEnsureHealthyPaths(t *testing.T) {
	runner, dir := newPkgRunner(t)
	bin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	// Healthy: dpkg audit exits 0 silently.
	if err := os.WriteFile(filepath.Join(bin, "dpkg"),
		[]byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	a := NewApt(runner, false)
	if err := a.EnsureHealthy(context.Background()); err != nil {
		t.Fatalf("EnsureHealthy healthy: %v", err)
	}

	// Unhealthy + not approved.
	if err := os.WriteFile(filepath.Join(bin, "dpkg"),
		[]byte("#!/bin/sh\necho broken ; exit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	a2 := NewApt(runner, false)
	a2.UserApprovedRepair = false
	err := a2.EnsureHealthy(context.Background())
	if err == nil || !strings.Contains(err.Error(), "repair was not approved") {
		t.Fatalf("expected repair-not-approved error, got %v", err)
	}
}

// TestAptRunDpkgConfigureAllFailure covers the repair-fails branch.
func TestAptRunDpkgConfigureAllFailure(t *testing.T) {
	runner, dir := newPkgRunner(t)
	bin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	// sudo exec's its argv so `sudo dpkg ...` resolves to our stub.
	if err := os.WriteFile(filepath.Join(bin, "sudo"),
		[]byte("#!/bin/sh\nexec \"$@\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "dpkg"),
		[]byte("#!/bin/sh\nexit 2\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	a := NewApt(runner, false)
	// Opt in to repair so we exercise the RunDpkgConfigureAll body
	// rather than the new not-approved short-circuit.
	a.UserApprovedRepair = true
	if err := a.RunDpkgConfigureAll(context.Background()); err == nil ||
		!strings.Contains(err.Error(), "dpkg --configure") {
		t.Fatalf("expected repair failure, got %v", err)
	}

	// sync.Once caches the error → subsequent call returns same err.
	if err := a.RunDpkgConfigureAll(context.Background()); err == nil {
		t.Fatal("expected cached repair error on second call")
	}
}

// TestAptInstallPartialFailureProducesBatchFailure drives the
// `attribute` fan-out: the apt stub exits non-zero with output that
// classifies as ErrAptFatal, but dpkg-query reports that one of the
// two packages IS installed. The returned error must be a
// *BatchFailure whose FailedNames lists only the genuinely-failed
// generic name (not both) and whose Wrapped error still errors.Is
// ErrAptFatal so the orchestrator's recoverable-vs-fatal branch
// still fires correctly.
func TestAptInstallPartialFailureProducesBatchFailure(t *testing.T) {
	runner, dir := newPkgRunner(t)
	bin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv(
		"PATH",
		bin+string(os.PathListSeparator)+os.Getenv("PATH"),
	)
	// apt-get install: emit a "held packages" line (classifies as
	// ErrAptFatal) then exit non-zero. The update subcommand must
	// still succeed so ensureUpdated doesn't short-circuit.
	writeExec(t, bin, "apt-get", `#!/bin/sh
if [ "$1" = "update" ]; then
  exit 0
fi
echo 'The following held packages have been kept back:'
exit 100
`)
	// sudo forwards to argv so apt-get stub above runs.
	writeExec(t, bin, "sudo", `#!/bin/sh
exec "$@"
`)
	// dpkg-query: report "curl" installed, "htop" not. The pkg
	// name is the final positional arg after `-W -f=${Status}`.
	writeExec(t, bin, "dpkg-query", `#!/bin/sh
eval pkg=\$$#
case "$pkg" in
  curl) printf 'install ok installed' ;;
  *) exit 1 ;;
esac
`)

	a := NewApt(runner, false)
	err := a.Install(context.Background(), "curl", "htop")
	if err == nil {
		t.Fatal("expected error from Install")
	}
	var bf *BatchFailure
	if !errors.As(err, &bf) {
		t.Fatalf(
			"expected *BatchFailure, got %T: %v", err, err,
		)
	}
	if len(bf.FailedNames) != 1 || bf.FailedNames[0] != "htop" {
		t.Errorf(
			"FailedNames = %v, want [htop]", bf.FailedNames,
		)
	}
	if !errors.Is(err, ErrAptFatal) {
		t.Errorf(
			"BatchFailure wrap should classify as ErrAptFatal, got %v",
			err,
		)
	}
}

// TestAptInstallTotalFailureReturnsClassifiedNotBatch asserts the
// "every generic failed" path returns the classified error directly
// (not wrapped in BatchFailure). This gates the orchestrator's
// recoverable-vs-fatal short-circuit: wrapping a single-tool total
// failure in BatchFailure would force the fan-out fallback to
// redo work pointlessly.
func TestAptInstallTotalFailureReturnsClassifiedNotBatch(t *testing.T) {
	runner, dir := newPkgRunner(t)
	bin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv(
		"PATH",
		bin+string(os.PathListSeparator)+os.Getenv("PATH"),
	)
	// 404 classifies as ErrAptFatal via the "hash sum mismatch"
	// neighborhood of patterns — use Release file which is in the
	// classifier.
	writeExec(t, bin, "apt-get", `#!/bin/sh
if [ "$1" = "update" ]; then
  exit 0
fi
echo 'E: Release file for http://archive/... is not valid yet'
exit 100
`)
	writeExec(t, bin, "sudo", `#!/bin/sh
exec "$@"
`)
	// Both packages absent.
	writeExec(t, bin, "dpkg-query", `#!/bin/sh
exit 1
`)

	a := NewApt(runner, false)
	err := a.Install(context.Background(), "curl", "htop")
	if err == nil {
		t.Fatal("expected error")
	}
	var bf *BatchFailure
	if errors.As(err, &bf) {
		t.Fatalf(
			"total failure should NOT wrap in BatchFailure; got %v",
			err,
		)
	}
	if !errors.Is(err, ErrAptFatal) {
		t.Errorf("expected ErrAptFatal, got %v", err)
	}
}

// TestAptInstallRecoverableDpkgLock replays the "dpkg lock held"
// output, which must classify as ErrDpkgLocked (recoverable) — NOT
// ErrAptFatal. A regression that lumps dpkg-lock into the fatal
// bucket would break the orchestrator's retry semantics.
func TestAptInstallRecoverableDpkgLock(t *testing.T) {
	runner, dir := newPkgRunner(t)
	bin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv(
		"PATH",
		bin+string(os.PathListSeparator)+os.Getenv("PATH"),
	)
	writeExec(t, bin, "apt-get", `#!/bin/sh
if [ "$1" = "update" ]; then exit 0; fi
echo 'E: Could not get lock /var/lib/dpkg/lock-frontend'
exit 100
`)
	writeExec(t, bin, "sudo", `#!/bin/sh
exec "$@"
`)
	writeExec(t, bin, "dpkg-query", `#!/bin/sh
exit 1
`)

	a := NewApt(runner, false)
	err := a.Install(context.Background(), "curl")
	if err == nil {
		t.Fatal("expected lock error")
	}
	if !errors.Is(err, ErrDpkgLocked) {
		t.Errorf("expected ErrDpkgLocked, got %v", err)
	}
	if errors.Is(err, ErrAptFatal) {
		t.Errorf(
			"lock must NOT classify as fatal — would break retry "+
				"logic; got %v", err,
		)
	}
}

// TestAptEnsureHealthyRepairRunsOnce drives the dirty → repair →
// clean sequence end-to-end: the first `dpkg --audit` exits non-zero
// (dirty), `sudo dpkg --configure -a` runs exactly once, and the
// post-repair `dpkg --audit` exits clean so EnsureHealthy returns
// nil. Calling EnsureHealthy a second time on the now-clean system
// must not re-run the repair — the sync.Once invariant.
func TestAptEnsureHealthyRepairRunsOnce(t *testing.T) {
	runner, dir := newPkgRunner(t)
	bin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv(
		"PATH",
		bin+string(os.PathListSeparator)+os.Getenv("PATH"),
	)
	stateFile := filepath.Join(dir, "healthy")
	repairCountFile := filepath.Join(dir, "repair-count")
	writeExec(t, bin, "dpkg", `#!/bin/sh
if [ "$1" = "--audit" ]; then
  if [ -f "`+stateFile+`" ]; then
    exit 0
  fi
  echo 'package is broken'
  exit 1
fi
exit 0
`)
	writeExec(t, bin, "sudo", `#!/bin/sh
if [ "$1" = "dpkg" ] && [ "$2" = "--configure" ] && [ "$3" = "-a" ]; then
  echo 1 >> `+repairCountFile+`
  : > "`+stateFile+`"
  exit 0
fi
exec "$@"
`)

	a := NewApt(runner, false)
	// Opt in so EnsureHealthy exercises the repair path.
	a.UserApprovedRepair = true
	if err := a.EnsureHealthy(context.Background()); err != nil {
		t.Fatalf("first EnsureHealthy: %v", err)
	}
	if err := a.EnsureHealthy(context.Background()); err != nil {
		t.Fatalf("second EnsureHealthy: %v", err)
	}
	data, err := os.ReadFile(repairCountFile)
	if err != nil {
		t.Fatal(err)
	}
	if count := strings.Count(string(data), "1"); count != 1 {
		t.Errorf(
			"expected exactly 1 repair invocation, got %d",
			count,
		)
	}
}

// TestAptEnsureHealthyStillDirtyAfterRepair: if repair succeeds but
// the re-probe still reports dirty, EnsureHealthy must return an
// error (never silent-heal) rather than claim success.
func TestAptEnsureHealthyStillDirtyAfterRepair(t *testing.T) {
	runner, dir := newPkgRunner(t)
	bin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv(
		"PATH",
		bin+string(os.PathListSeparator)+os.Getenv("PATH"),
	)
	// dpkg --audit ALWAYS reports dirty; configure -a "succeeds".
	writeExec(t, bin, "dpkg", `#!/bin/sh
if [ "$1" = "--audit" ]; then
  echo 'still broken'
  exit 1
fi
exit 0
`)
	writeExec(t, bin, "sudo", `#!/bin/sh
exec "$@"
`)

	a := NewApt(runner, false)
	// Opt in to force EnsureHealthy into the repair path so the
	// "still dirty after repair" branch actually runs.
	a.UserApprovedRepair = true
	err := a.EnsureHealthy(context.Background())
	if err == nil {
		t.Fatal(
			"expected EnsureHealthy to return error when dpkg " +
				"stays dirty after repair — never silent-heal",
		)
	}
	if !strings.Contains(err.Error(), "still inconsistent") {
		t.Errorf(
			"error should describe remaining inconsistency, got %v",
			err,
		)
	}
}

// TestAptIsInstalledRejectsRcState covers the empty-names and
// partial-installed branches of IsInstalled. A package that maps to
// multiple real names (e.g. nodejs -> nodejs,npm) must report false
// if ANY of the underlying names is absent — otherwise a half-
// installed tool re-invokes its install path and compounds breakage.
func TestAptIsInstalledRejectsRcState(t *testing.T) {
	runner, dir := newPkgRunner(t)
	bin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv(
		"PATH",
		bin+string(os.PathListSeparator)+os.Getenv("PATH"),
	)
	// dpkg-query: "nodejs" installed, "npm" returns rc-style status.
	writeExec(t, bin, "dpkg-query", `#!/bin/sh
eval pkg=\$$#
case "$pkg" in
  nodejs) printf 'install ok installed' ;;
  npm) printf 'deinstall ok config-files' ;;
  *) exit 1 ;;
esac
`)
	a := NewApt(runner, false)
	if a.IsInstalled("nodejs") {
		t.Error(
			"nodejs should report NOT installed when npm is in rc state",
		)
	}
	// Sanity: something the mapper returns an empty slice for isn't
	// in the default map, so MapName returns [name]. Use a name not
	// in the override map to prove the default path.
	if a.IsInstalled("this-is-not-installed") {
		t.Error(
			"unknown package reported installed",
		)
	}
}

// TestAptDetectDpkgHealthCaches confirms the sync.Once caching.
func TestAptDetectDpkgHealthCaches(t *testing.T) {
	runner, dir := newPkgRunner(t)
	bin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	callCountFile := filepath.Join(dir, "count")
	if err := os.WriteFile(filepath.Join(bin, "dpkg"), []byte(fmt.Sprintf(`#!/bin/sh
echo count >> %q
exit 0
`, callCountFile)), 0o755); err != nil {
		t.Fatal(err)
	}
	a := NewApt(runner, false)
	if _, err := a.DetectDpkgHealth(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := a.DetectDpkgHealth(context.Background()); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(callCountFile)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(data), "count") != 1 {
		t.Fatalf("expected one dpkg invocation, got %d:\n%s",
			strings.Count(string(data), "count"), data)
	}
}
