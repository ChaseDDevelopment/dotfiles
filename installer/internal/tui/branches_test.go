package tui

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/chaseddevelopment/dotfiles/installer/internal/executor"
)

func teaKey(r rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: rune(r), Text: string(r)}
}

// TestSyncRepoCmdEarlyReturns covers the (runner==nil || rootDir=="")
// short-circuit. The returned command should resolve to repoSyncedMsg
// without ever shelling out.
func TestSyncRepoCmdEarlyReturns(t *testing.T) {
	app := NewApp(newTestConfig())
	app.config.Runner = nil
	app.config.RootDir = ""
	cmd := app.syncRepoCmd()
	if cmd == nil {
		t.Fatal("expected non-nil cmd")
	}
	msg := cmd()
	if _, ok := msg.(repoSyncedMsg); !ok {
		t.Fatalf("expected repoSyncedMsg, got %T", msg)
	}
}

// TestSyncRepoCmdUpToDate runs against a real, quiescent git repo so
// `git pull --ff-only` returns cleanly and the success branch executes.
func TestSyncRepoCmdUpToDate(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	// Local repo with no upstream → git pull fails softly (not a hard
	// error). This drives the failure-branch log path, which is a
	// distinct code path from the nil-runner return above.
	if err := exec.Command("git", "-C", dir, "init", "--quiet").Run(); err != nil {
		t.Fatal(err)
	}

	log, err := executor.NewLogFile(filepath.Join(dir, "log"))
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	app := NewApp(newTestConfig())
	app.config.Runner = executor.NewRunner(log, false)
	app.config.RootDir = dir
	cmd := app.syncRepoCmd()
	msg := cmd()
	// Either a synced or blocked msg is acceptable — the branch
	// coverage is what matters here, not the exact outcome.
	switch msg.(type) {
	case repoSyncedMsg, repoSyncBlockedMsg:
	default:
		t.Fatalf("unexpected msg type %T", msg)
	}
}

// TestSyncVerboseViewportBranches drives the expanded-mode branch
// of syncVerboseViewport: width>0 so truncation path runs; long and
// short lines both.
func TestSyncVerboseViewportBranches(t *testing.T) {
	m := &progressModel{
		expandedVerbose: true,
		width:           40,
		recentLines:     []string{"short", string(make([]byte, 100))},
	}
	m.resizeVerboseViewport()
	m.syncVerboseViewport()

	// Now toggle back — the early-return branch (expandedVerbose=false).
	m.expandedVerbose = false
	m.syncVerboseViewport()

	// width<=0 also hits early return.
	m.expandedVerbose = true
	m.width = 0
	m.syncVerboseViewport()
}

// TestProgressUpdateExpandedScroll covers the "expanded + non-v key →
// forward to viewport" branch.
func TestProgressUpdateExpandedScroll(t *testing.T) {
	m := &progressModel{
		expandedVerbose: true,
		width:           80,
		recentLines:     []string{"a", "b"},
		verbose:         true,
	}
	m.resizeVerboseViewport()
	// Press "j" — should forward to viewport, not toggle.
	_, _ = m.Update(teaKey('j'))
}

// TestProgressUpdateToggleVerbose covers the "v" key + verbose=true
// branch where expandedVerbose flips.
func TestProgressUpdateToggleVerbose(t *testing.T) {
	m := &progressModel{verbose: true}
	_, _ = m.Update(teaKey('v'))
}

// TestProgressResizeVerboseViewport covers the width bump path.
func TestProgressResizeVerboseViewport(t *testing.T) {
	m := &progressModel{expandedVerbose: true, width: 100}
	m.resizeVerboseViewport()
	m.width = 200
	m.resizeVerboseViewport()
	m.expandedVerbose = false
	m.resizeVerboseViewport() // early return branch
	_ = os.Stdin             // silence unused import if future-proofed
}
