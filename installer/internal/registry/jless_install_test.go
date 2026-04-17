package registry

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chaseddevelopment/dotfiles/installer/internal/executor"
	"github.com/chaseddevelopment/dotfiles/installer/internal/platform"
)

// jlessPkgMgr captures Install calls so the test can assert
// which xcb packages were pulled in before cargo ran.
type jlessPkgMgr struct {
	stubPkgMgr
	installs [][]string
}

func (r *jlessPkgMgr) Install(_ context.Context, pkgs ...string) error {
	cp := make([]string, len(pkgs))
	copy(cp, pkgs)
	r.installs = append(r.installs, cp)
	return nil
}

// TestInstallJlessFromSourceApt asserts the apt path installs the
// libxcb dev headers before invoking cargo. This is the strategy
// dns3 (aarch64 Linux) falls through to when no prebuilt binary
// exists.
func TestInstallJlessFromSourceApt(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	fakebin := filepath.Join(home, "bin")
	if err := os.MkdirAll(fakebin, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fakebin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("JLESS_LOG", filepath.Join(home, "cargo.log"))

	cargoScript := `#!/bin/sh
printf 'cargo %s\n' "$*" >> "$JLESS_LOG"
exit 0
`
	if err := os.WriteFile(
		filepath.Join(fakebin, "cargo"), []byte(cargoScript), 0o755,
	); err != nil {
		t.Fatal(err)
	}

	log, err := executor.NewLogFile(filepath.Join(home, "test.log"))
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()

	mgr := &jlessPkgMgr{stubPkgMgr: stubPkgMgr{name: "apt"}}
	ic := &InstallContext{
		Runner: executor.NewRunner(log, false),
		PkgMgr: mgr,
		Platform: &platform.Platform{
			OS: platform.Linux, Arch: platform.ARM64,
			PackageManager: platform.PkgApt,
		},
	}

	if err := installJlessFromSource(context.Background(), ic); err != nil {
		t.Fatalf("installJlessFromSource: %v", err)
	}

	// Expect exactly one pkgmgr install call with the three apt packages.
	if len(mgr.installs) != 1 {
		t.Fatalf("installs = %d, want 1 (with xcb deps)", len(mgr.installs))
	}
	want := []string{"libxcb1-dev", "libxcb-shape0-dev", "libxcb-xfixes0-dev"}
	got := mgr.installs[0]
	if len(got) != len(want) {
		t.Fatalf("apt packages = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("apt pkg[%d] = %q, want %q", i, got[i], want[i])
		}
	}

	// Cargo log must record the `install jless` call.
	cargoLog, err := os.ReadFile(filepath.Join(home, "cargo.log"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(cargoLog), "install jless") {
		t.Fatalf("cargo log missing `install jless`: %q", cargoLog)
	}
}

// TestNerdFontLinuxInstallsFontconfigOnApt covers the kashyyyk
// "fc-cache: executable file not found in $PATH" failure: minimal
// Ubuntu images ship without fontconfig, so installNerdFontLinux
// now pre-installs it via the pkg manager on apt hosts.
func TestNerdFontLinuxInstallsFontconfigOnApt(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	fakebin := filepath.Join(home, "bin")
	if err := os.MkdirAll(fakebin, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fakebin+string(os.PathListSeparator)+os.Getenv("PATH"))

	// Minimal stubs — download + extract + fc-cache all succeed.
	for _, name := range []string{"curl", "tar", "fc-cache"} {
		if err := os.WriteFile(
			filepath.Join(fakebin, name), []byte("#!/bin/sh\nexit 0\n"), 0o755,
		); err != nil {
			t.Fatal(err)
		}
	}

	log, err := executor.NewLogFile(filepath.Join(home, "test.log"))
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()

	mgr := &jlessPkgMgr{stubPkgMgr: stubPkgMgr{name: "apt"}}
	ic := &InstallContext{
		Runner: executor.NewRunner(log, false),
		PkgMgr: mgr,
		Platform: &platform.Platform{
			OS: platform.Linux, Arch: platform.AMD64,
			PackageManager: platform.PkgApt,
		},
	}
	origLatest := latestVersionFn
	latestVersionFn = func(string, bool) (string, error) { return "3.4.0", nil }
	defer func() { latestVersionFn = origLatest }()

	if err := installNerdFontLinux(context.Background(), ic); err != nil {
		t.Fatalf("installNerdFontLinux: %v", err)
	}
	if len(mgr.installs) == 0 {
		t.Fatal("expected fontconfig pkg install, got none")
	}
	found := false
	for _, call := range mgr.installs {
		for _, pkg := range call {
			if pkg == "fontconfig" {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("fontconfig not requested from pkgmgr; installs=%v", mgr.installs)
	}
}

// TestXcbDepsForPkgMgr covers the distro mapping and the nil-safe
// no-op paths. brew/pacman return nil because jless has a prebuilt
// path there — the caller should skip the pkgmgr step entirely.
func TestXcbDepsForPkgMgr(t *testing.T) {
	cases := []struct {
		name string
		plat *platform.Platform
		want []string
	}{
		{"nil platform", nil, nil},
		{"apt", &platform.Platform{PackageManager: platform.PkgApt},
			[]string{"libxcb1-dev", "libxcb-shape0-dev", "libxcb-xfixes0-dev"}},
		{"dnf", &platform.Platform{PackageManager: platform.PkgDnf},
			[]string{"libxcb-devel", "xcb-util-devel"}},
		{"yum", &platform.Platform{PackageManager: platform.PkgYum},
			[]string{"libxcb-devel", "xcb-util-devel"}},
		{"brew → nil", &platform.Platform{PackageManager: platform.PkgBrew}, nil},
		{"pacman → nil", &platform.Platform{PackageManager: platform.PkgPacman}, nil},
	}
	for _, tc := range cases {
		got := xcbDepsForPkgMgr(tc.plat)
		if len(got) != len(tc.want) {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
			continue
		}
		for i := range tc.want {
			if got[i] != tc.want[i] {
				t.Errorf("%s[%d]: got %q, want %q", tc.name, i, got[i], tc.want[i])
			}
		}
	}
}
