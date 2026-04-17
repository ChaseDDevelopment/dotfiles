package orchestrator

import (
	"testing"

	"github.com/chaseddevelopment/dotfiles/installer/internal/engine"
)

// TestMaintenanceTasksEmittedUnconditionally covers the hydra bug:
// `setupTmux`/`setupNeovim` were guarded by "already configured"
// and never ran on re-installs, so tmux plugin prune + nvim drift
// heal silently didn't execute. The fix lifts those into dedicated
// `maintain-tmux` / `maintain-nvim` tasks that must exist in the
// task graph regardless of component-symlink state.
func TestMaintenanceTasksEmittedUnconditionally(t *testing.T) {
	bc := newTestBuildConfig(t)
	bc.DryRun = false // maintenance gates on !DryRun

	result := BuildInstallTasks(bc)

	want := map[string]bool{"maintain-tmux": false, "maintain-nvim": false}
	for _, task := range result.Tasks {
		if _, ok := want[task.ID]; ok {
			want[task.ID] = true
		}
	}
	for id, seen := range want {
		if !seen {
			t.Errorf("task %q missing from install graph", id)
		}
	}
}

// TestMaintenanceTasksSkipOnDryRun confirms the DryRun gate still
// holds — we don't want `maintain-tmux` to actually rm things when
// the user asked for a plan-only view.
func TestMaintenanceTasksSkipOnDryRun(t *testing.T) {
	bc := newTestBuildConfig(t)
	bc.DryRun = true

	result := BuildInstallTasks(bc)

	for _, task := range result.Tasks {
		if task.ID == "maintain-tmux" || task.ID == "maintain-nvim" {
			t.Errorf("dry-run emitted maintenance task %q", task.ID)
		}
	}
}

// TestMaintainTmuxDependsOnTpmWhenScheduled covers the install race
// fix: maintain-tmux runs install_plugins.sh, which requires TPM
// already on disk. If both tmux and tpm tool tasks are scheduled
// this run, maintain-tmux must depend on BOTH so it never fires
// before the tpm clone completes.
func TestMaintainTmuxDependsOnTpmWhenScheduled(t *testing.T) {
	bc := newTestBuildConfig(t)
	bc.DryRun = false        // enable maintenance task emission
	bc.ForceReinstall = true // force tmux + tpm onto the schedule

	result := BuildInstallTasks(bc)

	var maintainTmux *engine.Task
	for i := range result.Tasks {
		if result.Tasks[i].ID == "maintain-tmux" {
			maintainTmux = &result.Tasks[i]
			break
		}
	}
	if maintainTmux == nil {
		t.Fatal("maintain-tmux task missing from install graph")
	}

	wantDeps := map[string]bool{"tmux": false, "tpm": false}
	for _, dep := range maintainTmux.DependsOn {
		if _, ok := wantDeps[dep]; ok {
			wantDeps[dep] = true
		}
	}
	for dep, seen := range wantDeps {
		if !seen {
			t.Errorf("maintain-tmux missing dep %q (deps = %v)",
				dep, maintainTmux.DependsOn)
		}
	}
}
