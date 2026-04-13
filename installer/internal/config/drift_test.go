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
