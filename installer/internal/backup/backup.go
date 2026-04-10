package backup

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Manager handles creating timestamped backups of config files.
// Safe to use from multiple goroutines.
type Manager struct {
	dir     string
	dryRun  bool
	once    sync.Once
	initErr error
	mu      sync.Mutex
}

// NewManager creates a backup manager with a timestamped directory.
func NewManager(dryRun bool) *Manager {
	return &Manager{
		dir: filepath.Join(
			os.Getenv("HOME"),
			fmt.Sprintf(".dotfiles-backup-%s", time.Now().Format("20060102-150405")),
		),
		dryRun: dryRun,
	}
}

// Dir returns the backup directory path.
func (m *Manager) Dir() string { return m.dir }

// BackupFile copies a file or directory to the backup dir.
// Safe to call from multiple goroutines.
func (m *Manager) BackupFile(path string) error {
	if m.dryRun {
		return nil
	}
	if _, err := os.Lstat(path); os.IsNotExist(err) {
		return nil
	}

	m.once.Do(func() {
		m.initErr = os.MkdirAll(m.dir, 0o755)
	})
	if m.initErr != nil {
		return fmt.Errorf("create backup dir: %w", m.initErr)
	}

	dest := m.backupDest(path)
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("create backup subdir: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	return copyRecursive(path, dest)
}

// backupDest computes the backup destination for a given path,
// preserving directory structure relative to $HOME to avoid
// collisions between files with the same basename.
func (m *Manager) backupDest(path string) string {
	home := os.Getenv("HOME")
	rel, err := filepath.Rel(home, path)
	if err != nil || strings.HasPrefix(rel, "..") {
		// Path not under $HOME — fall back to basename.
		return filepath.Join(m.dir, filepath.Base(path))
	}
	return filepath.Join(m.dir, rel)
}

// Cleanup removes the backup directory.
func (m *Manager) Cleanup() error {
	if m.Exists() {
		return os.RemoveAll(m.dir)
	}
	return nil
}

// Exists returns true if the backup directory was created.
func (m *Manager) Exists() bool {
	_, err := os.Stat(m.dir)
	return err == nil
}

func copyRecursive(src, dst string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}

	// Handle symlinks.
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(src)
		if err != nil {
			return err
		}
		return os.Symlink(target, dst)
	}

	if info.IsDir() {
		return copyDir(src, dst, info)
	}
	return copyFile(src, dst, info)
}

func copyDir(src, dst string, info fs.FileInfo) error {
	if err := os.MkdirAll(dst, info.Mode()); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		if err := copyRecursive(srcPath, dstPath); err != nil {
			return err
		}
	}
	return nil
}

func copyFile(src, dst string, info fs.FileInfo) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}

	_, err = io.Copy(out, in)
	if closeErr := out.Close(); err == nil {
		err = closeErr
	}
	return err
}
