package backup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestBackupFileDryRun covers the dryRun=true no-op branch.
func TestBackupFileDryRun(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	m := NewManager(true)
	path := filepath.Join(home, "x")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := m.BackupFile(path); err != nil {
		t.Fatalf("dry-run BackupFile: %v", err)
	}
	if m.Exists() {
		t.Fatal("dry-run should not create backup dir")
	}
}

// TestBackupFileMissingPath covers the "path doesn't exist → no-op"
// branch.
func TestBackupFileMissingPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	m := NewManager(false)
	if err := m.BackupFile(filepath.Join(home, "missing")); err != nil {
		t.Fatalf("missing path BackupFile: %v", err)
	}
	if m.Exists() {
		t.Fatal("missing path should not trigger backup dir creation")
	}
}

// TestCleanupNoOp covers the Exists=false Cleanup branch.
func TestCleanupNoOp(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	m := NewManager(false)
	if err := m.Cleanup(); err != nil {
		t.Fatalf("Cleanup on empty manager: %v", err)
	}
}

// TestBackupDestFallsBackOutsideHome covers the "path not under $HOME
// → basename fallback" branch.
func TestBackupDestFallsBackOutsideHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	m := NewManager(false)
	got := m.backupDest("/tmp/weird/file")
	if filepath.Base(got) != "file" {
		t.Fatalf("expected basename fallback, got %q", got)
	}
	// And confirm the in-home path preserves structure.
	got = m.backupDest(filepath.Join(home, ".config", "x"))
	if !strings.Contains(got, filepath.Join(".config", "x")) {
		t.Fatalf("expected home-relative path, got %q", got)
	}
}

// TestRestoreMissingBackupDir covers the "backup dir not found" error.
func TestRestoreMissingBackupDir(t *testing.T) {
	if err := Restore(filepath.Join(t.TempDir(), "nope"), nil, false); err == nil ||
		!strings.Contains(err.Error(), "backup directory not found") {
		t.Fatalf("expected not-found error, got %v", err)
	}
}

// TestRestoreDryRunAndMissingSource covers the dry-run skip and the
// "source doesn't exist → continue" branches.
func TestRestoreDryRunAndMissingSource(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	backupDir := filepath.Join(home, ".dotfiles-backup-x")
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Source not present in backup → continue.
	if err := Restore(backupDir, []string{filepath.Join(home, "ghost")}, false); err != nil {
		t.Fatalf("Restore with missing source: %v", err)
	}

	// Dry-run: existing source, but no write should occur.
	relDir := filepath.Join(backupDir, ".config", "x")
	if err := os.MkdirAll(relDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(relDir, "x.conf"), []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(home, ".config", "x", "x.conf")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Restore(backupDir, []string{target}, true); err != nil {
		t.Fatalf("dry-run Restore: %v", err)
	}
	if data, _ := os.ReadFile(target); string(data) != "original" {
		t.Fatalf("dry-run must not overwrite target, got %q", data)
	}
}

// TestRestoreOutsideHomeSkips covers the "target not under $HOME → skip"
// branch.
func TestRestoreOutsideHomeSkips(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	backupDir := filepath.Join(home, ".dotfiles-backup-outside")
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// /etc/x can't be restored; the Rel check returns a "../" prefix and
	// Restore continues without error.
	if err := Restore(backupDir, []string{"/etc/x"}, false); err != nil {
		t.Fatalf("Restore outside home: %v", err)
	}
}

// TestListBackupsSkipsNonDirectories covers the "glob match is a file"
// branch where a non-dir match is ignored.
func TestListBackupsSkipsNonDirectories(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	// Plant a FILE matching the glob — should be skipped.
	if err := os.WriteFile(filepath.Join(home, ".dotfiles-backup-notadir"),
		[]byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	backups, err := ListBackups()
	if err != nil {
		t.Fatalf("ListBackups: %v", err)
	}
	if len(backups) != 0 {
		t.Fatalf("expected no backups (file glob match), got %#v", backups)
	}
}
