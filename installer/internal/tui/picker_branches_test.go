package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/chaseddevelopment/dotfiles/installer/internal/backup"
)

// TestUpdateComponentPickerNonKey covers the "msg is not a key →
// delegate to picker.Update" branch. A WindowSizeMsg should not
// alter the phase or component selection.
func TestUpdateComponentPickerNonKey(t *testing.T) {
	app := NewApp(newTestConfig())
	app.phase = PhaseComponentPicker
	model, _ := app.updateComponentPicker(
		tea.WindowSizeMsg{Width: 80, Height: 40},
	)
	updated := model.(AppModel)
	if updated.phase != PhaseComponentPicker {
		t.Fatalf("non-key msg must not change phase: %v", updated.phase)
	}
	if len(updated.config.SelectedComponents) != 0 {
		t.Fatalf(
			"non-key msg must not set selections: %v",
			updated.config.SelectedComponents,
		)
	}
}

// TestUpdateComponentPickerEscBack covers the esc-to-options-menu
// branch for install/custom-install flows. Asserts the phase
// transitions to PhaseOptionsMenu.
func TestUpdateComponentPickerEscBack(t *testing.T) {
	app := NewApp(newTestConfig())
	app.phase = PhaseComponentPicker
	app.config.Mode = ModeCustomInstall
	model, _ := app.updateComponentPicker(specialKeyPress(27)) // esc
	updated := model.(AppModel)
	if updated.phase != PhaseOptionsMenu {
		t.Fatalf("esc should go to options menu, got %v", updated.phase)
	}
}

// TestUpdateComponentPickerEscBackUninstall covers the uninstall
// variant: esc returns to main menu, not options.
func TestUpdateComponentPickerEscBackUninstall(t *testing.T) {
	app := NewApp(newTestConfig())
	app.phase = PhaseComponentPicker
	app.config.Mode = ModeUninstall
	model, _ := app.updateComponentPicker(specialKeyPress(27))
	updated := model.(AppModel)
	if updated.phase != PhaseMainMenu {
		t.Fatalf(
			"esc from uninstall picker should go main menu, got %v",
			updated.phase,
		)
	}
}

// TestUpdateComponentPickerEnterEmpty covers the "no selection →
// no-op" branch.
func TestUpdateComponentPickerEnterEmpty(t *testing.T) {
	app := NewApp(newTestConfig())
	app.phase = PhaseComponentPicker
	// Ensure nothing is selected in the picker.
	for i := range app.picker.items {
		app.picker.items[i].selected = false
	}
	model, _ := app.updateComponentPicker(specialKeyPress(tea.KeyEnter))
	updated := model.(AppModel)
	if updated.phase != PhaseComponentPicker {
		t.Fatalf(
			"enter on empty selection must stay on picker, got %v",
			updated.phase,
		)
	}
	if len(updated.config.SelectedComponents) != 0 {
		t.Fatalf(
			"no selection should not populate SelectedComponents: %v",
			updated.config.SelectedComponents,
		)
	}
}

// TestUpdateComponentPickerEnterWithSelection confirms enter with a
// selected component transitions to PhaseInstalling and captures
// the chosen component into config.SelectedComponents.
func TestUpdateComponentPickerEnterWithSelection(t *testing.T) {
	app := NewApp(newTestConfig())
	app.phase = PhaseComponentPicker
	// Mark the first non-"All" item as selected.
	if len(app.picker.items) < 2 {
		t.Skip("picker has no real components to select")
	}
	wantName := app.picker.items[1].name
	app.picker.items[1].selected = true

	model, _ := app.updateComponentPicker(specialKeyPress(tea.KeyEnter))
	updated := model.(AppModel)
	if updated.phase != PhaseInstalling {
		t.Fatalf(
			"enter with selection should go to installing, got %v",
			updated.phase,
		)
	}
	found := false
	for _, name := range updated.config.SelectedComponents {
		if name == wantName {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf(
			"SelectedComponents should include %q, got %v",
			wantName, updated.config.SelectedComponents,
		)
	}
}

// TestUpdateBackupPickerNonKey covers the equivalent branch on the
// backup picker. Asserts phase unchanged.
func TestUpdateBackupPickerNonKey(t *testing.T) {
	app := NewApp(newTestConfig())
	app.phase = PhaseBackupPicker
	model, _ := app.updateBackupPicker(
		tea.WindowSizeMsg{Width: 80, Height: 40},
	)
	updated := model.(AppModel)
	if updated.phase != PhaseBackupPicker {
		t.Fatalf("non-key must not change phase: %v", updated.phase)
	}
	if updated.config.SelectedBackup != "" {
		t.Fatalf(
			"non-key must not set SelectedBackup: %q",
			updated.config.SelectedBackup,
		)
	}
}

// TestUpdateBackupPickerEscBack covers the esc-to-main-menu branch.
func TestUpdateBackupPickerEscBack(t *testing.T) {
	app := NewApp(newTestConfig())
	app.phase = PhaseBackupPicker
	model, _ := app.updateBackupPicker(tea.KeyPressMsg{Code: 27})
	updated := model.(AppModel)
	if updated.phase != PhaseMainMenu {
		t.Fatalf("esc should go to main menu, got %v", updated.phase)
	}
}

// TestUpdateBackupPickerEnterEmpty covers the "no selection → no-op"
// branch. Asserts the phase stays and SelectedBackup remains empty.
func TestUpdateBackupPickerEnterEmpty(t *testing.T) {
	app := NewApp(newTestConfig())
	app.phase = PhaseBackupPicker
	// Explicitly empty items slice.
	app.backupPicker = backupPickerModel{}
	model, _ := app.updateBackupPicker(tea.KeyPressMsg{Code: 13})
	updated := model.(AppModel)
	if updated.phase != PhaseBackupPicker {
		t.Fatalf(
			"enter on empty selection must stay on picker, got %v",
			updated.phase,
		)
	}
	if updated.config.SelectedBackup != "" {
		t.Fatalf(
			"empty enter must not populate SelectedBackup: %q",
			updated.config.SelectedBackup,
		)
	}
}

// TestUpdateBackupPickerEnterWithSelection asserts enter with a
// populated picker captures the path and transitions phase.
func TestUpdateBackupPickerEnterWithSelection(t *testing.T) {
	app := NewApp(newTestConfig())
	app.phase = PhaseBackupPicker
	app.backupPicker = backupPickerModel{
		items: []backup.BackupInfo{
			{Date: "2024-01-01", Path: "/tmp/backup-a"},
		},
	}
	model, _ := app.updateBackupPicker(tea.KeyPressMsg{Code: 13})
	updated := model.(AppModel)
	if updated.phase != PhaseInstalling {
		t.Fatalf(
			"enter with selection should go to installing, got %v",
			updated.phase,
		)
	}
	if updated.config.SelectedBackup != "/tmp/backup-a" {
		t.Fatalf(
			"SelectedBackup = %q, want /tmp/backup-a",
			updated.config.SelectedBackup,
		)
	}
}
