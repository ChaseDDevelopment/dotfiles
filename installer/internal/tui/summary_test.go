package tui

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/chaseddevelopment/dotfiles/installer/internal/config"
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

// TestVisibleSteps_HidesNoOpSweep pins the rule that a successful
// sweep-repo-drift step disappears from the Results table when the
// run recorded no Repo warning — that combination means the sweep
// ran, found no drift, and the row would otherwise surface a
// misleading "1 installed" on a clean no-op run.
func TestVisibleSteps_HidesNoOpSweep(t *testing.T) {
	t.Parallel()
	m := newSummaryModel(false)
	m.warnings = config.NewTrackedFailures()
	m.steps = []stepResult{
		{
			id: "zsh", label: "zsh", action: "install",
			status: "installed", success: true,
		},
		{
			id: "sweep-repo-drift", label: "Restoring repo configs",
			action: "sweep", status: "swept", success: true,
		},
	}

	visible := m.visibleSteps()
	if len(visible) != 1 || visible[0].id != "zsh" {
		t.Fatalf(
			"no-op sweep should be filtered out; got %+v", visible,
		)
	}

	// When the sweep actually restored files, it records a Repo
	// warning. The row must then reappear so the user sees the work.
	m.warnings.Record(
		"Repo", "drift sweep",
		errors.New("originals saved to /tmp/backup"),
	)
	visible = m.visibleSteps()
	if len(visible) != 2 {
		t.Fatalf(
			"sweep should be visible when Repo warning exists; "+
				"got %+v", visible,
		)
	}
}

// TestCompletionView_ShowsAlreadyManifest verifies the manifest block
// is rendered when names are present, so a clean no-op run gives the
// user a verifiable list of what was inspected rather than just a
// count pill.
func TestCompletionView_ShowsAlreadyManifest(t *testing.T) {
	t.Parallel()
	m := newSummaryModel(false)
	m.warnings = config.NewTrackedFailures()
	m.alreadyInstalled = 3
	m.alreadyInstalledNames = []string{"bat", "eza", "fd"}
	m.alreadyConfigured = 2
	m.alreadyConfiguredNames = []string{"zsh", "tmux"}

	view := m.completionView(100, 40)
	for _, want := range []string{
		"Already installed (3)",
		"bat", "eza", "fd",
		"Already configured (2)",
		"zsh", "tmux",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("completion view missing %q", want)
		}
	}
}

// TestResultsColumnWidths covers the narrow and long-label code
// paths, ensuring the status column never collapses below its
// minimum and the name column tracks the longest label up to the
// 40-char cap.
func TestResultsColumnWidths(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		w        int
		steps    []stepResult
		wantComp int
	}{
		{
			name:     "short labels use floor",
			w:        100,
			steps:    []stepResult{{label: "zsh"}},
			wantComp: 14,
		},
		{
			name: "grows to fit longest label",
			w:    100,
			steps: []stepResult{
				{label: "Restoring repo configs"},
			},
			wantComp: 24,
		},
		{
			name: "cap at 40 for absurd labels",
			w:    200,
			steps: []stepResult{
				{label: strings.Repeat("x", 80)},
			},
			wantComp: 40,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			compW, statusW, durationW := resultsColumnWidths(
				tc.w, tc.steps,
			)
			if compW != tc.wantComp {
				t.Errorf(
					"compW = %d, want %d", compW, tc.wantComp,
				)
			}
			if statusW < 12 {
				t.Errorf("statusW = %d, must be >= 12", statusW)
			}
			if durationW != 8 {
				t.Errorf("durationW = %d, want 8", durationW)
			}
		})
	}
}
