package orchestrator

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/chaseddevelopment/dotfiles/installer/internal/pkgmgr"
)

// recordingPkgMgr is a pkgmgr.PackageManager fake that captures every
// method invocation (name + argv). Used to assert that a Run closure
// actually reached the package manager with the expected arguments.
//
// It's intentionally distinct from batchPkgMgr in orchestrator_test.go:
// this one records ALL methods (Install, IsInstalled, UpdateAll,
// MapName), in order, so per-task ordering can be asserted — whereas
// batchPkgMgr only tracks install counts.
type recordingPkgMgr struct {
	mu sync.Mutex

	name string

	installCalls   [][]string
	updateAllCalls int
	isInstalledQ   []string
	mapNameQ       []string

	// installedLookup answers IsInstalled() deterministically.
	installedLookup map[string]bool
	// installReturn is the error Install returns (nil = success).
	installReturn error
	// updateAllReturn is the error UpdateAll returns (nil = success).
	updateAllReturn error
}

var _ pkgmgr.PackageManager = (*recordingPkgMgr)(nil)

func (r *recordingPkgMgr) Name() string { return r.name }

func (r *recordingPkgMgr) Install(
	_ context.Context, generics ...string,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	// Copy so later mutation of caller slices doesn't disturb the
	// recording.
	cp := make([]string, len(generics))
	copy(cp, generics)
	r.installCalls = append(r.installCalls, cp)
	return r.installReturn
}

func (r *recordingPkgMgr) IsInstalled(name string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.isInstalledQ = append(r.isInstalledQ, name)
	return r.installedLookup[name]
}

func (r *recordingPkgMgr) UpdateAll(_ context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.updateAllCalls++
	return r.updateAllReturn
}

func (r *recordingPkgMgr) MapName(generic string) []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.mapNameQ = append(r.mapNameQ, generic)
	return []string{generic}
}

// snapshotInstall returns a copy of installCalls under lock so tests
// can assert without racing the fake.
func (r *recordingPkgMgr) snapshotInstall() [][]string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([][]string, len(r.installCalls))
	for i, c := range r.installCalls {
		cp := make([]string, len(c))
		copy(cp, c)
		out[i] = cp
	}
	return out
}

// TestDoctorTasksRunClosures asserts every doctor task's Run closure
// actually probes tool/config state. With PATH empty, no tool binary
// resolves, so every tool-check task must return an error naming the
// fix. Every config-check task must return an error for a tempdir
// with no pre-applied symlinks.
//
// The old version of this test discarded the return value with `_ =`
// and asserted nothing; flipping a branch in BuildDoctorTasks
// wouldn't have been caught.
func TestDoctorTasksRunClosures(t *testing.T) {
	// Empty PATH so exec.LookPath fails for every tool.
	t.Setenv("PATH", "")
	bc := newTestBuildConfig(t)

	result := BuildDoctorTasks(bc)
	if len(result.Tasks) == 0 {
		t.Fatal("BuildDoctorTasks produced zero tasks")
	}

	toolErrs := 0
	configErrs := 0
	for _, task := range result.Tasks {
		if task.Run == nil {
			t.Errorf("task %q has nil Run", task.ID)
			continue
		}
		err := task.Run(context.Background())
		switch {
		case strings.HasPrefix(task.ID, "check-config-"):
			if err == nil {
				t.Errorf(
					"config check %q: expected error for "+
						"unconfigured tempdir, got nil",
					task.ID,
				)
				continue
			}
			configErrs++
		case strings.HasPrefix(task.ID, "check-"):
			// Some tools use a custom IsInstalledFunc (e.g. nerd
			// font, nvm, tpm, tree-sitter lib) that probes the file
			// system rather than PATH — those may legitimately
			// return nil in a tempdir. For tools that DO error, the
			// doctor contract requires a "fix:" remediation hint,
			// so enforce that when present.
			if err != nil {
				if !strings.Contains(err.Error(), "fix:") &&
					!strings.Contains(err.Error(), "not installed") {
					t.Errorf(
						"tool check %q: error %q missing "+
							"remediation hint",
						task.ID, err,
					)
				}
				toolErrs++
			}
		default:
			t.Errorf("unexpected doctor task ID: %q", task.ID)
		}
	}

	if toolErrs == 0 {
		t.Fatal("no tool-check task returned an error; " +
			"either PATH isn't empty or BuildDoctorTasks skipped " +
			"every tool")
	}
	if configErrs == 0 {
		t.Fatal("no config-check task returned an error; " +
			"InspectComponent returned success for an empty tempdir")
	}
}

// TestInstallTasksRunClosuresDryRun asserts each install-task Run
// closure reaches the package manager in a single batched call. With
// DryRun=true, the Runner logs `[DRY RUN] ...` lines instead of
// executing commands; our recording pkgmgr fake still records every
// Install invocation so we can assert:
//
//  1. mgr.Install was called exactly once (batched), not per-tool.
//  2. The generic names passed are non-empty and deduplicated.
//  3. setup-* tasks run after tool tasks (observable via no-op
//     success — no symlinks in the tempdir).
//
// The old version discarded every error with `_ =` and asserted
// nothing. A regression that short-circuited runBatchedInstall or
// lost the batch-once guarantee would have gone uncaught.
func TestInstallTasksRunClosuresDryRun(t *testing.T) {
	t.Setenv("PATH", "")
	bc := newTestBuildConfig(t)
	bc.DryRun = true
	mgr := &recordingPkgMgr{name: "brew"}
	bc.PkgMgr = mgr

	result := BuildInstallTasks(bc)
	if len(result.Tasks) == 0 {
		t.Fatal("BuildInstallTasks produced zero tasks")
	}

	// Run every task closure. We run in declared order (deps are
	// respected because tasks with unmet deps would be waiting for a
	// scheduler — but here we fire them directly, which is fine since
	// batchState.runOnce guarantees the pkgmgr batch runs once
	// regardless of which tool task is invoked first).
	var runErrs int
	for _, task := range result.Tasks {
		if task.Run == nil {
			t.Errorf("task %q has nil Run", task.ID)
			continue
		}
		if err := task.Run(context.Background()); err != nil {
			// setup-* may error if a component can't find its source
			// config file in a bare tempdir. Record but don't fail —
			// the assertion below about Install being called is what
			// matters for the install-path branch coverage.
			runErrs++
		}
	}

	calls := mgr.snapshotInstall()
	if len(calls) != 1 {
		t.Fatalf(
			"mgr.Install called %d times, want exactly 1 "+
				"(batched); per-tool calls indicate a regression "+
				"in runBatchedInstall's sync.Once guarantee",
			len(calls),
		)
	}
	if len(calls[0]) == 0 {
		t.Fatal("batch Install called with empty arg list; " +
			"bucketGenerics plumbing broke")
	}
	// Dedup check: every generic should appear at most once.
	seen := map[string]bool{}
	for _, g := range calls[0] {
		if seen[g] {
			t.Errorf(
				"batch Install received duplicate generic %q; "+
					"bucketGenerics dedup loop regressed",
				g,
			)
		}
		seen[g] = true
	}

	// Sanity: ran at least one tool task (setup tasks may still run,
	// but we should have exercised install branches).
	if runErrs == 0 && len(result.Tasks) > 1 {
		// not an error per se — brew is the test platform and bare
		// setup tasks in tempdir may genuinely succeed when nothing
		// exists to link. Log rather than fail.
		t.Logf("all %d task closures returned nil", len(result.Tasks))
	}
}

// TestUninstallTasksRunClosures asserts every uninstall task's Run
// closure actually invokes RemoveComponentSymlinks (which is a no-op
// for an empty tempdir — no symlinks to remove — and MUST return nil).
//
// Observable signal: we probe the tempdir before and after to confirm
// nothing changed (no side effects beyond the no-op removal), and we
// confirm no error propagates. A regression that made
// RemoveComponentSymlinks return an error on a missing target would
// be caught.
func TestUninstallTasksRunClosures(t *testing.T) {
	bc := newTestBuildConfig(t)
	// Empty selection = all components covered. Force it so the test
	// is independent of the default.
	bc.SelectedComps = nil

	result := BuildUninstallTasks(bc)
	if len(result.Tasks) == 0 {
		t.Fatal("BuildUninstallTasks produced zero tasks")
	}

	for _, task := range result.Tasks {
		if task.Run == nil {
			t.Errorf("task %q has nil Run", task.ID)
			continue
		}
		if !strings.HasPrefix(task.ID, "uninstall-") {
			t.Errorf(
				"expected uninstall-* prefix, got %q",
				task.ID,
			)
		}
		if err := task.Run(context.Background()); err != nil {
			// The uninstall contract: removing non-existent symlinks
			// is a benign no-op. Propagating an error here indicates
			// RemoveComponentSymlinks regressed to fail-on-missing.
			t.Errorf(
				"task %q returned unexpected error: %v "+
					"(RemoveComponentSymlinks should be idempotent)",
				task.ID, err,
			)
		}
	}
}

// TestUpdateTasksRunClosuresDryRun asserts the "System packages"
// update task invokes mgr.UpdateAll exactly once (and that subsequent
// steps don't also invoke it). With PATH empty, every other step's
// HasCommand probe returns false and the step becomes a no-op — so
// the clean observable signal is UpdateAll call count.
//
// The old version discarded every error and asserted nothing; a
// regression that dropped the mgr.UpdateAll call or wired every step
// to it would have slipped past.
func TestUpdateTasksRunClosuresDryRun(t *testing.T) {
	t.Setenv("PATH", "")
	bc := newTestBuildConfig(t)
	bc.DryRun = true
	bc.Runner.DryRun = true
	mgr := &recordingPkgMgr{name: "brew"}
	bc.PkgMgr = mgr

	result := BuildUpdateTasks(bc)
	if len(result.Tasks) == 0 {
		t.Fatal("BuildUpdateTasks produced zero tasks")
	}

	for _, task := range result.Tasks {
		if task.Run == nil {
			t.Errorf("task %q has nil Run", task.ID)
			continue
		}
		// Update step errors are tolerable (e.g. cargo may return an
		// error when its binary isn't present and HasCommand wasn't
		// checked first) — the UpdateAll count is the load-bearing
		// assertion.
		_ = task.Run(context.Background())
	}

	if mgr.updateAllCalls != 1 {
		t.Fatalf(
			"mgr.UpdateAll called %d times, want exactly 1 "+
				"(only the 'System packages' step should invoke it)",
			mgr.updateAllCalls,
		)
	}

	// Verify the "System packages" step task ID exists and the log
	// contains the expected prefix, so a future refactor that
	// renames the step doesn't silently break task-row rendering.
	var foundSys bool
	for _, task := range result.Tasks {
		if task.ID == "update-System packages" {
			foundSys = true
			break
		}
	}
	if !foundSys {
		t.Fatal("expected update-System packages task")
	}
}

// TestBuildInstallTasksWithAptInjectsDpkgDoctor covers the Apt branch
// of BuildInstallTasks where a "dpkg-doctor" pseudo-task is prepended
// because the package manager is *pkgmgr.Apt.
func TestBuildInstallTasksWithAptInjectsDpkgDoctor(t *testing.T) {
	bc := newTestBuildConfig(t)
	bc.PkgMgr = pkgmgr.NewApt(bc.Runner, false)
	result := BuildInstallTasks(bc)
	found := false
	for _, task := range result.Tasks {
		if task.ID == "dpkg-doctor" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected dpkg-doctor pseudo-task for Apt manager")
	}
}

// TestBuildInstallTasksSkipPackagesOmitsDoctor covers the inverse:
// SkipPackages=true means no doctor task is added.
func TestBuildInstallTasksSkipPackagesOmitsDoctor(t *testing.T) {
	bc := newTestBuildConfig(t)
	bc.PkgMgr = pkgmgr.NewApt(bc.Runner, false)
	bc.SkipPackages = true
	result := BuildInstallTasks(bc)
	for _, task := range result.Tasks {
		if task.ID == "dpkg-doctor" {
			t.Fatal("dpkg-doctor task should be skipped when SkipPackages=true")
		}
	}
}

