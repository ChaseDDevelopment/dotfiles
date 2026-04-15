package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/chaseddevelopment/dotfiles/installer/internal/backup"
	"github.com/chaseddevelopment/dotfiles/installer/internal/executor"
)

func TestAllSymlinks_NonEmpty(t *testing.T) {
	entries := AllSymlinks()
	if len(entries) == 0 {
		t.Fatal("AllSymlinks() returned empty slice")
	}

	for i, e := range entries {
		if e.Source == "" {
			t.Errorf("entry[%d]: Source is empty", i)
		}
		if e.Target == "" {
			t.Errorf("entry[%d]: Target is empty", i)
		}
		if e.Component == "" {
			t.Errorf("entry[%d]: Component is empty", i)
		}
	}
}

func TestAllSymlinks_TargetsContainHOME(t *testing.T) {
	for _, e := range AllSymlinks() {
		if len(e.Target) < 5 || e.Target[:5] != "$HOME" {
			t.Errorf(
				"entry %s: Target %q does not start with $HOME",
				e.Source, e.Target,
			)
		}
	}
}

func TestManagedTargets_NoDuplicates(t *testing.T) {
	targets := ManagedTargets()
	if len(targets) == 0 {
		t.Fatal("ManagedTargets() returned empty slice")
	}

	seen := make(map[string]struct{})
	for _, tgt := range targets {
		if _, ok := seen[tgt]; ok {
			t.Errorf("duplicate target: %s", tgt)
		}
		seen[tgt] = struct{}{}
	}
}

func TestManagedTargets_SubsetOfAllSymlinks(t *testing.T) {
	all := make(map[string]struct{})
	for _, e := range AllSymlinks() {
		all[e.Target] = struct{}{}
	}

	for _, tgt := range ManagedTargets() {
		if _, ok := all[tgt]; !ok {
			t.Errorf(
				"ManagedTargets() contains %q not in AllSymlinks()",
				tgt,
			)
		}
	}
}

// setupTestDirs creates a temp directory structure simulating the
// dotfiles repo layout with a configs/ subdirectory and a target
// home. It returns (rootDir, homeDir).
func setupTestDirs(t *testing.T) (string, string) {
	t.Helper()
	tmp := t.TempDir()
	rootDir := filepath.Join(tmp, "repo")
	homeDir := filepath.Join(tmp, "home")
	if err := os.MkdirAll(
		filepath.Join(rootDir, "configs"), 0o755,
	); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", homeDir)
	return rootDir, homeDir
}

// createSourceFile creates a file under rootDir/configs/relPath.
func createSourceFile(
	t *testing.T, rootDir, relPath, content string,
) string {
	t.Helper()
	full := filepath.Join(rootDir, "configs", relPath)
	if err := os.MkdirAll(
		filepath.Dir(full), 0o755,
	); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		full, []byte(content), 0o644,
	); err != nil {
		t.Fatal(err)
	}
	return full
}

// createSourceDir creates a directory under rootDir/configs/relPath.
func createSourceDir(
	t *testing.T, rootDir, relPath string,
) string {
	t.Helper()
	full := filepath.Join(rootDir, "configs", relPath)
	if err := os.MkdirAll(full, 0o755); err != nil {
		t.Fatal(err)
	}
	return full
}

func TestInspectSymlink(t *testing.T) {
	rootDir, _ := setupTestDirs(t)

	sourcePath := createSourceFile(
		t, rootDir, "test/file.txt", "hello",
	)
	targetPath := os.ExpandEnv("$HOME/.config/test-file.txt")

	entry := SymlinkEntry{
		Source:    "test/file.txt",
		Target:    "$HOME/.config/test-file.txt",
		Component: "Test",
	}

	t.Run("missing target", func(t *testing.T) {
		status := InspectSymlink(entry, rootDir)
		if status != SymlinkMissing {
			t.Errorf("got %d, want SymlinkMissing", status)
		}
	})

	t.Run("correct symlink", func(t *testing.T) {
		if err := os.MkdirAll(
			filepath.Dir(targetPath), 0o755,
		); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(sourcePath, targetPath); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { os.Remove(targetPath) })

		status := InspectSymlink(entry, rootDir)
		if status != SymlinkAlreadyCorrect {
			t.Errorf("got %d, want SymlinkAlreadyCorrect", status)
		}
	})

	t.Run("wrong symlink target", func(t *testing.T) {
		if err := os.MkdirAll(
			filepath.Dir(targetPath), 0o755,
		); err != nil {
			t.Fatal(err)
		}
		wrongTarget := filepath.Join(t.TempDir(), "wrong")
		os.WriteFile(wrongTarget, []byte("wrong"), 0o644)
		os.Remove(targetPath)
		if err := os.Symlink(wrongTarget, targetPath); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { os.Remove(targetPath) })

		status := InspectSymlink(entry, rootDir)
		if status != SymlinkWouldReplace {
			t.Errorf(
				"got %d, want SymlinkWouldReplace", status,
			)
		}
	})

	t.Run("regular file at target", func(t *testing.T) {
		os.Remove(targetPath)
		if err := os.MkdirAll(
			filepath.Dir(targetPath), 0o755,
		); err != nil {
			t.Fatal(err)
		}
		os.WriteFile(targetPath, []byte("existing"), 0o644)
		t.Cleanup(func() { os.Remove(targetPath) })

		status := InspectSymlink(entry, rootDir)
		if status != SymlinkWouldReplace {
			t.Errorf(
				"got %d, want SymlinkWouldReplace", status,
			)
		}
	})
}

func TestInspectComponent(t *testing.T) {
	rootDir, homeDir := setupTestDirs(t)

	// Create source files for "Zsh" component entries.
	createSourceDir(t, rootDir, "zsh")
	createSourceFile(t, rootDir, "zsh/.zshenv", "env")

	tests := []struct {
		name      string
		setup     func(t *testing.T)
		component string
		want      string
	}{
		{
			name:      "no targets exist yields would configure",
			setup:     func(t *testing.T) {},
			component: "Zsh",
			want:      "would configure",
		},
		{
			name: "all correct yields already configured",
			setup: func(t *testing.T) {
				for _, e := range AllSymlinks() {
					if e.Component != "Zsh" {
						continue
					}
					src := filepath.Join(
						rootDir, "configs", e.Source,
					)
					tgt := os.ExpandEnv(e.Target)
					os.MkdirAll(filepath.Dir(tgt), 0o755)
					os.Symlink(src, tgt)
				}
			},
			component: "Zsh",
			want:      "already configured",
		},
		{
			name: "regular file yields would replace",
			setup: func(t *testing.T) {
				for _, e := range AllSymlinks() {
					if e.Component != "Zsh" {
						continue
					}
					tgt := os.ExpandEnv(e.Target)
					os.MkdirAll(filepath.Dir(tgt), 0o755)
					os.WriteFile(tgt, []byte("x"), 0o644)
				}
			},
			component: "Zsh",
			want:      "would replace",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up any targets before each subtest.
			for _, e := range AllSymlinks() {
				if e.Component == "Zsh" {
					os.RemoveAll(os.ExpandEnv(e.Target))
				}
			}
			// Restore HOME in case subtests mutate it.
			t.Setenv("HOME", homeDir)
			tt.setup(t)
			got := InspectComponent(tt.component, rootDir)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDiffSymlink(t *testing.T) {
	rootDir, _ := setupTestDirs(t)
	sourcePath := createSourceFile(
		t, rootDir, "diff/file.txt", "hello",
	)
	targetPath := os.ExpandEnv("$HOME/.config/diff-file.txt")

	entry := SymlinkEntry{
		Source:    "diff/file.txt",
		Target:    "$HOME/.config/diff-file.txt",
		Component: "Test",
	}

	t.Run("target missing returns empty", func(t *testing.T) {
		d := DiffSymlink(entry, rootDir)
		if d != "" {
			t.Errorf("expected empty, got %q", d)
		}
	})

	t.Run("correct symlink returns empty", func(t *testing.T) {
		os.MkdirAll(filepath.Dir(targetPath), 0o755)
		os.Symlink(sourcePath, targetPath)
		t.Cleanup(func() { os.Remove(targetPath) })

		d := DiffSymlink(entry, rootDir)
		if d != "" {
			t.Errorf("expected empty, got %q", d)
		}
	})

	t.Run("wrong symlink returns redirect info", func(t *testing.T) {
		os.Remove(targetPath)
		wrong := filepath.Join(t.TempDir(), "wrong")
		os.WriteFile(wrong, []byte("wrong"), 0o644)
		os.MkdirAll(filepath.Dir(targetPath), 0o755)
		os.Symlink(wrong, targetPath)
		t.Cleanup(func() { os.Remove(targetPath) })

		d := DiffSymlink(entry, rootDir)
		if d == "" {
			t.Error("expected non-empty diff")
		}
	})

	t.Run("regular file returns replace info", func(t *testing.T) {
		os.Remove(targetPath)
		os.MkdirAll(filepath.Dir(targetPath), 0o755)
		os.WriteFile(targetPath, []byte("x"), 0o644)
		t.Cleanup(func() { os.Remove(targetPath) })

		d := DiffSymlink(entry, rootDir)
		if d == "" {
			t.Error("expected non-empty diff for regular file")
		}
	})

	t.Run("directory returns replace directory", func(t *testing.T) {
		os.Remove(targetPath)
		os.MkdirAll(targetPath, 0o755)
		t.Cleanup(func() { os.RemoveAll(targetPath) })

		d := DiffSymlink(entry, rootDir)
		if d == "" {
			t.Error("expected non-empty diff for directory")
		}
	})
}

func TestDiffComponent(t *testing.T) {
	rootDir, _ := setupTestDirs(t)

	// Create source for Oh-My-Posh.
	createSourceFile(
		t, rootDir, "oh-my-posh/config.omp.yaml", "format=test",
	)

	// No targets exist, so no diffs (targets are missing,
	// DiffSymlink returns "").
	diffs := DiffComponent("OhMyPosh", rootDir)
	if len(diffs) != 0 {
		t.Errorf(
			"expected no diffs for missing targets, got %v", diffs,
		)
	}

	// Create a regular file at the target to trigger a diff.
	target := os.ExpandEnv("$HOME/.config/oh-my-posh/config.omp.yaml")
	os.MkdirAll(filepath.Dir(target), 0o755)
	os.WriteFile(target, []byte("old"), 0o644)

	diffs = DiffComponent("OhMyPosh", rootDir)
	if len(diffs) == 0 {
		t.Error("expected diffs for existing regular file target")
	}
}

func TestApplySymlink_CreatesSymlink(t *testing.T) {
	rootDir, _ := setupTestDirs(t)
	sourcePath := createSourceFile(
		t, rootDir, "apply/file.txt", "content",
	)

	entry := SymlinkEntry{
		Source:    "apply/file.txt",
		Target:    "$HOME/.config/apply-file.txt",
		Component: "Test",
	}

	bm := backup.NewManager(false)

	err := ApplySymlink(entry, rootDir, bm, false, nil)
	if err != nil {
		t.Fatalf("ApplySymlink: %v", err)
	}

	targetPath := os.ExpandEnv(entry.Target)
	link, err := os.Readlink(targetPath)
	if err != nil {
		t.Fatalf("target is not a symlink: %v", err)
	}

	canonSource, _ := filepath.Abs(sourcePath)
	canonLink, _ := filepath.Abs(link)
	if canonSource != canonLink {
		t.Errorf(
			"symlink points to %q, want %q",
			canonLink, canonSource,
		)
	}
}

func TestApplySymlink_AlreadyCorrect(t *testing.T) {
	rootDir, _ := setupTestDirs(t)
	sourcePath := createSourceFile(
		t, rootDir, "correct/file.txt", "content",
	)

	entry := SymlinkEntry{
		Source:    "correct/file.txt",
		Target:    "$HOME/.config/correct-file.txt",
		Component: "Test",
	}

	targetPath := os.ExpandEnv(entry.Target)
	os.MkdirAll(filepath.Dir(targetPath), 0o755)
	os.Symlink(sourcePath, targetPath)

	bm := backup.NewManager(false)

	err := ApplySymlink(entry, rootDir, bm, false, nil)
	if err != nil {
		t.Fatalf("ApplySymlink: %v", err)
	}

	// Symlink should still be correct (not recreated).
	link, _ := os.Readlink(targetPath)
	canonSource, _ := filepath.Abs(sourcePath)
	canonLink, _ := filepath.Abs(link)
	if canonSource != canonLink {
		t.Errorf("symlink was changed unexpectedly")
	}
}

func TestApplySymlink_BacksUpExistingFile(t *testing.T) {
	rootDir, _ := setupTestDirs(t)
	createSourceFile(t, rootDir, "bak/file.txt", "new")

	entry := SymlinkEntry{
		Source:    "bak/file.txt",
		Target:    "$HOME/.config/bak-file.txt",
		Component: "Test",
	}

	// Place an existing file at the target.
	targetPath := os.ExpandEnv(entry.Target)
	os.MkdirAll(filepath.Dir(targetPath), 0o755)
	os.WriteFile(targetPath, []byte("old-content"), 0o644)

	bm := backup.NewManager(false)

	err := ApplySymlink(entry, rootDir, bm, false, nil)
	if err != nil {
		t.Fatalf("ApplySymlink: %v", err)
	}

	// Verify the backup was created.
	if !bm.Exists() {
		t.Error("backup directory was not created")
	}

	// Verify the target is now a symlink.
	if _, err := os.Readlink(targetPath); err != nil {
		t.Errorf("target should be a symlink after apply: %v", err)
	}
}

func TestApplySymlink_MissingSource(t *testing.T) {
	rootDir, _ := setupTestDirs(t)

	entry := SymlinkEntry{
		Source:    "nonexistent/file.txt",
		Target:    "$HOME/.config/nope.txt",
		Component: "Test",
	}

	bm := backup.NewManager(false)

	err := ApplySymlink(entry, rootDir, bm, false, nil)
	if err == nil {
		t.Fatal("expected error for missing source")
	}
}

func TestApplySymlink_DryRun(t *testing.T) {
	rootDir, _ := setupTestDirs(t)
	createSourceFile(t, rootDir, "dry/file.txt", "content")

	entry := SymlinkEntry{
		Source:    "dry/file.txt",
		Target:    "$HOME/.config/dry-file.txt",
		Component: "Test",
	}

	bm := backup.NewManager(true)

	err := ApplySymlink(entry, rootDir, bm, true, nil)
	if err != nil {
		t.Fatalf("ApplySymlink dry run: %v", err)
	}

	targetPath := os.ExpandEnv(entry.Target)
	if _, err := os.Lstat(targetPath); err == nil {
		t.Error("dry run should not create the symlink")
	}
}

func TestApplySymlink_ReplacesWrongSymlink(t *testing.T) {
	rootDir, _ := setupTestDirs(t)
	createSourceFile(t, rootDir, "replace/file.txt", "right")

	entry := SymlinkEntry{
		Source:    "replace/file.txt",
		Target:    "$HOME/.config/replace-file.txt",
		Component: "Test",
	}

	// Create a symlink pointing to the wrong target.
	targetPath := os.ExpandEnv(entry.Target)
	os.MkdirAll(filepath.Dir(targetPath), 0o755)
	wrongSrc := filepath.Join(t.TempDir(), "wrong")
	os.WriteFile(wrongSrc, []byte("wrong"), 0o644)
	os.Symlink(wrongSrc, targetPath)

	bm := backup.NewManager(false)

	err := ApplySymlink(entry, rootDir, bm, false, nil)
	if err != nil {
		t.Fatalf("ApplySymlink: %v", err)
	}

	link, err := os.Readlink(targetPath)
	if err != nil {
		t.Fatalf("target should be a symlink: %v", err)
	}

	correctSource := filepath.Join(
		rootDir, "configs", "replace/file.txt",
	)
	canonSource, _ := filepath.Abs(correctSource)
	canonLink, _ := filepath.Abs(link)
	if canonSource != canonLink {
		t.Errorf(
			"symlink points to %q, want %q",
			canonLink, canonSource,
		)
	}
}

func TestApplyAllSymlinks(t *testing.T) {
	rootDir, _ := setupTestDirs(t)

	// Create source files matching the "OhMyPosh" component.
	createSourceFile(
		t, rootDir, "oh-my-posh/config.omp.yaml", "format=test",
	)

	bm := backup.NewManager(false)

	err := ApplyAllSymlinks("OhMyPosh", rootDir, bm, false, nil)
	if err != nil {
		t.Fatalf("ApplyAllSymlinks: %v", err)
	}

	// Verify the symlink was created.
	target := os.ExpandEnv("$HOME/.config/oh-my-posh/config.omp.yaml")
	if _, err := os.Readlink(target); err != nil {
		t.Errorf("Oh-My-Posh symlink not created: %v", err)
	}
}

func TestRemoveComponentSymlinks(t *testing.T) {
	rootDir, _ := setupTestDirs(t)
	sourcePath := createSourceFile(
		t, rootDir, "oh-my-posh/config.omp.yaml", "format=test",
	)

	// Create the symlink manually.
	target := os.ExpandEnv("$HOME/.config/oh-my-posh/config.omp.yaml")
	os.MkdirAll(filepath.Dir(target), 0o755)
	os.Symlink(sourcePath, target)

	err := RemoveComponentSymlinks("OhMyPosh", rootDir, nil)
	if err != nil {
		t.Fatalf("RemoveComponentSymlinks: %v", err)
	}

	if _, err := os.Lstat(target); err == nil {
		t.Error("symlink should have been removed")
	}
}

func TestRemoveComponentSymlinks_LeavesWrongTarget(t *testing.T) {
	rootDir, _ := setupTestDirs(t)
	createSourceFile(
		t, rootDir, "oh-my-posh/config.omp.yaml", "format=test",
	)

	// Create a symlink pointing somewhere else entirely.
	target := os.ExpandEnv("$HOME/.config/oh-my-posh/config.omp.yaml")
	os.MkdirAll(filepath.Dir(target), 0o755)
	wrong := filepath.Join(t.TempDir(), "other")
	os.WriteFile(wrong, []byte("other"), 0o644)
	os.Symlink(wrong, target)

	err := RemoveComponentSymlinks("OhMyPosh", rootDir, nil)
	if err != nil {
		t.Fatalf("RemoveComponentSymlinks: %v", err)
	}

	// The symlink should NOT be removed because it points elsewhere.
	if _, err := os.Lstat(target); err != nil {
		t.Error(
			"symlink pointing elsewhere should be left intact",
		)
	}
}

func TestRemoveComponentSymlinks_SkipsRegularFile(t *testing.T) {
	rootDir, _ := setupTestDirs(t)
	createSourceFile(
		t, rootDir, "oh-my-posh/config.omp.yaml", "format=test",
	)

	// Place a regular file at the target location.
	target := os.ExpandEnv("$HOME/.config/oh-my-posh/config.omp.yaml")
	os.MkdirAll(filepath.Dir(target), 0o755)
	os.WriteFile(target, []byte("user-data"), 0o644)

	err := RemoveComponentSymlinks("OhMyPosh", rootDir, nil)
	if err != nil {
		t.Fatalf("RemoveComponentSymlinks: %v", err)
	}

	// Regular file should be untouched.
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("regular file was removed: %v", err)
	}
	if string(data) != "user-data" {
		t.Error("regular file content was modified")
	}
}

func TestResolveSource_BaseOnly(t *testing.T) {
	rootDir, _ := setupTestDirs(t)
	createSourceFile(t, rootDir, "test/config", "base")

	// Without variant, base path is returned.
	got := resolveSource(rootDir, "test/config")
	want := filepath.Join(rootDir, "configs", "test/config")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestResolveSource_OSVariant(t *testing.T) {
	rootDir, _ := setupTestDirs(t)

	// Create both a base file and an OS-specific variant.
	createSourceFile(t, rootDir, "test/config2", "base")

	// runtime.GOOS gives us the current OS (e.g. "darwin").
	variant := "test/config2##" + goos()
	createSourceFile(t, rootDir, variant, "variant")

	got := resolveSource(rootDir, "test/config2")
	want := filepath.Join(rootDir, "configs", variant)
	if got != want {
		t.Errorf("got %q, want %q (expected OS variant)", got, want)
	}
}

func TestApplyAllSymlinks_ErrorOnMissingSource(t *testing.T) {
	rootDir, _ := setupTestDirs(t)

	// Oh-My-Posh source does NOT exist, so ApplyAllSymlinks should
	// fail on the first entry with a missing source error.
	bm := backup.NewManager(false)

	err := ApplyAllSymlinks("OhMyPosh", rootDir, bm, false, nil)
	if err == nil {
		t.Fatal("expected error when source is missing")
	}
}

func TestRollbackSymlinks(t *testing.T) {
	rootDir, _ := setupTestDirs(t)
	sourcePath := createSourceFile(
		t, rootDir, "oh-my-posh/config.omp.yaml", "format=test",
	)

	// Create the symlink as ApplySymlink would.
	target := os.ExpandEnv("$HOME/.config/oh-my-posh/config.omp.yaml")
	os.MkdirAll(filepath.Dir(target), 0o755)
	os.Symlink(sourcePath, target)

	// Verify symlink exists before rollback.
	if _, err := os.Lstat(target); err != nil {
		t.Fatalf("setup: symlink should exist: %v", err)
	}

	rollbackSymlinks("OhMyPosh", rootDir, nil)

	// After rollback, the symlink should be gone.
	if _, err := os.Lstat(target); err == nil {
		t.Error("symlink should have been removed by rollback")
	}
}

func TestRollbackSymlinks_SkipsWrongTarget(t *testing.T) {
	rootDir, _ := setupTestDirs(t)
	createSourceFile(
		t, rootDir, "oh-my-posh/config.omp.yaml", "format=test",
	)

	// Create a symlink pointing to the wrong place.
	target := os.ExpandEnv("$HOME/.config/oh-my-posh/config.omp.yaml")
	os.MkdirAll(filepath.Dir(target), 0o755)
	wrong := filepath.Join(t.TempDir(), "other")
	os.WriteFile(wrong, []byte("other"), 0o644)
	os.Symlink(wrong, target)

	rollbackSymlinks("OhMyPosh", rootDir, nil)

	// Symlink to wrong target should be left intact.
	if _, err := os.Lstat(target); err != nil {
		t.Error(
			"rollback should not remove symlink to wrong target",
		)
	}
}

func TestRollbackSymlinks_SkipsNonSymlink(t *testing.T) {
	rootDir, _ := setupTestDirs(t)
	createSourceFile(
		t, rootDir, "oh-my-posh/config.omp.yaml", "format=test",
	)

	// Place a regular file at the target.
	target := os.ExpandEnv("$HOME/.config/oh-my-posh/config.omp.yaml")
	os.MkdirAll(filepath.Dir(target), 0o755)
	os.WriteFile(target, []byte("data"), 0o644)

	rollbackSymlinks("OhMyPosh", rootDir, nil)

	// Regular file should not be removed.
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("regular file was removed: %v", err)
	}
	if string(data) != "data" {
		t.Error("file content was altered")
	}
}

func TestAllComponents_NonEmpty(t *testing.T) {
	comps := AllComponents()
	if len(comps) == 0 {
		t.Fatal("AllComponents() returned empty slice")
	}

	for i, c := range comps {
		if c.Name == "" {
			t.Errorf("component[%d]: Name is empty", i)
		}
		if c.Icon == "" {
			t.Errorf("component[%d] %q: Icon is empty", i, c.Name)
		}
	}
}

func TestManagedTargets_CountMatchesUniqueTargets(t *testing.T) {
	all := AllSymlinks()
	unique := make(map[string]struct{})
	for _, e := range all {
		unique[e.Target] = struct{}{}
	}

	targets := ManagedTargets()
	if len(targets) != len(unique) {
		t.Errorf(
			"ManagedTargets() returned %d, expected %d unique",
			len(targets), len(unique),
		)
	}
}

// goos returns runtime.GOOS for use in test helpers.
func goos() string {
	return runtime.GOOS
}

// newTestRunner creates an executor.Runner backed by a log file in
// the given temp directory, suitable for testing.
func newTestRunner(t *testing.T) *executor.Runner {
	t.Helper()
	logPath := filepath.Join(t.TempDir(), "test.log")
	lf, err := executor.NewLogFile(logPath)
	if err != nil {
		t.Fatalf("NewLogFile: %v", err)
	}
	t.Cleanup(func() { lf.Close() })
	r := executor.NewRunner(lf, false)
	r.EnableVerboseChannel(64)
	return r
}

func TestApplySymlink_WithRunner_CreatesSymlink(t *testing.T) {
	rootDir, _ := setupTestDirs(t)
	createSourceFile(
		t, rootDir, "runner/file.txt", "content",
	)

	entry := SymlinkEntry{
		Source:    "runner/file.txt",
		Target:    "$HOME/.config/runner-file.txt",
		Component: "Test",
	}

	bm := backup.NewManager(false)
	runner := newTestRunner(t)

	err := ApplySymlink(entry, rootDir, bm, false, runner)
	if err != nil {
		t.Fatalf("ApplySymlink with runner: %v", err)
	}

	// Verify verbose output was emitted.
	lines := runner.RecentLinesSnapshot()
	if len(lines) == 0 {
		t.Error("expected verbose output from runner")
	}

	targetPath := os.ExpandEnv(entry.Target)
	if _, err := os.Readlink(targetPath); err != nil {
		t.Errorf("target should be a symlink: %v", err)
	}
}

func TestApplySymlink_WithRunner_AlreadyCorrect(t *testing.T) {
	rootDir, _ := setupTestDirs(t)
	sourcePath := createSourceFile(
		t, rootDir, "rcorrect/file.txt", "content",
	)

	entry := SymlinkEntry{
		Source:    "rcorrect/file.txt",
		Target:    "$HOME/.config/rcorrect-file.txt",
		Component: "Test",
	}

	// Pre-create correct symlink.
	targetPath := os.ExpandEnv(entry.Target)
	os.MkdirAll(filepath.Dir(targetPath), 0o755)
	os.Symlink(sourcePath, targetPath)

	bm := backup.NewManager(false)
	runner := newTestRunner(t)

	err := ApplySymlink(entry, rootDir, bm, false, runner)
	if err != nil {
		t.Fatalf("ApplySymlink: %v", err)
	}

	// The "already correct" verbose line should have been emitted.
	lines := runner.RecentLinesSnapshot()
	found := false
	for _, l := range lines {
		if len(l) > 0 {
			found = true
		}
	}
	if !found {
		t.Error("expected verbose output for already-correct case")
	}
}

func TestApplySymlink_WithRunner_BackupAndReplace(t *testing.T) {
	rootDir, _ := setupTestDirs(t)
	createSourceFile(t, rootDir, "rbak/file.txt", "new")

	entry := SymlinkEntry{
		Source:    "rbak/file.txt",
		Target:    "$HOME/.config/rbak-file.txt",
		Component: "Test",
	}

	// Place existing file at target.
	targetPath := os.ExpandEnv(entry.Target)
	os.MkdirAll(filepath.Dir(targetPath), 0o755)
	os.WriteFile(targetPath, []byte("old"), 0o644)

	bm := backup.NewManager(false)
	runner := newTestRunner(t)

	err := ApplySymlink(entry, rootDir, bm, false, runner)
	if err != nil {
		t.Fatalf("ApplySymlink: %v", err)
	}

	// Verify verbose lines include "Backing up" and "Symlink".
	lines := runner.RecentLinesSnapshot()
	hasBackup := false
	hasSymlink := false
	for _, l := range lines {
		if len(l) > 8 && l[:9] == "Backing u" {
			hasBackup = true
		}
		if len(l) > 6 && l[:7] == "Symlink" {
			hasSymlink = true
		}
	}
	if !hasBackup {
		t.Error("expected 'Backing up' verbose line")
	}
	if !hasSymlink {
		t.Error("expected 'Symlink' verbose line")
	}
}

func TestRemoveComponentSymlinks_WithRunner(t *testing.T) {
	rootDir, _ := setupTestDirs(t)
	sourcePath := createSourceFile(
		t, rootDir, "oh-my-posh/config.omp.yaml", "format=test",
	)

	target := os.ExpandEnv("$HOME/.config/oh-my-posh/config.omp.yaml")
	os.MkdirAll(filepath.Dir(target), 0o755)
	os.Symlink(sourcePath, target)

	runner := newTestRunner(t)

	err := RemoveComponentSymlinks("OhMyPosh", rootDir, runner)
	if err != nil {
		t.Fatalf("RemoveComponentSymlinks: %v", err)
	}

	if _, err := os.Lstat(target); err == nil {
		t.Error("symlink should have been removed")
	}

	lines := runner.RecentLinesSnapshot()
	if len(lines) == 0 {
		t.Error("expected verbose output from runner")
	}
}
