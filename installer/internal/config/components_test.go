package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestClearZshInitCaches covers the contract the installer relies
// on: *.zsh files under ~/.cache/zsh are removed; subdirectories
// and non-.zsh files are preserved; a missing cache dir is a
// no-op rather than an error (first-run machines don't have one
// yet).
func TestClearZshInitCaches(t *testing.T) {
	t.Run("clears zsh files, preserves rest", func(t *testing.T) {
		home := t.TempDir()
		cacheDir := filepath.Join(home, ".cache", "zsh")
		if err := os.MkdirAll(cacheDir, 0o755); err != nil {
			t.Fatal(err)
		}
		// Files that SHOULD be cleared.
		for _, name := range []string{
			"zoxide.zsh", "atuin.zsh", "oh-my-posh.zsh",
		} {
			if err := os.WriteFile(
				filepath.Join(cacheDir, name), []byte("cached"), 0o644,
			); err != nil {
				t.Fatal(err)
			}
		}
		// Non-.zsh file + subdir that must survive.
		if err := os.WriteFile(
			filepath.Join(cacheDir, "notes.txt"), []byte("keep me"), 0o644,
		); err != nil {
			t.Fatal(err)
		}
		subdir := filepath.Join(cacheDir, "ohmyzsh")
		if err := os.MkdirAll(subdir, 0o755); err != nil {
			t.Fatal(err)
		}

		if err := clearZshInitCaches(home, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		for _, name := range []string{"zoxide.zsh", "atuin.zsh", "oh-my-posh.zsh"} {
			if _, err := os.Stat(filepath.Join(cacheDir, name)); !os.IsNotExist(err) {
				t.Errorf("%s should have been removed, err=%v", name, err)
			}
		}
		if _, err := os.Stat(filepath.Join(cacheDir, "notes.txt")); err != nil {
			t.Errorf("notes.txt was removed: %v", err)
		}
		if _, err := os.Stat(subdir); err != nil {
			t.Errorf("subdir was removed: %v", err)
		}
	})

	t.Run("missing cache dir is no-op", func(t *testing.T) {
		home := t.TempDir()
		// Do NOT create ~/.cache/zsh — first-run case.
		if err := clearZshInitCaches(home, nil); err != nil {
			t.Errorf("missing dir must be silent, got %v", err)
		}
	})
}
