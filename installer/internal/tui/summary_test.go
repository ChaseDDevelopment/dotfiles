package tui

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/chaseddevelopment/dotfiles/installer/internal/orchestrator"
)

func TestNewSummaryModel(t *testing.T) {
	t.Parallel()

	t.Run("dry run false", func(t *testing.T) {
		t.Parallel()
		m := newSummaryModel(false)
		if m.dryRun {
			t.Error("dryRun should be false")
		}
	})

	t.Run("dry run true", func(t *testing.T) {
		t.Parallel()
		m := newSummaryModel(true)
		if !m.dryRun {
			t.Error("dryRun should be true")
		}
	})
}

func TestSummaryModel_CompletionView(t *testing.T) {
	t.Parallel()
	m := newSummaryModel(false)
	m.steps = []stepResult{
		{
			label: "zsh", action: "install",
			status: "installed", success: true,
		},
		{
			label: "nvim", action: "configure",
			status: "configured", success: true,
		},
		{
			label: "rust", action: "install",
			status: "failed", success: false,
			err: errors.New("download failed"),
		},
	}
	m.startTime = time.Now().Add(-5 * time.Second)
	m.endTime = time.Now()

	view := m.View(80, 40)
	if view == "" {
		t.Error("completionView returned empty string")
	}
}

func TestSummaryModel_CompletionViewNoSteps(t *testing.T) {
	t.Parallel()
	m := newSummaryModel(false)
	view := m.View(80, 40)
	if view == "" {
		t.Error("completionView with no steps returned empty")
	}
}

func TestSummaryModel_DryRunView(t *testing.T) {
	t.Parallel()
	m := newSummaryModel(true)
	m.rows = []orchestrator.PlanRow{
		{Component: "zsh", Action: "install", Status: "would install"},
		{Component: "nvim", Action: "configure", Status: "would configure"},
		{
			Component: "tmux", Action: "install",
			Status: "already installed",
		},
	}

	view := m.View(80, 40)
	if view == "" {
		t.Error("dryRunView returned empty string")
	}
}

func TestSummaryModel_DryRunViewEmpty(t *testing.T) {
	t.Parallel()
	m := newSummaryModel(true)
	view := m.View(80, 40)
	if view == "" {
		t.Error("dryRunView with no rows returned empty")
	}
}

func TestSummaryModel_DoctorMode(t *testing.T) {
	t.Parallel()
	m := newSummaryModel(false)
	m.doctorMode = true
	m.steps = []stepResult{
		{
			label: "zsh", action: "install",
			status: "installed", success: true,
		},
	}

	view := m.View(80, 40)
	if view == "" {
		t.Error("doctor mode view returned empty string")
	}
}

func TestSummaryModel_CriticalFailure(t *testing.T) {
	t.Parallel()
	m := newSummaryModel(false)
	m.criticalFailure = true
	m.steps = []stepResult{
		{
			label: "brew", action: "install",
			status: "failed", success: false,
			err: errors.New("critical failure"),
		},
	}

	view := m.View(80, 40)
	if view == "" {
		t.Error("critical failure view returned empty string")
	}
}

func TestSummaryModel_WithLogPath(t *testing.T) {
	t.Parallel()
	m := newSummaryModel(false)
	m.logPath = "/tmp/install.log"
	m.steps = []stepResult{
		{
			label: "zsh", action: "install",
			status: "failed", success: false,
			err: errors.New("failed"),
		},
	}

	view := m.View(80, 40)
	if view == "" {
		t.Error("view with logPath returned empty string")
	}
}

func TestSummaryModel_WithAlreadyCounts(t *testing.T) {
	t.Parallel()
	m := newSummaryModel(false)
	m.alreadyInstalled = 3
	m.alreadyConfigured = 2

	view := m.View(80, 40)
	if view == "" {
		t.Error("view with already counts returned empty string")
	}
}

func TestSummaryModel_DryRunTableRows(t *testing.T) {
	t.Parallel()
	m := newSummaryModel(true)
	m.rows = []orchestrator.PlanRow{
		{Component: "zsh", Action: "install", Status: "would install"},
		{
			Component: "tmux", Action: "install",
			Status: "already installed",
		},
		{
			Component: "nvim", Action: "configure",
			Status: "would configure",
		},
	}

	body := m.dryRunTableRows(80)
	if body == "" {
		t.Error("dryRunTableRows returned empty string")
	}
}

func TestSummaryModel_DoctorTableRows(t *testing.T) {
	t.Parallel()
	m := newSummaryModel(false)
	m.doctorMode = true
	m.steps = []stepResult{
		{
			label: "zsh", action: "install",
			status: "installed", success: true,
		},
		{
			label: "rust", action: "install",
			status: "failed", success: false,
			err: errors.New("not found"),
		},
	}

	body := m.doctorTableRows(80)
	if body == "" {
		t.Error("doctorTableRows returned empty string")
	}
}

func TestSummaryModel_InitViewport(t *testing.T) {
	t.Parallel()
	m := newSummaryModel(true)
	m.rows = []orchestrator.PlanRow{
		{Component: "zsh", Action: "install", Status: "would install"},
	}

	// Should not panic.
	m.initViewport(80, 40)
	if !m.viewportReady {
		t.Error("viewportReady should be true after initViewport")
	}
}

func TestSummaryModel_InitDoctorViewportSmallList(t *testing.T) {
	t.Parallel()
	m := newSummaryModel(false)
	m.doctorMode = true
	m.steps = []stepResult{
		{
			label: "zsh", action: "install",
			status: "installed", success: true,
		},
	}

	// With very few steps, viewport should not be needed and the
	// rendered completion view must still surface the step label.
	m.initDoctorViewport(80, 40)
	if m.viewportReady {
		t.Fatal("viewportReady should stay false when steps fit inline")
	}
	out := m.completionView(80, 40)
	if !strings.Contains(out, "zsh") {
		t.Fatalf("completion view missing step label: %q", out)
	}
}

func TestSummaryModel_InitDoctorViewportLargeList(t *testing.T) {
	t.Parallel()
	m := newSummaryModel(false)
	m.doctorMode = true

	// Create enough steps to overflow.
	for i := 0; i < 50; i++ {
		m.steps = append(m.steps, stepResult{
			label: "tool", action: "install",
			status: "installed", success: true,
		})
	}

	m.initDoctorViewport(80, 30)
	if !m.viewportReady {
		t.Error("viewportReady should be true for large step list")
	}
}

func TestSummaryModel_ViewVariousWidths(t *testing.T) {
	t.Parallel()
	widths := []int{40, 60, 80, 120, 200}
	m := newSummaryModel(false)
	m.steps = []stepResult{
		{
			label: "zsh", action: "install",
			status: "installed", success: true,
		},
	}

	for _, w := range widths {
		view := m.View(w, 40)
		if view == "" {
			t.Errorf("View() returned empty at width %d", w)
		}
	}
}

func TestSummaryModel_DryRunViewVariousWidths(t *testing.T) {
	t.Parallel()
	widths := []int{40, 60, 80, 120}
	m := newSummaryModel(true)
	m.rows = []orchestrator.PlanRow{
		{Component: "zsh", Action: "install", Status: "would install"},
	}

	for _, w := range widths {
		view := m.View(w, 40)
		if view == "" {
			t.Errorf("dryRunView returned empty at width %d", w)
		}
	}
}

func TestSummaryModel_DryRunTableRowsAllStatuses(t *testing.T) {
	t.Parallel()
	m := newSummaryModel(true)
	m.rows = []orchestrator.PlanRow{
		{
			Component: "zsh", Action: "install",
			Status: "would install",
		},
		{
			Component: "nvim", Action: "configure",
			Status: "would configure",
		},
		{
			Component: "tmux", Action: "install",
			Status: "already installed",
		},
		{
			Component: "starship", Action: "configure",
			Status: "already configured",
		},
		{
			Component: "atuin", Action: "install",
			Status: "would replace",
		},
		{
			Component: "bat", Action: "install",
			Status: "outdated v0.1",
		},
		{
			Component: "eza", Action: "install",
			Status: "unknown status",
		},
	}

	body := m.dryRunTableRows(80)
	if body == "" {
		t.Error("dryRunTableRows returned empty for all statuses")
	}
}

func TestSummaryModel_CompletionViewAllActions(t *testing.T) {
	t.Parallel()
	m := newSummaryModel(false)
	m.steps = []stepResult{
		{
			label: "zsh", action: "install",
			status: "installed", success: true,
		},
		{
			label: "nvim", action: "configure",
			status: "configured", success: true,
		},
		{
			label: "tmux", action: "cleanup",
			status: "cleaned", success: true,
		},
		{
			label: "rust", action: "install",
			status: "failed", success: false,
			err: errors.New("download failed"),
		},
	}
	m.startTime = time.Now().Add(-10 * time.Second)
	m.endTime = time.Now()
	m.logPath = "/tmp/install.log"

	view := m.View(80, 40)
	if view == "" {
		t.Error("completionView with all actions returned empty")
	}
}

func TestSummaryModel_CompletionViewWithQuickStart(t *testing.T) {
	t.Parallel()
	m := newSummaryModel(false)
	m.steps = []stepResult{
		{
			label: "zsh", action: "install",
			status: "installed", success: true,
		},
		{
			label: "tmux", action: "install",
			status: "installed", success: true,
		},
		{
			label: "neovim", action: "install",
			status: "installed", success: true,
		},
	}

	view := m.View(80, 40)
	if view == "" {
		t.Error("completionView with quick start returned empty")
	}
}

func TestSummaryModel_DryRunViewWithViewport(t *testing.T) {
	t.Parallel()
	m := newSummaryModel(true)

	// Create many rows to trigger viewport.
	for i := 0; i < 50; i++ {
		m.rows = append(m.rows, orchestrator.PlanRow{
			Component: "tool", Action: "install",
			Status: "would install",
		})
	}

	// Init the viewport first.
	m.initViewport(80, 30)

	view := m.View(80, 30)
	if view == "" {
		t.Error("dryRunView with viewport returned empty")
	}
}

func TestSummaryModel_DryRunViewSmallHeight(t *testing.T) {
	t.Parallel()
	m := newSummaryModel(true)
	m.rows = []orchestrator.PlanRow{
		{Component: "zsh", Action: "install", Status: "would install"},
	}

	// Very small height.
	view := m.View(80, 10)
	if view == "" {
		t.Error("dryRunView with small height returned empty")
	}
}

func TestSummaryModel_DoctorTableRowsWithFailure(t *testing.T) {
	t.Parallel()
	m := newSummaryModel(false)
	m.doctorMode = true
	m.steps = []stepResult{
		{
			label: "zsh", action: "install",
			status: "installed", success: true,
		},
		{
			label: "nvim", action: "configure",
			status: "configured", success: true,
		},
		{
			label: "rust", action: "install",
			status: "failed", success: false,
			err: errors.New("a very long error message that exceeds the normal column width and should be truncated by the display logic"),
		},
		{
			label: "tmux", action: "cleanup",
			status: "cleaned", success: true,
		},
	}

	body := m.doctorTableRows(80)
	if body == "" {
		t.Error("doctorTableRows returned empty")
	}
}

func TestSummaryModel_CompletionViewErrorTruncation(t *testing.T) {
	t.Parallel()
	m := newSummaryModel(false)
	m.steps = []stepResult{
		{
			label: "rust", action: "install",
			status: "failed", success: false,
			err: errors.New("a very long error that should be truncated at some point because it is way too long for the table column"),
		},
	}

	view := m.View(80, 40)
	if view == "" {
		t.Error("completionView with long error returned empty")
	}
}

func TestSummaryModel_DoctorViewWithViewport(t *testing.T) {
	t.Parallel()
	m := newSummaryModel(false)
	m.doctorMode = true
	m.viewportReady = true

	// Need enough steps to make viewport content.
	for i := 0; i < 50; i++ {
		m.steps = append(m.steps, stepResult{
			label: "tool", action: "install",
			status: "installed", success: true,
		})
	}

	m.initDoctorViewport(80, 30)

	view := m.completionView(80, 30)
	if view == "" {
		t.Error("doctor completion view with viewport returned empty")
	}
}
