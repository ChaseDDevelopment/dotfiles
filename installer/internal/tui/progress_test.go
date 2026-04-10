package tui

import (
	"errors"
	"testing"
	"time"

	"charm.land/bubbles/v2/progress"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
)

func TestNewProgressModel(t *testing.T) {
	t.Parallel()
	m := newProgressModel()

	if m.toolStatuses == nil {
		t.Error("toolStatuses should be initialized")
	}
	if m.labelByID == nil {
		t.Error("labelByID should be initialized")
	}
	if m.done {
		t.Error("done should be false initially")
	}
	if m.totalTools != 0 {
		t.Error("totalTools should be 0 initially")
	}
	if m.doneCount != 0 {
		t.Error("doneCount should be 0 initially")
	}
}

func TestProgressModel_MarkActive(t *testing.T) {
	t.Parallel()
	m := newProgressModel()

	m.markActive("install-zsh", "Installing zsh")

	if m.totalTools != 1 {
		t.Errorf("totalTools = %d, want 1", m.totalTools)
	}
	if len(m.active) != 1 {
		t.Errorf("active tasks = %d, want 1", len(m.active))
	}
	if m.toolStatuses["zsh"] != statusActive {
		t.Errorf(
			"zsh status = %v, want statusActive",
			m.toolStatuses["zsh"],
		)
	}
	if len(m.toolNames) != 1 || m.toolNames[0] != "zsh" {
		t.Errorf("toolNames = %v, want [zsh]", m.toolNames)
	}
}

func TestProgressModel_MarkActiveDuplicate(t *testing.T) {
	t.Parallel()
	m := newProgressModel()

	m.markActive("install-zsh", "Installing zsh")
	m.markActive("setup-zsh", "Setting up zsh")

	// Should not create a duplicate entry for "zsh".
	if m.totalTools != 1 {
		t.Errorf("totalTools = %d, want 1 (no duplicates)", m.totalTools)
	}
	if len(m.active) != 2 {
		t.Errorf(
			"active tasks = %d, want 2 (both install and setup)",
			len(m.active),
		)
	}
}

func TestProgressModel_MarkDoneSuccess(t *testing.T) {
	t.Parallel()
	m := newProgressModel()

	m.markActive("install-tmux", "Installing tmux")
	m.markDone("install-tmux", nil)

	if m.toolStatuses["tmux"] != statusDone {
		t.Errorf(
			"tmux status = %v, want statusDone",
			m.toolStatuses["tmux"],
		)
	}
	if m.doneCount != 1 {
		t.Errorf("doneCount = %d, want 1", m.doneCount)
	}
	if len(m.active) != 0 {
		t.Errorf("active = %d, want 0", len(m.active))
	}
	if len(m.steps) != 1 {
		t.Fatalf("steps = %d, want 1", len(m.steps))
	}
	if !m.steps[0].success {
		t.Error("step should be successful")
	}
	if m.steps[0].status != "installed" {
		t.Errorf("status = %q, want 'installed'", m.steps[0].status)
	}
}

func TestProgressModel_MarkDoneFailure(t *testing.T) {
	t.Parallel()
	m := newProgressModel()

	m.markActive("install-rust", "Installing rust")
	m.markDone("install-rust", errors.New("download failed"))

	if m.toolStatuses["rust"] != statusFailed {
		t.Errorf(
			"rust status = %v, want statusFailed",
			m.toolStatuses["rust"],
		)
	}
	if len(m.steps) != 1 {
		t.Fatalf("steps = %d, want 1", len(m.steps))
	}
	if m.steps[0].success {
		t.Error("step should not be successful")
	}
	if m.steps[0].err == nil {
		t.Error("step error should not be nil")
	}
}

func TestProgressModel_MarkDoneActions(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		taskID     string
		taskLabel  string
		wantAction string
		wantStatus string
	}{
		{
			name:       "install action",
			taskID:     "install-nvim",
			taskLabel:  "Installing neovim",
			wantAction: "install",
			wantStatus: "installed",
		},
		{
			name:       "setup action",
			taskID:     "setup-nvim",
			taskLabel:  "Setting up neovim",
			wantAction: "configure",
			wantStatus: "configured",
		},
		{
			name:       "cleanup action",
			taskID:     "cleanup-nvim",
			taskLabel:  "Updating neovim",
			wantAction: "cleanup",
			wantStatus: "cleaned",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			m := newProgressModel()
			m.markActive(tt.taskID, tt.taskLabel)
			m.markDone(tt.taskID, nil)

			if len(m.steps) != 1 {
				t.Fatalf("steps = %d, want 1", len(m.steps))
			}
			if m.steps[0].action != tt.wantAction {
				t.Errorf(
					"action = %q, want %q",
					m.steps[0].action, tt.wantAction,
				)
			}
			if m.steps[0].status != tt.wantStatus {
				t.Errorf(
					"status = %q, want %q",
					m.steps[0].status, tt.wantStatus,
				)
			}
		})
	}
}

func TestProgressModel_MarkSkipped(t *testing.T) {
	t.Parallel()
	m := newProgressModel()

	m.markSkipped("install-yay", "Installing yay", "not on Arch")

	if m.toolStatuses["yay"] != statusSkipped {
		t.Errorf(
			"yay status = %v, want statusSkipped",
			m.toolStatuses["yay"],
		)
	}
	if m.doneCount != 1 {
		t.Errorf("doneCount = %d, want 1", m.doneCount)
	}
	if len(m.steps) != 1 {
		t.Fatalf("steps = %d, want 1", len(m.steps))
	}
	if m.steps[0].success {
		t.Error("skipped step should not be successful")
	}
}

func TestProgressModel_MarkSkippedEmptyLabel(t *testing.T) {
	t.Parallel()
	m := newProgressModel()

	// When label strips to empty, should fall back to ID.
	m.markSkipped("install-yay", "", "not on Arch")

	if _, ok := m.toolStatuses["install-yay"]; !ok {
		t.Error("should fall back to task ID when label is empty")
	}
}

func TestProgressModel_AllFinished(t *testing.T) {
	t.Parallel()

	t.Run("empty is not finished", func(t *testing.T) {
		t.Parallel()
		m := newProgressModel()
		if m.allFinished() {
			t.Error("empty model should not be finished")
		}
	})

	t.Run("active is not finished", func(t *testing.T) {
		t.Parallel()
		m := newProgressModel()
		m.markActive("install-zsh", "Installing zsh")
		if m.allFinished() {
			t.Error("model with active task should not be finished")
		}
	})

	t.Run("all done is finished", func(t *testing.T) {
		t.Parallel()
		m := newProgressModel()
		m.markActive("install-zsh", "Installing zsh")
		m.markDone("install-zsh", nil)
		if !m.allFinished() {
			t.Error("model should be finished when all done")
		}
	})

	t.Run("mixed done and failed is finished", func(t *testing.T) {
		t.Parallel()
		m := newProgressModel()
		m.markActive("install-zsh", "Installing zsh")
		m.markActive("install-tmux", "Installing tmux")
		m.markDone("install-zsh", nil)
		m.markDone("install-tmux", errors.New("fail"))
		if !m.allFinished() {
			t.Error("model should be finished with done+failed")
		}
	})

	t.Run("skipped counts as finished", func(t *testing.T) {
		t.Parallel()
		m := newProgressModel()
		m.markSkipped("install-yay", "Installing yay", "not Arch")
		if !m.allFinished() {
			t.Error("skipped-only model should be finished")
		}
	})
}

func TestProgressModel_RemoveActive(t *testing.T) {
	t.Parallel()
	m := newProgressModel()
	m.markActive("install-zsh", "Installing zsh")
	m.markActive("install-tmux", "Installing tmux")

	if len(m.active) != 2 {
		t.Fatalf("active = %d, want 2", len(m.active))
	}

	m.removeActive("install-zsh")
	if len(m.active) != 1 {
		t.Errorf("active = %d, want 1", len(m.active))
	}
	if m.active[0].id != "install-tmux" {
		t.Errorf("remaining active = %q, want install-tmux", m.active[0].id)
	}
}

func TestProgressModel_NameForID(t *testing.T) {
	t.Parallel()
	m := newProgressModel()

	// Unknown ID returns the ID itself.
	if got := m.nameForID("unknown-id"); got != "unknown-id" {
		t.Errorf("nameForID(unknown) = %q, want 'unknown-id'", got)
	}

	// Known ID returns the label.
	m.markActive("install-zsh", "Installing zsh")
	if got := m.nameForID("install-zsh"); got != "zsh" {
		t.Errorf("nameForID(install-zsh) = %q, want 'zsh'", got)
	}
}

func TestStripLabelPrefix(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		label string
		want  string
	}{
		{
			name:  "Installing prefix",
			label: "Installing zsh",
			want:  "zsh",
		},
		{
			name:  "Setting up prefix",
			label: "Setting up neovim",
			want:  "neovim",
		},
		{
			name:  "Updating prefix",
			label: "Updating tmux",
			want:  "tmux",
		},
		{
			name:  "no prefix",
			label: "zsh",
			want:  "zsh",
		},
		{
			name:  "empty string",
			label: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := stripLabelPrefix(tt.label); got != tt.want {
				t.Errorf(
					"stripLabelPrefix(%q) = %q, want %q",
					tt.label, got, tt.want,
				)
			}
		})
	}
}

func TestProgressModel_View(t *testing.T) {
	t.Parallel()

	t.Run("empty model", func(t *testing.T) {
		t.Parallel()
		m := newProgressModel()
		view := m.View(80)
		if view == "" {
			t.Error("View() returned empty string")
		}
	})

	t.Run("with active tools", func(t *testing.T) {
		t.Parallel()
		m := newProgressModel()
		m.markActive("install-zsh", "Installing zsh")
		m.markActive("install-tmux", "Installing tmux")
		view := m.View(80)
		if view == "" {
			t.Error("View() returned empty string")
		}
	})

	t.Run("with done tools", func(t *testing.T) {
		t.Parallel()
		m := newProgressModel()
		m.markActive("install-zsh", "Installing zsh")
		m.markDone("install-zsh", nil)
		m.done = true
		view := m.View(80)
		if view == "" {
			t.Error("View() returned empty string")
		}
	})

	t.Run("narrow width", func(t *testing.T) {
		t.Parallel()
		m := newProgressModel()
		m.markActive("install-zsh", "Installing zsh")
		view := m.View(50)
		if view == "" {
			t.Error("View() returned empty for narrow width")
		}
	})
}

func TestProgressModel_StatusIcon(t *testing.T) {
	t.Parallel()
	m := newProgressModel()

	statuses := []toolStatus{
		statusQueued,
		statusActive,
		statusDone,
		statusFailed,
		statusSkipped,
	}
	for _, s := range statuses {
		icon := m.statusIcon(s)
		if icon == "" {
			t.Errorf("statusIcon(%v) returned empty string", s)
		}
	}
}

func TestProgressModel_VerboseToggle(t *testing.T) {
	t.Parallel()
	m := newProgressModel()
	m.verbose = true

	m, _ = m.Update(keyPress('v'))
	if !m.expandedVerbose {
		t.Error("v should toggle expandedVerbose on")
	}

	m, _ = m.Update(keyPress('v'))
	if m.expandedVerbose {
		t.Error("v should toggle expandedVerbose off")
	}
}

func TestProgressModel_VerboseToggleIgnoredWhenNotVerbose(t *testing.T) {
	t.Parallel()
	m := newProgressModel()
	m.verbose = false

	m, _ = m.Update(keyPress('v'))
	if m.expandedVerbose {
		t.Error(
			"v should not toggle expandedVerbose when verbose is off",
		)
	}
}

func TestProgressModel_ViewWithVerbose(t *testing.T) {
	t.Parallel()
	m := newProgressModel()
	m.verbose = true
	m.markActive("install-zsh", "Installing zsh")
	m.recentLines = []string{
		"Checking for updates...",
		"Downloading package...",
		"Installing...",
	}

	view := m.View(80)
	if view == "" {
		t.Error("View() with verbose returned empty")
	}
}

func TestProgressModel_ViewWithExpandedVerbose(t *testing.T) {
	t.Parallel()
	m := newProgressModel()
	m.verbose = true
	m.expandedVerbose = true
	m.markActive("install-zsh", "Installing zsh")
	m.recentLines = []string{
		"Line 1", "Line 2", "Line 3", "Line 4",
		"Line 5", "Line 6", "Line 7", "Line 8",
		"Line 9", "Line 10",
	}

	view := m.View(80)
	if view == "" {
		t.Error("View() with expanded verbose returned empty")
	}
}

func TestProgressModel_ViewWithManyRecentLines(t *testing.T) {
	t.Parallel()
	m := newProgressModel()
	m.verbose = true
	m.markActive("install-zsh", "Installing zsh")

	// More than compactVerboseLines.
	for i := 0; i < 20; i++ {
		m.recentLines = append(
			m.recentLines,
			"A line of verbose output here",
		)
	}

	view := m.View(80)
	if view == "" {
		t.Error("View() with many recent lines returned empty")
	}
}

func TestProgressModel_ViewWithLongToolName(t *testing.T) {
	t.Parallel()
	m := newProgressModel()
	m.markActive(
		"install-very-long-tool-name-that-exceeds-column-width",
		"Installing very-long-tool-name-that-exceeds-column-width",
	)

	view := m.View(80)
	if view == "" {
		t.Error("View() with long tool name returned empty")
	}
}

func TestProgressModel_ViewWithLongVerboseLine(t *testing.T) {
	t.Parallel()
	m := newProgressModel()
	m.verbose = true
	m.markActive("install-zsh", "Installing zsh")

	longLine := ""
	for i := 0; i < 200; i++ {
		longLine += "x"
	}
	m.recentLines = []string{longLine}

	view := m.View(80)
	if view == "" {
		t.Error("View() with long verbose line returned empty")
	}
}

func TestProgressModel_Init(t *testing.T) {
	t.Parallel()
	m := newProgressModel()
	cmd := m.Init()
	if cmd == nil {
		t.Error("Init() should return a spinner tick command")
	}
}

func TestProgressModel_UpdateNonKeyMsg(t *testing.T) {
	t.Parallel()
	m := newProgressModel()
	m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 40})
	// Should not panic; model unchanged.
}

func TestProgressModel_ViewManyTools(t *testing.T) {
	t.Parallel()
	m := newProgressModel()

	// Add many tools to test grid layout.
	tools := []string{
		"zsh", "tmux", "neovim", "starship", "atuin",
		"ghostty", "yazi", "lazygit", "eza", "bat",
		"fd", "ripgrep", "fzf", "delta", "zoxide",
	}
	for _, tool := range tools {
		m.markActive(
			"install-"+tool,
			"Installing "+tool,
		)
	}
	for _, tool := range tools[:10] {
		m.markDone("install-"+tool, nil)
	}

	view := m.View(80)
	if view == "" {
		t.Error("View() with many tools returned empty")
	}
}

func TestProgressModel_ViewNarrowWidth(t *testing.T) {
	t.Parallel()
	m := newProgressModel()
	m.markActive("install-zsh", "Installing zsh")

	// Narrow width triggers 2-column layout.
	view := m.View(50)
	if view == "" {
		t.Error("View() at narrow width returned empty")
	}
}

func TestProgressModel_UpdateSpinnerTick(t *testing.T) {
	t.Parallel()
	m := newProgressModel()

	// Send a spinner tick message.
	msg := spinner.TickMsg{Time: time.Now()}
	m, cmd := m.Update(msg)

	// Spinner should process the tick (cmd may be nil or another tick).
	_ = cmd
	_ = m // Just verify no panic.
}

func TestProgressModel_UpdateProgressFrame(t *testing.T) {
	t.Parallel()
	m := newProgressModel()

	// Send a progress frame message.
	msg := progress.FrameMsg{}
	m, cmd := m.Update(msg)

	_ = cmd
	_ = m // Just verify no panic.
}

func TestProgressModel_ViewWithStartedAt(t *testing.T) {
	t.Parallel()
	m := newProgressModel()
	m.startedAt = time.Now().Add(-30 * time.Second)
	m.markActive("install-zsh", "Installing zsh")

	view := m.View(80)
	if view == "" {
		t.Error("View() with startedAt returned empty")
	}
}
