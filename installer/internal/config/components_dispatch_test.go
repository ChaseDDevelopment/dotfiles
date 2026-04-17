package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestRunPostInstallDispatchesAllComponents covers the switch in
// runPostInstall by calling it once per real component name. Each
// branch is a 1-2 statement pass-through, so hitting all of them
// pulls the dispatch coverage from ~22% up near 100%. Side effects
// are driven by stubbed PATH binaries from components_setup_test.
func TestRunPostInstallDispatchesAllComponents(t *testing.T) {
	sc, home := newComponentSetup(t)
	bin := filepath.Join(home, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	// Minimal stubs so every setup* helper finishes.
	for _, name := range []string{"tmux", "pgrep", "cargo", "nvim", "ya", "git", "zsh", "brew", "bash"} {
		writeTool(t, bin, name, "#!/bin/sh\nexit 0\n")
	}
	for _, comp := range []string{"Zsh", "Tmux", "Neovim", "Atuin", "Yazi", "Ghostty", "Git"} {
		if err := runPostInstall(context.Background(), comp, sc); err != nil {
			t.Fatalf("runPostInstall(%q): %v", comp, err)
		}
	}
}

// TestSetupComponentHappyPathAndDryRun drives the SetupComponent
// happy path (symlinks apply, post-install runs) and the dry-run
// branch (skip post-install).
func TestSetupComponentHappyPathAndDryRun(t *testing.T) {
	sc, home := newComponentSetup(t)
	bin := filepath.Join(home, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	writeTool(t, bin, "git", "#!/bin/sh\nexit 0\n")

	// Plant Git symlink sources under RootDir so ApplyAllSymlinks finds them.
	gitConfigs := filepath.Join(sc.RootDir, "configs", "git")
	if err := os.MkdirAll(gitConfigs, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"config", "ignore"} {
		if err := os.WriteFile(filepath.Join(gitConfigs, f), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(sc.RootDir, "configs", "lazygit"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := SetupComponent(context.Background(),
		Component{Name: "Git", RequiredCmd: "git"}, sc); err != nil {
		t.Fatalf("SetupComponent Git: %v", err)
	}

	sc.DryRun = true
	if err := SetupComponent(context.Background(),
		Component{Name: "Git", RequiredCmd: "git"}, sc); err != nil {
		t.Fatalf("SetupComponent Git dry-run: %v", err)
	}
}
