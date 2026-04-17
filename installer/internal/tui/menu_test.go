package tui

import (
	"errors"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/chaseddevelopment/dotfiles/installer/internal/backup"
)

// keyPress creates a KeyPressMsg for a printable character key.
func keyPress(char rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: char, Text: string(char)}
}

// specialKeyPress creates a KeyPressMsg for a special key (e.g., enter, up).
func specialKeyPress(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: code}
}

// ---------------------------------------------------------------------------
// Main Menu
// ---------------------------------------------------------------------------

func TestNewMainMenu(t *testing.T) {
	t.Parallel()
	m := newMainMenu()

	if len(m.items) == 0 {
		t.Fatal("newMainMenu() should have items")
	}
	if m.cursor != 0 {
		t.Errorf("cursor should start at 0, got %d", m.cursor)
	}

	// Verify expected menu items exist.
	modes := make(map[InstallMode]bool)
	for _, item := range m.items {
		modes[item.mode] = true
	}
	expectedModes := []InstallMode{
		ModeInstall, ModeCustomInstall, ModeDryRun,
		ModeUpdate, ModeRestore, ModeDoctor,
		ModeUninstall, ModeExit,
	}
	for _, mode := range expectedModes {
		if !modes[mode] {
			t.Errorf("expected mode %d in menu items", mode)
		}
	}
}

func TestMainMenu_CursorNavigation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		keys       []tea.KeyPressMsg
		wantCursor int
	}{
		{
			name:       "down moves cursor",
			keys:       []tea.KeyPressMsg{keyPress('j')},
			wantCursor: 1,
		},
		{
			name: "down twice moves to 2",
			keys: []tea.KeyPressMsg{
				keyPress('j'),
				keyPress('j'),
			},
			wantCursor: 2,
		},
		{
			name: "down then up returns to 0",
			keys: []tea.KeyPressMsg{
				specialKeyPress(tea.KeyDown),
				specialKeyPress(tea.KeyUp),
			},
			wantCursor: 0,
		},
		{
			name: "k moves up",
			keys: []tea.KeyPressMsg{
				keyPress('j'),
				keyPress('j'),
				keyPress('k'),
			},
			wantCursor: 1,
		},
		{
			name:       "up at top stays at 0",
			keys:       []tea.KeyPressMsg{specialKeyPress(tea.KeyUp)},
			wantCursor: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			m := newMainMenu()
			for _, key := range tt.keys {
				m, _ = m.Update(key)
			}
			if m.cursor != tt.wantCursor {
				t.Errorf(
					"cursor = %d, want %d",
					m.cursor, tt.wantCursor,
				)
			}
		})
	}
}

func TestMainMenu_DownAtBottom(t *testing.T) {
	t.Parallel()
	m := newMainMenu()
	last := len(m.items) - 1

	// Move to the last item.
	for i := 0; i < last+5; i++ {
		m, _ = m.Update(keyPress('j'))
	}
	if m.cursor != last {
		t.Errorf("cursor = %d, want %d (last item)", m.cursor, last)
	}
}

func TestMainMenu_Selected(t *testing.T) {
	t.Parallel()
	m := newMainMenu()

	// First item should be Install.
	if got := m.selected(); got != ModeInstall {
		t.Errorf("selected() at cursor 0 = %v, want ModeInstall", got)
	}

	// Move to last item (Exit).
	for i := 0; i < len(m.items)-1; i++ {
		m, _ = m.Update(keyPress('j'))
	}
	if got := m.selected(); got != ModeExit {
		t.Errorf("selected() at last item = %v, want ModeExit", got)
	}
}

func TestMainMenu_SelectedOutOfBounds(t *testing.T) {
	t.Parallel()
	m := newMainMenu()
	m.cursor = len(m.items) + 10
	if got := m.selected(); got != 0 {
		t.Errorf(
			"selected() with out-of-bounds cursor = %v, want 0",
			got,
		)
	}
}

func TestMainMenu_View(t *testing.T) {
	t.Parallel()
	m := newMainMenu()
	view := m.View(80)
	if view == "" {
		t.Error("View() returned empty string")
	}
	if len(view) < 20 {
		t.Error("View() output seems too short")
	}
}

func TestMainMenu_NonKeyMsg(t *testing.T) {
	t.Parallel()
	m := newMainMenu()
	original := m.cursor

	// A non-key message should not change cursor.
	m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 50})
	if m.cursor != original {
		t.Errorf(
			"cursor changed on non-key message: %d -> %d",
			original, m.cursor,
		)
	}
}

// ---------------------------------------------------------------------------
// Options Menu
// ---------------------------------------------------------------------------

func TestNewOptionsMenu(t *testing.T) {
	t.Parallel()
	m := newOptionsMenu()

	if len(m.options) == 0 {
		t.Fatal("newOptionsMenu() should have options")
	}
	if m.cursor != 0 {
		t.Errorf("cursor should start at 0, got %d", m.cursor)
	}

	// All options should start disabled.
	for _, opt := range m.options {
		if opt.enabled {
			t.Errorf("option %q should start disabled", opt.key)
		}
	}
}

func TestOptionsMenu_Toggle(t *testing.T) {
	t.Parallel()
	m := newOptionsMenu()

	// Toggle first option with space.
	m, _ = m.Update(keyPress(' '))
	if !m.options[0].enabled {
		t.Error("space should toggle first option on")
	}

	// Toggle again to disable.
	m, _ = m.Update(keyPress(' '))
	if m.options[0].enabled {
		t.Error("second space should toggle first option off")
	}
}

func TestOptionsMenu_ToggleWithX(t *testing.T) {
	t.Parallel()
	m := newOptionsMenu()
	m, _ = m.Update(keyPress('x'))
	if !m.options[0].enabled {
		t.Error("x should toggle option on")
	}
}

func TestOptionsMenu_CursorNavigation(t *testing.T) {
	t.Parallel()
	m := newOptionsMenu()
	m, _ = m.Update(keyPress('j'))
	if m.cursor != 1 {
		t.Errorf("cursor = %d, want 1", m.cursor)
	}
	m, _ = m.Update(keyPress('k'))
	if m.cursor != 0 {
		t.Errorf("cursor = %d, want 0", m.cursor)
	}
}

func TestOptionsMenu_OptionEnabled(t *testing.T) {
	t.Parallel()
	m := newOptionsMenu()

	// Initially all should be disabled.
	if m.optionEnabled("skip_update") {
		t.Error("skip_update should start disabled")
	}
	if m.optionEnabled("nonexistent_key") {
		t.Error("nonexistent key should return false")
	}

	// Toggle the first option.
	m, _ = m.Update(keyPress(' '))
	if !m.optionEnabled(m.options[0].key) {
		t.Errorf(
			"option %q should be enabled after toggle",
			m.options[0].key,
		)
	}
}

func TestOptionsMenu_View(t *testing.T) {
	t.Parallel()
	m := newOptionsMenu()
	view := m.View(80)
	if view == "" {
		t.Error("View() returned empty string")
	}
}

// ---------------------------------------------------------------------------
// Component Picker
// ---------------------------------------------------------------------------

func TestNewComponentPicker(t *testing.T) {
	t.Parallel()
	m := newComponentPicker()

	if len(m.items) == 0 {
		t.Fatal("newComponentPicker() should have items")
	}
	// First item should be "All".
	if m.items[0].name != "All" {
		t.Errorf("first item should be 'All', got %q", m.items[0].name)
	}
	if m.cursor != 0 {
		t.Errorf("cursor should start at 0, got %d", m.cursor)
	}
}

func TestComponentPicker_SelectAll(t *testing.T) {
	t.Parallel()
	m := newComponentPicker()

	// Toggle "All" — should select every item.
	m, _ = m.Update(keyPress(' '))

	for i, item := range m.items {
		if !item.selected {
			t.Errorf("item %d (%q) should be selected", i, item.name)
		}
	}

	// Toggle "All" off — should deselect every item.
	m, _ = m.Update(keyPress(' '))
	for i, item := range m.items {
		if item.selected {
			t.Errorf(
				"item %d (%q) should be deselected",
				i, item.name,
			)
		}
	}
}

func TestComponentPicker_SelectIndividual(t *testing.T) {
	t.Parallel()
	m := newComponentPicker()
	if len(m.items) < 2 {
		t.Skip("not enough components to test individual selection")
	}

	// Move to second item and select it.
	m, _ = m.Update(keyPress('j'))
	m, _ = m.Update(keyPress(' '))

	if !m.items[1].selected {
		t.Error("second item should be selected")
	}
	if m.items[0].selected {
		t.Error("'All' should not be selected")
	}
}

func TestComponentPicker_SelectedComponents(t *testing.T) {
	t.Parallel()
	m := newComponentPicker()

	// No selections initially.
	sel := m.selectedComponents()
	if len(sel) != 0 {
		t.Errorf(
			"selectedComponents() = %v, want empty slice",
			sel,
		)
	}

	// Select "All".
	m, _ = m.Update(keyPress(' '))
	sel = m.selectedComponents()
	if len(sel) != len(m.items) {
		t.Errorf(
			"selectedComponents() len = %d, want %d",
			len(sel), len(m.items),
		)
	}
}

func TestComponentPicker_CursorNavigation(t *testing.T) {
	t.Parallel()
	m := newComponentPicker()
	m, _ = m.Update(specialKeyPress(tea.KeyDown))
	if m.cursor != 1 {
		t.Errorf("cursor = %d, want 1", m.cursor)
	}
	m, _ = m.Update(specialKeyPress(tea.KeyUp))
	if m.cursor != 0 {
		t.Errorf("cursor = %d, want 0", m.cursor)
	}
}

func TestComponentPicker_View(t *testing.T) {
	t.Parallel()
	m := newComponentPicker()
	view := m.View(80)
	if view == "" {
		t.Error("View() returned empty string")
	}
}

// ---------------------------------------------------------------------------
// Backup Picker
// ---------------------------------------------------------------------------

func TestBackupPicker_EmptyItems(t *testing.T) {
	t.Parallel()
	m := backupPickerModel{}
	if got := m.selected(); got != "" {
		t.Errorf("selected() with no items should be empty, got %q", got)
	}
}

func TestBackupPicker_View(t *testing.T) {
	t.Parallel()
	m := backupPickerModel{}
	view := m.View(80)
	if view == "" {
		t.Error("View() returned empty string for empty backup picker")
	}
}

func TestBackupPicker_CursorNavigation(t *testing.T) {
	t.Parallel()
	m := backupPickerModel{}
	// Navigating with no items should not panic.
	m, _ = m.Update(keyPress('j'))
	m, _ = m.Update(keyPress('k'))
	if m.cursor != 0 {
		t.Errorf("cursor = %d, want 0", m.cursor)
	}
}

func TestBackupPicker_WithItems(t *testing.T) {
	t.Parallel()
	m := backupPickerModel{
		items: []backup.BackupInfo{
			{Date: "2024-01-01", Path: "/tmp/backup1"},
			{Date: "2024-02-01", Path: "/tmp/backup2"},
			{Date: "2024-03-01", Path: "/tmp/backup3"},
		},
	}

	// Selected should return first item.
	if got := m.selected(); got != "/tmp/backup1" {
		t.Errorf("selected() = %q, want '/tmp/backup1'", got)
	}

	// Navigate down and verify.
	m, _ = m.Update(keyPress('j'))
	if m.cursor != 1 {
		t.Errorf("cursor = %d, want 1", m.cursor)
	}
	if got := m.selected(); got != "/tmp/backup2" {
		t.Errorf("selected() = %q, want '/tmp/backup2'", got)
	}

	// Navigate to last.
	m, _ = m.Update(keyPress('j'))
	if got := m.selected(); got != "/tmp/backup3" {
		t.Errorf("selected() = %q, want '/tmp/backup3'", got)
	}

	// Past end stays at last.
	m, _ = m.Update(keyPress('j'))
	if m.cursor != 2 {
		t.Errorf("cursor = %d, want 2 (last)", m.cursor)
	}

	// Navigate up.
	m, _ = m.Update(keyPress('k'))
	if m.cursor != 1 {
		t.Errorf("cursor = %d, want 1", m.cursor)
	}

	// Up at top stays at 0.
	m, _ = m.Update(keyPress('k'))
	m, _ = m.Update(keyPress('k'))
	if m.cursor != 0 {
		t.Errorf("cursor = %d, want 0", m.cursor)
	}
}

func TestBackupPicker_ViewWithItems(t *testing.T) {
	t.Parallel()
	m := backupPickerModel{
		items: []backup.BackupInfo{
			{Date: "2024-01-01", Path: "/tmp/backup1"},
			{Date: "2024-02-01", Path: "/tmp/backup2"},
		},
	}
	view := m.View(80)
	if view == "" {
		t.Error("View() with items returned empty")
	}
}

func TestBackupPicker_ViewWithError(t *testing.T) {
	t.Parallel()
	m := backupPickerModel{
		err: errors.New("failed to list backups"),
	}
	view := m.View(80)
	if view == "" {
		t.Error("View() with error returned empty")
	}
}

func TestBackupPicker_NonKeyMsg(t *testing.T) {
	t.Parallel()
	m := backupPickerModel{
		items: []backup.BackupInfo{
			{Date: "2024-01-01", Path: "/tmp/backup1"},
		},
	}
	original := m.cursor
	m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 40})
	if m.cursor != original {
		t.Error("non-key message should not change cursor")
	}
}

// ---------------------------------------------------------------------------
// Menu item properties
// ---------------------------------------------------------------------------

func TestMenuItems_HaveLabelsAndIcons(t *testing.T) {
	t.Parallel()
	m := newMainMenu()
	for i, item := range m.items {
		if item.label == "" {
			t.Errorf("item %d has empty label", i)
		}
		if item.icon == "" {
			t.Errorf("item %d (%s) has empty icon", i, item.label)
		}
	}
}

func TestOptionsMenu_AllKeysNonEmpty(t *testing.T) {
	t.Parallel()
	m := newOptionsMenu()
	for i, opt := range m.options {
		if opt.key == "" {
			t.Errorf("option %d has empty key", i)
		}
		if opt.label == "" {
			t.Errorf("option %d has empty label", i)
		}
	}
}

func TestOptionsMenu_DownAtBottom(t *testing.T) {
	t.Parallel()
	m := newOptionsMenu()
	last := len(m.options) - 1
	for i := 0; i <= last+3; i++ {
		m, _ = m.Update(keyPress('j'))
	}
	if m.cursor != last {
		t.Errorf("cursor = %d, want %d", m.cursor, last)
	}
}

func TestOptionsMenu_UpAtTop(t *testing.T) {
	t.Parallel()
	m := newOptionsMenu()
	m, _ = m.Update(keyPress('k'))
	if m.cursor != 0 {
		t.Errorf("cursor = %d, want 0", m.cursor)
	}
}

func TestOptionsMenu_NonKeyMsg(t *testing.T) {
	t.Parallel()
	m := newOptionsMenu()
	m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 40})
	if m.cursor != 0 {
		t.Error("non-key message should not change cursor")
	}
}

func TestComponentPicker_DownAtBottom(t *testing.T) {
	t.Parallel()
	m := newComponentPicker()
	last := len(m.items) - 1
	for i := 0; i <= last+3; i++ {
		m, _ = m.Update(keyPress('j'))
	}
	if m.cursor != last {
		t.Errorf("cursor = %d, want %d", m.cursor, last)
	}
}

func TestComponentPicker_UpAtTop(t *testing.T) {
	t.Parallel()
	m := newComponentPicker()
	m, _ = m.Update(keyPress('k'))
	if m.cursor != 0 {
		t.Errorf("cursor = %d, want 0", m.cursor)
	}
}

func TestComponentPicker_NonKeyMsg(t *testing.T) {
	t.Parallel()
	m := newComponentPicker()
	m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 40})
	if m.cursor != 0 {
		t.Error("non-key message should not change cursor")
	}
}

func TestMainMenu_ViewNarrow(t *testing.T) {
	t.Parallel()
	m := newMainMenu()
	view := m.View(30)
	if view == "" {
		t.Error("View() returned empty for narrow width")
	}
}

func TestOptionsMenu_ViewNarrow(t *testing.T) {
	t.Parallel()
	m := newOptionsMenu()
	view := m.View(30)
	if view == "" {
		t.Error("View() returned empty for narrow width")
	}
}

func TestComponentPicker_ViewNarrow(t *testing.T) {
	t.Parallel()
	m := newComponentPicker()
	view := m.View(30)
	if view == "" {
		t.Error("View() returned empty for narrow width")
	}
}

func TestMainMenu_ViewSelected(t *testing.T) {
	t.Parallel()
	m := newMainMenu()
	// Move to different items and verify view renders.
	for i := 0; i < len(m.items); i++ {
		view := m.View(80)
		if view == "" {
			t.Errorf("View() returned empty at cursor %d", i)
		}
		m, _ = m.Update(keyPress('j'))
	}
}

func TestOptionsMenu_ViewWithToggle(t *testing.T) {
	t.Parallel()
	m := newOptionsMenu()
	// Toggle first option and verify view.
	m, _ = m.Update(keyPress(' '))
	view := m.View(80)
	if view == "" {
		t.Error("View() returned empty with toggled option")
	}
}

func TestComponentPicker_ViewWithSelection(t *testing.T) {
	t.Parallel()
	m := newComponentPicker()
	// Select first item and move cursor.
	m, _ = m.Update(keyPress(' '))
	m, _ = m.Update(keyPress('j'))
	view := m.View(80)
	if view == "" {
		t.Error("View() returned empty with selection")
	}
}
