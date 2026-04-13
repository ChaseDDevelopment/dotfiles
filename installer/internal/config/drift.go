package config

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/chaseddevelopment/dotfiles/installer/internal/backup"
)

// DetectRepoDrift returns tracked paths under configs/ that have
// uncommitted changes. Untracked files and paths outside configs/
// are out of scope because the dotfiles use case here is "restore
// the repo's canonical configs after an install script mutated
// them" — touching anything else would clobber legitimate user
// edits (e.g. installer source being actively worked on).
//
// Returned paths are repo-relative ("configs/zsh/.zshenv") so
// callers can pass them back to `git checkout --` without
// re-resolving against the working tree.
func DetectRepoDrift(rootDir string) ([]string, error) {
	cmd := exec.Command(
		"git", "-C", rootDir,
		"status", "--porcelain=v1", "--", "configs/",
	)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git status: %w", err)
	}
	var drifted []string
	for _, line := range strings.Split(string(out), "\n") {
		if len(line) < 4 {
			continue
		}
		// Porcelain v1 format: XY<space><path>. X=index, Y=worktree.
		// Skip untracked ("??") — these are runtime-generated files
		// we don't want to touch. Anything else (M_, _M, MM, AM, ...)
		// indicates a tracked file with uncommitted changes.
		status := line[:2]
		if status == "??" {
			continue
		}
		path := strings.TrimSpace(line[3:])
		// Handle renames "old -> new" — take the new path.
		if idx := strings.Index(path, " -> "); idx >= 0 {
			path = path[idx+4:]
		}
		// Double-check the path is under configs/ even though we
		// passed the pathspec — defense against any git quirk where
		// submodule updates or renames emit paths outside the spec.
		if !strings.HasPrefix(path, "configs/") {
			continue
		}
		drifted = append(drifted, path)
	}
	return drifted, nil
}

// BackupAndReset saves each repo-relative path through the backup
// manager (timestamped dir, existing restore flow works), then
// restores both the index and working tree to HEAD for those
// paths. Returns the backup directory so callers can cite it in
// user-visible messages.
//
// Only call this with paths from DetectRepoDrift — this function
// does NOT re-validate scope. Passing paths outside configs/ would
// let this helper clobber anything tracked under the repo root.
func BackupAndReset(
	rootDir string,
	bm *backup.Manager,
	paths []string,
) (backupDir string, err error) {
	if len(paths) == 0 {
		return "", nil
	}
	for _, rel := range paths {
		abs := filepath.Join(rootDir, rel)
		if err := bm.BackupFile(abs); err != nil {
			return bm.Dir(), fmt.Errorf(
				"backup %s: %w", rel, err,
			)
		}
	}
	// Clear both staged and unstaged drift so the caller can retry
	// a blocked fast-forward pull against a fully clean tree.
	args := append([]string{
		"-C", rootDir,
		"restore", "--staged", "--worktree", "--source=HEAD", "--",
	}, paths...)
	if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
		return bm.Dir(), fmt.Errorf(
			"git restore: %w\n%s", err, strings.TrimSpace(string(out)),
		)
	}
	return bm.Dir(), nil
}
