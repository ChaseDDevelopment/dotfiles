package registry

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestResolveCargoFallback covers the case dns1 hit: cargo is
// installed at ~/.cargo/bin/cargo but not on the installer's PATH
// because augmentPath() snapshotted PATH before rust ran.
func TestResolveCargoFallback(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX-only path layout")
	}
	home := t.TempDir()
	cargoBin := filepath.Join(home, ".cargo", "bin")
	if err := os.MkdirAll(cargoBin, 0o755); err != nil {
		t.Fatal(err)
	}
	cargoPath := filepath.Join(cargoBin, "cargo")
	if err := os.WriteFile(cargoPath, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("HOME", home)
	t.Setenv("PATH", "/usr/bin:/bin") // no cargo here

	got := resolveCargo()
	if got != cargoPath {
		t.Fatalf("resolveCargo() = %q, want %q", got, cargoPath)
	}
}

// TestResolveCargoPrefersPATH ensures we don't needlessly reach into
// $HOME when cargo is already on PATH (matches behavior on a healthy
// macOS/homebrew box).
func TestResolveCargoPrefersPATH(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX-only path layout")
	}
	dir := t.TempDir()
	pathCargo := filepath.Join(dir, "cargo")
	if err := os.WriteFile(pathCargo, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Also drop a fallback under HOME; resolveCargo should NOT pick it.
	home := t.TempDir()
	fallbackBin := filepath.Join(home, ".cargo", "bin")
	if err := os.MkdirAll(fallbackBin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fallbackBin, "cargo"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("HOME", home)
	t.Setenv("PATH", dir)

	got := resolveCargo()
	if got != pathCargo {
		t.Fatalf("resolveCargo() = %q, want %q (PATH version)", got, pathCargo)
	}
}

// TestResolveCargoMissing returns the bare "cargo" when neither PATH
// nor ~/.cargo/bin has the binary — the subsequent exec produces the
// real "executable file not found" error the user needs to see.
func TestResolveCargoMissing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PATH", "/nonexistent")

	if got := resolveCargo(); got != "cargo" {
		t.Fatalf("resolveCargo() = %q, want %q", got, "cargo")
	}
}
