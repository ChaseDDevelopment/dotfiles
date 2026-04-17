package tui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/chaseddevelopment/dotfiles/installer/internal/executor"
	"github.com/chaseddevelopment/dotfiles/installer/internal/pkgmgr"
)

// writeDpkgShim plants a PATH-shim `dpkg` that reports healthy iff
// the marker file exists, and a `sudo` that "repairs" by touching
// the marker. Returns the bin directory.
//
// The state-file trick lets a single test drive dpkg from dirty →
// clean over multiple calls (first DetectDpkgHealth flags broken,
// then after sudo dpkg --configure -a the second DetectDpkgHealth
// returns healthy).
func writeDpkgShim(t *testing.T, markerClean string) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	dpkgScript := "#!/bin/sh\n" +
		"if [ \"$1\" = \"--audit\" ]; then\n" +
		"  if [ -f \"" + markerClean + "\" ]; then\n" +
		"    exit 0\n" +
		"  fi\n" +
		"  printf 'dpkg: broken packages'\n" +
		"  exit 1\n" +
		"fi\n" +
		"exit 0\n"
	if err := os.WriteFile(
		filepath.Join(bin, "dpkg"), []byte(dpkgScript), 0o755,
	); err != nil {
		t.Fatal(err)
	}
	sudoScript := "#!/bin/sh\n" +
		"if [ \"$1\" = \"dpkg\" ] && [ \"$2\" = \"--configure\" ] && [ \"$3\" = \"-a\" ]; then\n" +
		"  : > \"" + markerClean + "\"\n" +
		"fi\n" +
		"exit 0\n"
	if err := os.WriteFile(
		filepath.Join(bin, "sudo"), []byte(sudoScript), 0o755,
	); err != nil {
		t.Fatal(err)
	}
	return bin
}

// TestPreflightDpkgHealthNonApt exercises the fast path: when the
// configured package manager is not Apt, preflight must return
// (nil, false) without touching any shell commands, and must leave
// the phase unchanged.
func TestPreflightDpkgHealthNonApt(t *testing.T) {
	app := NewApp(newTestConfig())
	app.phase = PhaseInstalling
	// PkgMgr left as nil → type assertion fails → early return.
	cmd, blocked := app.preflightDpkgHealth()
	if cmd != nil || blocked {
		t.Fatalf(
			"non-apt preflight should return (nil, false), got (%v, %v)",
			cmd, blocked,
		)
	}
	if app.phase != PhaseInstalling {
		t.Fatalf("phase must not change for non-apt, got %v", app.phase)
	}
}

// TestPreflightDpkgHealthHealthy drives preflight with a shim dpkg
// that reports clean. Expected: (nil, false) and phase untouched.
// This also exercises the UserApprovedRepair=false reset line,
// which guards against a stale prior approval silently granting
// repair on a fresh session.
func TestPreflightDpkgHealthHealthy(t *testing.T) {
	dir := t.TempDir()
	log, err := executor.NewLogFile(filepath.Join(dir, "t.log"))
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	runner := executor.NewRunner(log, false)

	// Pre-touch the marker so dpkg --audit reports clean.
	marker := filepath.Join(dir, "healthy")
	if err := os.WriteFile(marker, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	bin := writeDpkgShim(t, marker)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	app := NewApp(newTestConfig())
	app.config.Runner = runner
	apt := pkgmgr.NewApt(runner, false)
	apt.UserApprovedRepair = true // pre-set stale approval
	app.config.PkgMgr = apt
	app.phase = PhaseInstalling

	cmd, blocked := app.preflightDpkgHealth()
	if cmd != nil || blocked {
		t.Fatalf(
			"healthy preflight should return (nil, false), got (%v, %v)",
			cmd, blocked,
		)
	}
	if apt.UserApprovedRepair {
		t.Fatal("stale UserApprovedRepair must be reset on preflight")
	}
	if app.phase != PhaseInstalling {
		t.Fatalf(
			"phase must stay on installing when dpkg clean, got %v",
			app.phase,
		)
	}
}

// TestUpdateDpkgRepairNonKey covers the non-keypress no-op branch.
func TestUpdateDpkgRepairNonKey(t *testing.T) {
	app := NewApp(newTestConfig())
	app.phase = PhaseDpkgRepair
	model, cmd := app.updateDpkgRepair(
		tea.WindowSizeMsg{Width: 80, Height: 40},
	)
	// Accept either pointer or value return — see more_test.go.
	var updated AppModel
	switch mm := model.(type) {
	case *AppModel:
		updated = *mm
	case AppModel:
		updated = mm
	default:
		t.Fatalf("unexpected model type %T", model)
	}
	if updated.phase != PhaseDpkgRepair {
		t.Fatalf("non-key must leave phase on repair, got %v", updated.phase)
	}
	if cmd != nil {
		t.Fatal("non-key must not return a command")
	}
}

// TestUpdateDpkgRepairAuthorize covers the "r" keypress branch:
// UserApprovedRepair flips to true, log captures the consent line,
// and runInstallTasks is invoked (returning a Cmd path since the
// post-repair dpkg shim now reports clean).
//
// Mode is ModeRestore so runInstallTasks takes the no-preflight
// branch and builds restore tasks against an empty tempdir (zero
// tasks → direct Summary transition without touching the engine).
func TestUpdateDpkgRepairAuthorize(t *testing.T) {
	dir := t.TempDir()
	log, err := executor.NewLogFile(filepath.Join(dir, "t.log"))
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	runner := executor.NewRunner(log, false)

	// dpkg still dirty initially — updateDpkgRepair "r" shouldn't
	// care; it flips the flag and hands control to runInstallTasks.
	marker := filepath.Join(dir, "healthy")
	bin := writeDpkgShim(t, marker)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	app := NewApp(newTestConfig())
	app.config.Runner = runner
	apt := pkgmgr.NewApt(runner, false)
	app.config.PkgMgr = apt
	app.dpkgApt = apt
	// DryRun + ModeRestore → preflight skipped, dry-run short-circuit
	// to PhaseSummary. This isolates the consent flip + handoff path
	// without spinning up the engine.
	app.config.DryRun = true
	app.config.Mode = ModeRestore
	app.config.RootDir = dir

	model, _ := app.updateDpkgRepair(teaKey('r'))
	var updated AppModel
	switch mm := model.(type) {
	case *AppModel:
		updated = *mm
	case AppModel:
		updated = mm
	default:
		t.Fatalf("unexpected model type %T", model)
	}
	if !apt.UserApprovedRepair {
		t.Fatal("r key must set UserApprovedRepair=true")
	}
	// runInstallTasks' dry-run branch lands on PhaseSummary.
	if updated.phase != PhaseSummary {
		t.Fatalf(
			"authorize+runInstallTasks(dry-run) should land on summary, got %v",
			updated.phase,
		)
	}
	logData, _ := os.ReadFile(log.Path())
	if !strings.Contains(string(logData), "user authorized repair") {
		t.Fatalf("log missing consent line: %s", logData)
	}
}

// TestUpdateDpkgRepairAbortCancelsEngine covers the abort branch
// when an engine cancel func is wired — it must be invoked, phase
// must transition to Summary with criticalFailure, and the abort
// line must appear in the log.
func TestUpdateDpkgRepairAbortCancelsEngine(t *testing.T) {
	dir := t.TempDir()
	log, err := executor.NewLogFile(filepath.Join(dir, "t.log"))
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()

	app := NewApp(newTestConfig())
	app.config.Runner = executor.NewRunner(log, false)
	app.phase = PhaseDpkgRepair

	cancelCalled := false
	app.cancelEngine = func() { cancelCalled = true }

	model, _ := app.updateDpkgRepair(teaKey('a'))
	updated := model.(AppModel)
	if !cancelCalled {
		t.Fatal("abort must invoke cancelEngine")
	}
	if updated.phase != PhaseSummary {
		t.Fatalf("abort should land on summary, got %v", updated.phase)
	}
	if !updated.summary.criticalFailure {
		t.Fatal("abort should mark criticalFailure")
	}
	if updated.summary.endTime.IsZero() {
		t.Fatal("abort should stamp endTime")
	}
	logData, _ := os.ReadFile(log.Path())
	if !strings.Contains(string(logData), "user aborted") {
		t.Fatalf("log missing abort line: %s", logData)
	}
}

// TestRunInstallTasksBlocksOnDpkgRepair drives the preflight-
// blocks-install path: fake dpkg reports dirty → runInstallTasks
// MUST transition to PhaseDpkgRepair and NOT proceed to summary.
//
// This is the load-bearing "never silent-heal" rule — if the gate
// regresses the installer would silently run apt into a broken
// dpkg state.
func TestRunInstallTasksBlocksOnDpkgRepair(t *testing.T) {
	dir := t.TempDir()
	log, err := executor.NewLogFile(filepath.Join(dir, "t.log"))
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()

	marker := filepath.Join(dir, "healthy") // absent → dirty
	bin := writeDpkgShim(t, marker)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	app := NewApp(newTestConfig())
	app.config.Runner = executor.NewRunner(log, false)
	app.config.PkgMgr = pkgmgr.NewApt(app.config.Runner, false)
	app.config.Mode = ModeInstall // preflight gate applies here
	app.config.RootDir = dir

	cmd := app.runInstallTasks()
	if cmd != nil {
		t.Fatal("blocked preflight should return nil Cmd (wait for user)")
	}
	if app.phase != PhaseDpkgRepair {
		t.Fatalf(
			"runInstallTasks must land on PhaseDpkgRepair, got %v",
			app.phase,
		)
	}
	// Must NOT have jumped to summary.
	if app.summary.criticalFailure {
		t.Fatal("blocked preflight should not pre-mark critical failure")
	}
}

// TestRunInstallTasksDryRunDoctor covers the ModeDoctor branch that
// sets summary.doctorMode=true and the DryRun short-circuit to
// summary.
func TestRunInstallTasksDryRunDoctor(t *testing.T) {
	app := NewApp(newTestConfig())
	app.config.DryRun = true
	app.config.Mode = ModeDoctor

	cmd := app.runInstallTasks()
	if cmd != nil {
		t.Fatal("dry-run should not return a Cmd")
	}
	if !app.summary.doctorMode {
		t.Fatal("ModeDoctor must set summary.doctorMode")
	}
	if app.phase != PhaseSummary {
		t.Fatalf(
			"dry-run doctor should end on summary, got %v", app.phase,
		)
	}
}

// TestRunInstallTasksDryRunRestore exercises the ModeRestore branch
// of the switch in runInstallTasks plus the dry-run early exit.
// ModeRestore is safe to hit with a nil PkgMgr (no apt preflight
// invoked for non-install modes) and asserts the summary start/end
// timestamps are equal on dry-run (no elapsed work).
func TestRunInstallTasksDryRunRestore(t *testing.T) {
	dir := t.TempDir()
	log, err := executor.NewLogFile(filepath.Join(dir, "t.log"))
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()

	app := NewApp(newTestConfig())
	app.config.Runner = executor.NewRunner(log, false)
	app.config.DryRun = true
	app.config.Mode = ModeRestore
	app.config.RootDir = dir

	cmd := app.runInstallTasks()
	if cmd != nil {
		t.Fatal("dry-run restore should not return a Cmd")
	}
	if app.phase != PhaseSummary {
		t.Fatalf(
			"dry-run restore should land on summary, got %v", app.phase,
		)
	}
	if app.summary.startTime.IsZero() || app.summary.endTime.IsZero() {
		t.Fatal("dry-run must stamp summary start/end times")
	}
	if !app.summary.startTime.Equal(app.summary.endTime) {
		t.Fatal(
			"dry-run should stamp start==end (no elapsed work)",
		)
	}
}

// TestSyncRepoCmdAutoRestoreRetrySuccess exercises the auto-restore
// inner retry path: first git invocation fails with a configs/-only
// dirty worktree (so bodyDriftInScope accepts), BackupAndReset
// succeeds, retry pull succeeds → repoSyncedMsg.
//
// The test uses a PATH-shim git that alternates exit codes keyed on
// a counter file, and seeds a real git repo + drifted configs/ file
// so DetectRepoDrift returns a non-empty slice.
func TestSyncRepoCmdAutoRestoreRetrySuccess(t *testing.T) {
	// Seed a real repo with an initial commit so DetectRepoDrift
	// has a HEAD to diff against.
	repo := t.TempDir()
	runGit := func(args ...string) {
		t.Helper()
		c := exec.Command("git", args...)
		c.Dir = repo
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	runGit("init", "--quiet")
	runGit("config", "user.email", "test@example.com")
	runGit("config", "user.name", "Test")
	runGit("config", "commit.gpgsign", "false")
	if err := os.MkdirAll(
		filepath.Join(repo, "configs", "zsh"), 0o755,
	); err != nil {
		t.Fatal(err)
	}
	zshrc := filepath.Join(repo, "configs", "zsh", ".zshrc")
	if err := os.WriteFile(zshrc, []byte("# original\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit("add", ".")
	runGit("commit", "--quiet", "-m", "seed")
	// Drift the tracked file inside configs/ to exercise the auto-
	// restore branch.
	if err := os.WriteFile(
		zshrc, []byte("# drifted by tool\n"), 0o644,
	); err != nil {
		t.Fatal(err)
	}

	// Shim git: first call exits 1 with "would be overwritten",
	// subsequent calls exit 0 (simulating retry success after
	// BackupAndReset restores the file).
	bin := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	counter := filepath.Join(bin, "count")
	script := "#!/bin/sh\n" +
		"n=$(cat \"" + counter + "\" 2>/dev/null || echo 0)\n" +
		"n=$((n+1))\n" +
		"printf '%s' \"$n\" > \"" + counter + "\"\n" +
		"# only shim `git pull`; pass anything else through to real git\n" +
		"if [ \"$1\" != \"pull\" ]; then\n" +
		"  exec /usr/bin/env PATH=\"$ORIG_PATH\" git \"$@\"\n" +
		"fi\n" +
		"if [ \"$n\" = \"1\" ]; then\n" +
		"  printf 'error: Your local changes to the following files would be overwritten by merge:\\n\\tconfigs/zsh/.zshrc\\nPlease commit your changes or stash them.\\n'\n" +
		"  exit 1\n" +
		"fi\n" +
		"printf 'Already up to date.\\n'\n" +
		"exit 0\n"
	if err := os.WriteFile(
		filepath.Join(bin, "git"), []byte(script), 0o755,
	); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ORIG_PATH", os.Getenv("PATH"))
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	log, err := executor.NewLogFile(filepath.Join(t.TempDir(), "log"))
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	app := NewApp(newTestConfig())
	app.config.Runner = executor.NewRunner(log, false)
	app.config.RootDir = repo

	msg := app.syncRepoCmd()()
	if _, ok := msg.(repoSyncedMsg); !ok {
		t.Fatalf("expected repoSyncedMsg after auto-restore retry, got %T", msg)
	}
	logData, _ := os.ReadFile(log.Path())
	if !strings.Contains(string(logData), "Auto-restored") {
		t.Fatalf("auto-restore path should log recovery: %s", logData)
	}
}
