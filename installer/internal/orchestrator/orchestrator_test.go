package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chaseddevelopment/dotfiles/installer/internal/config"
	"github.com/chaseddevelopment/dotfiles/installer/internal/engine"
	"github.com/chaseddevelopment/dotfiles/installer/internal/executor"
	"github.com/chaseddevelopment/dotfiles/installer/internal/pkgmgr"
	"github.com/chaseddevelopment/dotfiles/installer/internal/platform"
	"github.com/chaseddevelopment/dotfiles/installer/internal/registry"
	"github.com/chaseddevelopment/dotfiles/installer/internal/state"
)

// stubPkgMgr implements pkgmgr.PackageManager for testing without
// running real package manager commands.
type stubPkgMgr struct {
	name string
}

func (s *stubPkgMgr) Name() string { return s.name }

func (s *stubPkgMgr) Install(
	_ context.Context, _ ...string,
) error {
	return nil
}

func (s *stubPkgMgr) IsInstalled(_ string) bool {
	return false
}

func (s *stubPkgMgr) UpdateAll(_ context.Context) error {
	return nil
}

func (s *stubPkgMgr) MapName(generic string) []string {
	return []string{generic}
}

type batchPkgMgr struct {
	stubPkgMgr
	err          error
	installed    map[string]bool
	installCalls int
}

func (b *batchPkgMgr) Install(
	_ context.Context, names ...string,
) error {
	b.installCalls++
	if b.installed == nil {
		b.installed = map[string]bool{}
	}
	for _, name := range names {
		if b.err == nil {
			b.installed[name] = true
		}
	}
	return b.err
}

func (b *batchPkgMgr) IsInstalled(name string) bool {
	return b.installed[name]
}

// newTestBuildConfig creates a BuildConfig suitable for testing.
// It uses a stub package manager, real platform detection, a
// temp-dir-backed runner, and an in-memory state store.
func newTestBuildConfig(t *testing.T) *BuildConfig {
	t.Helper()

	dir := t.TempDir()
	logPath := filepath.Join(dir, "test.log")
	lf, err := executor.NewLogFile(logPath)
	if err != nil {
		t.Fatalf("NewLogFile() error = %v", err)
	}
	t.Cleanup(func() { lf.Close() })

	runner := executor.NewRunner(lf, true) // dry-run = true

	plat := &platform.Platform{
		OS:             platform.MacOS,
		Arch:           platform.ARM64,
		OSName:         "macOS",
		PackageManager: platform.PkgBrew,
	}

	st := state.NewStore(filepath.Join(dir, "state.json"))

	return &BuildConfig{
		Runner:   runner,
		PkgMgr:   &stubPkgMgr{name: "brew"},
		Platform: plat,
		State:    st,
		RootDir:  dir,
		DryRun:   true,
	}
}

func TestBatchHelpers(t *testing.T) {
	t.Run("runOnce partial failure maps failed generics", func(t *testing.T) {
		mgr := &batchPkgMgr{
			stubPkgMgr: stubPkgMgr{name: "apt"},
			err: &pkgmgr.BatchFailure{
				FailedNames: []string{"b"},
				Wrapped:     errors.New("partial"),
			},
		}
		var bs batchState
		bs.runOnce(context.Background(), mgr, []string{"a", "b"})
		if mgr.installCalls != 1 {
			t.Fatalf("installCalls = %d, want 1", mgr.installCalls)
		}
		if _, ok := bs.failedGenerics["b"]; !ok {
			t.Fatalf("failedGenerics = %#v, want b present", bs.failedGenerics)
		}
	})

	t.Run("runBatchedInstall records success and fallbacks failed tool", func(t *testing.T) {
		bc := newTestBuildConfig(t)
		bc.Runner.DryRun = true
		store := bc.State
		mgr := &batchPkgMgr{stubPkgMgr: stubPkgMgr{name: "brew"}, installed: map[string]bool{}}
		ic := &registry.InstallContext{Runner: bc.Runner, PkgMgr: mgr, Platform: bc.Platform}
		bs := &batchState{}

		okTool := &registry.Tool{Name: "ok", Command: "ok"}
		okEntry := &batchEntry{
			tool: *okTool,
			strategy: &registry.InstallStrategy{
				Method: registry.MethodPackageManager,
			},
			genericPkg: "ok",
		}
		if err := runBatchedInstall(context.Background(), okTool, okEntry, ic, bc.Platform, bs, []string{"ok"}, store); err != nil {
			t.Fatalf("runBatchedInstall success: %v", err)
		}
		if mgr.installCalls != 1 {
			t.Fatalf("installCalls = %d, want 1", mgr.installCalls)
		}
		if _, ok := store.LookupTool("ok"); !ok {
			t.Fatal("expected state record for successful batch install")
		}

		fallbackCalled := false
		mgr = &batchPkgMgr{
			stubPkgMgr: stubPkgMgr{name: "brew"},
			err: &pkgmgr.BatchFailure{
				FailedNames: []string{"fail"},
				Wrapped:     errors.New("partial"),
			},
			installed: map[string]bool{},
		}
		ic = &registry.InstallContext{Runner: bc.Runner, PkgMgr: mgr, Platform: bc.Platform}
		bs = &batchState{}
		failTool := &registry.Tool{
			Name: "fail", Command: "fail",
			Strategies: []registry.InstallStrategy{
				{Method: registry.MethodPackageManager, Package: "fail"},
				{Method: registry.MethodCustom, CustomFunc: func(_ context.Context, _ *registry.InstallContext) error {
					fallbackCalled = true
					return nil
				}},
			},
		}
		failEntry := &batchEntry{
			tool:       *failTool,
			strategy:   &registry.InstallStrategy{Method: registry.MethodPackageManager, Package: "fail"},
			genericPkg: "fail",
		}
		if err := runBatchedInstall(context.Background(), failTool, failEntry, ic, bc.Platform, bs, []string{"fail"}, store); err != nil {
			t.Fatalf("runBatchedInstall fallback: %v", err)
		}
		if !fallbackCalled {
			t.Fatal("expected fallback strategy to run for failed batch tool")
		}
	})

	t.Run("runBatchedInstall propagates fatal apt", func(t *testing.T) {
		bc := newTestBuildConfig(t)
		mgr := &batchPkgMgr{stubPkgMgr: stubPkgMgr{name: "apt"}, err: fmt.Errorf("wrapped: %w", pkgmgr.ErrAptFatal)}
		ic := &registry.InstallContext{Runner: bc.Runner, PkgMgr: mgr, Platform: bc.Platform}
		bs := &batchState{}
		tool := &registry.Tool{Name: "fatal", Command: "fatal"}
		entry := &batchEntry{
			tool:       *tool,
			strategy:   &registry.InstallStrategy{Method: registry.MethodPackageManager, Package: "fatal"},
			genericPkg: "fatal",
		}
		err := runBatchedInstall(context.Background(), tool, entry, ic, bc.Platform, bs, []string{"fatal"}, bc.State)
		if err == nil || !strings.Contains(err.Error(), "fatal condition") {
			t.Fatalf("expected fatal apt error, got %v", err)
		}
	})

	t.Run("resource helpers", func(t *testing.T) {
		if !hasResource([]engine.Resource{engine.ResDpkg}, engine.ResDpkg) {
			t.Fatal("expected hasResource true")
		}
		if _, ok := pkgMgrResource("brew"); !ok {
			t.Fatal("expected brew resource")
		}
		if _, ok := pkgMgrResource("unknown"); ok {
			t.Fatal("unexpected resource for unknown manager")
		}
	})
}

// ---------- isComponentSelected ----------

func TestIsComponentSelected(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		selected []string
		comp     string
		want     bool
	}{
		{
			name:     "nil means all selected",
			selected: nil,
			comp:     "Zsh",
			want:     true,
		},
		{
			name:     "All selects everything",
			selected: []string{"All"},
			comp:     "Neovim",
			want:     true,
		},
		{
			name:     "exact match",
			selected: []string{"Zsh", "Tmux"},
			comp:     "Tmux",
			want:     true,
		},
		{
			name:     "not in list",
			selected: []string{"Zsh", "Tmux"},
			comp:     "Neovim",
			want:     false,
		},
		{
			name:     "empty list selects nothing",
			selected: []string{},
			comp:     "Zsh",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			bc := &BuildConfig{
				SelectedComps: tt.selected,
			}
			got := bc.isComponentSelected(tt.comp)
			if got != tt.want {
				t.Errorf(
					"isComponentSelected(%q) = %v, want %v",
					tt.comp, got, tt.want,
				)
			}
		})
	}
}

// ---------- BuildInstallTasks ----------

func TestBuildInstallTasksBasic(t *testing.T) {
	t.Parallel()
	bc := newTestBuildConfig(t)

	result := BuildInstallTasks(bc)

	if len(result.Tasks) == 0 {
		t.Fatal("expected at least one task")
	}

	// Verify all task IDs are non-empty and unique.
	idSet := make(map[string]struct{})
	for _, task := range result.Tasks {
		if task.ID == "" {
			t.Error("task has empty ID")
		}
		if _, exists := idSet[task.ID]; exists {
			t.Errorf("duplicate task ID: %q", task.ID)
		}
		idSet[task.ID] = struct{}{}
	}

	// Verify all labels are non-empty.
	for _, task := range result.Tasks {
		if task.Label == "" {
			t.Errorf("task %q has empty Label", task.ID)
		}
	}

	// Verify dependencies reference valid task IDs.
	for _, task := range result.Tasks {
		for _, dep := range task.DependsOn {
			if _, exists := idSet[dep]; !exists {
				t.Errorf(
					"task %q depends on %q which is not in task set",
					task.ID, dep,
				)
			}
		}
	}

	// Verify all Run functions are non-nil.
	for _, task := range result.Tasks {
		if task.Run == nil {
			t.Errorf("task %q has nil Run function", task.ID)
		}
	}
}

func TestBuildInstallTasksWithSelectedComponents(t *testing.T) {
	t.Parallel()
	bc := newTestBuildConfig(t)
	bc.SelectedComps = []string{"Zsh"}

	result := BuildInstallTasks(bc)

	// Should have setup tasks only for Zsh (plus tool installs).
	for _, task := range result.Tasks {
		if len(task.ID) > 6 && task.ID[:6] == "setup-" {
			compName := task.ID[6:]
			if compName != "Zsh" {
				t.Errorf(
					"unexpected setup task for %q "+
						"when only Zsh selected",
					compName,
				)
			}
		}
	}
}

func TestBuildInstallTasksSkipPackages(t *testing.T) {
	t.Parallel()
	bc := newTestBuildConfig(t)
	bc.SkipPackages = true

	result := BuildInstallTasks(bc)

	// With packages skipped, there should be no tool install tasks.
	// Only setup tasks (setup-*) should remain.
	for _, task := range result.Tasks {
		if task.ID[:6] != "setup-" && task.ID != "cleanup-backup" {
			t.Errorf(
				"expected only setup tasks with SkipPackages, "+
					"found %q",
				task.ID,
			)
		}
	}
}

func TestBuildInstallTasksForceReinstall(t *testing.T) {
	t.Parallel()
	bc := newTestBuildConfig(t)
	bc.ForceReinstall = true

	result := BuildInstallTasks(bc)

	// With force reinstall, there should be no
	// "already installed" entries.
	if result.AlreadyInstalled != 0 {
		t.Errorf(
			"AlreadyInstalled = %d, want 0 with ForceReinstall",
			result.AlreadyInstalled,
		)
	}
}

func TestBuildInstallTasksCleanBackup(t *testing.T) {
	t.Parallel()
	bc := newTestBuildConfig(t)
	bc.CleanBackup = true
	bc.DryRun = false

	result := BuildInstallTasks(bc)

	// There should be a cleanup-backup task.
	found := false
	for _, task := range result.Tasks {
		if task.ID == "cleanup-backup" {
			found = true
			// Cleanup should depend on all other tasks.
			if len(task.DependsOn) == 0 {
				t.Error(
					"cleanup-backup should depend on " +
						"other tasks",
				)
			}
			break
		}
	}
	if !found {
		t.Error("expected cleanup-backup task")
	}
}

func TestBuildInstallTasksCleanBackupDryRun(t *testing.T) {
	t.Parallel()
	bc := newTestBuildConfig(t)
	bc.CleanBackup = true
	bc.DryRun = true

	result := BuildInstallTasks(bc)

	// In dry-run mode, no cleanup-backup task should be created.
	for _, task := range result.Tasks {
		if task.ID == "cleanup-backup" {
			t.Error(
				"cleanup-backup task should not be " +
					"created in dry-run mode",
			)
		}
	}

	// But the plan row should still be present.
	found := false
	for _, row := range result.PlanRows {
		if row.Component == "Backup" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected Backup plan row in dry-run mode")
	}
}

func TestBuildInstallTasksPlanRows(t *testing.T) {
	t.Parallel()
	bc := newTestBuildConfig(t)

	result := BuildInstallTasks(bc)

	if len(result.PlanRows) == 0 {
		t.Fatal("expected at least one plan row")
	}

	// Every plan row should have a non-empty Component.
	for _, row := range result.PlanRows {
		if row.Component == "" {
			t.Error("plan row has empty Component")
		}
		if row.Action == "" {
			t.Error("plan row has empty Action")
		}
		if row.Status == "" {
			t.Error("plan row has empty Status")
		}
	}
}

func TestBuildInstallTasksEmptySelectedComponents(t *testing.T) {
	t.Parallel()
	bc := newTestBuildConfig(t)
	bc.SelectedComps = []string{}

	result := BuildInstallTasks(bc)

	// No setup tasks should be created for empty selection.
	for _, task := range result.Tasks {
		if len(task.ID) > 6 && task.ID[:6] == "setup-" {
			t.Errorf(
				"unexpected setup task %q with empty selection",
				task.ID,
			)
		}
	}
}

// ---------- BuildRestoreTasks ----------

func TestBuildRestoreTasks(t *testing.T) {
	t.Parallel()
	bc := newTestBuildConfig(t)
	bc.SelectedBackup = "/some/backup/path"

	result := BuildRestoreTasks(bc)

	if len(result.Tasks) != 1 {
		t.Fatalf(
			"expected 1 task, got %d",
			len(result.Tasks),
		)
	}

	task := result.Tasks[0]
	if task.ID != "restore" {
		t.Errorf("task ID = %q, want %q", task.ID, "restore")
	}
	if task.Label != "Restoring from backup" {
		t.Errorf("task Label = %q", task.Label)
	}
	if task.Run == nil {
		t.Error("Run func should not be nil")
	}
}

func TestBuildRestoreTasksNoBackupSelected(t *testing.T) {
	t.Parallel()
	bc := newTestBuildConfig(t)
	bc.SelectedBackup = ""

	result := BuildRestoreTasks(bc)

	if len(result.Tasks) != 1 {
		t.Fatalf(
			"expected 1 task, got %d",
			len(result.Tasks),
		)
	}

	// Running with empty backup should return an error.
	err := result.Tasks[0].Run(context.Background())
	if err == nil {
		t.Error("expected error for empty backup path")
	}
	if err.Error() != "no backup selected" {
		t.Errorf("error = %q, want %q", err, "no backup selected")
	}
}

// ---------- BuildUninstallTasks ----------

func TestBuildUninstallTasks(t *testing.T) {
	t.Parallel()
	bc := newTestBuildConfig(t)

	result := BuildUninstallTasks(bc)

	// Should have one uninstall task per component.
	allComps := config.AllComponents()
	if len(result.Tasks) != len(allComps) {
		t.Errorf(
			"expected %d uninstall tasks, got %d",
			len(allComps), len(result.Tasks),
		)
	}

	for _, task := range result.Tasks {
		if task.ID == "" {
			t.Error("task has empty ID")
		}
		if task.Label == "" {
			t.Error("task has empty Label")
		}
		if task.Run == nil {
			t.Errorf("task %q has nil Run", task.ID)
		}
		// Uninstall tasks should have no dependencies.
		if len(task.DependsOn) != 0 {
			t.Errorf(
				"uninstall task %q has unexpected deps: %v",
				task.ID, task.DependsOn,
			)
		}
	}
}

func TestBuildUninstallTasksSelectedOnly(t *testing.T) {
	t.Parallel()
	bc := newTestBuildConfig(t)
	bc.SelectedComps = []string{"Zsh", "Git"}

	result := BuildUninstallTasks(bc)

	if len(result.Tasks) != 2 {
		t.Errorf(
			"expected 2 uninstall tasks, got %d",
			len(result.Tasks),
		)
	}

	ids := make(map[string]bool)
	for _, task := range result.Tasks {
		ids[task.ID] = true
	}
	if !ids["uninstall-Zsh"] {
		t.Error("missing uninstall-Zsh task")
	}
	if !ids["uninstall-Git"] {
		t.Error("missing uninstall-Git task")
	}
}

func TestBuildUninstallTasksEmptySelection(t *testing.T) {
	t.Parallel()
	bc := newTestBuildConfig(t)
	bc.SelectedComps = []string{}

	result := BuildUninstallTasks(bc)

	if len(result.Tasks) != 0 {
		t.Errorf(
			"expected 0 uninstall tasks with empty selection, "+
				"got %d",
			len(result.Tasks),
		)
	}
}

func TestBuildUninstallTaskIDFormat(t *testing.T) {
	t.Parallel()
	bc := newTestBuildConfig(t)

	result := BuildUninstallTasks(bc)

	for _, task := range result.Tasks {
		if len(task.ID) <= 10 {
			t.Errorf("task ID %q is unexpectedly short", task.ID)
			continue
		}
		prefix := task.ID[:10]
		if prefix != "uninstall-" {
			t.Errorf(
				"task ID %q should start with 'uninstall-'",
				task.ID,
			)
		}
	}
}

// ---------- BuildDoctorTasks ----------

func TestBuildDoctorTasks(t *testing.T) {
	t.Parallel()
	bc := newTestBuildConfig(t)

	result := BuildDoctorTasks(bc)

	if len(result.Tasks) == 0 {
		t.Fatal("expected at least one doctor task")
	}

	idSet := make(map[string]struct{})
	for _, task := range result.Tasks {
		if task.ID == "" {
			t.Error("task has empty ID")
		}
		if _, exists := idSet[task.ID]; exists {
			t.Errorf("duplicate task ID: %q", task.ID)
		}
		idSet[task.ID] = struct{}{}

		if task.Label == "" {
			t.Errorf("task %q has empty Label", task.ID)
		}
		if task.Run == nil {
			t.Errorf("task %q has nil Run", task.ID)
		}
	}

	// Doctor tasks should have no dependencies.
	for _, task := range result.Tasks {
		if len(task.DependsOn) != 0 {
			t.Errorf(
				"doctor task %q has unexpected deps: %v",
				task.ID, task.DependsOn,
			)
		}
	}
}

func TestBuildDoctorTasksContainsToolChecks(t *testing.T) {
	t.Parallel()
	bc := newTestBuildConfig(t)

	result := BuildDoctorTasks(bc)

	hasToolCheck := false
	hasConfigCheck := false
	for _, task := range result.Tasks {
		if len(task.ID) > 6 && task.ID[:6] == "check-" {
			if len(task.ID) > 13 &&
				task.ID[:13] == "check-config-" {
				hasConfigCheck = true
			} else {
				hasToolCheck = true
			}
		}
	}
	if !hasToolCheck {
		t.Error("expected at least one tool check task")
	}
	if !hasConfigCheck {
		t.Error("expected at least one config check task")
	}
}

func TestBuildDoctorTasksConfigCheckCount(t *testing.T) {
	t.Parallel()
	bc := newTestBuildConfig(t)

	result := BuildDoctorTasks(bc)

	allComps := config.AllComponents()
	configChecks := 0
	for _, task := range result.Tasks {
		if len(task.ID) > 13 &&
			task.ID[:13] == "check-config-" {
			configChecks++
		}
	}
	if configChecks != len(allComps) {
		t.Errorf(
			"expected %d config check tasks, got %d",
			len(allComps), configChecks,
		)
	}
}

// ---------- BuildUpdateTasks ----------

func TestBuildUpdateTasks(t *testing.T) {
	t.Parallel()
	bc := newTestBuildConfig(t)

	result := BuildUpdateTasks(bc)

	if len(result.Tasks) == 0 {
		t.Fatal("expected at least one update task")
	}

	idSet := make(map[string]struct{})
	for _, task := range result.Tasks {
		if task.ID == "" {
			t.Error("task has empty ID")
		}
		if _, exists := idSet[task.ID]; exists {
			t.Errorf("duplicate task ID: %q", task.ID)
		}
		idSet[task.ID] = struct{}{}

		if task.Label == "" {
			t.Errorf("task %q has empty Label", task.ID)
		}
		if task.Run == nil {
			t.Errorf("task %q has nil Run", task.ID)
		}
	}

	// Verify dependency references are valid.
	for _, task := range result.Tasks {
		for _, dep := range task.DependsOn {
			if _, exists := idSet[dep]; !exists {
				t.Errorf(
					"task %q depends on %q "+
						"which is not in task set",
					task.ID, dep,
				)
			}
		}
	}
}

func TestBuildUpdateTasksIDPrefix(t *testing.T) {
	t.Parallel()
	bc := newTestBuildConfig(t)

	result := BuildUpdateTasks(bc)

	for _, task := range result.Tasks {
		if len(task.ID) < 8 || task.ID[:7] != "update-" {
			t.Errorf(
				"update task ID %q should start with 'update-'",
				task.ID,
			)
		}
	}
}

func TestBuildUpdateTasksDependOnSystemPackages(t *testing.T) {
	t.Parallel()
	bc := newTestBuildConfig(t)

	result := BuildUpdateTasks(bc)

	// The "System packages" step should be first and have no deps.
	// Other steps should depend on it.
	sysID := ""
	for _, task := range result.Tasks {
		if task.ID == "update-System packages" {
			sysID = task.ID
			if len(task.DependsOn) != 0 {
				t.Errorf(
					"system packages task should have no deps, "+
						"got %v",
					task.DependsOn,
				)
			}
			break
		}
	}
	if sysID == "" {
		t.Fatal("missing update-System packages task")
	}

	// At least one other task should depend on it.
	depFound := false
	for _, task := range result.Tasks {
		if task.ID == sysID {
			continue
		}
		for _, dep := range task.DependsOn {
			if dep == sysID {
				depFound = true
				break
			}
		}
		if depFound {
			break
		}
	}
	if !depFound {
		t.Error(
			"expected at least one update task to depend on " +
				"system packages",
		)
	}
}

func TestBuildUpdateTasksNoSelfUpdateForDev(t *testing.T) {
	t.Parallel()
	bc := newTestBuildConfig(t)
	bc.Version = "dev"

	result := BuildUpdateTasks(bc)

	for _, task := range result.Tasks {
		if task.ID == "update-dotsetup self-update" {
			t.Error(
				"self-update task should not exist for " +
					"dev version",
			)
		}
	}
}

func TestBuildUpdateTasksWithVersion(t *testing.T) {
	t.Parallel()
	bc := newTestBuildConfig(t)
	bc.Version = "v1.0.0"

	result := BuildUpdateTasks(bc)

	found := false
	for _, task := range result.Tasks {
		if task.ID == "update-dotsetup self-update" {
			found = true
			break
		}
	}
	if !found {
		t.Error(
			"expected self-update task for non-dev version",
		)
	}
}

// ---------- resourcesForTool ----------

func TestResourcesForTool(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		tool    registry.Tool
		mgrName string
		want    []engine.Resource
	}{
		{
			name: "brew package manager",
			tool: registry.Tool{
				Strategies: []registry.InstallStrategy{
					{
						Method: registry.MethodPackageManager,
					},
				},
			},
			mgrName: "brew",
			want:    []engine.Resource{engine.ResBrew},
		},
		{
			name: "apt package manager",
			tool: registry.Tool{
				Strategies: []registry.InstallStrategy{
					{
						Method: registry.MethodPackageManager,
					},
				},
			},
			mgrName: "apt",
			want:    []engine.Resource{engine.ResDpkg},
		},
		{
			name: "cargo method",
			tool: registry.Tool{
				Strategies: []registry.InstallStrategy{
					{
						Method: registry.MethodCargo,
					},
				},
			},
			mgrName: "brew",
			want:    []engine.Resource{engine.ResCargo},
		},
		{
			name: "custom method with brew manager",
			tool: registry.Tool{
				Strategies: []registry.InstallStrategy{
					{
						Method:   registry.MethodCustom,
						Managers: []string{"brew"},
					},
				},
			},
			mgrName: "brew",
			want:    []engine.Resource{engine.ResBrew},
		},
		{
			name: "custom method with apt manager",
			tool: registry.Tool{
				Strategies: []registry.InstallStrategy{
					{
						Method:   registry.MethodCustom,
						Managers: []string{"apt"},
					},
				},
			},
			mgrName: "apt",
			want:    []engine.Resource{engine.ResDpkg},
		},
		{
			name: "strategy not applicable",
			tool: registry.Tool{
				Strategies: []registry.InstallStrategy{
					{
						Method:   registry.MethodPackageManager,
						Managers: []string{"apt"},
					},
				},
			},
			mgrName: "brew",
			want:    nil,
		},
		{
			name:    "no strategies",
			tool:    registry.Tool{},
			mgrName: "brew",
			want:    nil,
		},
		{
			name: "github release returns nil",
			tool: registry.Tool{
				Strategies: []registry.InstallStrategy{
					{
						Method: registry.MethodGitHubRelease,
					},
				},
			},
			mgrName: "brew",
			want:    nil,
		},
		{
			name: "script returns nil",
			tool: registry.Tool{
				Strategies: []registry.InstallStrategy{
					{
						Method: registry.MethodScript,
					},
				},
			},
			mgrName: "brew",
			want:    nil,
		},
		{
			name: "uses first applicable strategy",
			tool: registry.Tool{
				Strategies: []registry.InstallStrategy{
					{
						Method:   registry.MethodPackageManager,
						Managers: []string{"apt"},
					},
					{
						Method: registry.MethodCargo,
					},
				},
			},
			mgrName: "brew",
			want:    []engine.Resource{engine.ResCargo},
		},
		{
			name: "pacman maps to ResPacman",
			tool: registry.Tool{
				Strategies: []registry.InstallStrategy{
					{
						Method: registry.MethodPackageManager,
					},
				},
			},
			mgrName: "pacman",
			want:    []engine.Resource{engine.ResPacman},
		},
		{
			name: "dnf maps to ResRpm",
			tool: registry.Tool{
				Strategies: []registry.InstallStrategy{
					{Method: registry.MethodPackageManager},
				},
			},
			mgrName: "dnf",
			want:    []engine.Resource{engine.ResRpm},
		},
		{
			name: "yum and zypper share ResRpm",
			tool: registry.Tool{
				Strategies: []registry.InstallStrategy{
					{Method: registry.MethodPackageManager},
				},
			},
			mgrName: "zypper",
			want:    []engine.Resource{engine.ResRpm},
		},
		{
			name: "script with AcquiresDpkg reserves ResDpkg even under brew manager",
			tool: registry.Tool{
				Strategies: []registry.InstallStrategy{
					{
						Method:       registry.MethodScript,
						AcquiresDpkg: true,
					},
				},
			},
			mgrName: "brew",
			want:    []engine.Resource{engine.ResDpkg},
		},
		{
			name: "apt + cargo strategies union to {ResDpkg, ResCargo}",
			tool: registry.Tool{
				Strategies: []registry.InstallStrategy{
					{Method: registry.MethodPackageManager},
					{Method: registry.MethodCargo},
				},
			},
			mgrName: "apt",
			// Sorted alphabetically: cargo < dpkg
			want: []engine.Resource{engine.ResCargo, engine.ResDpkg},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := resourcesForTool(&tt.tool, tt.mgrName)
			if len(got) != len(tt.want) {
				t.Fatalf(
					"resourcesForTool() = %v, want %v",
					got, tt.want,
				)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf(
						"resource[%d] = %v, want %v",
						i, got[i], tt.want[i],
					)
				}
			}
		})
	}
}

// ---------- Cross-cutting structural tests ----------

func TestAllBuildResultsHaveNonNilRunFunctions(t *testing.T) {
	t.Parallel()
	bc := newTestBuildConfig(t)

	builders := map[string]func(*BuildConfig) BuildResult{
		"Install":   BuildInstallTasks,
		"Restore":   BuildRestoreTasks,
		"Uninstall": BuildUninstallTasks,
		"Doctor":    BuildDoctorTasks,
	}

	for name, builder := range builders {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			localBC := *bc
			if name == "Restore" {
				localBC.SelectedBackup = "/fake/backup"
			}
			result := builder(&localBC)
			for _, task := range result.Tasks {
				if task.Run == nil {
					t.Errorf(
						"%s: task %q has nil Run",
						name, task.ID,
					)
				}
			}
		})
	}
}

func TestAllBuildResultsHaveUniqueIDs(t *testing.T) {
	t.Parallel()
	bc := newTestBuildConfig(t)

	builders := map[string]func(*BuildConfig) BuildResult{
		"Install":   BuildInstallTasks,
		"Restore":   BuildRestoreTasks,
		"Uninstall": BuildUninstallTasks,
		"Doctor":    BuildDoctorTasks,
	}

	for name, builder := range builders {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			localBC := *bc
			if name == "Restore" {
				localBC.SelectedBackup = "/fake/backup"
			}
			result := builder(&localBC)
			idSet := make(map[string]struct{})
			for _, task := range result.Tasks {
				if _, exists := idSet[task.ID]; exists {
					t.Errorf(
						"%s: duplicate task ID %q",
						name, task.ID,
					)
				}
				idSet[task.ID] = struct{}{}
			}
		})
	}
}

func TestBuildInstallTasksSetupDependsOnRequiredTool(t *testing.T) {
	t.Parallel()
	bc := newTestBuildConfig(t)
	bc.ForceReinstall = true

	result := BuildInstallTasks(bc)

	// Build a map of task IDs for lookup.
	taskMap := make(map[string]engine.Task)
	for _, task := range result.Tasks {
		taskMap[task.ID] = task
	}

	// For each component that has a RequiredCmd, check that the
	// corresponding setup task depends on that command's install
	// task (if it exists).
	for _, comp := range config.AllComponents() {
		if comp.RequiredCmd == "" {
			continue
		}
		setupID := "setup-" + comp.Name
		setupTask, ok := taskMap[setupID]
		if !ok {
			continue // setup task might be skipped
		}
		// Check if the required tool has an install task.
		if _, hasInstall := taskMap[comp.RequiredCmd]; hasInstall {
			depFound := false
			for _, dep := range setupTask.DependsOn {
				if dep == comp.RequiredCmd {
					depFound = true
					break
				}
			}
			if !depFound {
				t.Errorf(
					"setup-%s should depend on %q "+
						"install task",
					comp.Name, comp.RequiredCmd,
				)
			}
		}
	}
}

func TestBuildInstallTasksLinuxPlatform(t *testing.T) {
	t.Parallel()
	bc := newTestBuildConfig(t)
	bc.Platform = &platform.Platform{
		OS:             platform.Linux,
		Arch:           platform.AMD64,
		OSName:         "Ubuntu",
		PackageManager: platform.PkgApt,
	}
	bc.PkgMgr = &stubPkgMgr{name: "apt"}

	result := BuildInstallTasks(bc)

	// Should still produce tasks successfully.
	if len(result.Tasks) == 0 {
		t.Fatal("expected at least one task for Linux build")
	}

	// Verify IDs are unique.
	idSet := make(map[string]struct{})
	for _, task := range result.Tasks {
		if _, exists := idSet[task.ID]; exists {
			t.Errorf("duplicate task ID: %q", task.ID)
		}
		idSet[task.ID] = struct{}{}
	}
}

func TestPlanRowStruct(t *testing.T) {
	t.Parallel()

	row := PlanRow{
		Component: "Zsh",
		Action:    "Package",
		Status:    "would install",
	}
	if row.Component != "Zsh" {
		t.Errorf("Component = %q", row.Component)
	}
	if row.Action != "Package" {
		t.Errorf("Action = %q", row.Action)
	}
	if row.Status != "would install" {
		t.Errorf("Status = %q", row.Status)
	}
}

func TestBuildDoctorTasksToolCheckRunNotInstalled(t *testing.T) {
	t.Parallel()
	bc := newTestBuildConfig(t)

	result := BuildDoctorTasks(bc)

	// Find a tool check task and run it. Most tools won't be
	// installed in the test environment, so we expect errors.
	ctx := context.Background()
	for _, task := range result.Tasks {
		if len(task.ID) > 6 && task.ID[:6] == "check-" &&
			!(len(task.ID) > 13 &&
				task.ID[:13] == "check-config-") {
			err := task.Run(ctx)
			// The error message should contain useful info.
			// We don't assert specific status since the test
			// environment may or may not have tools installed.
			// We just verify the Run function executes without
			// panic.
			_ = err
			return
		}
	}
	t.Error("no tool check tasks found to run")
}

func TestBuildDoctorTasksConfigCheckRunNotConfigured(t *testing.T) {
	t.Parallel()
	bc := newTestBuildConfig(t)
	// Use a temp dir with no config files, so InspectComponent
	// returns "would configure".
	bc.RootDir = t.TempDir()

	result := BuildDoctorTasks(bc)

	ctx := context.Background()
	for _, task := range result.Tasks {
		if len(task.ID) > 13 &&
			task.ID[:13] == "check-config-" {
			err := task.Run(ctx)
			if err == nil {
				t.Errorf(
					"expected error from %q with no configs",
					task.ID,
				)
			}
			return
		}
	}
	t.Error("no config check tasks found")
}

func TestBuildDoctorTasksToolCheckWithVerbose(t *testing.T) {
	t.Parallel()
	bc := newTestBuildConfig(t)
	bc.Runner.EnableVerboseChannel(100)

	result := BuildDoctorTasks(bc)

	ctx := context.Background()
	for _, task := range result.Tasks {
		if len(task.ID) > 6 && task.ID[:6] == "check-" &&
			!(len(task.ID) > 13 &&
				task.ID[:13] == "check-config-") {
			// Run the task; it will either emit verbose output
			// on success or return an error.
			task.Run(ctx)
			return
		}
	}
}

func TestBuildRestoreTasksRunWithBackup(t *testing.T) {
	t.Parallel()
	bc := newTestBuildConfig(t)
	// Use a non-existent path that will cause Restore to fail,
	// but exercises the code path.
	bc.SelectedBackup = filepath.Join(t.TempDir(), "fake-backup")

	result := BuildRestoreTasks(bc)

	ctx := context.Background()
	err := result.Tasks[0].Run(ctx)
	// The backup path doesn't exist, so this should fail
	// but exercise the restore code path.
	_ = err
}

func TestBuildUninstallTasksRunFunction(t *testing.T) {
	t.Parallel()
	bc := newTestBuildConfig(t)
	bc.SelectedComps = []string{"Zsh"}
	bc.RootDir = t.TempDir()

	result := BuildUninstallTasks(bc)

	if len(result.Tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(result.Tasks))
	}

	// Run the uninstall task. With no symlinks present, it
	// should complete without error.
	ctx := context.Background()
	err := result.Tasks[0].Run(ctx)
	if err != nil {
		t.Errorf("uninstall Run() error = %v", err)
	}
}

func TestBuildInstallTasksWithNilState(t *testing.T) {
	t.Parallel()
	bc := newTestBuildConfig(t)
	bc.State = nil

	// Should not panic.
	result := BuildInstallTasks(bc)
	if len(result.Tasks) == 0 {
		t.Fatal("expected at least one task")
	}
}

func TestBuildUpdateTasksEmptyVersion(t *testing.T) {
	t.Parallel()
	bc := newTestBuildConfig(t)
	bc.Version = ""

	result := BuildUpdateTasks(bc)

	for _, task := range result.Tasks {
		if task.ID == "update-dotsetup self-update" {
			t.Error(
				"self-update task should not exist for " +
					"empty version",
			)
		}
	}
}

func TestBuildDoctorTasksRunAllToolChecks(t *testing.T) {
	t.Parallel()
	bc := newTestBuildConfig(t)
	bc.Runner.EnableVerboseChannel(500)

	result := BuildDoctorTasks(bc)

	ctx := context.Background()
	toolChecks := 0
	for _, task := range result.Tasks {
		// Only run tool checks (not config checks).
		if len(task.ID) > 13 &&
			task.ID[:13] == "check-config-" {
			continue
		}
		toolChecks++
		// Run the check. We don't care about the error -- we
		// just need the code paths exercised.
		task.Run(ctx)
	}
	if toolChecks == 0 {
		t.Error("expected at least one tool check")
	}
}

func TestBuildDoctorTasksRunAllConfigChecks(t *testing.T) {
	t.Parallel()
	bc := newTestBuildConfig(t)
	bc.RootDir = t.TempDir()

	result := BuildDoctorTasks(bc)

	ctx := context.Background()
	configChecks := 0
	for _, task := range result.Tasks {
		if len(task.ID) > 13 &&
			task.ID[:13] == "check-config-" {
			configChecks++
			err := task.Run(ctx)
			// With empty rootDir and no symlinks, every
			// config should report some non-configured status.
			if err == nil {
				// Some configs might be "already configured"
				// if the test environment happens to have them.
				continue
			}
		}
	}
	if configChecks == 0 {
		t.Error("expected at least one config check")
	}
}

func TestBuildUpdateTasksRunFunctions(t *testing.T) {
	t.Parallel()
	bc := newTestBuildConfig(t)

	result := BuildUpdateTasks(bc)

	ctx := context.Background()
	// Run the first update task to exercise the closure.
	if len(result.Tasks) > 0 {
		// These are dry-run so they use a stub package manager.
		// The system packages task calls mgr.UpdateAll which
		// returns nil for our stub.
		for _, task := range result.Tasks {
			if task.ID == "update-System packages" {
				err := task.Run(ctx)
				if err != nil {
					t.Errorf(
						"system packages update error = %v",
						err,
					)
				}
				break
			}
		}
	}
}

func TestBuildInstallTasksSetupRunFunctions(t *testing.T) {
	t.Parallel()
	bc := newTestBuildConfig(t)
	bc.SkipPackages = true
	bc.ForceReinstall = true
	bc.DryRun = true
	bc.RootDir = t.TempDir()

	result := BuildInstallTasks(bc)

	// Run all setup tasks. They are in dry-run mode so they
	// should exercise the code path without side effects.
	ctx := context.Background()
	for _, task := range result.Tasks {
		if len(task.ID) > 6 && task.ID[:6] == "setup-" {
			// Some may fail due to missing required commands,
			// which is expected.
			task.Run(ctx)
		}
	}
}

func TestBuildInstallTasksCleanBackupRunFunction(t *testing.T) {
	t.Parallel()
	bc := newTestBuildConfig(t)
	bc.SkipPackages = true
	bc.SelectedComps = []string{}
	bc.CleanBackup = true
	bc.DryRun = false

	result := BuildInstallTasks(bc)

	ctx := context.Background()
	for _, task := range result.Tasks {
		if task.ID == "cleanup-backup" {
			// Run the cleanup task. The backup dir may not
			// exist, but it exercises the code path.
			task.Run(ctx)
			return
		}
	}
	t.Error("expected cleanup-backup task")
}

func TestBuildDoctorTasksNilRunner(t *testing.T) {
	t.Parallel()
	bc := newTestBuildConfig(t)
	bc.Runner = nil

	result := BuildDoctorTasks(bc)

	// Should still build tasks without panicking.
	if len(result.Tasks) == 0 {
		t.Fatal("expected at least one task")
	}

	// Run a tool check task to exercise the nil Runner path.
	ctx := context.Background()
	for _, task := range result.Tasks {
		if len(task.ID) > 6 && task.ID[:6] == "check-" &&
			!(len(task.ID) > 13 &&
				task.ID[:13] == "check-config-") {
			task.Run(ctx)
			return
		}
	}
}

func TestBuildDoctorConfigCheckAllStatuses(t *testing.T) {
	t.Parallel()
	bc := newTestBuildConfig(t)
	// Use a temp dir that does NOT have the config source files,
	// so InspectComponent will report "would configure".
	tmpDir := t.TempDir()
	bc.RootDir = tmpDir

	result := BuildDoctorTasks(bc)

	ctx := context.Background()
	var configErrs []string
	for _, task := range result.Tasks {
		if len(task.ID) > 13 &&
			task.ID[:13] == "check-config-" {
			err := task.Run(ctx)
			if err != nil {
				configErrs = append(configErrs, err.Error())
			}
		}
	}
	// With no configs set up, most should report errors.
	if len(configErrs) == 0 {
		t.Error("expected at least one config check error")
	}
}

func TestBuildDoctorToolCheckNotInstalledHasHint(t *testing.T) {
	t.Parallel()
	bc := newTestBuildConfig(t)

	result := BuildDoctorTasks(bc)

	ctx := context.Background()
	for _, task := range result.Tasks {
		if len(task.ID) > 6 && task.ID[:6] == "check-" &&
			!(len(task.ID) > 13 &&
				task.ID[:13] == "check-config-") {
			err := task.Run(ctx)
			if err != nil {
				errMsg := err.Error()
				// If the tool is not installed, the error
				// should contain a hint about installing.
				if strings.Contains(errMsg, "not installed") {
					if !strings.Contains(errMsg, "fix:") {
						t.Errorf(
							"not-installed error for %q "+
								"missing hint: %s",
							task.ID, errMsg,
						)
					}
					return
				}
			}
		}
	}
	// If all tools happen to be installed, skip the assertion.
}

func TestBuildInstallTasksOutdatedPlanRow(t *testing.T) {
	t.Parallel()
	bc := newTestBuildConfig(t)

	result := BuildInstallTasks(bc)

	// Just verify plan rows have valid statuses.
	for _, row := range result.PlanRows {
		validStatuses := []string{
			"already installed",
			"would install",
			"already configured",
			"would configure",
			"would replace",
		}
		valid := false
		for _, vs := range validStatuses {
			if strings.HasPrefix(row.Status, vs) ||
				strings.HasPrefix(row.Status, "outdated") {
				valid = true
				break
			}
		}
		if !valid {
			t.Errorf(
				"unexpected plan row status %q for %q",
				row.Status, row.Component,
			)
		}
	}
}

func TestBuildConfigDefaults(t *testing.T) {
	t.Parallel()

	bc := &BuildConfig{}
	if bc.DryRun {
		t.Error("DryRun should default to false")
	}
	if bc.ForceReinstall {
		t.Error("ForceReinstall should default to false")
	}
	if bc.SkipPackages {
		t.Error("SkipPackages should default to false")
	}
	if bc.CleanBackup {
		t.Error("CleanBackup should default to false")
	}
	if bc.SelectedComps != nil {
		t.Error("SelectedComps should default to nil")
	}
}
