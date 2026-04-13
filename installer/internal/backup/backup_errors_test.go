package backup

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// skipIfRootOrWindows skips chmod-based tests under root (perms are
// bypassed) and on Windows (different perm model). Mirrors the
// pattern in branches_test.go.
func skipIfRootOrWindows(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("chmod-based perm denial unreliable on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("running as root bypasses chmod permission denial")
	}
}

// TestBackupFileInitMkdirAllError forces the once.Do MkdirAll to
// fail by making the parent of m.dir read-only, hitting the
// initErr branch at backup.go:54-55.
func TestBackupFileInitMkdirAllError(t *testing.T) {
	skipIfRootOrWindows(t)

	home := t.TempDir()
	t.Setenv("HOME", home)

	// Lock down the parent so MkdirAll on a child can't succeed.
	parent := filepath.Join(home, "locked")
	if err := os.MkdirAll(parent, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(parent, 0o400); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(parent, 0o755) })

	src := filepath.Join(home, "file")
	if err := os.WriteFile(src, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := NewManager(false)
	// Aim m.dir at a path under the locked parent.
	m.dir = filepath.Join(parent, "backup-target")

	err := m.BackupFile(src)
	if err == nil {
		t.Fatal("expected init MkdirAll error, got nil")
	}
	if !strings.Contains(err.Error(), "create backup dir") {
		t.Fatalf("expected wrap 'create backup dir', got %v", err)
	}
}

// TestBackupFileSubdirMkdirAllError causes the per-call subdir
// MkdirAll to fail by pre-planting a regular file at the subdir
// path (backup.go:59-61).
func TestBackupFileSubdirMkdirAllError(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	// Use a backup dir under HOME that we control fully.
	m := NewManager(false)
	m.dir = filepath.Join(home, "backup")
	if err := os.MkdirAll(m.dir, 0o755); err != nil {
		t.Fatal(err)
	}

	// We will back up $HOME/sub/file. backupDest puts that at
	// $m.dir/sub/file, so MkdirAll must create $m.dir/sub. Plant
	// a regular file at $m.dir/sub so MkdirAll fails.
	subDir := filepath.Join(home, "sub")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(subDir, "file")
	if err := os.WriteFile(src, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// File-blocker at the destination subdir path.
	if err := os.WriteFile(
		filepath.Join(m.dir, "sub"), []byte("blocker"), 0o644,
	); err != nil {
		t.Fatal(err)
	}

	err := m.BackupFile(src)
	if err == nil {
		t.Fatal("expected subdir MkdirAll error, got nil")
	}
	if !strings.Contains(err.Error(), "create backup subdir") {
		t.Fatalf("expected wrap 'create backup subdir', got %v", err)
	}
}

// TestCopyRecursiveLstatError covers backup.go:97-99 by passing a
// non-existent path (Lstat returns an error that is NOT IsNotExist
// when the parent itself is unreadable — but a plain non-existent
// path also returns a fs.PathError, exercising the same return).
func TestCopyRecursiveLstatError(t *testing.T) {
	skipIfRootOrWindows(t)

	parent := filepath.Join(t.TempDir(), "blind")
	if err := os.MkdirAll(parent, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(parent, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(parent, 0o755) })

	src := filepath.Join(parent, "nope")
	dst := filepath.Join(t.TempDir(), "dst")
	if err := copyRecursive(src, dst); err == nil {
		t.Fatal("expected Lstat error from unreadable parent, got nil")
	}
}

// TestCopyDirMkdirDstError covers backup.go:117-119 by pre-planting
// a regular file where copyDir wants to MkdirAll a directory.
func TestCopyDirMkdirDstError(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "srcdir")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	// dst exists as a regular file — MkdirAll will fail.
	dst := filepath.Join(tmp, "dst-blocker")
	if err := os.WriteFile(dst, []byte("file"), 0o644); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := copyDir(src, dst, info); err == nil {
		t.Fatal("expected MkdirAll dst error, got nil")
	}
}

// TestCopyDirReadDirError covers backup.go:121-123 by chmod 0o000
// on src so ReadDir fails.
func TestCopyDirReadDirError(t *testing.T) {
	skipIfRootOrWindows(t)

	tmp := t.TempDir()
	src := filepath.Join(tmp, "src")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	// Plant a child so the dir isn't trivial.
	if err := os.WriteFile(
		filepath.Join(src, "child"), []byte("x"), 0o644,
	); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(src, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(src, 0o755) })

	info := dummyDirInfo(0o755)
	dst := filepath.Join(tmp, "dst")
	if err := copyDir(src, dst, info); err == nil {
		t.Fatal("expected ReadDir error, got nil")
	}
}

// TestCopyDirRecursivePropagatesError covers backup.go:127-129:
// the recursive copyRecursive returns an error from a child entry
// and copyDir must propagate it. We trigger by making the child
// dst path write-protected (chmod the parent dst to 0o500 after
// initial creation? Simpler: pre-create a directory at the child
// dstPath where copyFile expects to OpenFile a regular file —
// OpenFile on a directory path returns EISDIR).
func TestCopyDirRecursivePropagatesError(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "src")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(src, "child.txt"), []byte("data"), 0o644,
	); err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(tmp, "dst")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	// Pre-create child.txt as a DIRECTORY at dst — OpenFile will fail.
	if err := os.MkdirAll(
		filepath.Join(dst, "child.txt"), 0o755,
	); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := copyDir(src, dst, info); err == nil {
		t.Fatal("expected recursive copyFile error, got nil")
	}
}

// TestCopyFileOpenSrcError covers backup.go:136-138 (src does not
// exist).
func TestCopyFileOpenSrcError(t *testing.T) {
	tmp := t.TempDir()
	missing := filepath.Join(tmp, "ghost")
	dst := filepath.Join(tmp, "dst")
	info := dummyFileInfo(0o644)
	if err := copyFile(missing, dst, info); err == nil {
		t.Fatal("expected open-src error, got nil")
	}
}

// TestCopyFileCreateDstError covers backup.go:142-144 by making the
// dst parent unwritable.
func TestCopyFileCreateDstError(t *testing.T) {
	skipIfRootOrWindows(t)

	tmp := t.TempDir()
	src := filepath.Join(tmp, "src")
	if err := os.WriteFile(src, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}

	parent := filepath.Join(tmp, "ro")
	if err := os.MkdirAll(parent, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(parent, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(parent, 0o755) })

	dst := filepath.Join(parent, "dst")
	info, err := os.Stat(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := copyFile(src, dst, info); err == nil {
		t.Fatal("expected create-dst error, got nil")
	}
}

// dummyDirInfo / dummyFileInfo provide an fs.FileInfo for
// callsites that only consult Mode() — the actual file may or may
// not exist on disk.
type stubInfo struct {
	mode fs.FileMode
}

func (s stubInfo) Name() string      { return "stub" }
func (s stubInfo) Size() int64       { return 0 }
func (s stubInfo) Mode() fs.FileMode { return s.mode }
func (s stubInfo) ModTime() time.Time {
	return time.Time{}
}
func (s stubInfo) IsDir() bool { return s.mode.IsDir() }
func (s stubInfo) Sys() any     { return nil }

func dummyDirInfo(perm fs.FileMode) fs.FileInfo {
	return stubInfo{mode: fs.ModeDir | perm}
}
func dummyFileInfo(perm fs.FileMode) fs.FileInfo {
	return stubInfo{mode: perm}
}
