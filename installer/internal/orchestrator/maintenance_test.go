package orchestrator

import (
	"testing"
	"time"

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

	want := map[string]bool{
		"maintain-tmux":     false,
		"maintain-nvim":     false,
		"ensure-zsh-login":  false,
	}
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
		switch task.ID {
		case "maintain-tmux", "maintain-nvim", "ensure-zsh-login":
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

// TestSetupNeovimOrderedAfterToolchainsWhenScheduled covers the
// build-race fix: the nvim headless plugin sync compiles treesitter
// parsers (needs the tree-sitter CLI) and blink.cmp's Rust matcher
// (needs cargo). When those toolchains are installed this run, the
// setup must be ordered AFTER them — via a soft `After` edge, so a
// toolchain install failure orders but never skips the nvim setup.
func TestSetupNeovimOrderedAfterToolchainsWhenScheduled(t *testing.T) {
	bc := newTestBuildConfig(t)
	bc.DryRun = false
	bc.ForceReinstall = true // force tree-sitter + cargo onto the schedule

	result := BuildInstallTasks(bc)

	var setupNvim *engine.Task
	for i := range result.Tasks {
		if result.Tasks[i].ID == "setup-Neovim" {
			setupNvim = &result.Tasks[i]
			break
		}
	}
	if setupNvim == nil {
		t.Fatal("setup-Neovim task missing from install graph")
	}

	wantAfter := map[string]bool{"tree-sitter": false, "cargo": false}
	for _, dep := range setupNvim.After {
		if _, ok := wantAfter[dep]; ok {
			wantAfter[dep] = true
		}
	}
	for dep, seen := range wantAfter {
		if !seen {
			t.Errorf("setup-Neovim missing soft-order dep %q (After = %v)",
				dep, setupNvim.After)
		}
	}
}

// TestEnsureZshLoginDependsOnZshWhenScheduled covers the pluto
// regression: chsh was buried inside setupZsh, which skips when
// Zsh symlinks are already correct. Lifting it into a dedicated
// `ensure-zsh-login` task means it runs every install — but it
// still needs to wait for the zsh binary task when zsh is being
// installed this run, otherwise chsh may fire before the binary
// exists.
func TestEnsureZshLoginDependsOnZshWhenScheduled(t *testing.T) {
	bc := newTestBuildConfig(t)
	bc.DryRun = false
	bc.ForceReinstall = true // force zsh onto the schedule

	result := BuildInstallTasks(bc)

	var ensure *engine.Task
	for i := range result.Tasks {
		if result.Tasks[i].ID == "ensure-zsh-login" {
			ensure = &result.Tasks[i]
			break
		}
	}
	if ensure == nil {
		t.Fatal("ensure-zsh-login task missing from install graph")
	}

	foundZshDep := false
	for _, dep := range ensure.DependsOn {
		if dep == "zsh" {
			foundZshDep = true
			break
		}
	}
	if !foundZshDep {
		t.Errorf("ensure-zsh-login missing zsh dep (deps = %v)",
			ensure.DependsOn)
	}
}

// TestMaintainNeovimTimeoutAndOrdering covers the bootstrap move: the heavy
// headless build (clone ~40 repos + compile treesitter parsers + Rust matcher)
// now lives in the always-run maintain-nvim task, so it must (a) carry a long
// Timeout to survive multi-minute cold builds rather than the engine's 10-min
// default, and (b) soft-order AFTER the build toolchain when it's scheduled.
func TestMaintainNeovimTimeoutAndOrdering(t *testing.T) {
	bc := newTestBuildConfig(t)
	bc.DryRun = false
	bc.ForceReinstall = true // force tree-sitter + cargo onto the schedule

	result := BuildInstallTasks(bc)

	var maintainNvim *engine.Task
	for i := range result.Tasks {
		if result.Tasks[i].ID == "maintain-nvim" {
			maintainNvim = &result.Tasks[i]
			break
		}
	}
	if maintainNvim == nil {
		t.Fatal("maintain-nvim task missing from install graph")
	}

	if maintainNvim.Timeout < 20*time.Minute {
		t.Errorf("maintain-nvim Timeout = %v, want >= 20m for cold builds",
			maintainNvim.Timeout)
	}

	wantAfter := map[string]bool{"tree-sitter": false, "cargo": false}
	for _, dep := range maintainNvim.After {
		if _, ok := wantAfter[dep]; ok {
			wantAfter[dep] = true
		}
	}
	for dep, seen := range wantAfter {
		if !seen {
			t.Errorf("maintain-nvim missing soft-order dep %q (After = %v)",
				dep, maintainNvim.After)
		}
	}

	// Its nvim bootstrap compiles the blink matcher via cargo, so it must
	// hold ResCargo to never overlap `rustup update`.
	if !hasResource(maintainNvim.Resources, engine.ResCargo) {
		t.Errorf("maintain-nvim missing ResCargo (Resources = %v)",
			maintainNvim.Resources)
	}
}

// TestCargoWorkSerializedOnResCargo guards the toolchain-corruption fix: every
// task that runs rustup/cargo/rustc must hold the single ResCargo lock so
// `rustup update` can't rewrite the shared toolchain under a running compile.
func TestCargoWorkSerializedOnResCargo(t *testing.T) {
	t.Parallel()
	bc := newTestBuildConfig(t)
	bc.DryRun = false
	bc.ForceReinstall = true // schedule rust + the cargo-update pass

	result := BuildInstallTasks(bc)
	byID := map[string]*engine.Task{}
	for i := range result.Tasks {
		byID[result.Tasks[i].ID] = &result.Tasks[i]
	}

	// Folded update-pass steps that run `rustup update` / `cargo install`.
	for _, id := range []string{"update-Rust toolchain", "update-Cargo binaries"} {
		tk := byID[id]
		if tk == nil {
			t.Fatalf("%s task missing from install graph", id)
		}
		if !hasResource(tk.Resources, engine.ResCargo) {
			t.Errorf("%s missing ResCargo (Resources = %v)", id, tk.Resources)
		}
	}

	// The rust toolchain install (Command "cargo", MethodScript rustup.sh
	// with AcquiresCargo) must lock cargo too.
	if tk := byID["cargo"]; tk == nil {
		t.Fatal("rust install task (cargo) missing from install graph")
	} else if !hasResource(tk.Resources, engine.ResCargo) {
		t.Errorf("rust install task missing ResCargo (Resources = %v)", tk.Resources)
	}
}
