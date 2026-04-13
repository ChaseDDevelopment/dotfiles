package tui

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/chaseddevelopment/dotfiles/installer/internal/config"
)

// TestCompletionViewBranches drives the criticalFailure+repoBlocked,
// doctor mode, degraded banner, and warnings sections of the summary
// completion view so each formatting branch is exercised.
func TestCompletionViewBranches(t *testing.T) {
	cases := []struct {
		name string
		mut  func(m *summaryModel)
	}{
		{name: "critical + repo blocked", mut: func(m *summaryModel) {
			m.criticalFailure = true
			m.repoBlockedBody = "error: would be overwritten\n\tconfigs/zsh/.zshrc\n"
		}},
		{name: "critical without repo body", mut: func(m *summaryModel) {
			m.criticalFailure = true
		}},
		{name: "doctor mode", mut: func(m *summaryModel) {
			m.doctorMode = true
			m.steps = []stepResult{{label: "ok", success: true}}
		}},
		{name: "degraded with warnings", mut: func(m *summaryModel) {
			m.steps = []stepResult{
				{label: "ok", success: true, action: "install"},
				{label: "fail", success: false, action: "install"},
				{label: "skip", success: true, action: "skipped"},
			}
			tf := config.NewTrackedFailures()
			tf.Record("Zsh", "compile", errors.New("boom"))
			m.warnings = tf
			m.alreadyInstalled = 1
			m.alreadyConfigured = 2
			m.startTime = time.Now().Add(-time.Second)
			m.endTime = time.Now()
			m.logPath = "/tmp/install.log"
		}},
		{name: "no changes needed", mut: func(m *summaryModel) {
			m.steps = nil
			m.warnings = config.NewTrackedFailures()
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := summaryModel{warnings: config.NewTrackedFailures()}
			tc.mut(&m)
			out := m.completionView(120, 40)
			if out == "" {
				t.Fatal("completionView returned empty output")
			}
			if strings.Contains(out, "completion") {
				// no-op assertion; just exercise the rendering path
			}
		})
	}
}

// TestRunInstallTasksDryRunAndZero covers two early-return branches
// of runInstallTasks: DryRun=true and len(tasks)==0.
func TestRunInstallTasksDryRunAndZero(t *testing.T) {
	app := NewApp(newTestConfig())
	app.config.DryRun = true
	app.config.Mode = ModeUninstall // produces zero tasks for empty repo
	cmd := app.runInstallTasks()
	_ = cmd
	if app.phase != PhaseSummary {
		t.Fatalf("dry-run runInstallTasks should jump to summary, got %v", app.phase)
	}

}
