package config

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/chaseddevelopment/dotfiles/installer/internal/backup"
)

// skipIfRootOrWindowsConfig skips chmod-based tests under root or
// Windows. Mirrors the pattern from backup tests / state tests.
func skipIfRootOrWindowsConfig(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("chmod-based perm denial unreliable on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("running as root bypasses chmod permission denial")
	}
}

// TestSetupComponentApplyAllSymlinksError covers the error branch
// in SetupComponent at components.go:73-75 when ApplyAllSymlinks
// returns an error (e.g. missing source).
func TestSetupComponentApplyAllSymlinksError(t *testing.T) {
	sc, home := newComponentSetup(t)
	bin := filepath.Join(home, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	writeTool(t, bin, "git", "#!/bin/sh\nexit 0\n")

	// RootDir has no configs/git → ApplyAllSymlinks fails on the
	// first Git source lookup.
	err := SetupComponent(
		context.Background(),
		Component{Name: "Git", RequiredCmd: "git"},
		sc,
	)
	if err == nil ||
		!strings.Contains(err.Error(), "symlinks for Git") {
		t.Fatalf("expected symlinks error, got %v", err)
	}
}

// TestSetupComponentRunPostInstallErrorRollback covers
// components.go:81-85: ApplyAllSymlinks succeeds, then
// runPostInstall (setupNeovim) fails because one of the nvim
// dirs can't be created, and rollbackSymlinks must run.
func TestSetupComponentRunPostInstallErrorRollback(t *testing.T) {
	sc, home := newComponentSetup(t)

	// Plant a fake nvim on PATH so the RequiredCmd check passes.
	bin := filepath.Join(home, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	writeTool(t, bin, "nvim", "#!/bin/sh\nexit 0\n")

	// Plant a configs/nvim source dir so ApplyAllSymlinks succeeds
	// for the Neovim component before post-install runs.
	nvimSrc := filepath.Join(sc.RootDir, "configs", "nvim")
	if err := os.MkdirAll(nvimSrc, 0o755); err != nil {
		t.Fatal(err)
	}

	// Let ApplyAllSymlinks create $HOME/.config/nvim symlink first.
	// Then we want setupNeovim's MkdirAll($HOME/.local/share/nvim)
	// to fail. Block it by planting a regular file at
	// $HOME/.local/share so MkdirAll(..., "nvim") returns ENOTDIR.
	if err := os.MkdirAll(
		filepath.Join(home, ".local"), 0o755,
	); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(home, ".local", "share"),
		[]byte("blocker"), 0o644,
	); err != nil {
		t.Fatal(err)
	}

	err := SetupComponent(
		context.Background(),
		Component{Name: "Neovim", RequiredCmd: "nvim"},
		sc,
	)
	if err == nil {
		t.Fatal("expected setupNeovim error")
	}

	// Rollback should have removed any Neovim component symlinks.
	for _, e := range AllSymlinks() {
		if e.Component != "Neovim" {
			continue
		}
		tgt := os.ExpandEnv(e.Target)
		if link, err := os.Readlink(tgt); err == nil {
			t.Errorf(
				"Neovim symlink %s still present after rollback (-> %s)",
				tgt, link,
			)
		}
	}
}

// TestSetupComponentRunUserHookErrorRollback covers
// components.go:88-91: post-install succeeds, user hook returns
// non-zero exit, rollback runs.
func TestSetupComponentRunUserHookErrorRollback(t *testing.T) {
	sc, home := newComponentSetup(t)

	bin := filepath.Join(home, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	writeTool(t, bin, "git", "#!/bin/sh\nexit 0\n")

	// Plant Git config sources so symlinks succeed.
	gitConfigs := filepath.Join(sc.RootDir, "configs", "git")
	if err := os.MkdirAll(gitConfigs, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"config", "ignore"} {
		if err := os.WriteFile(
			filepath.Join(gitConfigs, f), []byte("x"), 0o644,
		); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(
		filepath.Join(sc.RootDir, "configs", "lazygit"), 0o755,
	); err != nil {
		t.Fatal(err)
	}

	// Drop a failing user hook for Git.
	hookDir := filepath.Join(sc.RootDir, "configs", "git", "hooks")
	if err := os.MkdirAll(hookDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(hookDir, "post-install.sh"),
		[]byte("#!/bin/sh\nexit 1\n"), 0o755,
	); err != nil {
		t.Fatal(err)
	}

	err := SetupComponent(
		context.Background(),
		Component{Name: "Git", RequiredCmd: "git"},
		sc,
	)
	if err == nil {
		t.Fatal("expected user hook error to propagate")
	}

	// Rollback should have removed all Git symlinks applied.
	for _, e := range AllSymlinks() {
		if e.Component != "Git" {
			continue
		}
		tgt := os.ExpandEnv(e.Target)
		if _, err := os.Readlink(tgt); err == nil {
			t.Errorf(
				"Git symlink %s still present after user-hook rollback",
				tgt,
			)
		}
	}
}

// TestSetupZshXDGMkdirAllError covers components.go:180-182 by
// pre-planting a regular file at one of the XDG dirs (~/.cache).
func TestSetupZshXDGMkdirAllError(t *testing.T) {
	sc, home := newComponentSetup(t)

	// Block ~/.cache as a regular file. ~/.config comes earlier in
	// the loop and is created cleanly first.
	if err := os.MkdirAll(
		filepath.Join(home, ".config"), 0o755,
	); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(
		filepath.Join(home, ".local", "share"), 0o755,
	); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(
		filepath.Join(home, ".local", "state", "zsh"), 0o755,
	); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(
		filepath.Join(home, ".local", "bin"), 0o755,
	); err != nil {
		t.Fatal(err)
	}
	// Block .cache by making it a regular file.
	if err := os.WriteFile(
		filepath.Join(home, ".cache"), []byte("blocker"), 0o644,
	); err != nil {
		t.Fatal(err)
	}

	err := setupZsh(context.Background(), sc)
	if err == nil ||
		!strings.Contains(err.Error(), "create dir") {
		t.Fatalf("expected create dir error, got %v", err)
	}
}

// TestSetupZshSymlinkZshrcSkipsBackup covers the inverse of the
// stale-.zshrc branch (components.go:189-201): when ~/.zshrc is
// already a symlink we MUST NOT call BackupFile, then we still
// remove it so the post-install can re-link.
func TestSetupZshSymlinkZshrcSkipsBackup(t *testing.T) {
	sc, home := newComponentSetup(t)

	bin := filepath.Join(home, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	for _, n := range []string{"zsh", "brew", "git"} {
		writeTool(t, bin, n, "#!/bin/sh\nexit 0\n")
	}
	// Antidote already present so no install attempt.
	antidote := filepath.Join(
		home, ".config", "zsh", ".antidote", "antidote.zsh",
	)
	if err := os.MkdirAll(filepath.Dir(antidote), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(antidote, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	// Plant a SYMLINK at ~/.zshrc — backup must be skipped, but
	// the link must still be removed.
	staleTarget := filepath.Join(home, "real-zshrc")
	if err := os.WriteFile(staleTarget, []byte("real"), 0o644); err != nil {
		t.Fatal(err)
	}
	zshrc := filepath.Join(home, ".zshrc")
	if err := os.Symlink(staleTarget, zshrc); err != nil {
		t.Fatal(err)
	}

	if err := setupZsh(context.Background(), sc); err != nil {
		t.Fatalf("setupZsh: %v", err)
	}
	if _, err := os.Lstat(zshrc); !os.IsNotExist(err) {
		t.Fatalf("expected .zshrc symlink removal, err=%v", err)
	}
	// Backup dir should NOT have been created (the symlink case
	// short-circuits the backup call).
	if sc.Backup.Exists() {
		t.Error("backup dir created for a symlink stale .zshrc")
	}
}

// TestApplySymlinkBackupFailure covers symlinks.go:237-239 (well,
// actually 290-292 in the version with staging — bm.BackupFile
// returns an error).  We force BackupFile to fail by using a
// dryRun=false manager whose home points at an unwritable parent
// while the target file IS present.
func TestApplySymlinkBackupFailure(t *testing.T) {
	skipIfRootOrWindowsConfig(t)

	rootDir, home := setupTestDirs(t)
	createSourceFile(t, rootDir, "bf/file.txt", "new")

	entry := SymlinkEntry{
		Source:    "bf/file.txt",
		Target:    "$HOME/.config/bf-file.txt",
		Component: "Test",
	}

	// Pre-create the target so BackupFile is invoked.
	target := os.ExpandEnv(entry.Target)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Lock HOME after creating the target file so BackupFile's
	// MkdirAll(backup dir under HOME) fails.
	if err := os.Chmod(home, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(home, 0o755) })

	bm := backup.NewManager(false)
	err := ApplySymlink(entry, rootDir, bm, false, nil)
	if err == nil {
		t.Fatal("expected backup error from ApplySymlink")
	}
	if !strings.Contains(err.Error(), "backup") {
		t.Fatalf("expected wrap 'backup', got %v", err)
	}
}

// TestApplySymlinkClearStaleStageError covers symlinks.go:306-308:
// a leftover staging path that isn't removable (e.g. it's a
// non-empty directory that os.Remove can't unlink).
func TestApplySymlinkClearStaleStageError(t *testing.T) {
	rootDir, _ := setupTestDirs(t)
	createSourceFile(t, rootDir, "stage/file.txt", "x")

	entry := SymlinkEntry{
		Source:    "stage/file.txt",
		Target:    "$HOME/.config/stage-file.txt",
		Component: "Test",
	}

	target := os.ExpandEnv(entry.Target)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	// Plant a NON-EMPTY directory at target+".new" — os.Remove on
	// a non-empty dir returns ENOTEMPTY (not IsNotExist), which
	// triggers the "clear stale stage" error branch.
	stage := target + ".new"
	if err := os.MkdirAll(filepath.Join(stage, "child"), 0o755); err != nil {
		t.Fatal(err)
	}

	bm := backup.NewManager(false)
	err := ApplySymlink(entry, rootDir, bm, false, nil)
	if err == nil {
		t.Fatal("expected clear-stale-stage error, got nil")
	}
	if !strings.Contains(err.Error(), "clear stale stage") {
		t.Fatalf("expected 'clear stale stage' wrap, got %v", err)
	}
}

// TestSetupGitMkdirAllError covers components.go:430-432: the
// MkdirAll($HOME/.config/git) call fails because that path is
// pre-planted as a regular file.
func TestSetupGitMkdirAllError(t *testing.T) {
	sc, home := newComponentSetup(t)
	if err := os.MkdirAll(
		filepath.Join(home, ".config"), 0o755,
	); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(home, ".config", "git"),
		[]byte("blocker"), 0o644,
	); err != nil {
		t.Fatal(err)
	}
	err := setupGit(context.Background(), sc)
	if err == nil {
		t.Fatal("expected setupGit MkdirAll error")
	}
}

// TestSetupGhosttyHeadless covers the no-desktop-environment
// branch (components.go:413-421) on a Linux platform without DE
// signals.
func TestSetupGhosttyHeadless(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("Ghostty IsDesktopEnvironment always true on darwin")
	}
	sc, _ := newComponentSetup(t)
	// Strip every desktop-env signal so IsDesktopEnvironment is
	// false on Linux.
	for _, v := range []string{
		"DISPLAY", "WAYLAND_DISPLAY",
		"XDG_CURRENT_DESKTOP", "XDG_SESSION_DESKTOP",
		"DESKTOP_SESSION", "GNOME_DESKTOP_SESSION_ID",
		"KDE_FULL_SESSION", "MIR_SOCKET",
	} {
		t.Setenv(v, "")
	}
	if err := setupGhostty(context.Background(), sc); err != nil {
		t.Fatalf("setupGhostty headless: %v", err)
	}
}

// TestApplySymlinkRemovesNonEmptyDirTarget covers
// symlinks.go:321-327: target is an existing non-empty regular dir,
// gets RemoveAll'd, and then renamed.
func TestApplySymlinkRemovesNonEmptyDirTarget(t *testing.T) {
	rootDir, _ := setupTestDirs(t)
	createSourceFile(t, rootDir, "rd/file.txt", "new")

	entry := SymlinkEntry{
		Source:    "rd/file.txt",
		Target:    "$HOME/.config/rd-file.txt",
		Component: "Test",
	}

	// Pre-create target as a non-empty directory.
	target := os.ExpandEnv(entry.Target)
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(target, "leftover"), []byte("x"), 0o644,
	); err != nil {
		t.Fatal(err)
	}

	bm := backup.NewManager(false)
	if err := ApplySymlink(entry, rootDir, bm, false, nil); err != nil {
		t.Fatalf("ApplySymlink: %v", err)
	}
	if _, err := os.Readlink(target); err != nil {
		t.Fatalf("expected target to be a symlink, got %v", err)
	}
}
