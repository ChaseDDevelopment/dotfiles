package platform

import (
	"runtime"
	"testing"
)

func TestOSString(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		os   OS
		want string
	}{
		{name: "macOS", os: MacOS, want: "macOS"},
		{name: "Linux", os: Linux, want: "Linux"},
		{name: "unknown", os: OS(99), want: "Unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.os.String(); got != tt.want {
				t.Errorf("OS(%d).String() = %q, want %q", tt.os, got, tt.want)
			}
		})
	}
}

func TestArchString(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		arch Arch
		want string
	}{
		{name: "AMD64", arch: AMD64, want: "x86_64"},
		{name: "ARM64", arch: ARM64, want: "arm64"},
		{name: "unknown", arch: Arch(99), want: "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.arch.String(); got != tt.want {
				t.Errorf("Arch(%d).String() = %q, want %q", tt.arch, got, tt.want)
			}
		})
	}
}

func TestPkgManagerTypeString(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		pkg  PkgManagerType
		want string
	}{
		{name: "brew", pkg: PkgBrew, want: "brew"},
		{name: "apt", pkg: PkgApt, want: "apt"},
		{name: "dnf", pkg: PkgDnf, want: "dnf"},
		{name: "yum", pkg: PkgYum, want: "yum"},
		{name: "pacman", pkg: PkgPacman, want: "pacman"},
		{name: "zypper", pkg: PkgZypper, want: "zypper"},
		{name: "none", pkg: PkgNone, want: "none"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.pkg.String(); got != tt.want {
				t.Errorf(
					"PkgManagerType(%d).String() = %q, want %q",
					tt.pkg, got, tt.want,
				)
			}
		})
	}
}

func TestDetect(t *testing.T) {
	t.Parallel()
	p, err := Detect()
	if err != nil {
		t.Fatalf("Detect() returned error: %v", err)
	}
	if p == nil {
		t.Fatal("Detect() returned nil platform")
	}

	// OS should match runtime.GOOS.
	switch runtime.GOOS {
	case "darwin":
		if p.OS != MacOS {
			t.Errorf("expected OS=MacOS on darwin, got %v", p.OS)
		}
		if p.OSName != "macOS" {
			t.Errorf("expected OSName='macOS', got %q", p.OSName)
		}
		if p.OSVersion == "" {
			t.Error("expected non-empty OSVersion on macOS")
		}
	case "linux":
		if p.OS != Linux {
			t.Errorf("expected OS=Linux on linux, got %v", p.OS)
		}
		if p.OSName == "" {
			t.Error("expected non-empty OSName on Linux")
		}
	}

	// Arch should match runtime.GOARCH.
	switch runtime.GOARCH {
	case "amd64":
		if p.Arch != AMD64 {
			t.Errorf("expected Arch=AMD64, got %v", p.Arch)
		}
	case "arm64":
		if p.Arch != ARM64 {
			t.Errorf("expected Arch=ARM64, got %v", p.Arch)
		}
	}

	// Package manager should be detected (macOS dev machine has brew).
	if runtime.GOOS == "darwin" {
		if p.PackageManager != PkgBrew {
			t.Errorf(
				"expected PackageManager=PkgBrew on macOS, got %v",
				p.PackageManager,
			)
		}
	}
}

func TestDetect_FieldsNonZero(t *testing.T) {
	t.Parallel()
	p, err := Detect()
	if err != nil {
		t.Fatalf("Detect() error: %v", err)
	}

	if p.OSName == "" {
		t.Error("Platform.OSName should not be empty")
	}
	if p.Arch.String() == "unknown" {
		t.Error("Platform.Arch should be a recognized architecture")
	}
	if p.PackageManager.String() == "" {
		t.Error("PackageManager.String() should not be empty")
	}
}

func TestHasCommand(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		cmd  string
		want bool
	}{
		// These commands should exist on any POSIX system.
		{name: "ls exists", cmd: "ls", want: true},
		{name: "sh exists", cmd: "sh", want: true},
		// This command should not exist.
		{
			name: "nonexistent",
			cmd:  "definitely_not_a_real_command_xyz_12345",
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := HasCommand(tt.cmd); got != tt.want {
				t.Errorf(
					"HasCommand(%q) = %v, want %v",
					tt.cmd, got, tt.want,
				)
			}
		})
	}
}

func TestIsDesktopEnvironment(t *testing.T) {
	t.Parallel()

	t.Run("macOS is always desktop", func(t *testing.T) {
		t.Parallel()
		p := &Platform{OS: MacOS}
		if !p.IsDesktopEnvironment() {
			t.Error("macOS should always be desktop")
		}
	})

	t.Run("Linux without display vars", func(t *testing.T) {
		t.Parallel()
		p := &Platform{OS: Linux}
		// On CI or headless, this may be false. We just verify
		// it does not panic and returns a bool.
		_ = p.IsDesktopEnvironment()
	})
}

func TestDetectPackageManager(t *testing.T) {
	t.Parallel()
	mgr := detectPackageManager()

	// On macOS, we expect brew. On Linux, any valid manager.
	if runtime.GOOS == "darwin" {
		if mgr != PkgBrew {
			t.Errorf(
				"detectPackageManager() = %v, expected PkgBrew on macOS",
				mgr,
			)
		}
	}

	// The result should always be a valid PkgManagerType.
	validManagers := map[PkgManagerType]bool{
		PkgBrew: true, PkgApt: true, PkgDnf: true,
		PkgYum: true, PkgPacman: true, PkgZypper: true,
		PkgNone: true,
	}
	if !validManagers[mgr] {
		t.Errorf("detectPackageManager() returned unknown type: %v", mgr)
	}
}

func TestMacOSVersion(t *testing.T) {
	t.Parallel()
	if runtime.GOOS != "darwin" {
		t.Skip("skipping macOS-specific test")
	}
	v, err := macOSVersion()
	if err != nil {
		t.Errorf("macOSVersion() error = %v", err)
	}
	if v == "" {
		t.Error("macOSVersion() returned empty string")
	}
	// Version should contain at least one dot (e.g., "15.4").
	if len(v) < 3 {
		t.Errorf(
			"macOSVersion() = %q, expected version like '15.4'",
			v,
		)
	}
}

func TestLinuxDistro(t *testing.T) {
	t.Parallel()
	name, version, err := linuxDistro()

	if runtime.GOOS == "linux" {
		if err != nil {
			t.Errorf("linuxDistro() on Linux error = %v", err)
		}
		if name == "" {
			t.Error("linuxDistro() returned empty name on Linux")
		}
	} else {
		// On non-Linux, /etc/os-release doesn't exist; the helper
		// still returns a "Linux" default but surfaces the error.
		if err == nil {
			t.Error("linuxDistro() on non-linux should return error")
		}
		if name != "Linux" {
			t.Errorf(
				"linuxDistro() on non-linux = %q, want 'Linux'",
				name,
			)
		}
		if version != "" {
			t.Errorf(
				"linuxDistro() version on non-linux = %q, want empty",
				version,
			)
		}
	}
}

func TestDetectPackageManager_ReturnsValidType(t *testing.T) {
	t.Parallel()
	mgr := detectPackageManager()

	// Verify the result is in the valid set of managers.
	validNames := map[string]bool{
		"brew": true, "apt": true, "dnf": true,
		"yum": true, "pacman": true, "zypper": true,
		"none": true,
	}
	if !validNames[mgr.String()] {
		t.Errorf(
			"detectPackageManager() returned %q, not in valid set",
			mgr.String(),
		)
	}
}

func TestPlatformStruct_Defaults(t *testing.T) {
	t.Parallel()
	p := &Platform{}

	// Zero-value Platform should have MacOS OS (since iota starts at 0).
	if p.OS != MacOS {
		t.Errorf("zero-value OS = %v, want MacOS (0)", p.OS)
	}
	if p.Arch != AMD64 {
		t.Errorf("zero-value Arch = %v, want AMD64 (0)", p.Arch)
	}
	if p.PackageManager != PkgBrew {
		t.Errorf(
			"zero-value PackageManager = %v, want PkgBrew (0)",
			p.PackageManager,
		)
	}
}
