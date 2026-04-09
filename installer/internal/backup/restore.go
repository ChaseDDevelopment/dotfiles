package backup

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ManagedPaths lists all config paths the installer manages.
var ManagedPaths = []string{
	"$HOME/.config/zsh",
	"$HOME/.config/nvim",
	"$HOME/.config/starship.toml",
	"$HOME/.config/atuin",
	"$HOME/.config/ghostty",
	"$HOME/.zshenv",
	"$HOME/.config/tmux",
	"$HOME/.tmux.conf",
	"$HOME/.tmux",
	"$HOME/.config/yazi",
	"$HOME/.config/git/config",
	"$HOME/.config/lazygit",
}

// Restore restores managed paths from a backup directory.
func Restore(backupDir string, dryRun bool) error {
	if _, err := os.Stat(backupDir); os.IsNotExist(err) {
		return fmt.Errorf("backup directory not found: %s", backupDir)
	}

	home := os.Getenv("HOME")
	for _, path := range ManagedPaths {
		target := os.ExpandEnv(path)

		// Compute backup source using relative path (matches
		// BackupFile's directory-preserving layout).
		rel, err := filepath.Rel(home, target)
		if err != nil || strings.HasPrefix(rel, "..") {
			continue
		}
		source := filepath.Join(backupDir, rel)

		if _, err := os.Stat(source); os.IsNotExist(err) {
			// Fall back to legacy basename layout for old backups.
			source = filepath.Join(backupDir, filepath.Base(target))
			if _, err := os.Stat(source); os.IsNotExist(err) {
				continue
			}
		}

		if dryRun {
			continue
		}

		os.RemoveAll(target)
		if err := copyRecursive(source, target); err != nil {
			return fmt.Errorf("restore %s: %w", target, err)
		}
	}
	return nil
}

// BackupInfo describes a discovered backup directory.
type BackupInfo struct {
	Path string
	Date string // formatted from the directory name
}

// ListBackups finds all ~/.dotfiles-backup-* directories.
func ListBackups() ([]BackupInfo, error) {
	home := os.Getenv("HOME")
	pattern := filepath.Join(home, ".dotfiles-backup-*")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}

	var backups []BackupInfo
	for _, m := range matches {
		info, err := os.Stat(m)
		if err != nil || !info.IsDir() {
			continue
		}
		name := filepath.Base(m)
		date := strings.TrimPrefix(name, ".dotfiles-backup-")
		backups = append(backups, BackupInfo{Path: m, Date: date})
	}

	sort.Slice(backups, func(i, j int) bool {
		return backups[i].Date > backups[j].Date // newest first
	})
	return backups, nil
}
