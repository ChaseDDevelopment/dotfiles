package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

// TestUpdateComponentPickerNonKey covers the "msg is not a key →
// delegate to picker.Update" branch.
func TestUpdateComponentPickerNonKey(t *testing.T) {
	app := NewApp(newTestConfig())
	app.phase = PhaseComponentPicker
	model, _ := app.updateComponentPicker(tea.WindowSizeMsg{Width: 80, Height: 40})
	if _, ok := model.(AppModel); !ok {
		t.Fatalf("expected AppModel, got %T", model)
	}
}

// TestUpdateBackupPickerNonKey covers the equivalent branch on the
// backup picker.
func TestUpdateBackupPickerNonKey(t *testing.T) {
	app := NewApp(newTestConfig())
	app.phase = PhaseBackupPicker
	model, _ := app.updateBackupPicker(tea.WindowSizeMsg{Width: 80, Height: 40})
	if _, ok := model.(AppModel); !ok {
		t.Fatalf("expected AppModel, got %T", model)
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
// branch.
func TestUpdateBackupPickerEnterEmpty(t *testing.T) {
	app := NewApp(newTestConfig())
	app.phase = PhaseBackupPicker
	model, _ := app.updateBackupPicker(tea.KeyPressMsg{Code: 13})
	updated := model.(AppModel)
	if updated.phase != PhaseBackupPicker {
		t.Fatalf("enter on empty selection should stay on picker, got %v", updated.phase)
	}
}
