package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/chaseddevelopment/dotfiles/installer/internal/backup"
)

// SymlinkEntry defines one config symlink mapping.
type SymlinkEntry struct {
	Source    string // relative to configs/ dir in the repo
	Target    string // absolute destination (supports $HOME)
	IsDir     bool   // true = symlink entire directory
	Component string // which component this belongs to
}

// AllSymlinks returns the complete declarative symlink map.
func AllSymlinks() []SymlinkEntry {
	return []SymlinkEntry{
		// Zsh
		{Source: "zsh", Target: "$HOME/.config/zsh", IsDir: true, Component: "Zsh"},
		{Source: "zsh/.zshenv", Target: "$HOME/.zshenv", IsDir: false, Component: "Zsh"},

		// Tmux
		{Source: "tmux", Target: "$HOME/.config/tmux", IsDir: true, Component: "Tmux"},
		{Source: "tmux/tmux.conf", Target: "$HOME/.tmux.conf", IsDir: false, Component: "Tmux"},

		// Neovim
		{Source: "nvim", Target: "$HOME/.config/nvim", IsDir: true, Component: "Neovim"},

		// Starship
		{Source: "starship/starship.toml", Target: "$HOME/.config/starship.toml", IsDir: false, Component: "Starship"},

		// Atuin
		{Source: "atuin", Target: "$HOME/.config/atuin", IsDir: true, Component: "Atuin"},

		// Ghostty
		{Source: "ghostty", Target: "$HOME/.config/ghostty", IsDir: true, Component: "Ghostty"},

		// Yazi
		{Source: "yazi", Target: "$HOME/.config/yazi", IsDir: true, Component: "Yazi"},

		// Git (single file to preserve other files in ~/.config/git/)
		{Source: "git/config", Target: "$HOME/.config/git/config", IsDir: false, Component: "Git"},

		// Lazygit
		{Source: "lazygit", Target: "$HOME/.config/lazygit", IsDir: true, Component: "Git"},
	}
}

// ApplySymlink creates a single symlink, backing up the existing target.
func ApplySymlink(entry SymlinkEntry, rootDir string, bm *backup.Manager, dryRun bool) error {
	source := filepath.Join(rootDir, "configs", entry.Source)
	target := os.ExpandEnv(entry.Target)

	// Verify source exists.
	if _, err := os.Stat(source); os.IsNotExist(err) {
		return fmt.Errorf("source not found: %s", source)
	}

	// Check if symlink already points correctly.
	if existing, err := os.Readlink(target); err == nil && existing == source {
		return nil // already correct
	}

	if dryRun {
		return nil
	}

	// Backup existing target if it exists.
	if _, err := os.Lstat(target); err == nil {
		bm.BackupFile(target)
	}

	// Ensure parent directory exists.
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(target), err)
	}

	// Remove stale target and create symlink.
	os.RemoveAll(target)
	if err := os.Symlink(source, target); err != nil {
		return fmt.Errorf("symlink %s -> %s: %w", source, target, err)
	}

	return nil
}

// ApplyAllSymlinks creates all symlinks for a given component.
func ApplyAllSymlinks(component, rootDir string, bm *backup.Manager, dryRun bool) error {
	for _, entry := range AllSymlinks() {
		if entry.Component != component {
			continue
		}
		if err := ApplySymlink(entry, rootDir, bm, dryRun); err != nil {
			return err
		}
	}
	return nil
}
