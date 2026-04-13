package registry

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestInstallGhCLIFailureBranches walks each `if err != nil` branch
// in installGhCLI by replacing one stub at a time with a failing
// variant. Each subtest uses a fresh PATH so the prior test's stub
// doesn't leak.
func TestInstallGhCLIFailureBranches(t *testing.T) {
	type plant struct {
		name, body string
	}
	mkBin := func(t *testing.T, plants []plant) (*InstallContext, string) {
		t.Helper()
		ic, home := setupClosureEnv(t, "apt")
		bin := filepath.Join(home, "bin")
		// Apply per-test overrides.
		for _, p := range plants {
			if err := os.WriteFile(filepath.Join(bin, p.name), []byte(p.body), 0o755); err != nil {
				t.Fatal(err)
			}
		}
		return ic, home
	}

	cases := []struct {
		name    string
		plants  []plant
		wantSub string
	}{
		{
			name: "dpkg arch empty",
			plants: []plant{{
				name: "dpkg",
				body: "#!/bin/sh\nprintf ''\nexit 0\n",
			}},
			wantSub: "empty arch",
		},
		{
			name: "dpkg returns error",
			plants: []plant{{
				name: "dpkg",
				body: "#!/bin/sh\nexit 1\n",
			}},
			wantSub: "detect arch",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ic, _ := mkBin(t, tc.plants)
			err := installGhCLI(context.Background(), ic)
			if err == nil || !strings.Contains(err.Error(), tc.wantSub) {
				t.Fatalf("installGhCLI = %v, want %q substring", err, tc.wantSub)
			}
		})
	}
}

// TestIsNerdFontInstalledFalse covers the "no font directory hit →
// false" branch — plant nothing under any candidate path so the
// scan exhausts and returns false.
func TestIsNerdFontInstalledFalse(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	// Empty PATH so the darwin brew probe also fails fast.
	t.Setenv("PATH", "")
	if isNerdFontInstalled() {
		t.Fatal("expected false when no nerd fonts present")
	}
}
