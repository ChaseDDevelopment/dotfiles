// Package orchestrator builds engine task graphs for each
// installer mode (install, update, restore, uninstall, doctor).
// It is pure orchestration logic with no TUI dependencies.
package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/chaseddevelopment/dotfiles/installer/internal/backup"
	"github.com/chaseddevelopment/dotfiles/installer/internal/config"
	"github.com/chaseddevelopment/dotfiles/installer/internal/engine"
	"github.com/chaseddevelopment/dotfiles/installer/internal/executor"
	"github.com/chaseddevelopment/dotfiles/installer/internal/pkgmgr"
	"github.com/chaseddevelopment/dotfiles/installer/internal/platform"
	"github.com/chaseddevelopment/dotfiles/installer/internal/registry"
	"github.com/chaseddevelopment/dotfiles/installer/internal/state"
	"github.com/chaseddevelopment/dotfiles/installer/internal/update"
)

// PlanRow holds one row of the dry-run summary table.
type PlanRow struct {
	Component string
	Action    string
	Status    string
}

// BuildConfig collects parameters needed to build task graphs.
type BuildConfig struct {
	Runner           *executor.Runner
	PkgMgr           pkgmgr.PackageManager
	Platform         *platform.Platform
	State            *state.Store
	RootDir          string
	DryRun           bool
	ForceReinstall   bool
	SkipPackages     bool
	SkipUpdate       bool
	CleanBackup      bool
	SelectedBackup   string
	SelectedComps    []string // nil = all
	Version          string   // build version for self-update
	// Failures collects best-effort post-install warnings from
	// component setup hooks. Shared across all tasks in one run so
	// the summary screen can show everything that didn't succeed.
	Failures *config.TrackedFailures
}

// BuildResult is returned by each Build* function.
type BuildResult struct {
	Tasks             []engine.Task
	PlanRows          []PlanRow
	AlreadyInstalled  int
	AlreadyConfigured int

	// Names behind the counts above. The summary UI renders these as
	// a manifest so a clean no-op run shows the user exactly which
	// tools and components were inspected, not just the totals.
	AlreadyInstalledNames  []string
	AlreadyConfiguredNames []string
}

// isComponentSelected checks whether a component name appears in
// the selected list, or returns true when the list is nil (all).
func (bc *BuildConfig) isComponentSelected(name string) bool {
	if bc.SelectedComps == nil {
		return true
	}
	for _, c := range bc.SelectedComps {
		if c == "All" || c == name {
			return true
		}
	}
	return false
}

// BuildInstallTasks creates the task graph for a fresh install.
func BuildInstallTasks(bc *BuildConfig) BuildResult {
	var (
		tasks       []engine.Task
		rows        []PlanRow
		toolIDs     []string
		alreadyInst int
		alreadyCfg  int
		instNames   []string
		cfgNames    []string
	)

	runner := bc.Runner
	mgr := bc.PkgMgr
	plat := bc.Platform
	mgrName := mgr.Name()

	// Insert the dpkg-doctor pseudo-task for apt-based systems. It
	// probes `dpkg --audit` + `/var/lib/dpkg/updates/` once, and
	// (per the current interim without the TUI modal) runs
	// `sudo dpkg --configure -a` if unhealthy. Every tool task
	// whose resource set includes ResDpkg takes the doctor as a
	// dependency so it always runs first under the same semaphore.
	var dpkgDoctorID string
	if apt, ok := mgr.(*pkgmgr.Apt); ok && !bc.SkipPackages {
		dpkgDoctorID = "dpkg-doctor"
		tasks = append(tasks, engine.Task{
			ID:        dpkgDoctorID,
			Label:     "Checking dpkg health",
			Critical:  true,
			Resources: []engine.Resource{engine.ResDpkg},
			Run: func(ctx context.Context) error {
				return apt.EnsureHealthy(ctx)
			},
		})
	}

	if !bc.SkipPackages {
		tools := registry.AllTools()

		// Pass 1: identify already-installed tools.
		installedSet := map[string]bool{}
		for _, t := range tools {
			if !registry.ShouldInstall(&t, plat) {
				installedSet[t.Command] = true
				continue
			}
			if registry.CheckInstalled(&t) == registry.StatusInstalled {
				installedSet[t.Command] = true
			}
		}

		// Pass 2: partition tools into batch-eligible (first
		// applicable strategy is MethodPackageManager under the
		// active manager) vs individual. Collecting the batch set
		// BEFORE creating tasks means each tool in a bucket can
		// share a single pkgmgr invocation via batchState.runOnce.
		var toInstall []registry.Tool
		bucket := make([]batchEntry, 0) // preserves registry order
		bs := &batchState{}

		for _, t := range tools {
			if !registry.ShouldInstall(&t, plat) {
				continue
			}
			status := registry.CheckInstalled(&t)
			if !bc.ForceReinstall && status == registry.StatusInstalled {
				alreadyInst++
				instNames = append(instNames, t.Name)
				rows = append(rows, PlanRow{
					Component: t.Name, Action: "Package",
					Status: "already installed",
				})
				if runner != nil && runner.Log != nil {
					runner.Log.Write(fmt.Sprintf(
						"SKIP: %s — already installed", t.Name,
					))
				}
				continue
			}
			planStatus := "would install"
			if status == registry.StatusOutdated {
				ver := registry.InstalledVersion(&t)
				planStatus = fmt.Sprintf(
					"outdated (%s → %s)", ver, t.MinVersion,
				)
			}
			rows = append(rows, PlanRow{
				Component: t.Name, Action: "Package",
				Status: planStatus,
			})
			toInstall = append(toInstall, t)
			if s := registry.FirstPkgMgrStrategy(&t, mgrName); s != nil && s.Package != "" {
				bucket = append(bucket, batchEntry{
					tool:       t,
					strategy:   s,
					genericPkg: s.Package,
				})
			}
		}

		// Build the deduplicated generic-name list for the batch
		// install. Order matches bucket order so mock-runner tests
		// see a stable shell command.
		var bucketGenerics []string
		seen := make(map[string]bool)
		for _, e := range bucket {
			if !seen[e.genericPkg] {
				seen[e.genericPkg] = true
				bucketGenerics = append(bucketGenerics, e.genericPkg)
			}
		}

		// Dependency injection: tools that touch dpkg must wait for
		// the doctor pseudo-task. We detect dpkg-touching via the
		// task's computed Resources set (which includes ResDpkg
		// whenever a MethodPackageManager[apt] strategy, an apt
		// Managers entry, or AcquiresDpkg applies).
		_ = dpkgDoctorID // silence unused when SkipPackages is true on brew

		// Per-tool task bodies. For tools in the bucket, the first
		// task to run executes the shared batch install via
		// sync.Once; the others wait on the Once and then fan out
		// per-tool post-install + RecordInstall (for successes) or
		// ExecuteInstallSkippingPkgMgr (for partial-batch failures)
		// — without re-invoking the package manager.
		for _, t := range toInstall {
			t := t
			taskID := t.Command
			var deps []string
			for _, dep := range t.DependsOn {
				if !installedSet[dep] {
					deps = append(deps, dep)
				}
			}
			// Derive implicit deps from each applicable strategy.
			deps = appendDerivedDeps(deps, &t, mgrName, installedSet)

			// Find whether this tool is batched; if so, capture its
			// generic pkgmgr name + strategy for the closure.
			var entry *batchEntry
			for i := range bucket {
				if bucket[i].tool.Command == t.Command {
					entry = &bucket[i]
					break
				}
			}

			run := func(ctx context.Context) error {
				ic := &registry.InstallContext{
					Runner:         runner,
					PkgMgr:         mgr,
					Platform:       plat,
					ForceReinstall: bc.ForceReinstall,
				}
				if entry != nil {
					return runBatchedInstall(
						ctx, &t, entry, ic, plat, bs,
						bucketGenerics, bc.State,
					)
				}
				// Non-batched tool — existing per-tool path.
				if err := registry.ExecuteInstall(
					ctx, &t, ic, plat,
				); err != nil {
					return err
				}
				if bc.State != nil {
					ver := registry.InstalledVersion(&t)
					bc.State.RecordInstall(
						t.Name, ver, "install",
					)
				}
				return nil
			}

			resources := resourcesForTool(&t, mgrName)
			if dpkgDoctorID != "" && hasResource(resources, engine.ResDpkg) {
				deps = append(deps, dpkgDoctorID)
			}

			tasks = append(tasks, engine.Task{
				ID:        taskID,
				Label:     fmt.Sprintf("Installing %s", t.Name),
				Critical:  t.Critical,
				DependsOn: deps,
				Resources: resources,
				Run:       run,
			})
			toolIDs = append(toolIDs, taskID)
		}
	}

	// Component setup (symlinks + hooks).
	bm := backup.NewManager(bc.DryRun)
	var setupIDs []string
	for _, comp := range config.AllComponents() {
		if !bc.isComponentSelected(comp.Name) {
			continue
		}
		status := config.InspectComponent(comp.Name, bc.RootDir)
		if status == "already configured" && !bc.ForceReinstall {
			alreadyCfg++
			cfgNames = append(cfgNames, comp.Name)
			rows = append(rows, PlanRow{
				Component: comp.Name, Action: "Setup",
				Status: "already configured",
			})
			if runner != nil && runner.Log != nil {
				runner.Log.Write(fmt.Sprintf(
					"SKIP: %s — already configured", comp.Name,
				))
			}
			continue
		}
		if status == "would replace" {
			diffs := config.DiffComponent(comp.Name, bc.RootDir)
			if len(diffs) > 0 {
				status = "would replace: " + diffs[0]
				if len(diffs) > 1 {
					status = fmt.Sprintf(
						"would replace (%d files)", len(diffs),
					)
				}
			}
		}
		rows = append(rows, PlanRow{
			Component: comp.Name, Action: "Setup", Status: status,
		})
		taskID := "setup-" + comp.Name

		// Each setup task only depends on its own required tool
		// (if that tool is being installed). This prevents an
		// unrelated tool failure from skipping all setups.
		var setupDeps []string
		if comp.RequiredCmd != "" {
			for _, tid := range toolIDs {
				if tid == comp.RequiredCmd {
					setupDeps = append(setupDeps, tid)
					break
				}
			}
		}

		tasks = append(tasks, engine.Task{
			ID:        taskID,
			Label:     fmt.Sprintf("Setting up %s", comp.Name),
			DependsOn: setupDeps,
			Run: func(ctx context.Context) error {
				sc := &config.SetupContext{
					Runner:   runner,
					RootDir:  bc.RootDir,
					Backup:   bm,
					DryRun:   bc.DryRun,
					Platform: plat,
					Failures: bc.Failures,
				}
				return config.SetupComponent(ctx, comp, sc)
			},
		})
		setupIDs = append(setupIDs, taskID)
	}

	// Post-install drift sweep: if any install script slipped an
	// append past the NoProfileModify env vars, restore the repo
	// configs to HEAD and record the backup dir as a warning. This
	// runs after every install+setup task so the next `git pull`
	// isn't blocked by accumulated cruft.
	driftSweepAdded := false
	if !bc.DryRun && bc.RootDir != "" {
		driftSweepAdded = true
		allDeps := make(
			[]string, 0, len(toolIDs)+len(setupIDs),
		)
		allDeps = append(allDeps, toolIDs...)
		allDeps = append(allDeps, setupIDs...)
		tasks = append(tasks, engine.Task{
			ID:        "sweep-repo-drift",
			Label:     "Restoring repo configs",
			DependsOn: allDeps,
			Run: func(_ context.Context) error {
				drifted, err := config.DetectRepoDrift(bc.RootDir)
				if err != nil || len(drifted) == 0 {
					return err
				}
				backupDir, rerr := config.BackupAndReset(
					bc.RootDir, bm, drifted,
				)
				if rerr != nil {
					return rerr
				}
				runner.Log.Write(fmt.Sprintf(
					"Drift sweep: restored %d config file(s); "+
						"originals saved to %s",
					len(drifted), backupDir,
				))
				bc.Failures.Record(
					"Repo",
					fmt.Sprintf(
						"restored %d file(s) mutated by install scripts",
						len(drifted),
					),
					fmt.Errorf("originals saved to %s", backupDir),
				)
				return nil
			},
		})
	}

	// Cleanup backup directory if requested.
	if bc.CleanBackup {
		rows = append(rows, PlanRow{
			Component: "Backup", Action: "Cleanup",
			Status: "would remove",
		})
		if !bc.DryRun {
			allDeps := make(
				[]string, 0, len(toolIDs)+len(setupIDs)+1,
			)
			allDeps = append(allDeps, toolIDs...)
			allDeps = append(allDeps, setupIDs...)
			// sweep-repo-drift may have saved files into this same
			// backup manager; make sure cleanup doesn't race by
			// running after the sweep, when the sweep exists.
			if driftSweepAdded {
				allDeps = append(allDeps, "sweep-repo-drift")
			}
			tasks = append(tasks, engine.Task{
				ID:        "cleanup-backup",
				Label:     "Cleaning up backup",
				DependsOn: allDeps,
				Run: func(_ context.Context) error {
					return bm.Cleanup()
				},
			})
		}
	}

	return BuildResult{
		Tasks:                  tasks,
		PlanRows:               rows,
		AlreadyInstalled:       alreadyInst,
		AlreadyConfigured:      alreadyCfg,
		AlreadyInstalledNames:  instNames,
		AlreadyConfiguredNames: cfgNames,
	}
}

// BuildUpdateTasks creates the task graph for updating tools.
func BuildUpdateTasks(bc *BuildConfig) BuildResult {
	var tasks []engine.Task
	updateSteps := update.AllSteps(
		bc.Runner, bc.PkgMgr, bc.Platform,
	)

	if step := update.SelfUpdateStep(
		bc.Runner, bc.Version,
	); step != nil {
		updateSteps = append(updateSteps, *step)
	}

	sysID := ""
	for _, s := range updateSteps {
		s := s
		if bc.SkipUpdate && s.Name == "System packages" {
			continue
		}
		id := "update-" + s.Name
		var deps []string
		if s.Name == "System packages" {
			sysID = id
		} else if sysID != "" {
			deps = []string{sysID}
		}
		tasks = append(tasks, engine.Task{
			ID:        id,
			Label:     fmt.Sprintf("Updating %s", s.Name),
			DependsOn: deps,
			Run: func(ctx context.Context) error {
				return s.Fn(ctx)
			},
		})
	}
	return BuildResult{Tasks: tasks}
}

// BuildRestoreTasks creates the task graph for restoring a backup.
func BuildRestoreTasks(bc *BuildConfig) BuildResult {
	backupPath := bc.SelectedBackup
	return BuildResult{
		Tasks: []engine.Task{
			{
				ID:    "restore",
				Label: "Restoring from backup",
				Run: func(_ context.Context) error {
					if backupPath == "" {
						return fmt.Errorf("no backup selected")
					}
					return backup.Restore(
						backupPath,
						config.ManagedTargets(),
						bc.DryRun,
					)
				},
			},
		},
	}
}

// BuildUninstallTasks creates the task graph for removing configs.
func BuildUninstallTasks(bc *BuildConfig) BuildResult {
	var tasks []engine.Task
	for _, comp := range config.AllComponents() {
		if !bc.isComponentSelected(comp.Name) {
			continue
		}
		tasks = append(tasks, engine.Task{
			ID:    "uninstall-" + comp.Name,
			Label: fmt.Sprintf("Removing %s", comp.Name),
			Run: func(_ context.Context) error {
				return config.RemoveComponentSymlinks(
					comp.Name, bc.RootDir, bc.Runner,
				)
			},
		})
	}
	return BuildResult{Tasks: tasks}
}

// BuildDoctorTasks creates the task graph for health checks.
func BuildDoctorTasks(bc *BuildConfig) BuildResult {
	var tasks []engine.Task

	for _, t := range registry.AllTools() {
		if !registry.ShouldInstall(&t, bc.Platform) {
			continue
		}
		t := t
		tasks = append(tasks, engine.Task{
			ID:    "check-" + t.Command,
			Label: "Checking " + t.Name,
			Run: func(_ context.Context) error {
				status := registry.CheckInstalled(&t)
				switch status {
				case registry.StatusNotInstalled:
					hint := ""
					if t.Command != "" {
						hint = fmt.Sprintf(
							" (fix: run installer or install %q manually)",
							t.Command,
						)
					}
					return fmt.Errorf("not installed%s", hint)
				case registry.StatusOutdated:
					ver := registry.InstalledVersion(&t)
					return fmt.Errorf(
						"outdated: have %s, need %s (fix: run Update from main menu)",
						ver, t.MinVersion,
					)
				}
				// Log version on success for verbose output.
				if bc.Runner != nil {
					if ver := registry.InstalledVersion(&t); ver != "" {
						bc.Runner.EmitVerbose(
							fmt.Sprintf("  %s: %s", t.Name, ver),
						)
					}
				}
				return nil
			},
		})
	}

	for _, comp := range config.AllComponents() {
		comp := comp
		tasks = append(tasks, engine.Task{
			ID:    "check-config-" + comp.Name,
			Label: "Checking " + comp.Name + " config",
			Run: func(_ context.Context) error {
				status := config.InspectComponent(
					comp.Name, bc.RootDir,
				)
				switch status {
				case "already configured":
					return nil
				case "would replace":
					return fmt.Errorf(
						"config conflicts detected (fix: run Install to update symlinks)",
					)
				case "would configure":
					return fmt.Errorf(
						"not configured (fix: run Install to create symlinks)",
					)
				default:
					return fmt.Errorf("%s", status)
				}
			},
		})
	}

	return BuildResult{Tasks: tasks}
}

// resourcesForTool returns the UNION of engine resources that any
// applicable strategy for this tool might acquire. Computing the
// union (rather than stopping at the first strategy) is what makes
// fallthrough safe: if strategy 0 is apt and strategy 1 is a script
// that itself shells out to apt, the task must hold ResDpkg
// regardless of which strategy ultimately runs — otherwise the
// fallthrough path races other apt work concurrently.
//
// Each strategy contributes:
//   - The pkgmgr-native resource for MethodPackageManager under the
//     active manager (ResDpkg/ResRpm/ResPacman/ResBrew).
//   - ResCargo for MethodCargo.
//   - ResDpkg when AcquiresDpkg is explicitly declared on the
//     strategy (e.g. MethodCustom that embeds `sudo apt install`).
//   - Legacy: MethodCustom with "apt"/"brew" in Managers still maps
//     to the matching resource for backward compatibility with
//     strategy definitions that predate AcquiresDpkg.
func resourcesForTool(
	t *registry.Tool,
	mgrName string,
) []engine.Resource {
	set := make(map[engine.Resource]struct{})
	add := func(r engine.Resource) { set[r] = struct{}{} }

	for _, s := range t.Strategies {
		if !s.AppliesTo(mgrName) {
			continue
		}
		switch s.Method {
		case registry.MethodPackageManager:
			if r, ok := pkgMgrResource(mgrName); ok {
				add(r)
			}
		case registry.MethodCargo:
			add(engine.ResCargo)
		case registry.MethodCustom, registry.MethodScript:
			// Honor the explicit AcquiresDpkg flag regardless of
			// which managers the strategy is gated on — scripts
			// can legitimately target any manager and still shell
			// out to apt for a pre-install step.
			if s.AcquiresDpkg {
				add(engine.ResDpkg)
			}
			// Legacy Managers-based detection for MethodCustom:
			// kept so existing strategy declarations don't silently
			// change behavior mid-refactor.
			for _, m := range s.Managers {
				if r, ok := pkgMgrResource(m); ok && m == mgrName {
					add(r)
				}
			}
		}
	}

	if len(set) == 0 {
		return nil
	}
	out := make([]engine.Resource, 0, len(set))
	for r := range set {
		out = append(out, r)
	}
	// Sort for deterministic ordering — makes test assertions and
	// log output stable.
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// batchEntry describes one tool's participation in the shared
// per-manager batch install bucket.
type batchEntry struct {
	tool       registry.Tool
	strategy   *registry.InstallStrategy
	genericPkg string
}

// batchState coordinates the cross-tool package-manager batch
// install. The first batched tool task to execute runs the shared
// install via sync.Once; the rest wait on the Once and then consult
// failedGenerics (populated only on partial failure) to decide
// whether they install cleanly or need fallback strategies.
//
// Fields after once.Do completes are immutable, so reads from the
// per-tool tasks don't need additional locking.
type batchState struct {
	once           sync.Once
	err            error
	failedGenerics map[string]struct{} // nil = total failure (all failed)
}

// runOnce is the sync.Once body: invokes the manager's batch install
// exactly once for the whole bucket, then classifies the outcome so
// per-tool tasks can consume it without re-entering.
func (b *batchState) runOnce(
	ctx context.Context,
	mgr pkgmgr.PackageManager,
	generics []string,
) {
	b.once.Do(func() {
		if len(generics) == 0 {
			return
		}
		err := mgr.Install(ctx, generics...)
		b.err = err
		if err == nil {
			return
		}
		var bf *pkgmgr.BatchFailure
		if errors.As(err, &bf) {
			b.failedGenerics = make(map[string]struct{}, len(bf.FailedNames))
			for _, n := range bf.FailedNames {
				b.failedGenerics[n] = struct{}{}
			}
			return
		}
		// Total-failure path: every caller-supplied generic is
		// unconfirmed. Leave failedGenerics nil as a sentinel for
		// "all failed" — per-tool tasks then probe IsInstalled to
		// distinguish actual-installs-despite-error from true
		// failures that must fall through.
		b.failedGenerics = nil
	})
}

// runBatchedInstall is the Run closure for a per-tool task whose
// first applicable strategy is the shared batch's MethodPackageManager.
// It ensures the batch ran (via sync.Once), then routes this
// specific tool to one of three outcomes:
//
//  1. Batch installed everything cleanly → fire this tool's
//     PostInstall + record install.
//  2. Partial batch failure and THIS tool was in the failed subset,
//     OR total failure and this tool is not actually present →
//     run fallback strategies (skipping MethodPackageManager so we
//     don't re-attempt the just-failed pkgmgr path).
//  3. Classified error that's terminal (ErrAptFatal) → propagate;
//     no cargo/GitHub fallthrough on fatal apt conditions.
func runBatchedInstall(
	ctx context.Context,
	t *registry.Tool,
	entry *batchEntry,
	ic *registry.InstallContext,
	p *platform.Platform,
	bs *batchState,
	generics []string,
	st *state.Store,
) error {
	bs.runOnce(ctx, ic.PkgMgr, generics)

	failedThisTool := false
	switch {
	case bs.err == nil:
		// happy path
	case bs.failedGenerics != nil:
		_, failedThisTool = bs.failedGenerics[entry.genericPkg]
	default:
		// Total batch failure. This tool might still be installed
		// (e.g. the shell errored on pkg 3 of 5 after 1+2 already
		// applied) — probe before assuming failure.
		failedThisTool = !ic.PkgMgr.IsInstalled(entry.genericPkg)
	}

	if !failedThisTool {
		// Success path (clean or despite-error): post-install + record.
		if err := registry.RunPostInstall(ctx, entry.strategy, ic); err != nil {
			return fmt.Errorf(
				"%s: post-install after batch install: %w",
				t.Name, err,
			)
		}
		if st != nil {
			ver := registry.InstalledVersion(t)
			st.RecordInstall(t.Name, ver, "install")
		}
		return nil
	}

	// Failure path for this tool. If the batch classified as
	// fatal-apt, honor that — do NOT fall through to cargo or
	// GitHub. The user is expected to fix the underlying apt state.
	if errors.Is(bs.err, pkgmgr.ErrAptFatal) {
		return fmt.Errorf(
			"%s: apt reported a fatal condition; "+
				"not attempting fallback strategies: %w",
			t.Name, bs.err,
		)
	}

	if err := registry.ExecuteInstallSkippingPkgMgr(ctx, t, ic, p); err != nil {
		return err
	}
	if st != nil {
		ver := registry.InstalledVersion(t)
		st.RecordInstall(t.Name, ver, "install")
	}
	return nil
}

// hasResource reports whether r appears in rs. Used to decide
// whether a tool task should take the dpkg-doctor as a dependency.
func hasResource(rs []engine.Resource, r engine.Resource) bool {
	for _, x := range rs {
		if x == r {
			return true
		}
	}
	return false
}

// containsString reports whether s is present in xs. Used for small
// local dedup of derived dep lists where pulling in `slices.Contains`
// would be more noise than value.
func containsString(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

// appendDerivedDeps extends deps with implicit command-level
// dependencies derived from t's applicable strategies. Each Method
// with an entry in registry.MethodRequires contributes its required
// command (e.g. MethodCargo → "cargo"); a strategy's explicit
// Requires list contributes additively. Dependencies already in
// installedSet or already in deps are skipped so we don't add
// redundant edges. Exported-adjacent so orchestrator tests can
// verify the derivation without going through BuildInstallTasks.
func appendDerivedDeps(
	deps []string,
	t *registry.Tool,
	mgrName string,
	installedSet map[string]bool,
) []string {
	for _, s := range t.Strategies {
		if !s.AppliesTo(mgrName) {
			continue
		}
		needs := make([]string, 0, len(s.Requires)+1)
		if cmd, ok := registry.MethodRequires[s.Method]; ok {
			needs = append(needs, cmd)
		}
		needs = append(needs, s.Requires...)
		for _, cmd := range needs {
			if cmd == "" || cmd == t.Command {
				continue
			}
			if installedSet[cmd] || containsString(deps, cmd) {
				continue
			}
			deps = append(deps, cmd)
		}
	}
	return deps
}

// pkgMgrResource maps a manager name (as used in
// InstallStrategy.Managers and platform detection) to its
// corresponding engine resource. dnf/yum/zypper all share the rpm
// database and therefore the same semaphore.
func pkgMgrResource(mgrName string) (engine.Resource, bool) {
	switch mgrName {
	case "apt":
		return engine.ResDpkg, true
	case "brew":
		return engine.ResBrew, true
	case "pacman":
		return engine.ResPacman, true
	case "dnf", "yum", "zypper":
		return engine.ResRpm, true
	}
	return "", false
}
