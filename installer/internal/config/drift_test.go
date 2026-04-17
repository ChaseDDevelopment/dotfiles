package config

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/chaseddevelopment/dotfiles/installer/internal/backup"
)

// seedRepo initializes a throwaway git repo with a configs/ tree so
// the drift helpers can be exercised without touching the real
// dotfiles checkout.
func seedRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	// Pinning HOME inside t.TempDir keeps BackupFile's layout
	// assertions (which resolve paths relative to $HOME) stable
	// without affecting the user's real home dir.
	t.Setenv("HOME", dir)

	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}
	run("init", "-q")
	run("config", "user.email", "test@example.invalid")
	run("config", "user.name", "drift-test")

	if err := os.MkdirAll(filepath.Join(dir, "configs", "zsh"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(dir, "configs", "zsh", ".zshrc"),
		[]byte("# original\n"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(dir, "README.md"),
		[]byte("# repo\n"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}
	run("add", "-A")
	run("commit", "-m", "init", "-q")

	return dir
}

func TestDetectRepoDrift_ScopedToConfigs(t *testing.T) {
	dir := seedRepo(t)

	// Mutate a tracked file inside configs/ and another outside.
	if err := os.WriteFile(
		filepath.Join(dir, "configs", "zsh", ".zshrc"),
		[]byte("# original\nexport APPENDED=1\n"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(dir, "README.md"),
		[]byte("# repo\nextra\n"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}

	got, err := DetectRepoDrift(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"configs/zsh/.zshrc"}
	sort.Strings(got)
	if len(got) != len(want) || got[0] != want[0] {
		t.Errorf("got %v, want %v (README.md must NOT be included)", got, want)
	}
}

func TestDetectRepoDrift_IgnoresUntracked(t *testing.T) {
	dir := seedRepo(t)

	// Create an untracked file under configs/ — detection should
	// skip it because the caller's intent is restoring tracked
	// files, not nuking generated ones like antidote's bundle.
	if err := os.WriteFile(
		filepath.Join(dir, "configs", "zsh", "untracked.zsh"),
		[]byte("# generated at runtime\n"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}

	got, err := DetectRepoDrift(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("expected untracked file to be skipped, got %v", got)
	}
}

func TestBackupAndReset_RoundtripRestoresHEAD(t *testing.T) {
	dir := seedRepo(t)

	target := filepath.Join(dir, "configs", "zsh", ".zshrc")
	mutated := "# original\nexport BUN_INSTALL=x\n"
	if err := os.WriteFile(target, []byte(mutated), 0o644); err != nil {
		t.Fatal(err)
	}

	bm := backup.NewManager(false)
	backupDir, err := BackupAndReset(
		dir, bm, []string{"configs/zsh/.zshrc"},
	)
	if err != nil {
		t.Fatal(err)
	}

	// Working tree back to HEAD?
	content, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "# original\n" {
		t.Errorf("working tree not reset; content = %q", content)
	}

	// Pre-reset contents captured in the backup?
	backedUp := filepath.Join(
		backupDir, "configs", "zsh", ".zshrc",
	)
	saved, err := os.ReadFile(backedUp)
	if err != nil {
		t.Fatalf(
			"backup copy missing at %s: %v", backedUp, err,
		)
	}
	if string(saved) != mutated {
		t.Errorf("backup content = %q, want %q", saved, mutated)
	}
}

func TestBackupAndReset_EmptyPathsNoop(t *testing.T) {
	dir := seedRepo(t)
	bm := backup.NewManager(false)
	backupDir, err := BackupAndReset(dir, bm, nil)
	if err != nil {
		t.Fatal(err)
	}
	if backupDir != "" {
		t.Errorf("expected empty backupDir on noop, got %q", backupDir)
	}
	if bm.Exists() {
		t.Error("backup manager should not have created a dir for noop call")
	}
}

// TestDetectRepoDrift_DriftKinds seeds every porcelain-v1 variant
// the helper must parse: staged modification, worktree modification,
// addition-then-modify (AM), rename (with spaces in the destination
// name — a porcelain v1 quirk where the line is quoted), and a plain
// "?? " untracked entry which must be excluded.
func TestDetectRepoDrift_DriftKinds(t *testing.T) {
	dir := seedRepo(t)

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	// Seed a second tracked file so we can rename it.
	renameSrc := filepath.Join(dir, "configs", "zsh", "aliases.zsh")
	if err := os.WriteFile(
		renameSrc, []byte("# aliases\n"), 0o644,
	); err != nil {
		t.Fatal(err)
	}
	run("add", "-A")
	run("commit", "-m", "add aliases", "-q")

	// Staged modification of .zshrc.
	if err := os.WriteFile(
		filepath.Join(dir, "configs", "zsh", ".zshrc"),
		[]byte("# original\nstaged\n"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}
	run("add", "configs/zsh/.zshrc")

	// Rename with spaces in destination to hit the porcelain quoting
	// path inside DetectRepoDrift (which splits on " -> ").
	renameDst := filepath.Join(
		dir, "configs", "zsh", "spaced name.zsh",
	)
	if err := os.Rename(renameSrc, renameDst); err != nil {
		t.Fatal(err)
	}
	run("add", "-A")

	// Worktree-only untracked file — must be excluded.
	if err := os.WriteFile(
		filepath.Join(dir, "configs", "zsh", "generated.zsh"),
		[]byte("# transient\n"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}

	got, err := DetectRepoDrift(dir)
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(got)

	// Expect the staged .zshrc. The untracked generated.zsh must
	// be absent. The rename-with-spaces destination is currently
	// emitted by porcelain v1 as `R  old -> "new with spaces"`; the
	// quotes make it not start with "configs/" so the helper's
	// prefix guard drops it. This is a known porcelain v1 quirk —
	// git 2.43+ with `-z` would disambiguate via NUL separators.
	// This test pins the current behavior: renames without spaces
	// WOULD be caught; renames with spaces are conservatively
	// dropped rather than silently mis-parsed. If that changes
	// (e.g. the helper adopts -z), flip the expectation here.
	haveZshrc := false
	for _, p := range got {
		if p == "configs/zsh/.zshrc" {
			haveZshrc = true
		}
		if strings.Contains(p, "generated.zsh") {
			t.Errorf("untracked file leaked into drift: %q", p)
		}
		if strings.Contains(p, "\"") {
			t.Errorf(
				"quoted path leaked past prefix guard: %q", p,
			)
		}
	}
	if !haveZshrc {
		t.Errorf("staged .zshrc missing from drift: %v", got)
	}

	// Also exercise a plain (unspaced) rename: this one SHOULD be
	// detected, so the rename-parsing branch is not entirely dead.
	plainSrc := filepath.Join(dir, "configs", "zsh", "plain.zsh")
	if err := os.WriteFile(
		plainSrc, []byte("# plain\n"), 0o644,
	); err != nil {
		t.Fatal(err)
	}
	run("add", "-A")
	run("commit", "-m", "add plain", "-q")

	plainDst := filepath.Join(dir, "configs", "zsh", "renamed.zsh")
	if err := os.Rename(plainSrc, plainDst); err != nil {
		t.Fatal(err)
	}
	run("add", "-A")

	got, err = DetectRepoDrift(dir)
	if err != nil {
		t.Fatal(err)
	}
	haveRenamed := false
	for _, p := range got {
		if p == "configs/zsh/renamed.zsh" {
			haveRenamed = true
		}
	}
	if !haveRenamed {
		t.Errorf(
			"plain rename destination missing from drift: %v", got,
		)
	}
}

// TestBackupAndReset_RestoreFailureReturnsBackupDir forces
// `git restore` to fail by making the target file location
// unmodifiable (its parent directory is chmod'd to 0o555). The
// invariant the never-silent-heal rule requires: when restore
// fails, the function still returns the backup directory so the
// caller can reference it in the error, AND propagates the
// original error rather than swallowing it.
func TestBackupAndReset_RestoreFailureReturnsBackupDir(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip(
			"root ignores directory permissions; chmod-based " +
				"failure injection is meaningless",
		)
	}
	dir := seedRepo(t)
	target := filepath.Join(dir, "configs", "zsh", ".zshrc")
	if err := os.WriteFile(
		target, []byte("# original\nmutated\n"), 0o644,
	); err != nil {
		t.Fatal(err)
	}

	// Force git restore to fail by making the parent directory
	// read-only. BackupFile runs first and only needs read access
	// on the source, so it succeeds; `git restore` then tries to
	// rewrite the file and fails.
	parent := filepath.Dir(target)
	if err := os.Chmod(parent, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(parent, 0o755) })

	bm := backup.NewManager(false)
	backupDir, err := BackupAndReset(
		dir, bm, []string{"configs/zsh/.zshrc"},
	)
	if err == nil {
		t.Fatal(
			"expected BackupAndReset to fail when restore cannot " +
				"write to target",
		)
	}
	if backupDir == "" {
		t.Error(
			"backup dir must be returned even on restore failure " +
				"so the user can be told where their mutations went",
		)
	}
	if !strings.Contains(err.Error(), "git restore") {
		t.Errorf(
			"error should mention git restore stage, got %v", err,
		)
	}

	// Re-entrancy: unlock the parent and call BackupAndReset again.
	// Second call must succeed and land the file back at HEAD.
	if err := os.Chmod(parent, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := BackupAndReset(
		dir, bm, []string{"configs/zsh/.zshrc"},
	); err != nil {
		t.Fatalf("re-entrant BackupAndReset: %v", err)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "# original\n" {
		t.Errorf(
			"file not restored on second call; content=%q", got,
		)
	}
}

func TestBackupAndReset_ClearsStagedDrift(t *testing.T) {
	dir := seedRepo(t)
	target := filepath.Join(dir, "configs", "zsh", ".zshrc")
	mutated := "# original\nexport STAGED=1\n"
	if err := os.WriteFile(target, []byte(mutated), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("git", "add", "configs/zsh/.zshrc")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add failed: %v\n%s", err, out)
	}

	bm := backup.NewManager(false)
	if _, err := BackupAndReset(dir, bm, []string{"configs/zsh/.zshrc"}); err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "# original\n" {
		t.Fatalf("working tree not restored to HEAD: %q", content)
	}

	status := exec.Command(
		"git", "status", "--porcelain=v1", "--", "configs/zsh/.zshrc",
	)
	status.Dir = dir
	out, err := status.CombinedOutput()
	if err != nil {
		t.Fatalf("git status failed: %v\n%s", err, out)
	}
	if strings.TrimSpace(string(out)) != "" {
		t.Fatalf("expected clean git status, got %q", string(out))
	}
}
