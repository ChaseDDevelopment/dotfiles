package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/chaseddevelopment/dotfiles/installer/internal/backup"
	"github.com/chaseddevelopment/dotfiles/installer/internal/executor"
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
		// Tmux session wrapper — lands in ~/.local/bin (already on PATH
		// via .zshenv) so `ssh host tmux-session` works from non-
		// interactive sshd shells where brew PATH is absent.
		{Source: "tmux/scripts/tmux-session", Target: "$HOME/.local/bin/tmux-session", IsDir: false, Component: "Tmux"},

		// Neovim
		{Source: "nvim", Target: "$HOME/.config/nvim", IsDir: true, Component: "Neovim"},

		// Oh-My-Posh
		{Source: "oh-my-posh/config.omp.yaml", Target: "$HOME/.config/oh-my-posh/config.omp.yaml", IsDir: false, Component: "OhMyPosh"},

		// Atuin
		{Source: "atuin", Target: "$HOME/.config/atuin", IsDir: true, Component: "Atuin"},

		// Ghostty
		{Source: "ghostty", Target: "$HOME/.config/ghostty", IsDir: true, Component: "Ghostty"},

		// Yazi
		{Source: "yazi", Target: "$HOME/.config/yazi", IsDir: true, Component: "Yazi"},

		// Git (single files to preserve other files in ~/.config/git/)
		{Source: "git/config", Target: "$HOME/.config/git/config", IsDir: false, Component: "Git"},
		{Source: "git/ignore", Target: "$HOME/.config/git/ignore", IsDir: false, Component: "Git"},

		// Lazygit
		{Source: "lazygit", Target: "$HOME/.config/lazygit", IsDir: true, Component: "Git"},
	}
}

// ManagedTargets returns the de-duplicated list of target paths
// managed by the installer, derived from AllSymlinks(). Targets
// use $HOME (unexpanded) so callers can os.ExpandEnv as needed.
func ManagedTargets() []string {
	seen := make(map[string]struct{})
	var targets []string
	for _, entry := range AllSymlinks() {
		if _, ok := seen[entry.Target]; ok {
			continue
		}
		seen[entry.Target] = struct{}{}
		targets = append(targets, entry.Target)
	}
	return targets
}

// SymlinkStatus represents the inspection state of a symlink entry.
type SymlinkStatus int

const (
	// SymlinkAlreadyCorrect means the target symlink points to the correct source.
	SymlinkAlreadyCorrect SymlinkStatus = iota
	// SymlinkMissing means the target does not exist.
	SymlinkMissing
	// SymlinkWouldReplace means the target exists but is a regular file/dir
	// or a symlink pointing to the wrong location.
	SymlinkWouldReplace
)

// InspectSymlink checks the state of a single symlink entry without modifying anything.
func InspectSymlink(entry SymlinkEntry, rootDir string) SymlinkStatus {
	source := resolveSource(rootDir, entry.Source)
	target := os.ExpandEnv(entry.Target)

	// If the target doesn't exist at all, it's missing.
	info, err := os.Lstat(target)
	if err != nil {
		return SymlinkMissing
	}

	// If it's a symlink, check where it points.
	if info.Mode()&os.ModeSymlink != 0 {
		existing, err := os.Readlink(target)
		if err != nil {
			return SymlinkWouldReplace
		}
		// Canonicalize both paths for reliable comparison.
		canonSource, err1 := filepath.Abs(source)
		canonExisting, err2 := filepath.Abs(existing)
		if err1 == nil && err2 == nil && canonSource == canonExisting {
			return SymlinkAlreadyCorrect
		}
		return SymlinkWouldReplace
	}

	// Target exists as a regular file or directory.
	return SymlinkWouldReplace
}

// InspectComponent aggregates symlink statuses for a component into a
// single human-readable status string.
func InspectComponent(component, rootDir string) string {
	allCorrect := true
	anyReplace := false

	for _, entry := range AllSymlinks() {
		if entry.Component != component {
			continue
		}
		switch InspectSymlink(entry, rootDir) {
		case SymlinkAlreadyCorrect:
			// fine
		case SymlinkMissing:
			allCorrect = false
		case SymlinkWouldReplace:
			allCorrect = false
			anyReplace = true
		}
	}

	if allCorrect {
		return "already configured"
	}
	if anyReplace {
		return "would replace"
	}
	return "would configure"
}

// resolveSource returns the OS-specific source path if a variant
// exists (e.g. configs/git/config##darwin), otherwise the base path.
// Variants use the "##os" suffix convention (similar to YADM).
func resolveSource(rootDir, source string) string {
	base := filepath.Join(rootDir, "configs", source)
	variant := base + "##" + runtime.GOOS
	if _, err := os.Stat(variant); err == nil {
		return variant
	}
	return base
}

// DiffSymlink returns a human-readable description of what would
// change if the symlink were applied. Returns "" when the symlink
// is already correct or the target is missing (nothing to diff).
func DiffSymlink(entry SymlinkEntry, rootDir string) string {
	source := resolveSource(rootDir, entry.Source)
	target := os.ExpandEnv(entry.Target)

	info, err := os.Lstat(target)
	if err != nil {
		return "" // target doesn't exist
	}

	// If it's a symlink pointing elsewhere, show the redirect.
	if info.Mode()&os.ModeSymlink != 0 {
		existing, err := os.Readlink(target)
		if err != nil {
			return fmt.Sprintf("%s: unreadable symlink", target)
		}
		canonSource, _ := filepath.Abs(source)
		canonExisting, _ := filepath.Abs(existing)
		if canonSource == canonExisting {
			return "" // already correct
		}
		return fmt.Sprintf(
			"%s: symlink %s → %s",
			target, existing, source,
		)
	}

	// Regular file/dir — indicate replacement.
	kind := "file"
	if info.IsDir() {
		kind = "directory"
	}
	return fmt.Sprintf(
		"%s: replace %s with symlink → %s",
		target, kind, source,
	)
}

// DiffComponent returns diff descriptions for all symlinks in a
// component that would be modified.
func DiffComponent(component, rootDir string) []string {
	var diffs []string
	for _, entry := range AllSymlinks() {
		if entry.Component != component {
			continue
		}
		if d := DiffSymlink(entry, rootDir); d != "" {
			diffs = append(diffs, d)
		}
	}
	return diffs
}

// RemoveComponentSymlinks removes all symlinks for a component
// that point to the dotfiles repo. Regular files/dirs are left
// untouched to avoid data loss.
func RemoveComponentSymlinks(
	component, rootDir string,
	runner *executor.Runner,
) error {
	for _, entry := range AllSymlinks() {
		if entry.Component != component {
			continue
		}
		target := os.ExpandEnv(entry.Target)
		link, err := os.Readlink(target)
		if err != nil {
			continue // not a symlink
		}
		source := resolveSource(rootDir, entry.Source)
		canonSource, err1 := filepath.Abs(source)
		canonLink, err2 := filepath.Abs(link)
		// Only remove when we can verify both paths AND they match.
		// If either Abs call fails we don't know who owns the link,
		// so leave it alone rather than delete blind.
		if err1 != nil || err2 != nil || canonSource != canonLink {
			continue
		}
		if err := os.Remove(target); err != nil {
			return fmt.Errorf("remove symlink %s: %w", target, err)
		}
		if runner != nil {
			runner.EmitVerbose("Removed " + target)
		}
	}
	return nil
}

// ApplySymlink creates a single symlink, backing up the existing
// target. The runner parameter is used for verbose status output
// and may be nil.
func ApplySymlink(
	entry SymlinkEntry,
	rootDir string,
	bm *backup.Manager,
	dryRun bool,
	runner *executor.Runner,
) error {
	source := resolveSource(rootDir, entry.Source)
	target := os.ExpandEnv(entry.Target)

	// Verify source exists.
	if _, err := os.Stat(source); os.IsNotExist(err) {
		return fmt.Errorf("source not found: %s", source)
	}

	// Check if symlink already points correctly (canonicalize
	// both paths for reliable comparison, matching InspectSymlink).
	if existing, err := os.Readlink(target); err == nil {
		canonSource, err1 := filepath.Abs(source)
		canonExisting, err2 := filepath.Abs(existing)
		if err1 == nil && err2 == nil &&
			canonSource == canonExisting {
			if runner != nil {
				runner.EmitVerbose(
					"✓ " + target + " (already correct)",
				)
			}
			return nil // already correct
		}
	}

	if dryRun {
		return nil
	}

	// Backup existing target if it exists.
	if _, err := os.Lstat(target); err == nil {
		if runner != nil {
			runner.EmitVerbose("Backing up " + target)
		}
		if err := bm.BackupFile(target); err != nil {
			return fmt.Errorf("backup %s: %w", target, err)
		}
	}

	// Ensure parent directory exists.
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(target), err)
	}

	// Stage the new symlink at target+".new" then os.Rename over
	// the final path. Rename of a symlink on the same filesystem is
	// atomic, so a crash between "remove old" and "create new" can't
	// leave the user with no target anymore.
	stagePath := target + ".new"
	// Remove any stale staging link from a previous crashed run.
	if err := os.Remove(stagePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("clear stale stage %s: %w", stagePath, err)
	}
	if runner != nil {
		runner.EmitVerbose("Symlink " + target)
	}
	if err := os.Symlink(source, stagePath); err != nil {
		return fmt.Errorf("symlink %s -> %s: %w", source, stagePath, err)
	}
	// On POSIX, os.Rename replaces the destination atomically whether
	// it's a file, dir, or existing symlink, as long as dir→non-dir
	// (and vice versa) isn't attempted. If the existing target is a
	// directory that's not empty, we need to clear it first — but
	// only after the staging link exists, so a crash either leaves
	// the old target in place or the new symlink in place.
	if info, err := os.Lstat(target); err == nil {
		if info.IsDir() && info.Mode()&os.ModeSymlink == 0 {
			if err := os.RemoveAll(target); err != nil {
				_ = os.Remove(stagePath)
				return fmt.Errorf("remove %s: %w", target, err)
			}
		}
	}
	if err := os.Rename(stagePath, target); err != nil {
		_ = os.Remove(stagePath)
		return fmt.Errorf(
			"rename %s -> %s: %w", stagePath, target, err,
		)
	}

	return nil
}

// ApplyAllSymlinks creates all symlinks for a given component.
// The runner parameter is used for verbose status output and may
// be nil.
func ApplyAllSymlinks(
	component, rootDir string,
	bm *backup.Manager,
	dryRun bool,
	runner *executor.Runner,
) error {
	for _, entry := range AllSymlinks() {
		if entry.Component != component {
			continue
		}
		if err := ApplySymlink(
			entry, rootDir, bm, dryRun, runner,
		); err != nil {
			return err
		}
	}
	return nil
}
