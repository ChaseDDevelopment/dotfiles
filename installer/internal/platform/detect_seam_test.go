package platform

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLinuxDistroAndDetectWithSeams(t *testing.T) {
	origOSRelease := osReleasePath
	origGOOS := goosFn
	origGOARCH := goarchFn
	origHasCommand := hasCommandFn
	origMacVersion := macOSVersionFn
	origLinuxDistro := linuxDistroFn
	origDetectPkg := detectPkgMgrFn
	defer func() {
		osReleasePath = origOSRelease
		goosFn = origGOOS
		goarchFn = origGOARCH
		hasCommandFn = origHasCommand
		macOSVersionFn = origMacVersion
		linuxDistroFn = origLinuxDistro
		detectPkgMgrFn = origDetectPkg
	}()

	path := filepath.Join(t.TempDir(), "os-release")
	if err := os.WriteFile(path, []byte("NAME=\"Test Linux\"\nVERSION_ID=\"9\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	osReleasePath = path
	name, version, err := linuxDistro()
	if err != nil || name != "Test Linux" || version != "9" {
		t.Fatalf("linuxDistro = %q %q err=%v", name, version, err)
	}

	if err := os.WriteFile(path, []byte("VERSION_ID=\"9\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	name, version, err = linuxDistro()
	if err == nil || name != "Linux" {
		t.Fatalf("expected missing NAME fallback, got %q %q err=%v", name, version, err)
	}

	goosFn = func() string { return "linux" }
	goarchFn = func() string { return "amd64" }
	linuxDistroFn = func() (string, string, error) { return "Ubuntu", "24.04", nil }
	detectPkgMgrFn = func() PkgManagerType { return PkgApt }
	hasCommandFn = func(name string) bool { return name == "nala" || name == "paru" }
	p, err := Detect()
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if p.OS != Linux || p.Arch != AMD64 || p.PackageManager != PkgApt || !p.HasNala || p.HasYay || !p.HasParu {
		t.Fatalf("unexpected platform detect result: %#v", p)
	}

	goosFn = func() string { return "weirdos" }
	if _, err := Detect(); err == nil {
		t.Fatal("expected unsupported OS error")
	}
	goosFn = func() string { return "linux" }
	goarchFn = func() string { return "mips" }
	if _, err := Detect(); err == nil {
		t.Fatal("expected unsupported arch error")
	}
}

// TestDetectPackageManagerWithSeams is the sole branch-coverage test
// for detectPackageManager. It exists because the function is a
// static if-ladder keyed on exact binary-name string literals
// ("apt-get", "dnf", "pacman", ...). A typo in any of those literals
// would silently regress package-manager detection, so driving every
// branch via the hasCommandFn seam catches that class of bug. It
// does NOT attempt to assert anything beyond the mapping itself —
// the if-ladder is the specification.
func TestDetectPackageManagerWithSeams(t *testing.T) {
	origGOOS := goosFn
	origHasCommand := hasCommandFn
	defer func() {
		goosFn = origGOOS
		hasCommandFn = origHasCommand
	}()

	cases := []struct {
		name    string
		goos    string
		present map[string]bool
		want    PkgManagerType
	}{
		{
			name:    "linux apt-get wins over brew",
			goos:    "linux",
			present: map[string]bool{"brew": true, "apt-get": true},
			want:    PkgApt,
		},
		{
			name:    "darwin prefers brew",
			goos:    "darwin",
			present: map[string]bool{"brew": true, "apt-get": true},
			want:    PkgBrew,
		},
		{
			name:    "linux dnf",
			goos:    "linux",
			present: map[string]bool{"dnf": true},
			want:    PkgDnf,
		},
		{
			name:    "linux yum",
			goos:    "linux",
			present: map[string]bool{"yum": true},
			want:    PkgYum,
		},
		{
			name:    "linux pacman",
			goos:    "linux",
			present: map[string]bool{"pacman": true},
			want:    PkgPacman,
		},
		{
			name:    "linux zypper",
			goos:    "linux",
			present: map[string]bool{"zypper": true},
			want:    PkgZypper,
		},
		{
			name:    "linux brew fallback when no native mgr",
			goos:    "linux",
			present: map[string]bool{"brew": true},
			want:    PkgBrew,
		},
		{
			name:    "linux no supported manager",
			goos:    "linux",
			present: map[string]bool{},
			want:    PkgNone,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			goosFn = func() string { return tc.goos }
			present := tc.present
			hasCommandFn = func(name string) bool { return present[name] }
			if got := detectPackageManager(); got != tc.want {
				t.Fatalf(
					"detectPackageManager(%s, %v) = %v, want %v",
					tc.goos, present, got, tc.want,
				)
			}
		})
	}
}
