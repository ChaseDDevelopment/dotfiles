package backup

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// Manager handles creating timestamped backups of config files.
type Manager struct {
	dir     string
	dryRun  bool
	created bool
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
func (m *Manager) BackupFile(path string) error {
	if m.dryRun {
		return nil
	}
	if _, err := os.Lstat(path); os.IsNotExist(err) {
		return nil
	}

	if !m.created {
		if err := os.MkdirAll(m.dir, 0o755); err != nil {
			return fmt.Errorf("create backup dir: %w", err)
		}
		m.created = true
	}

	dest := filepath.Join(m.dir, filepath.Base(path))
	return copyRecursive(path, dest)
}

// Cleanup removes the backup directory.
func (m *Manager) Cleanup() error {
	if m.created {
		return os.RemoveAll(m.dir)
	}
	return nil
}

// Exists returns true if the backup directory was created.
func (m *Manager) Exists() bool { return m.created }

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
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
