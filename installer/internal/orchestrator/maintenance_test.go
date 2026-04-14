package orchestrator

import (
	"testing"
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
