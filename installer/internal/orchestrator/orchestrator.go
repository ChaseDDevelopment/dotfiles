// Package orchestrator builds engine task graphs for each
// installer mode (install, update, restore, uninstall, doctor).
// It is pure orchestration logic with no TUI dependencies.
package orchestrator

import (
	"context"
	"fmt"

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
}

// BuildResult is returned by each Build* function.
type BuildResult struct {
	Tasks             []engine.Task
	PlanRows          []PlanRow
	AlreadyInstalled  int
	AlreadyConfigured int
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
		tasks      []engine.Task
		rows       []PlanRow
		toolIDs    []string
		alreadyInst int
		alreadyCfg  int
	)

	runner := bc.Runner
	mgr := bc.PkgMgr
	plat := bc.Platform
	mgrName := mgr.Name()

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

		// Pass 2: create install tasks.
		for _, t := range tools {
			if !registry.ShouldInstall(&t, plat) {
				continue
			}
			status := registry.CheckInstalled(&t)
			if !bc.ForceReinstall && status == registry.StatusInstalled {
				alreadyInst++
				rows = append(rows, PlanRow{
					Component: t.Name, Action: "Package",
					Status: "already installed",
				})
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

			taskID := t.Command

			var deps []string
			for _, dep := range t.DependsOn {
				if !installedSet[dep] {
					deps = append(deps, dep)
				}
			}

			tasks = append(tasks, engine.Task{
				ID:        taskID,
				Label:     fmt.Sprintf("Installing %s", t.Name),
				Critical:  t.Critical,
				DependsOn: deps,
				Resources: resourcesForTool(&t, mgrName),
				Run: func(ctx context.Context) error {
					ic := &registry.InstallContext{
						Runner:         runner,
						PkgMgr:         mgr,
						Platform:       plat,
						ForceReinstall: bc.ForceReinstall,
					}
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
				},
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
			rows = append(rows, PlanRow{
				Component: comp.Name, Action: "Setup",
				Status: "already configured",
			})
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
				}
				return config.SetupComponent(ctx, comp, sc)
			},
		})
		setupIDs = append(setupIDs, taskID)
	}

	// Cleanup backup directory if requested.
	if bc.CleanBackup {
		rows = append(rows, PlanRow{
			Component: "Backup", Action: "Cleanup",
			Status: "would remove",
		})
		if !bc.DryRun {
			allDeps := make(
				[]string, 0, len(toolIDs)+len(setupIDs),
			)
			allDeps = append(allDeps, toolIDs...)
			allDeps = append(allDeps, setupIDs...)
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
		Tasks:             tasks,
		PlanRows:          rows,
		AlreadyInstalled:  alreadyInst,
		AlreadyConfigured: alreadyCfg,
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

// resourcesForTool determines which engine resources a tool needs
// based on its first applicable install strategy.
func resourcesForTool(
	t *registry.Tool,
	mgrName string,
) []engine.Resource {
	for _, s := range t.Strategies {
		if !s.AppliesTo(mgrName) {
			continue
		}
		switch s.Method {
		case registry.MethodPackageManager:
			if mgrName == "apt" {
				return []engine.Resource{engine.ResApt}
			}
			if mgrName == "brew" {
				return []engine.Resource{engine.ResBrew}
			}
		case registry.MethodCargo:
			return []engine.Resource{engine.ResCargo}
		case registry.MethodCustom:
			for _, m := range s.Managers {
				if m == "apt" && mgrName == "apt" {
					return []engine.Resource{engine.ResApt}
				}
				if m == "brew" && mgrName == "brew" {
					return []engine.Resource{engine.ResBrew}
				}
			}
		}
		return nil
	}
	return nil
}
