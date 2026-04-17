package backup

import (
	"os"
	"path/filepath"
	"testing"
)

func TestManagerHelpersAndCopyRecursive(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	srcDir := filepath.Join(home, "src")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	srcFile := filepath.Join(srcDir, "file.txt")
	if err := os.WriteFile(srcFile, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	linkPath := filepath.Join(home, "link.txt")
	if err := os.Symlink(srcFile, linkPath); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}

	m := NewManager(false)
	if m.Dir() == "" {
		t.Fatal("Dir() should not be empty")
	}
	if m.Exists() {
		t.Fatal("new manager should not exist yet")
	}
	if err := m.BackupFile(srcDir); err != nil {
		t.Fatalf("BackupFile(dir): %v", err)
	}
	if err := m.BackupFile(linkPath); err != nil {
		t.Fatalf("BackupFile(link): %v", err)
	}
	if !m.Exists() {
		t.Fatal("backup dir should exist after backup")
	}
	backedFile := filepath.Join(m.Dir(), "src", "file.txt")
	data, err := os.ReadFile(backedFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello" {
		t.Fatalf("backed file = %q", data)
	}
	if target, err := os.Readlink(filepath.Join(m.Dir(), "link.txt")); err != nil || target != srcFile {
		t.Fatalf("backed symlink target = %q err=%v", target, err)
	}
	if err := m.Cleanup(); err != nil {
		t.Fatalf("Cleanup: %v", err)
	}
	if m.Exists() {
		t.Fatal("backup dir should be removed after cleanup")
	}
}

func TestRestoreAndListBackups(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	backupDir := filepath.Join(home, ".dotfiles-backup-20260101-000000.000000001")
	if err := os.MkdirAll(filepath.Join(backupDir, ".config", "zsh"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backupDir, ".config", "zsh", ".zshrc"), []byte("restored"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backupDir, "tmux.conf"), []byte("legacy"), 0o644); err != nil {
		t.Fatal(err)
	}

	targetZsh := filepath.Join(home, ".config", "zsh", ".zshrc")
	if err := os.MkdirAll(filepath.Dir(targetZsh), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(targetZsh, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	targetTmux := filepath.Join(home, "tmux.conf")
	if err := os.WriteFile(targetTmux, []byte("old-tmux"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Restore(backupDir, []string{targetZsh, targetTmux}, false); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if data, _ := os.ReadFile(targetZsh); string(data) != "restored" {
		t.Fatalf("zsh restored data = %q", data)
	}
	if data, _ := os.ReadFile(targetTmux); string(data) != "legacy" {
		t.Fatalf("tmux restored data = %q", data)
	}

	newer := filepath.Join(home, ".dotfiles-backup-20260102-000000.000000001")
	if err := os.MkdirAll(newer, 0o755); err != nil {
		t.Fatal(err)
	}
	backups, err := ListBackups()
	if err != nil {
		t.Fatalf("ListBackups: %v", err)
	}
	if len(backups) < 2 || backups[0].Path != newer {
		t.Fatalf("unexpected backup ordering: %#v", backups)
	}
}
