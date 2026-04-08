package backup

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBackupPreservesPath(t *testing.T) {
	// Create two files with the same basename in different dirs.
	tmpDir := t.TempDir()
	dirA := filepath.Join(tmpDir, "a")
	dirB := filepath.Join(tmpDir, "b")
	os.MkdirAll(dirA, 0o755)
	os.MkdirAll(dirB, 0o755)

	fileA := filepath.Join(dirA, "config")
	fileB := filepath.Join(dirB, "config")
	os.WriteFile(fileA, []byte("content-a"), 0o644)
	os.WriteFile(fileB, []byte("content-b"), 0o644)

	// Override HOME so backupDest computes relative paths from tmpDir.
	origHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	bm := NewManager(false)
	// Override the backup dir to be under tmpDir for cleanup.
	bm.dir = filepath.Join(tmpDir, "backup")

	if err := bm.BackupFile(fileA); err != nil {
		t.Fatalf("BackupFile(a): %v", err)
	}
	if err := bm.BackupFile(fileB); err != nil {
		t.Fatalf("BackupFile(b): %v", err)
	}

	// Both files should exist in the backup dir with distinct paths.
	backedA := filepath.Join(bm.dir, "a", "config")
	backedB := filepath.Join(bm.dir, "b", "config")

	contentA, err := os.ReadFile(backedA)
	if err != nil {
		t.Fatalf("backup of a/config missing: %v", err)
	}
	contentB, err := os.ReadFile(backedB)
	if err != nil {
		t.Fatalf("backup of b/config missing: %v", err)
	}

	if string(contentA) != "content-a" {
		t.Errorf("a/config content = %q, want %q", contentA, "content-a")
	}
	if string(contentB) != "content-b" {
		t.Errorf("b/config content = %q, want %q", contentB, "content-b")
	}
}

func TestBackupSkipsDryRun(t *testing.T) {
	bm := NewManager(true)
	err := bm.BackupFile("/nonexistent/path")
	if err != nil {
		t.Errorf("dry run should return nil, got %v", err)
	}
}

func TestBackupSkipsNonexistent(t *testing.T) {
	bm := NewManager(false)
	err := bm.BackupFile("/nonexistent/path/that/does/not/exist")
	if err != nil {
		t.Errorf("nonexistent path should return nil, got %v", err)
	}
}
