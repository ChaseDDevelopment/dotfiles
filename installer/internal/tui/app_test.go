package tui

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/chaseddevelopment/dotfiles/installer/internal/backup"
	"github.com/chaseddevelopment/dotfiles/installer/internal/engine"
	"github.com/chaseddevelopment/dotfiles/installer/internal/executor"
	"github.com/chaseddevelopment/dotfiles/installer/internal/orchestrator"
	"github.com/chaseddevelopment/dotfiles/installer/internal/platform"
)

func newTestConfig() *AppConfig {
	return &AppConfig{
		Platform: &platform.Platform{
			OS:     platform.MacOS,
			Arch:   platform.ARM64,
			OSName: "macOS",
		},
	}
}

func TestNewApp(t *testing.T) {
	t.Parallel()
	cfg := newTestConfig()
	app := NewApp(cfg)

	if app.phase != PhaseMainMenu {
		t.Errorf("phase = %v, want PhaseMainMenu", app.phase)
	}
	if app.config != cfg {
		t.Error("config not set correctly")
	}
	if app.quitting {
		t.Error("quitting should be false initially")
	}
}

func TestAppModel_Init(t *testing.T) {
	t.Parallel()
	app := NewApp(newTestConfig())
	cmd := app.Init()
	if cmd != nil {
		t.Error("Init() should return nil (no initial command)")
	}
}

func TestAppModel_ViewMainMenu(t *testing.T) {
	t.Parallel()
	app := NewApp(newTestConfig())
	app.width = 80
	app.height = 40

	v := app.View()
	if v.Content == "" {
		t.Error("View() returned empty content in main menu phase")
	}
}

func TestAppModel_ViewDefaultWidth(t *testing.T) {
	t.Parallel()
	app := NewApp(newTestConfig())
	// width=0 should default to 80.
	v := app.View()
	if v.Content == "" {
		t.Error("View() returned empty content with default width")
	}
}

func TestAppModel_ViewQuitting(t *testing.T) {
	t.Parallel()
	app := NewApp(newTestConfig())
	app.quitting = true

	v := app.View()
	if v.Content != "" {
		t.Error("View() should return empty content when quitting")
	}
}

func TestAppModel_WindowSizeMsg(t *testing.T) {
	t.Parallel()
	app := NewApp(newTestConfig())

	msg := tea.WindowSizeMsg{Width: 120, Height: 50}
	model, _ := app.Update(msg)
	updated := model.(AppModel)

	if updated.width != 120 {
		t.Errorf("width = %d, want 120", updated.width)
	}
	if updated.height != 50 {
		t.Errorf("height = %d, want 50", updated.height)
	}
}

func TestAppModel_QuitWithQ(t *testing.T) {
	t.Parallel()
	app := NewApp(newTestConfig())
	app.phase = PhaseMainMenu

	msg := keyPress('q')
	model, cmd := app.Update(msg)
	updated := model.(AppModel)

	if !updated.quitting {
		t.Error("q should set quitting to true in main menu")
	}
	if cmd == nil {
		t.Error("q should return a quit command")
	}
}

func TestAppModel_QuitWithCtrlC(t *testing.T) {
	t.Parallel()
	app := NewApp(newTestConfig())
	app.phase = PhaseMainMenu

	msg := tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	model, cmd := app.Update(msg)
	updated := model.(AppModel)

	if !updated.quitting {
		t.Error("ctrl+c should set quitting to true")
	}
	if cmd == nil {
		t.Error("ctrl+c should return a quit command")
	}
}

func TestAppModel_QuitBlockedDuringInstall(t *testing.T) {
	t.Parallel()
	app := NewApp(newTestConfig())
	app.phase = PhaseInstalling

	msg := keyPress('q')
	model, _ := app.Update(msg)
	updated := model.(AppModel)

	if updated.quitting {
		t.Error("q should not quit during install phase")
	}
}

func TestAppModel_MainMenuNavigation(t *testing.T) {
	t.Parallel()
	app := NewApp(newTestConfig())
	app.phase = PhaseMainMenu

	// Navigate down.
	model, _ := app.Update(keyPress('j'))
	updated := model.(AppModel)

	if updated.mainMenu.cursor != 1 {
		t.Errorf("cursor = %d, want 1", updated.mainMenu.cursor)
	}
}

func TestAppModel_MainMenuSelectExit(t *testing.T) {
	t.Parallel()
	app := NewApp(newTestConfig())
	app.phase = PhaseMainMenu

	// Navigate to Exit (last item).
	for i := 0; i < len(app.mainMenu.items)-1; i++ {
		model, _ := app.Update(keyPress('j'))
		app = model.(AppModel)
	}

	// Press enter.
	model, cmd := app.Update(specialKeyPress(tea.KeyEnter))
	updated := model.(AppModel)

	if !updated.quitting {
		t.Error("selecting Exit should set quitting to true")
	}
	if cmd == nil {
		t.Error("selecting Exit should return quit command")
	}
}

func TestAppModel_MainMenuSelectInstall(t *testing.T) {
	t.Parallel()
	app := NewApp(newTestConfig())
	app.phase = PhaseMainMenu

	// Install is the first item, press enter.
	model, _ := app.Update(specialKeyPress(tea.KeyEnter))
	updated := model.(AppModel)

	if updated.phase != PhaseOptionsMenu {
		t.Errorf(
			"phase = %v, want PhaseOptionsMenu",
			updated.phase,
		)
	}
}

func TestAppModel_OptionsMenuEscGoesBack(t *testing.T) {
	t.Parallel()
	app := NewApp(newTestConfig())
	app.phase = PhaseOptionsMenu

	model, _ := app.Update(specialKeyPress(tea.KeyEscape))
	updated := model.(AppModel)

	if updated.phase != PhaseMainMenu {
		t.Errorf(
			"phase = %v, want PhaseMainMenu after esc",
			updated.phase,
		)
	}
}

func TestAppModel_ComponentPickerEscGoesBack(t *testing.T) {
	t.Parallel()

	t.Run("custom install goes to options", func(t *testing.T) {
		t.Parallel()
		app := NewApp(newTestConfig())
		app.phase = PhaseComponentPicker
		app.config.Mode = ModeCustomInstall

		model, _ := app.Update(specialKeyPress(tea.KeyEscape))
		updated := model.(AppModel)

		if updated.phase != PhaseOptionsMenu {
			t.Errorf(
				"phase = %v, want PhaseOptionsMenu",
				updated.phase,
			)
		}
	})

	t.Run("uninstall goes to main menu", func(t *testing.T) {
		t.Parallel()
		app := NewApp(newTestConfig())
		app.phase = PhaseComponentPicker
		app.config.Mode = ModeUninstall

		model, _ := app.Update(specialKeyPress(tea.KeyEscape))
		updated := model.(AppModel)

		if updated.phase != PhaseMainMenu {
			t.Errorf(
				"phase = %v, want PhaseMainMenu",
				updated.phase,
			)
		}
	})
}

func TestAppModel_BackupPickerEscGoesBack(t *testing.T) {
	t.Parallel()
	app := NewApp(newTestConfig())
	app.phase = PhaseBackupPicker

	model, _ := app.Update(specialKeyPress(tea.KeyEscape))
	updated := model.(AppModel)

	if updated.phase != PhaseMainMenu {
		t.Errorf(
			"phase = %v, want PhaseMainMenu",
			updated.phase,
		)
	}
}

func TestAppModel_SummaryEnterReturnsToMenu(t *testing.T) {
	t.Parallel()
	app := NewApp(newTestConfig())
	app.phase = PhaseSummary

	model, _ := app.Update(specialKeyPress(tea.KeyEnter))
	updated := model.(AppModel)

	if updated.phase != PhaseMainMenu {
		t.Errorf(
			"phase = %v, want PhaseMainMenu",
			updated.phase,
		)
	}
}

func TestAppModel_SummaryQQuits(t *testing.T) {
	t.Parallel()
	app := NewApp(newTestConfig())
	app.phase = PhaseSummary

	model, cmd := app.Update(keyPress('q'))
	updated := model.(AppModel)

	if !updated.quitting {
		t.Error("q in summary should quit")
	}
	if cmd == nil {
		t.Error("q in summary should return quit command")
	}
}

func TestAppModel_ViewAllPhases(t *testing.T) {
	t.Parallel()
	phases := []Phase{
		PhaseMainMenu,
		PhaseOptionsMenu,
		PhaseComponentPicker,
		PhaseBackupPicker,
		PhaseInstalling,
		PhaseSummary,
	}

	for _, phase := range phases {
		app := NewApp(newTestConfig())
		app.phase = phase
		app.width = 80
		app.height = 40
		app.failedTaskLabel = "test-tool"
		app.failedTaskErr = nil

		v := app.View()
		if v.Content == "" {
			t.Errorf(
				"View() returned empty for phase %v", phase,
			)
		}
	}
}

func TestAppModel_FailurePromptView(t *testing.T) {
	t.Parallel()
	app := NewApp(newTestConfig())
	app.phase = PhaseFailurePrompt
	app.width = 80
	app.height = 40
	app.failedTaskLabel = "brew"
	app.failedTaskErr = errTestFailure

	v := app.View()
	if v.Content == "" {
		t.Error("failurePromptView returned empty")
	}
}

// errTestFailure is a package-level error for test assertions.
var errTestFailure = testError("critical failure")

type testError string

func (e testError) Error() string { return string(e) }

func TestPhaseConstants(t *testing.T) {
	t.Parallel()

	// Verify each phase has a unique value.
	phases := []Phase{
		PhaseMainMenu, PhaseOptionsMenu,
		PhaseComponentPicker, PhaseBackupPicker,
		PhaseInstalling, PhaseFailurePrompt,
		PhaseSummary,
	}
	seen := make(map[Phase]bool)
	for _, p := range phases {
		if seen[p] {
			t.Errorf("duplicate phase value: %v", p)
		}
		seen[p] = true
	}
}

func TestInstallModeConstants(t *testing.T) {
	t.Parallel()

	// Verify each mode has a unique value.
	modes := []InstallMode{
		ModeInstall, ModeCustomInstall, ModeDryRun,
		ModeUpdate, ModeRestore, ModeDoctor,
		ModeUninstall, ModeExit,
	}
	seen := make(map[InstallMode]bool)
	for _, m := range modes {
		if seen[m] {
			t.Errorf("duplicate mode value: %v", m)
		}
		seen[m] = true
	}
}

func TestAppModel_MainMenuSelectDryRun(t *testing.T) {
	t.Parallel()
	app := NewApp(newTestConfig())
	app.phase = PhaseMainMenu

	// Navigate to Dry Run (index 2).
	for i := 0; i < 2; i++ {
		model, _ := app.Update(keyPress('j'))
		app = model.(AppModel)
	}

	model, _ := app.Update(specialKeyPress(tea.KeyEnter))
	updated := model.(AppModel)

	if !updated.config.DryRun {
		t.Error("DryRun should be true after selecting Dry Run")
	}
	if updated.phase != PhaseOptionsMenu {
		t.Errorf(
			"phase = %v, want PhaseOptionsMenu",
			updated.phase,
		)
	}
}

func TestAppModel_MainMenuSelectCustomInstall(t *testing.T) {
	t.Parallel()
	app := NewApp(newTestConfig())
	app.phase = PhaseMainMenu

	// Navigate to Custom Install (index 1).
	model, _ := app.Update(keyPress('j'))
	app = model.(AppModel)

	model, _ = app.Update(specialKeyPress(tea.KeyEnter))
	updated := model.(AppModel)

	if updated.phase != PhaseOptionsMenu {
		t.Errorf(
			"phase = %v, want PhaseOptionsMenu",
			updated.phase,
		)
	}
}

func TestAppModel_MainMenuSelectRestore(t *testing.T) {
	t.Parallel()
	app := NewApp(newTestConfig())
	app.phase = PhaseMainMenu

	// Navigate to Restore (index 4).
	for i := 0; i < 4; i++ {
		model, _ := app.Update(keyPress('j'))
		app = model.(AppModel)
	}

	model, _ := app.Update(specialKeyPress(tea.KeyEnter))
	updated := model.(AppModel)

	if updated.phase != PhaseBackupPicker {
		t.Errorf(
			"phase = %v, want PhaseBackupPicker",
			updated.phase,
		)
	}
}

func TestAppModel_MainMenuSelectUninstall(t *testing.T) {
	t.Parallel()
	app := NewApp(newTestConfig())
	app.phase = PhaseMainMenu

	// Navigate to Uninstall (index 6).
	for i := 0; i < 6; i++ {
		model, _ := app.Update(keyPress('j'))
		app = model.(AppModel)
	}

	model, _ := app.Update(specialKeyPress(tea.KeyEnter))
	updated := model.(AppModel)

	if updated.phase != PhaseComponentPicker {
		t.Errorf(
			"phase = %v, want PhaseComponentPicker",
			updated.phase,
		)
	}
}

func TestAppModel_ComponentPickerEnterNoSelection(t *testing.T) {
	t.Parallel()
	app := NewApp(newTestConfig())
	app.phase = PhaseComponentPicker

	// Enter with no selections should stay on component picker.
	model, _ := app.Update(specialKeyPress(tea.KeyEnter))
	updated := model.(AppModel)

	if updated.phase != PhaseComponentPicker {
		t.Errorf(
			"phase = %v, want PhaseComponentPicker (no selection)",
			updated.phase,
		)
	}
}

func TestAppModel_BackupPickerEnterNoSelection(t *testing.T) {
	t.Parallel()
	app := NewApp(newTestConfig())
	app.phase = PhaseBackupPicker
	app.backupPicker = backupPickerModel{} // empty

	model, _ := app.Update(specialKeyPress(tea.KeyEnter))
	updated := model.(AppModel)

	if updated.phase != PhaseBackupPicker {
		t.Errorf(
			"phase = %v, want PhaseBackupPicker (no backup selected)",
			updated.phase,
		)
	}
}

func TestAppModel_SummaryEscReturnsToMenu(t *testing.T) {
	t.Parallel()
	app := NewApp(newTestConfig())
	app.phase = PhaseSummary

	model, _ := app.Update(specialKeyPress(tea.KeyEscape))
	updated := model.(AppModel)

	if updated.phase != PhaseMainMenu {
		t.Errorf(
			"phase = %v, want PhaseMainMenu after esc",
			updated.phase,
		)
	}
}

func TestAppModel_SummaryWindowResize(t *testing.T) {
	t.Parallel()
	app := NewApp(newTestConfig())
	app.phase = PhaseSummary

	msg := tea.WindowSizeMsg{Width: 120, Height: 50}
	model, _ := app.Update(msg)
	updated := model.(AppModel)

	if updated.width != 120 {
		t.Errorf("width = %d, want 120", updated.width)
	}
}

func TestAppModel_SummaryWindowResizeDryRun(t *testing.T) {
	t.Parallel()
	app := NewApp(newTestConfig())
	app.phase = PhaseSummary
	app.summary.dryRun = true

	msg := tea.WindowSizeMsg{Width: 120, Height: 50}
	model, _ := app.Update(msg)
	updated := model.(AppModel)

	if updated.width != 120 {
		t.Errorf("width = %d, want 120", updated.width)
	}
	if !updated.summary.viewportReady {
		t.Error("viewportReady should be true after resize")
	}
}

func TestAppModel_SummaryWindowResizeDoctor(t *testing.T) {
	t.Parallel()
	app := NewApp(newTestConfig())
	app.phase = PhaseSummary
	app.summary.doctorMode = true
	// Add enough steps to trigger viewport.
	for i := 0; i < 50; i++ {
		app.summary.steps = append(app.summary.steps, stepResult{
			label: "tool", action: "install",
			status: "installed", success: true,
		})
	}

	msg := tea.WindowSizeMsg{Width: 80, Height: 30}
	model, _ := app.Update(msg)
	updated := model.(AppModel)

	if updated.width != 80 {
		t.Errorf("width = %d, want 80", updated.width)
	}
}

func TestReturnToMainMenu(t *testing.T) {
	t.Parallel()
	app := NewApp(newTestConfig())
	app.phase = PhaseSummary
	app.config.DryRun = true
	app.config.SelectedComponents = []string{"zsh", "tmux"}

	app.returnToMainMenu()

	if app.phase != PhaseMainMenu {
		t.Errorf("phase = %v, want PhaseMainMenu", app.phase)
	}
	if app.config.DryRun {
		t.Error("DryRun should be reset to false")
	}
	if app.config.SelectedComponents != nil {
		t.Error("SelectedComponents should be nil")
	}
	if app.cancelEngine != nil {
		t.Error("cancelEngine should be nil")
	}
	if app.eventCh != nil {
		t.Error("eventCh should be nil")
	}
}

// --------------------------------------------------------------------------
// Engine event handling tests
// --------------------------------------------------------------------------

func TestAppModel_UpdateInstalling_TaskStarted(t *testing.T) {
	t.Parallel()
	app := NewApp(newTestConfig())
	app.phase = PhaseInstalling
	ch := make(chan any, 10)
	app.eventCh = ch

	msg := engine.TaskStartedMsg{ID: "install-zsh", Label: "Installing zsh"}
	model, _ := app.Update(msg)
	updated := model.(AppModel)

	if len(updated.progress.active) != 1 {
		t.Errorf(
			"active = %d, want 1",
			len(updated.progress.active),
		)
	}
}

func TestAppModel_UpdateInstalling_TaskDoneSuccess(t *testing.T) {
	t.Parallel()
	app := NewApp(newTestConfig())
	app.phase = PhaseInstalling
	ch := make(chan any, 10)
	app.eventCh = ch

	// Start a task first.
	app.progress.markActive("install-zsh", "Installing zsh")

	msg := engine.TaskDoneMsg{
		ID: "install-zsh", Label: "Installing zsh",
	}
	model, _ := app.Update(msg)
	updated := model.(AppModel)

	// With one task done and all finished, should go to summary.
	if updated.phase != PhaseSummary {
		t.Errorf(
			"phase = %v, want PhaseSummary",
			updated.phase,
		)
	}
}

func TestAppModel_UpdateInstalling_TaskDoneCriticalFailure(t *testing.T) {
	t.Parallel()
	app := NewApp(newTestConfig())
	app.phase = PhaseInstalling
	ch := make(chan any, 10)
	app.eventCh = ch

	app.progress.markActive("install-brew", "Installing brew")
	app.progress.markActive("install-zsh", "Installing zsh")

	msg := engine.TaskDoneMsg{
		ID: "install-brew", Label: "Installing brew",
		Err: errors.New("brew failed"), Critical: true,
	}
	model, _ := app.Update(msg)
	updated := model.(AppModel)

	if updated.phase != PhaseFailurePrompt {
		t.Errorf(
			"phase = %v, want PhaseFailurePrompt",
			updated.phase,
		)
	}
	if updated.failedTaskLabel == "" {
		t.Error("failedTaskLabel should be set")
	}
}

func TestAppModel_UpdateInstalling_TaskSkipped(t *testing.T) {
	t.Parallel()
	app := NewApp(newTestConfig())
	app.phase = PhaseInstalling
	ch := make(chan any, 10)
	app.eventCh = ch

	msg := engine.TaskSkippedMsg{
		ID: "install-yay", Label: "Installing yay",
		Reason: "not Arch Linux",
	}
	model, _ := app.Update(msg)
	updated := model.(AppModel)

	if updated.progress.doneCount != 1 {
		t.Errorf(
			"doneCount = %d, want 1",
			updated.progress.doneCount,
		)
	}
}

func TestAppModel_UpdateInstalling_AllDone(t *testing.T) {
	t.Parallel()
	app := NewApp(newTestConfig())
	app.phase = PhaseInstalling

	msg := engine.AllDoneMsg{}
	model, _ := app.Update(msg)
	updated := model.(AppModel)

	if updated.phase != PhaseSummary {
		t.Errorf(
			"phase = %v, want PhaseSummary",
			updated.phase,
		)
	}
}

func TestAppModel_UpdateFailurePrompt_Continue(t *testing.T) {
	t.Parallel()
	app := NewApp(newTestConfig())
	app.phase = PhaseFailurePrompt
	ch := make(chan any, 10)
	app.eventCh = ch
	app.failedTaskLabel = "brew"
	app.failedTaskErr = errors.New("failed")

	// Mark a tool active so allFinished returns false.
	app.progress.markActive("install-zsh", "Installing zsh")

	msg := keyPress('c')
	model, _ := app.Update(msg)
	updated := model.(AppModel)

	if updated.phase != PhaseInstalling {
		t.Errorf(
			"phase = %v, want PhaseInstalling after continue",
			updated.phase,
		)
	}
}

func TestAppModel_UpdateFailurePrompt_ContinueAllDone(t *testing.T) {
	t.Parallel()
	app := NewApp(newTestConfig())
	app.phase = PhaseFailurePrompt
	ch := make(chan any, 10)
	app.eventCh = ch
	app.failedTaskLabel = "brew"
	app.failedTaskErr = errors.New("failed")

	// Only one task which already failed, so allFinished returns true.
	app.progress.markActive("install-brew", "Installing brew")
	app.progress.markDone("install-brew", errors.New("failed"))

	msg := keyPress('s')
	model, _ := app.Update(msg)
	updated := model.(AppModel)

	if updated.phase != PhaseSummary {
		t.Errorf(
			"phase = %v, want PhaseSummary (all done)",
			updated.phase,
		)
	}
}

func TestAppModel_UpdateFailurePrompt_Abort(t *testing.T) {
	t.Parallel()
	app := NewApp(newTestConfig())
	app.phase = PhaseFailurePrompt
	ch := make(chan any, 10)
	app.eventCh = ch
	app.failedTaskLabel = "brew"
	app.failedTaskErr = errors.New("failed")

	msg := keyPress('a')
	model, _ := app.Update(msg)
	updated := model.(AppModel)

	if updated.phase != PhaseSummary {
		t.Errorf(
			"phase = %v, want PhaseSummary after abort",
			updated.phase,
		)
	}
	if !updated.summary.criticalFailure {
		t.Error("criticalFailure should be true after abort")
	}
}

func TestAppModel_UpdateFailurePrompt_EngineEvents(t *testing.T) {
	t.Parallel()
	app := NewApp(newTestConfig())
	app.phase = PhaseFailurePrompt
	ch := make(chan any, 10)
	app.eventCh = ch

	// Engine events should be processed while showing prompt.
	startMsg := engine.TaskStartedMsg{
		ID: "install-zsh", Label: "Installing zsh",
	}
	model, _ := app.Update(startMsg)
	updated := model.(AppModel)

	if len(updated.progress.active) != 1 {
		t.Error("task started event should be processed")
	}

	doneMsg := engine.TaskDoneMsg{
		ID: "install-zsh", Label: "Installing zsh",
	}
	model, _ = updated.Update(doneMsg)
	updated = model.(AppModel)

	if updated.progress.doneCount != 1 {
		t.Error("task done event should be processed")
	}

	skipMsg := engine.TaskSkippedMsg{
		ID: "install-yay", Label: "Installing yay",
		Reason: "skipped",
	}
	model, _ = updated.Update(skipMsg)
	updated = model.(AppModel)

	if updated.progress.doneCount != 2 {
		t.Error("task skipped event should be processed")
	}

	allDoneMsg := engine.AllDoneMsg{}
	model, _ = updated.Update(allDoneMsg)
	updated = model.(AppModel)

	if !updated.progress.done {
		t.Error("AllDoneMsg should set progress.done")
	}
}

func TestAppModel_SyncRepoNilRunner(t *testing.T) {
	t.Parallel()
	app := NewApp(newTestConfig())
	app.config.Runner = nil
	// Should not panic.
	app.syncRepo()
}

func TestAppModel_SyncRepoEmptyRootDir(t *testing.T) {
	t.Parallel()
	app := NewApp(newTestConfig())
	app.config.RootDir = ""
	// Should not panic.
	app.syncRepo()
}

func TestAppModel_SaveStateNilState(t *testing.T) {
	t.Parallel()
	app := NewApp(newTestConfig())
	app.config.State = nil
	// Should not panic.
	app.saveState()
}

func TestAppModel_BuildConfig(t *testing.T) {
	t.Parallel()
	cfg := newTestConfig()
	cfg.Mode = ModeCustomInstall
	cfg.SelectedComponents = []string{"zsh", "tmux"}
	cfg.RootDir = "/tmp/dotfiles"
	cfg.ForceReinstall = true
	cfg.SkipPackages = true
	cfg.CleanBackup = true
	cfg.DryRun = true

	app := NewApp(cfg)
	app.config.Mode = ModeCustomInstall

	bc := app.buildConfig()

	if bc.Platform != cfg.Platform {
		t.Error("buildConfig Platform mismatch")
	}
	if bc.RootDir != "/tmp/dotfiles" {
		t.Errorf("buildConfig RootDir = %q, want /tmp/dotfiles", bc.RootDir)
	}
	if !bc.ForceReinstall {
		t.Error("buildConfig ForceReinstall should be true")
	}
	if !bc.SkipPackages {
		t.Error("buildConfig SkipPackages should be true")
	}
	if !bc.CleanBackup {
		t.Error("buildConfig CleanBackup should be true")
	}
	if !bc.DryRun {
		t.Error("buildConfig DryRun should be true")
	}
	if len(bc.SelectedComps) != 2 {
		t.Errorf(
			"buildConfig SelectedComps len = %d, want 2",
			len(bc.SelectedComps),
		)
	}
}

func TestAppModel_BuildConfigInstallMode(t *testing.T) {
	t.Parallel()
	cfg := newTestConfig()
	cfg.Mode = ModeInstall
	cfg.SelectedComponents = []string{"zsh"}

	app := NewApp(cfg)
	bc := app.buildConfig()

	// In non-custom mode, SelectedComps should be nil.
	if bc.SelectedComps != nil {
		t.Error(
			"buildConfig SelectedComps should be nil for ModeInstall",
		)
	}
}

func TestAppModel_ApplyResult(t *testing.T) {
	t.Parallel()
	app := NewApp(newTestConfig())

	result := orchestrator.BuildResult{
		PlanRows: []orchestrator.PlanRow{
			{
				Component: "zsh", Action: "install",
				Status: "would install",
			},
		},
		AlreadyInstalled:  3,
		AlreadyConfigured: 2,
		Tasks: []engine.Task{
			{ID: "install-zsh", Label: "Installing zsh"},
		},
	}

	tasks := app.applyResult(result)

	if len(tasks) != 1 {
		t.Errorf("tasks len = %d, want 1", len(tasks))
	}
	if len(app.config.PlanRows) != 1 {
		t.Errorf(
			"PlanRows len = %d, want 1",
			len(app.config.PlanRows),
		)
	}
	if app.summary.alreadyInstalled != 3 {
		t.Errorf(
			"alreadyInstalled = %d, want 3",
			app.summary.alreadyInstalled,
		)
	}
	if app.summary.alreadyConfigured != 2 {
		t.Errorf(
			"alreadyConfigured = %d, want 2",
			app.summary.alreadyConfigured,
		)
	}
}

func TestListenCmd(t *testing.T) {
	t.Parallel()
	ch := make(chan any, 1)
	ch <- engine.TaskStartedMsg{ID: "test", Label: "test"}

	cmd := listenCmd(ch)
	msg := cmd()

	if _, ok := msg.(engine.TaskStartedMsg); !ok {
		t.Errorf("expected TaskStartedMsg, got %T", msg)
	}
}

func TestListenCmd_ClosedChannel(t *testing.T) {
	t.Parallel()
	ch := make(chan any)
	close(ch)

	cmd := listenCmd(ch)
	msg := cmd()

	if _, ok := msg.(engine.AllDoneMsg); !ok {
		t.Errorf("expected AllDoneMsg on closed channel, got %T", msg)
	}
}

func TestDrainCmd(t *testing.T) {
	t.Parallel()
	ch := make(chan any, 5)
	ch <- engine.TaskStartedMsg{}
	ch <- engine.TaskDoneMsg{}
	ch <- engine.TaskSkippedMsg{}
	close(ch)

	cmd := drainCmd(ch)
	msg := cmd()

	if _, ok := msg.(engine.AllDoneMsg); !ok {
		t.Errorf("expected AllDoneMsg after drain, got %T", msg)
	}
}

func TestAppModel_UpdateInstalling_NonCriticalFailure(t *testing.T) {
	t.Parallel()
	app := NewApp(newTestConfig())
	app.phase = PhaseInstalling
	ch := make(chan any, 10)
	app.eventCh = ch

	app.progress.markActive("install-bat", "Installing bat")
	app.progress.markActive("install-zsh", "Installing zsh")

	// Non-critical failure should not go to failure prompt.
	msg := engine.TaskDoneMsg{
		ID: "install-bat", Label: "Installing bat",
		Err: errors.New("bat failed"), Critical: false,
	}
	model, _ := app.Update(msg)
	updated := model.(AppModel)

	if updated.phase == PhaseFailurePrompt {
		t.Error("non-critical failure should not go to failure prompt")
	}
}

func TestAppModel_UpdateInstalling_TaskDoneWithDoctorMode(t *testing.T) {
	t.Parallel()
	app := NewApp(newTestConfig())
	app.phase = PhaseInstalling
	app.width = 80
	app.height = 40
	app.summary.doctorMode = true
	ch := make(chan any, 10)
	app.eventCh = ch

	app.progress.markActive("check-zsh", "Installing zsh")

	msg := engine.TaskDoneMsg{
		ID: "check-zsh", Label: "Installing zsh",
	}
	model, _ := app.Update(msg)
	updated := model.(AppModel)

	if updated.phase != PhaseSummary {
		t.Errorf(
			"phase = %v, want PhaseSummary",
			updated.phase,
		)
	}
}

func TestAppModel_UpdateInstalling_SavesStateOnSuccess(t *testing.T) {
	t.Parallel()
	app := NewApp(newTestConfig())
	app.phase = PhaseInstalling
	ch := make(chan any, 10)
	app.eventCh = ch

	app.progress.markActive("install-zsh", "Installing zsh")
	app.progress.markActive("install-tmux", "Installing tmux")

	// Success should trigger save (we don't have state, so it's a no-op).
	msg := engine.TaskDoneMsg{
		ID: "install-zsh", Label: "Installing zsh",
	}
	model, _ := app.Update(msg)
	updated := model.(AppModel)

	// Should still be installing since tmux is active.
	if updated.phase != PhaseInstalling {
		t.Errorf(
			"phase = %v, want PhaseInstalling",
			updated.phase,
		)
	}
}

func TestAppModel_OptionsMenuNavigation(t *testing.T) {
	t.Parallel()
	app := NewApp(newTestConfig())
	app.phase = PhaseOptionsMenu

	model, _ := app.Update(keyPress('j'))
	updated := model.(AppModel)

	if updated.options.cursor != 1 {
		t.Errorf("cursor = %d, want 1", updated.options.cursor)
	}
}

func TestAppModel_OptionsMenuToggle(t *testing.T) {
	t.Parallel()
	app := NewApp(newTestConfig())
	app.phase = PhaseOptionsMenu

	model, _ := app.Update(keyPress(' '))
	updated := model.(AppModel)

	if !updated.options.options[0].enabled {
		t.Error("space should toggle first option")
	}
}

func TestAppModel_SummaryHandlesAllDoneMsg(t *testing.T) {
	t.Parallel()
	app := NewApp(newTestConfig())
	app.phase = PhaseSummary

	// AllDoneMsg in summary should be ignored gracefully.
	model, _ := app.Update(engine.AllDoneMsg{})
	updated := model.(AppModel)

	if updated.phase != PhaseSummary {
		t.Errorf(
			"phase = %v, want PhaseSummary",
			updated.phase,
		)
	}
}

func TestAppModel_SummaryHandlesStraggerEvents(t *testing.T) {
	t.Parallel()
	app := NewApp(newTestConfig())
	app.phase = PhaseSummary

	// Straggler engine events should be ignored.
	model, _ := app.Update(engine.TaskStartedMsg{})
	updated := model.(AppModel)
	if updated.phase != PhaseSummary {
		t.Error("straggler TaskStartedMsg should not change phase")
	}

	model, _ = updated.Update(engine.TaskDoneMsg{})
	updated = model.(AppModel)
	if updated.phase != PhaseSummary {
		t.Error("straggler TaskDoneMsg should not change phase")
	}

	model, _ = updated.Update(engine.TaskSkippedMsg{})
	updated = model.(AppModel)
	if updated.phase != PhaseSummary {
		t.Error("straggler TaskSkippedMsg should not change phase")
	}
}

func TestAppModel_FailurePromptQQuitsGlobally(t *testing.T) {
	t.Parallel()
	app := NewApp(newTestConfig())
	app.phase = PhaseFailurePrompt
	ch := make(chan any, 10)
	app.eventCh = ch
	app.failedTaskLabel = "brew"
	app.failedTaskErr = errors.New("failed")

	// 'q' in failure prompt hits the global quit handler
	// since phase != PhaseInstalling.
	msg := keyPress('q')
	model, cmd := app.Update(msg)
	updated := model.(AppModel)

	if !updated.quitting {
		t.Error("q should trigger global quit from failure prompt")
	}
	if cmd == nil {
		t.Error("q should return a quit command")
	}
}

func TestAppModel_FailurePromptAbortWithCancelEngine(t *testing.T) {
	t.Parallel()
	app := NewApp(newTestConfig())
	app.phase = PhaseFailurePrompt
	ch := make(chan any, 10)
	app.eventCh = ch
	app.failedTaskLabel = "brew"
	app.failedTaskErr = errors.New("failed")

	cancelCalled := false
	app.cancelEngine = func() { cancelCalled = true }

	msg := keyPress('a')
	model, _ := app.Update(msg)
	_ = model.(AppModel)

	if !cancelCalled {
		t.Error("abort should call cancelEngine")
	}
}

func TestAppModel_CtrlCWithCancelEngine(t *testing.T) {
	t.Parallel()
	app := NewApp(newTestConfig())
	app.phase = PhaseInstalling

	cancelCalled := false
	app.cancelEngine = func() { cancelCalled = true }

	msg := tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	model, _ := app.Update(msg)
	updated := model.(AppModel)

	if !updated.quitting {
		t.Error("ctrl+c should set quitting")
	}
	if !cancelCalled {
		t.Error("ctrl+c should call cancelEngine")
	}
}

func newTestRunner(t *testing.T) *executor.Runner {
	t.Helper()
	logPath := filepath.Join(t.TempDir(), "test.log")
	lf, err := executor.NewLogFile(logPath)
	if err != nil {
		t.Fatalf("NewLogFile: %v", err)
	}
	t.Cleanup(func() { lf.Close() })
	return executor.NewRunner(lf, true)
}

func newTestConfigWithRunner(t *testing.T) *AppConfig {
	t.Helper()
	cfg := newTestConfig()
	cfg.Runner = newTestRunner(t)
	return cfg
}

func TestAppModel_OptionsMenuEnterSetsConfig(t *testing.T) {
	t.Parallel()
	cfg := newTestConfigWithRunner(t)
	cfg.Mode = ModeInstall
	app := NewApp(cfg)
	app.phase = PhaseOptionsMenu

	// Toggle options to exercise the config mapping path.
	// Toggle skip_update (index 0).
	model, _ := app.Update(keyPress(' '))
	app = model.(AppModel)
	// Move to verbose (index 3) and toggle.
	for i := 0; i < 3; i++ {
		model, _ = app.Update(keyPress('j'))
		app = model.(AppModel)
	}
	model, _ = app.Update(keyPress(' '))
	app = model.(AppModel)

	// Verify options are set in the model before enter.
	if !app.options.optionEnabled("skip_update") {
		t.Error("skip_update should be toggled on")
	}
	if !app.options.optionEnabled("verbose") {
		t.Error("verbose should be toggled on")
	}
}

func TestAppModel_OptionsMenuEnterCustomMode(t *testing.T) {
	t.Parallel()
	cfg := newTestConfigWithRunner(t)
	cfg.Mode = ModeCustomInstall
	app := NewApp(cfg)
	app.phase = PhaseOptionsMenu

	model, _ := app.Update(specialKeyPress(tea.KeyEnter))
	updated := model.(AppModel)

	if updated.phase != PhaseComponentPicker {
		t.Errorf(
			"phase = %v, want PhaseComponentPicker for custom install",
			updated.phase,
		)
	}
}

func TestAppModel_ComponentPickerSelectedStored(t *testing.T) {
	t.Parallel()
	app := NewApp(newTestConfig())
	app.phase = PhaseComponentPicker

	// Select "All" to verify items are selected.
	model, _ := app.Update(keyPress(' '))
	app = model.(AppModel)

	sel := app.picker.selectedComponents()
	if len(sel) == 0 {
		t.Error("selectedComponents should not be empty after All")
	}
}

func TestAppModel_BackupPickerEnterWithSelection(t *testing.T) {
	t.Parallel()

	// Create a real backup directory.
	tmpDir := t.TempDir()
	backupDir := filepath.Join(
		tmpDir, ".dotfiles-backup-20240101-120000",
	)
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	cfg := newTestConfigWithRunner(t)
	cfg.Mode = ModeRestore
	cfg.RootDir = tmpDir
	app := NewApp(cfg)
	app.phase = PhaseBackupPicker
	app.backupPicker = backupPickerModel{
		items: []backup.BackupInfo{
			{Date: "2024-01-01", Path: backupDir},
		},
	}

	// Enter with a backup selected.
	model, _ := app.Update(specialKeyPress(tea.KeyEnter))
	updated := model.(AppModel)

	if updated.config.SelectedBackup != backupDir {
		t.Errorf(
			"SelectedBackup = %q, want %q",
			updated.config.SelectedBackup, backupDir,
		)
	}
}

func TestAppModel_SummaryViewportScrollForward(t *testing.T) {
	t.Parallel()
	app := NewApp(newTestConfig())
	app.phase = PhaseSummary
	app.width = 80
	app.height = 40
	app.summary.dryRun = true

	// Add rows to trigger viewport.
	for i := 0; i < 50; i++ {
		app.summary.rows = append(
			app.summary.rows,
			orchestrator.PlanRow{
				Component: "tool", Action: "install",
				Status: "would install",
			},
		)
	}
	app.summary.initViewport(80, 40)

	// Send a down arrow to the summary viewport.
	model, _ := app.Update(specialKeyPress(tea.KeyDown))
	_ = model.(AppModel)
	// Just verify it doesn't panic.
}

func TestAppModel_SummaryBackspace(t *testing.T) {
	t.Parallel()
	app := NewApp(newTestConfig())
	app.phase = PhaseSummary

	model, _ := app.Update(specialKeyPress(tea.KeyBackspace))
	updated := model.(AppModel)

	if updated.phase != PhaseMainMenu {
		t.Errorf(
			"phase = %v, want PhaseMainMenu after backspace",
			updated.phase,
		)
	}
}

func TestAppModel_SummaryTimestamps(t *testing.T) {
	t.Parallel()
	app := NewApp(newTestConfig())
	app.phase = PhaseInstalling
	ch := make(chan any, 10)
	app.eventCh = ch

	app.progress.markActive("install-zsh", "Installing zsh")
	app.summary.startTime = time.Now().Add(-5 * time.Second)

	msg := engine.TaskDoneMsg{
		ID: "install-zsh", Label: "Installing zsh",
	}
	model, _ := app.Update(msg)
	updated := model.(AppModel)

	if updated.summary.endTime.IsZero() {
		t.Error("endTime should be set after all tasks complete")
	}
}
