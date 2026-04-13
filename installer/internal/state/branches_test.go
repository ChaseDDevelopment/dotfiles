package state

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestSaveMkdirFailure covers the MkdirAll-fails branch by placing
// the state file inside a path whose parent is a regular file
// (MkdirAll refuses to create a child of a file).
func TestSaveMkdirFailure(t *testing.T) {
	dir := t.TempDir()
	// Plant a regular file where the state dir would go.
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := NewStore(filepath.Join(blocker, "state.json"))
	if err := s.Save(); err == nil {
		t.Fatal("expected MkdirAll error on child-of-file path")
	}
}

// TestSaveTempCreateFailure covers the os.CreateTemp-fails branch by
// making the state directory read-only.
func TestSaveTempCreateFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("posix permissions semantics not portable to windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root bypasses read-only directory perms")
	}
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(dir, 0o755)
	s := NewStore(filepath.Join(dir, "state.json"))
	err := s.Save()
	if err == nil || !strings.Contains(err.Error(), "create temp") {
		t.Fatalf("expected create-temp error, got %v", err)
	}
}
