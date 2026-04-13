package platform

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestDetectPackageManagerOrder(t *testing.T) {
	dir := t.TempDir()
	fakebin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(fakebin, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fakebin)

	write := func(name string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(fakebin, name), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	write("brew")
	if runtimeGOOS := os.Getenv("GOOS_FOR_TEST"); runtimeGOOS != "" {
		_ = runtimeGOOS
	}
	// On Linux hosts, brew is only a fallback; apt-get should win.
	write("apt-get")
	want := PkgApt
	if runtime.GOOS == "darwin" {
		want = PkgBrew
	}
	if got := detectPackageManager(); got != want {
		t.Fatalf("detectPackageManager with apt-get+brew = %v, want %v", got, want)
	}

	if err := os.Remove(filepath.Join(fakebin, "apt-get")); err != nil {
		t.Fatal(err)
	}
	write("pacman")
	if got := detectPackageManager(); got != PkgPacman && got != PkgBrew {
		t.Fatalf("unexpected package manager detection: %v", got)
	}
}
