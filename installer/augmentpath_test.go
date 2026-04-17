package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestAugmentPathAddsNonexistentDirs covers the ruff/uv regression:
// augmentPath previously gated each entry on os.Stat, so when a
// tool installed mid-run created ~/.local/bin (for uv) or ~/.cargo/
// bin (for rust), subsequent tasks failed to find the binary because
// PATH was snapshotted before the dirs existed. After the fix, all
// target dirs must be on PATH whether or not they exist yet.
func TestAugmentPathAddsNonexistentDirs(t *testing.T) {
	home := t.TempDir() // fresh HOME, none of the target dirs exist
	t.Setenv("HOME", home)
	t.Setenv("PATH", "/usr/bin:/bin")

	augmentPath()

	got := os.Getenv("PATH")
	want := []string{
		filepath.Join(home, ".cargo", "bin"),
		filepath.Join(home, ".local", "bin"),
		filepath.Join(home, ".bun", "bin"),
		filepath.Join(home, ".atuin", "bin"),
		filepath.Join(home, ".dotnet"),
		"/usr/local/go/bin",
	}
	for _, d := range want {
		if !strings.Contains(got, d) {
			t.Errorf("PATH missing %q even though augmentPath should add unconditionally\nPATH=%s", d, got)
		}
	}
	// Original entries must still be present.
	for _, d := range []string{"/usr/bin", "/bin"} {
		if !strings.Contains(got, d) {
			t.Errorf("PATH dropped pre-existing entry %q\nPATH=%s", d, got)
		}
	}
}
