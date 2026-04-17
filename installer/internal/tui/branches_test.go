package tui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

// gitShimDir writes a PATH-shim `git` that echoes stdout/exit as
// configured and returns the directory so tests can inject it into
// the process PATH via t.Setenv.
//
// Pattern lifted from pkgmgr/apt_exec_test.go (writeExec + t.Setenv).
func gitShimDir(t *testing.T, stdout string, exitCode int) string {
	t.Helper()
	dir := t.TempDir()
	script := "#!/bin/sh\n"
	if stdout != "" {
		// Use printf so embedded newlines render literally.
		script += "printf '%s' " + shellQuote(stdout) + "\n"
	}
	script += "exit " + intToStr(exitCode) + "\n"
	if err := os.WriteFile(
		filepath.Join(dir, "git"), []byte(script), 0o755,
	); err != nil {
		t.Fatal(err)
	}
	return dir
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func intToStr(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var buf []byte
	for i > 0 {
		buf = append([]byte{byte('0' + i%10)}, buf...)
		i /= 10
	}
	if neg {
		buf = append([]byte{'-'}, buf...)
	}
	return string(buf)
}

// TestSyncRepoCmdUpToDateStub uses a PATH-shim git that exits 0 to
// drive the "clean fast-forward" success branch. Expects
// repoSyncedMsg (not repoSyncBlockedMsg).
func TestSyncRepoCmdUpToDateStub(t *testing.T) {
	shim := gitShimDir(t, "Already up to date.\n", 0)
	t.Setenv(
		"PATH",
		shim+string(os.PathListSeparator)+os.Getenv("PATH"),
	)
	dir := t.TempDir()
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
	if _, ok := msg.(repoSyncedMsg); !ok {
		t.Fatalf("expected repoSyncedMsg, got %T", msg)
	}
	// Fake git stdout should have been recorded in the log.
	got, _ := os.ReadFile(log.Path())
	if !strings.Contains(string(got), "Already up to date") {
		t.Fatalf("log missing stdout body: %s", got)
	}
}

// TestSyncRepoCmdBlockedOnDirty drives the hard-fail branch: git
// exits non-zero with "would be overwritten" output and the drift
// is OUTSIDE configs/ so auto-restore does not apply. Expects
// repoSyncBlockedMsg with the body carried through.
func TestSyncRepoCmdBlockedOnDirty(t *testing.T) {
	// Body mentioning a path outside configs/ so the auto-restore
	// scope check refuses and we fall to the block path.
	body := "error: Your local changes would be overwritten by merge:\n" +
		"\tinstaller/main.go\n" +
		"Please commit your changes or stash them.\n"
	shim := gitShimDir(t, body, 1)
	t.Setenv(
		"PATH",
		shim+string(os.PathListSeparator)+os.Getenv("PATH"),
	)
	dir := t.TempDir()
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
	blocked, ok := msg.(repoSyncBlockedMsg)
	if !ok {
		t.Fatalf("expected repoSyncBlockedMsg, got %T", msg)
	}
	if !strings.Contains(blocked.body, "would be overwritten") {
		t.Fatalf("blocked body missing git output: %q", blocked.body)
	}
}

// TestSyncVerboseViewportBranches drives the expanded-mode branch
// of syncVerboseViewport: width>0 so truncation path runs; long and
// short lines both. Now asserts the viewport content actually
// reflects the lines and the scroll position.
func TestSyncVerboseViewportBranches(t *testing.T) {
	m := &progressModel{
		expandedVerbose: true,
		width:           40,
		height:          30,
		recentLines:     []string{"short", strings.Repeat("x", 100)},
	}
	m.resizeVerboseViewport()
	m.syncVerboseViewport()

	content := m.verboseViewport.GetContent()
	if !strings.Contains(content, "short") {
		t.Fatalf("viewport missing 'short': %q", content)
	}
	// Long line should have been truncated with an ellipsis.
	if !strings.Contains(content, "…") {
		t.Fatalf("viewport long line should be truncated with …: %q", content)
	}

	// Width > 0 path must have set a positive viewport width.
	if m.verboseViewport.Width() <= 0 {
		t.Fatalf("viewport width not set: %d", m.verboseViewport.Width())
	}

	// Toggle back — early-return branch. Content should NOT change.
	m.expandedVerbose = false
	m.recentLines = append(m.recentLines, "ignored-line")
	m.syncVerboseViewport()
	if strings.Contains(m.verboseViewport.GetContent(), "ignored-line") {
		t.Fatal("syncVerboseViewport should no-op when not expanded")
	}

	// width<=0 also hits early return.
	m.expandedVerbose = true
	m.width = 0
	prev := m.verboseViewport.GetContent()
	m.syncVerboseViewport()
	if m.verboseViewport.GetContent() != prev {
		t.Fatal("syncVerboseViewport with width<=0 should no-op")
	}
}

// TestProgressUpdateExpandedScroll covers the "expanded + non-v key →
// forward to viewport" branch. Asserts the viewport YOffset advances
// when we inject scroll-down keys.
func TestProgressUpdateExpandedScroll(t *testing.T) {
	var lines []string
	for i := 0; i < 200; i++ {
		lines = append(lines, "line-"+intToStr(i))
	}
	m := progressModel{
		expandedVerbose: true,
		width:           80,
		height:          30,
		recentLines:     lines,
		verbose:         true,
	}
	m.resizeVerboseViewport()
	m.syncVerboseViewport()
	// Start from the top so we can observe a positive offset after
	// scrolling down. GotoBottom is invoked by syncVerboseViewport.
	m.verboseViewport.GotoTop()

	before := m.verboseViewport.YOffset()
	// Press "j" — should forward to viewport and scroll down.
	m2, _ := m.Update(teaKey('j'))
	after := m2.verboseViewport.YOffset()
	if after <= before {
		t.Fatalf(
			"expected scroll-down to advance YOffset, before=%d after=%d",
			before, after,
		)
	}
	// verbose flag should still be set (no accidental toggle).
	if !m2.expandedVerbose {
		t.Fatal("j must not collapse the viewport")
	}
}

// TestProgressUpdateToggleVerbose covers the "v" key + verbose=true
// branch where expandedVerbose flips. Now asserts the flip happens.
func TestProgressUpdateToggleVerbose(t *testing.T) {
	m := progressModel{
		verbose:     true,
		width:       80,
		height:      30,
		recentLines: []string{"one", "two"},
	}
	if m.expandedVerbose {
		t.Fatal("precondition: expandedVerbose should start false")
	}
	m2, _ := m.Update(teaKey('v'))
	if !m2.expandedVerbose {
		t.Fatal("v on verbose model should expand")
	}
	// A second press collapses.
	m3, _ := m2.Update(teaKey('v'))
	if m3.expandedVerbose {
		t.Fatal("second v press should collapse")
	}

	// v on a non-verbose model must NOT toggle.
	nv := progressModel{verbose: false}
	nv2, _ := nv.Update(teaKey('v'))
	if nv2.expandedVerbose {
		t.Fatal("v should not toggle when verbose=false")
	}
}

// TestProgressResizeVerboseViewport covers the width bump path.
// Asserts the viewport width follows the terminal width changes.
func TestProgressResizeVerboseViewport(t *testing.T) {
	// Start narrow so the clamp (contentWidth caps at 80, floors at 40)
	// doesn't mask the growth from subsequent resizes.
	m := &progressModel{expandedVerbose: true, width: 44, height: 40}
	m.resizeVerboseViewport()
	w1 := m.verboseViewport.Width()
	h1 := m.verboseViewport.Height()
	if w1 <= 0 || h1 <= 0 {
		t.Fatalf("viewport not sized on initial resize: w=%d h=%d", w1, h1)
	}

	m.width = 100
	m.resizeVerboseViewport()
	w2 := m.verboseViewport.Width()
	if w2 <= w1 {
		t.Fatalf("wider terminal should grow viewport: w1=%d w2=%d", w1, w2)
	}

	// Early-return branch when not expanded: dimensions must NOT
	// regress toward zero (i.e., the call is a genuine no-op).
	m.expandedVerbose = false
	m.resizeVerboseViewport()
	if m.verboseViewport.Width() != w2 {
		t.Fatal("resize while collapsed must not mutate viewport")
	}

	// And the zero-width early-return case.
	m.expandedVerbose = true
	m.width = 0
	m.resizeVerboseViewport()
	if m.verboseViewport.Width() != w2 {
		t.Fatal("resize with width<=0 must not mutate viewport")
	}

	_ = exec.Command // keep import usage explicit if future refactor
}
