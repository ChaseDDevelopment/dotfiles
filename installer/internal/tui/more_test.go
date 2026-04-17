package tui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/chaseddevelopment/dotfiles/installer/internal/config"
	"github.com/chaseddevelopment/dotfiles/installer/internal/engine"
	"github.com/chaseddevelopment/dotfiles/installer/internal/executor"
	"github.com/chaseddevelopment/dotfiles/installer/internal/pkgmgr"
	"github.com/chaseddevelopment/dotfiles/installer/internal/state"
)

func TestSummaryHelpersAndBodyScope(t *testing.T) {
	if got := formatDuration(500 * time.Microsecond); got != "<1ms" {
		t.Fatalf("formatDuration(<1ms) = %q", got)
	}
	if got := formatDuration(250 * time.Millisecond); got != "250ms" {
		t.Fatalf("formatDuration(ms) = %q", got)
	}
	if got := formatDuration(1500 * time.Millisecond); !strings.Contains(got, "1.5s") {
		t.Fatalf("formatDuration(seconds) = %q", got)
	}
	if got := formatDuration(65 * time.Second); got != "1m5s" {
		t.Fatalf("formatDuration(minutes) = %q", got)
	}

	body := "error\n\tconfigs/zsh/.zshrc\n\tconfigs/tmux/tmux.conf\n"
	if !bodyDriftInScope(body, []string{"configs/zsh/.zshrc", "configs/tmux/tmux.conf"}) {
		t.Fatal("expected body drift to be in scope")
	}
	if bodyDriftInScope(body, []string{"configs/zsh/.zshrc"}) {
		t.Fatal("unexpected in-scope result when a path is missing")
	}
}

func TestCriticalFailureAndSaveStateError(t *testing.T) {
	app := NewApp(newTestConfig())
	app.summary.criticalFailure = true
	if !app.CriticalFailure() {
		t.Fatal("expected CriticalFailure true")
	}

	dir := t.TempDir()
	log, err := executor.NewLogFile(filepath.Join(dir, "test.log"))
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	app.config.Runner = executor.NewRunner(log, false)
	parentFile := filepath.Join(dir, "not-a-dir")
	if err := os.WriteFile(parentFile, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	app.config.State = state.NewStore(filepath.Join(parentFile, "state.json"))
	app.saveState()
	data, err := os.ReadFile(log.Path())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "WARNING: save state") {
		t.Fatalf("expected save-state warning in log:\n%s", data)
	}
}

func TestDpkgPreflightAndRepairFlow(t *testing.T) {
	dir := t.TempDir()
	log, err := executor.NewLogFile(filepath.Join(dir, "test.log"))
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	runner := executor.NewRunner(log, false)
	fakebin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(fakebin, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fakebin+string(os.PathListSeparator)+os.Getenv("PATH"))
	stateFile := filepath.Join(dir, "audit-state")
	if err := os.WriteFile(filepath.Join(fakebin, "dpkg"), []byte(`#!/bin/sh
if [ "$1" = "--audit" ]; then
  if [ -f "`+stateFile+`" ]; then
    exit 0
  fi
  printf 'broken packages'
  exit 1
fi
exit 0
`), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fakebin, "sudo"), []byte(`#!/bin/sh
if [ "$1" = "dpkg" ] && [ "$2" = "--configure" ] && [ "$3" = "-a" ]; then
  : > "`+stateFile+`"
fi
exit 0
`), 0o755); err != nil {
		t.Fatal(err)
	}

	app := NewApp(newTestConfig())
	app.config.Runner = runner
	app.config.PkgMgr = pkgmgr.NewApt(runner, false)
	if _, blocked := app.preflightDpkgHealth(); !blocked {
		t.Fatal("expected unhealthy dpkg state to block engine")
	}
	if app.phase != PhaseDpkgRepair {
		t.Fatalf("phase = %v, want PhaseDpkgRepair", app.phase)
	}
	view := app.dpkgRepairView(100)
	if !strings.Contains(view, "dpkg state inconsistent") {
		t.Fatalf("unexpected dpkg repair view: %s", view)
	}

	model, _ := app.updateDpkgRepair(tea.KeyPressMsg{Code: 's'})
	var updated AppModel
	switch mm := model.(type) {
	case *AppModel:
		updated = *mm
	case AppModel:
		updated = mm
	default:
		t.Fatalf("unexpected model type %T", model)
	}
	if updated.dpkgApt == nil || updated.dpkgApt.UserApprovedRepair {
		t.Fatalf("skip repair should leave approval false")
	}

	app = NewApp(newTestConfig())
	app.config.Runner = runner
	app.config.PkgMgr = pkgmgr.NewApt(runner, false)
	if _, blocked := app.preflightDpkgHealth(); !blocked {
		t.Fatal("expected unhealthy dpkg state to block engine")
	}
	model, _ = app.updateDpkgRepair(tea.KeyPressMsg{Code: 'a'})
	switch mm := model.(type) {
	case *AppModel:
		updated = *mm
	case AppModel:
		updated = mm
	default:
		t.Fatalf("unexpected model type %T", model)
	}
	if updated.phase != PhaseSummary || !updated.summary.criticalFailure {
		t.Fatalf("abort repair should go to critical summary, got phase=%v critical=%v", updated.phase, updated.summary.criticalFailure)
	}
}

func TestRunInstallTasksAndFailurePromptHelpers(t *testing.T) {
	app := NewApp(newTestConfig())
	app.summary.endTime = time.Now()
	app.phase = PhaseFailurePrompt
	model, _ := app.updateFailurePrompt(tea.KeyPressMsg{Code: 'a'})
	updated := model.(AppModel)
	if updated.phase != PhaseSummary || !updated.summary.criticalFailure {
		t.Fatalf("abort from failure prompt should mark critical failure")
	}

	tf := config.NewTrackedFailures()
	tf.Record("Zsh", "compile", errors.New("boom"))
	app.summary.warnings = tf
	app.failedTaskLabel = "Zsh"
	app.failedTaskErr = errors.New("critical failure: compile died")
	view := app.failurePromptView(80)
	// The rendered prompt must surface (a) the failed task label,
	// (b) the underlying error text, and (c) the remediation hint
	// listing the [c]ontinue / [a]bort options. Any missing piece
	// would mislead the user on a live failure.
	if !strings.Contains(view, "failed") {
		t.Fatalf("failurePromptView missing 'failed': %s", view)
	}
	if !strings.Contains(view, "Zsh") {
		t.Fatalf("failurePromptView missing task label 'Zsh': %s", view)
	}
	if !strings.Contains(view, "compile died") {
		t.Fatalf(
			"failurePromptView missing error detail 'compile died': %s",
			view,
		)
	}
	if !strings.Contains(view, "Continue without it") {
		t.Fatalf(
			"failurePromptView missing [c] remediation hint: %s", view,
		)
	}
	if !strings.Contains(view, "Abort install") {
		t.Fatalf(
			"failurePromptView missing [a] remediation hint: %s", view,
		)
	}

	app = NewApp(newTestConfig())
	app.config.Mode = ModeDoctor
	app.progress.steps = []stepResult{{label: "zsh", success: true}}
	app.progress.done = true
	app.progress.doneCount = 1
	model, _ = app.updateInstalling(engine.AllDoneMsg{})
	updated = model.(AppModel)
	if updated.phase != PhaseSummary {
		t.Fatalf("AllDone should transition to summary, got %v", updated.phase)
	}
}
