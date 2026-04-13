package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chaseddevelopment/dotfiles/installer/internal/backup"
)

// TestApplySymlink_RegularFileGoesThroughBackup verifies the
// never-silent-overwrite rule: when a regular (non-symlink) file
// already exists at the target, ApplySymlink must back it up before
// replacing it with a symlink. A regression that dropped the backup
// branch would silently destroy user data; this test pins the
// invariant that bm.Exists() is true after the apply and the old
// content is recoverable.
func TestApplySymlink_RegularFileIsBackedUpNotOverwritten(
	t *testing.T,
) {
	rootDir, _ := setupTestDirs(t)
	createSourceFile(
		t, rootDir, "conflict/file.txt", "new-content",
	)

	entry := SymlinkEntry{
		Source:    "conflict/file.txt",
		Target:    "$HOME/.config/conflict-file.txt",
		Component: "Test",
	}

	targetPath := os.ExpandEnv(entry.Target)
	if err := os.MkdirAll(
		filepath.Dir(targetPath), 0o755,
	); err != nil {
		t.Fatal(err)
	}
	// Place a regular file at the target with distinctive content so
	// we can spot a silent overwrite.
	const precious = "USER-PRECIOUS-CONTENT"
	if err := os.WriteFile(
		targetPath, []byte(precious), 0o644,
	); err != nil {
		t.Fatal(err)
	}

	bm := backup.NewManager(false)
	if err := ApplySymlink(entry, rootDir, bm, false, nil); err != nil {
		t.Fatalf("ApplySymlink: %v", err)
	}

	// Target must now be a symlink to the repo source.
	if _, err := os.Readlink(targetPath); err != nil {
		t.Fatalf("target should be a symlink: %v", err)
	}

	// Backup dir must exist; original content must be recoverable.
	if !bm.Exists() {
		t.Fatal("backup manager did not create a dir — user data lost")
	}
	found := false
	_ = filepath.Walk(bm.Dir(),
		func(p string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			data, rerr := os.ReadFile(p)
			if rerr != nil {
				return nil
			}
			if strings.Contains(string(data), precious) {
				found = true
			}
			return nil
		})
	if !found {
		t.Error(
			"original file contents missing from backup dir — " +
				"regular file was silently overwritten",
		)
	}
}

// TestApplyAllSymlinks_Idempotent applies the same component twice
// and asserts: (a) the second call returns nil, (b) no backup dir is
// created by the second call (there's nothing to back up), and (c)
// the symlink still points to the canonical source.
func TestApplyAllSymlinks_Idempotent(t *testing.T) {
	rootDir, _ := setupTestDirs(t)
	createSourceFile(
		t, rootDir, "starship/starship.toml", "format=test",
	)

	bm1 := backup.NewManager(false)
	if err := ApplyAllSymlinks(
		"Starship", rootDir, bm1, false, nil,
	); err != nil {
		t.Fatalf("first apply: %v", err)
	}

	target := os.ExpandEnv("$HOME/.config/starship.toml")
	linkBefore, err := os.Readlink(target)
	if err != nil {
		t.Fatalf("first apply did not create a symlink: %v", err)
	}

	// Capture inode via Lstat so we can confirm nothing was replaced.
	infoBefore, err := os.Lstat(target)
	if err != nil {
		t.Fatal(err)
	}

	bm2 := backup.NewManager(false)
	if err := ApplyAllSymlinks(
		"Starship", rootDir, bm2, false, nil,
	); err != nil {
		t.Fatalf("second apply (should be idempotent): %v", err)
	}

	// No churn: the second apply must not have made a new backup
	// and must not have replaced the symlink.
	if bm2.Exists() {
		t.Error(
			"second apply created a backup dir — not idempotent",
		)
	}
	linkAfter, err := os.Readlink(target)
	if err != nil {
		t.Fatalf("symlink missing after second apply: %v", err)
	}
	if linkBefore != linkAfter {
		t.Errorf(
			"symlink target changed: %q -> %q",
			linkBefore, linkAfter,
		)
	}
	infoAfter, err := os.Lstat(target)
	if err != nil {
		t.Fatal(err)
	}
	if !infoBefore.ModTime().Equal(infoAfter.ModTime()) {
		t.Error(
			"symlink mod-time changed; second apply " +
				"re-created the link unnecessarily",
		)
	}
}

// TestRollbackSymlinks_RestoresPreApplyState simulates a partial
// batch failure: apply two of three component entries successfully,
// then invoke rollbackSymlinks (the hook used by components.go after
// a failed post-install). After rollback the FS should match the
// pre-apply state — the symlinks created by the half-finished batch
// are gone, and any regular file that was present is untouched
// (rollbackSymlinks refuses to delete non-owned paths).
func TestRollbackSymlinks_RestoresPreApplyState(t *testing.T) {
	rootDir, _ := setupTestDirs(t)

	// Git component has two entries: configs/git/config and
	// configs/git/ignore plus configs/lazygit (directory). We seed
	// only the first two sources and apply them manually to
	// simulate a batch that stopped mid-way before lazygit.
	createSourceFile(t, rootDir, "git/config", "config")
	createSourceFile(t, rootDir, "git/ignore", "ignore")

	bm := backup.NewManager(false)

	// Pre-apply snapshot: nothing at any target.
	targets := []string{}
	for _, e := range AllSymlinks() {
		if e.Component != "Git" {
			continue
		}
		tgt := os.ExpandEnv(e.Target)
		targets = append(targets, tgt)
		if _, err := os.Lstat(tgt); err == nil {
			t.Fatalf("pre-condition failed: %s already exists", tgt)
		}
	}

	// Apply only the two file entries (simulating the partial
	// success before lazygit would have failed).
	for _, e := range AllSymlinks() {
		if e.Component != "Git" || e.IsDir {
			continue
		}
		if err := ApplySymlink(
			e, rootDir, bm, false, nil,
		); err != nil {
			t.Fatalf("partial apply %s: %v", e.Source, err)
		}
	}

	// Confirm two symlinks now exist.
	for _, e := range AllSymlinks() {
		if e.Component != "Git" || e.IsDir {
			continue
		}
		tgt := os.ExpandEnv(e.Target)
		if _, err := os.Readlink(tgt); err != nil {
			t.Fatalf("expected symlink at %s: %v", tgt, err)
		}
	}

	// Invoke rollback — must remove the applied symlinks.
	rollbackSymlinks("Git", rootDir, nil)

	for _, tgt := range targets {
		if _, err := os.Lstat(tgt); err == nil {
			t.Errorf(
				"rollback did not remove %s", tgt,
			)
		}
	}
}
