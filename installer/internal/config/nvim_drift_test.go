package config

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// initGitRepo initializes a bare-minimum git repo in dir, writes a
// single file, and commits it. Returns the HEAD rev.
func initGitRepo(t *testing.T, dir string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available in test environment")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		// Keep tests hermetic — no global config mutation.
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t",
			"GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t",
			"GIT_COMMITTER_EMAIL=t@t",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	run("init", "-q", "-b", "main")
	if err := os.WriteFile(
		filepath.Join(dir, "a"), []byte("x"), 0o644,
	); err != nil {
		t.Fatal(err)
	}
	run("add", "a")
	run("-c", "commit.gpgsign=false", "commit", "-qm", "init")
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	return string(out[:len(out)-1])
}

// TestNvimDriftedClonesDetectsMismatch seeds a plugin clone at
// commit X and records commit Y in the lockfile — the drift scan
// should flag the dir for removal.
func TestNvimDriftedClonesDetectsMismatch(t *testing.T) {
	home := t.TempDir()
	optDir := filepath.Join(home, ".local", "share", "nvim",
		"site", "pack", "core", "opt", "fakeplugin")
	head := initGitRepo(t, optDir)

	// Record a different (fake) rev in the lockfile.
	lockPath := filepath.Join(home, "lock.json")
	bogusRev := "0000000000000000000000000000000000000000"
	if head == bogusRev {
		t.Fatalf("test setup: real HEAD collides with bogus rev")
	}
	if err := os.WriteFile(lockPath, []byte(
		`{"plugins":{"fakeplugin":{"rev":"`+bogusRev+`"}}}`,
	), 0o644); err != nil {
		t.Fatal(err)
	}

	drifted, err := nvimDriftedClones(lockPath, home)
	if err != nil {
		t.Fatal(err)
	}
	if len(drifted) != 1 || drifted[0] != optDir {
		t.Fatalf("expected drift on %s, got %v", optDir, drifted)
	}
}

// TestNvimDriftedClonesMatchesRev confirms a clone at the exact
// locked rev is NOT flagged (no redundant wipes).
func TestNvimDriftedClonesMatchesRev(t *testing.T) {
	home := t.TempDir()
	optDir := filepath.Join(home, ".local", "share", "nvim",
		"site", "pack", "core", "opt", "fakeplugin")
	head := initGitRepo(t, optDir)

	lockPath := filepath.Join(home, "lock.json")
	if err := os.WriteFile(lockPath, []byte(
		`{"plugins":{"fakeplugin":{"rev":"`+head+`"}}}`,
	), 0o644); err != nil {
		t.Fatal(err)
	}

	drifted, err := nvimDriftedClones(lockPath, home)
	if err != nil {
		t.Fatal(err)
	}
	if len(drifted) != 0 {
		t.Fatalf("expected no drift, got %v", drifted)
	}
}

// TestNvimDriftedClonesMissingLockfile handles the fresh-machine
// case — no lockfile means nothing to compare, not an error.
func TestNvimDriftedClonesMissingLockfile(t *testing.T) {
	home := t.TempDir()
	drifted, err := nvimDriftedClones(
		filepath.Join(home, "nope.json"), home,
	)
	if err != nil {
		t.Fatalf("expected nil err on missing lockfile, got %v", err)
	}
	if len(drifted) != 0 {
		t.Fatalf("expected no drift, got %v", drifted)
	}
}

// TestNvimDriftedClonesSkipsMissingClone — the lockfile records a
// plugin but its clone dir doesn't exist yet. The add step will
// clone fresh; drift scan should leave it alone.
func TestNvimDriftedClonesSkipsMissingClone(t *testing.T) {
	home := t.TempDir()
	lockPath := filepath.Join(home, "lock.json")
	if err := os.WriteFile(lockPath, []byte(
		`{"plugins":{"notcloned":{"rev":"deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"}}}`,
	), 0o644); err != nil {
		t.Fatal(err)
	}
	drifted, err := nvimDriftedClones(lockPath, home)
	if err != nil {
		t.Fatal(err)
	}
	if len(drifted) != 0 {
		t.Fatalf("expected no drift for missing clone, got %v", drifted)
	}
}

// TestNvimDriftedClonesSkipsBlankRev — some lockfile entries lack
// a rev (fresh plugins never vim.pack.update'd). Skip them rather
// than crash.
func TestNvimDriftedClonesSkipsBlankRev(t *testing.T) {
	home := t.TempDir()
	optDir := filepath.Join(home, ".local", "share", "nvim",
		"site", "pack", "core", "opt", "fakeplugin")
	_ = initGitRepo(t, optDir)

	lockPath := filepath.Join(home, "lock.json")
	if err := os.WriteFile(lockPath, []byte(
		`{"plugins":{"fakeplugin":{"rev":""}}}`,
	), 0o644); err != nil {
		t.Fatal(err)
	}
	drifted, err := nvimDriftedClones(lockPath, home)
	if err != nil {
		t.Fatal(err)
	}
	if len(drifted) != 0 {
		t.Fatalf("expected no drift for blank rev, got %v", drifted)
	}
}
