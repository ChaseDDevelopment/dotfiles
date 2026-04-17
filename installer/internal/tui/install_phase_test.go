package tui

import (
	"errors"
	"testing"

	"github.com/chaseddevelopment/dotfiles/installer/internal/config"
	"github.com/chaseddevelopment/dotfiles/installer/internal/engine"
)

// installTestApp builds an AppModel ready to receive engine events.
// Before these tests existed, updateInstalling and updateFailurePrompt
// had zero coverage — the dock incident showed how much can hide in
// phase transitions when they're only exercised by hand.
func installTestApp() AppModel {
	cfg := newTestConfig()
	cfg.Failures = config.NewTrackedFailures()
	app := NewApp(cfg)
	app.phase = PhaseInstalling
	app.width = 120
	app.height = 40
	// eventCh stays nil — listenCmd builds a Cmd that blocks on
	// receive, which never runs in-process so nil is fine. Tests
	// that assert on returned Cmds can check != nil.
	return app
}

func TestUpdateInstalling_TaskStartedMarksProgress(t *testing.T) {
	t.Parallel()
	app := installTestApp()
	msg := engine.TaskStartedMsg{ID: "install-eza", Label: "Installing eza"}
	model, cmd := app.Update(msg)
	updated := model.(AppModel)

	if updated.phase != PhaseInstalling {
		t.Errorf("phase = %v, want PhaseInstalling", updated.phase)
	}
	if status, ok := updated.progress.toolStatuses["eza"]; !ok || status != statusActive {
		t.Errorf("progress did not record active task, status=%v ok=%v", status, ok)
	}
	if cmd == nil {
		t.Error("expected listenCmd to be returned so the next event is consumed")
	}
}

func TestUpdateInstalling_NonCriticalDoneStaysInInstall(t *testing.T) {
	t.Parallel()
	app := installTestApp()
	// Seed two tasks: one we're about to finish with error, plus
	// another still active so allFinished() is false and the phase
	// shouldn't auto-transition to summary yet.
	app.progress.markActive("install-eza", "Installing eza")
	app.progress.markActive("install-rg", "Installing rg")
	msg := engine.TaskDoneMsg{
		ID: "install-eza", Label: "Installing eza",
		Err: errors.New("boom"), Critical: false,
	}
	model, _ := app.Update(msg)
	updated := model.(AppModel)

	if updated.phase != PhaseInstalling {
		t.Errorf("non-critical failure should not leave install phase, got %v", updated.phase)
	}
	if len(updated.progress.steps) != 1 || updated.progress.steps[0].success {
		t.Errorf("expected one failed step, got %+v", updated.progress.steps)
	}
}

func TestUpdateInstalling_CriticalDoneShowsPrompt(t *testing.T) {
	t.Parallel()
	app := installTestApp()
	app.progress.markActive("install-nvim", "Installing nvim")
	msg := engine.TaskDoneMsg{
		ID: "install-nvim", Label: "Installing nvim",
		Err: errors.New("apt locked"), Critical: true,
	}
	model, _ := app.Update(msg)
	updated := model.(AppModel)

	if updated.phase != PhaseFailurePrompt {
		t.Errorf("critical failure should transition to PhaseFailurePrompt, got %v", updated.phase)
	}
	if updated.failedTaskErr == nil {
		t.Error("failedTaskErr should be set for the prompt to render")
	}
	if updated.failedTaskLabel != "nvim" {
		t.Errorf("failedTaskLabel = %q, want %q", updated.failedTaskLabel, "nvim")
	}
}

func TestUpdateInstalling_AllDoneTransitionsToSummary(t *testing.T) {
	t.Parallel()
	app := installTestApp()
	app.progress.markActive("install-eza", "Installing eza")
	app.progress.markDone("install-eza", nil)

	model, _ := app.Update(engine.AllDoneMsg{})
	updated := model.(AppModel)

	if updated.phase != PhaseSummary {
		t.Errorf("AllDoneMsg should transition to PhaseSummary, got %v", updated.phase)
	}
	if len(updated.summary.steps) != 1 {
		t.Errorf("summary should capture 1 step, got %d", len(updated.summary.steps))
	}
}

// TestUpdateInstalling_AllFinishedTransitionsWithoutAllDone verifies
// the belt-and-suspenders path: if every tracked task has reached a
// terminal state but the engine hasn't closed the channel yet, the
// TUI still moves on rather than waiting forever.
func TestUpdateInstalling_AllFinishedTransitionsWithoutAllDone(t *testing.T) {
	t.Parallel()
	app := installTestApp()
	app.progress.markActive("install-eza", "Installing eza")
	msg := engine.TaskDoneMsg{
		ID: "install-eza", Label: "Installing eza", Err: nil,
	}
	model, _ := app.Update(msg)
	updated := model.(AppModel)

	if updated.phase != PhaseSummary {
		t.Errorf("completion of all tasks should reach summary, got %v", updated.phase)
	}
}

func TestUpdateInstalling_RepoSyncBlockedAborts(t *testing.T) {
	t.Parallel()
	app := installTestApp()
	msg := repoSyncBlockedMsg{body: "error: Your local changes to the following files would be overwritten by merge: x.txt"}
	model, _ := app.Update(msg)
	updated := model.(AppModel)

	if updated.phase != PhaseSummary {
		t.Errorf("repoSyncBlockedMsg should go straight to summary, got %v", updated.phase)
	}
	if !updated.summary.criticalFailure {
		t.Error("summary.criticalFailure should be true on repo sync block")
	}
	if updated.summary.repoBlockedBody == "" {
		t.Error("repoBlockedBody should carry the git output for display")
	}
}

func TestUpdateFailurePrompt_ContinueReturnsToInstalling(t *testing.T) {
	t.Parallel()
	app := installTestApp()
	app.phase = PhaseFailurePrompt
	// Seed an active (still-running) task so the "continue" branch
	// doesn't short-circuit into Summary on its own.
	app.progress.markActive("install-other", "Installing other")

	model, _ := app.Update(keyPress('c'))
	updated := model.(AppModel)

	if updated.phase != PhaseInstalling {
		t.Errorf("[c] should return to installing phase, got %v", updated.phase)
	}
}

func TestUpdateFailurePrompt_AbortSetsCriticalFailure(t *testing.T) {
	t.Parallel()
	app := installTestApp()
	app.phase = PhaseFailurePrompt
	model, _ := app.Update(keyPress('a'))
	updated := model.(AppModel)

	if updated.phase != PhaseSummary {
		t.Errorf("[a] should transition to summary, got %v", updated.phase)
	}
	if !updated.summary.criticalFailure {
		t.Error("abort from failure prompt must set summary.criticalFailure")
	}
}

// TestUpdateFailurePrompt_AllDoneAutoTransitions covers the specific
// fix for the case where the engine finished while the user was
// still staring at the failure prompt. The previous code left the
// prompt hanging; after the fix we auto-transition to summary so
// nothing silently lingers.
func TestUpdateFailurePrompt_AllDoneAutoTransitions(t *testing.T) {
	t.Parallel()
	app := installTestApp()
	app.phase = PhaseFailurePrompt
	// A task that ran and succeeded after the failed one.
	app.progress.markActive("install-other", "Installing other")
	app.progress.markDone("install-other", nil)

	model, cmd := app.Update(engine.AllDoneMsg{})
	updated := model.(AppModel)

	if updated.phase != PhaseSummary {
		t.Errorf("AllDoneMsg in failure prompt should auto-transition, got %v", updated.phase)
	}
	if cmd == nil {
		t.Error("expected drainCmd to be returned so the channel is fully consumed")
	}
}
