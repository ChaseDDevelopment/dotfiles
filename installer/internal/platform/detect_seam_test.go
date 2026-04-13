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

func TestDetectPackageManagerWithSeams(t *testing.T) {
	origGOOS := goosFn
	origHasCommand := hasCommandFn
	defer func() {
		goosFn = origGOOS
		hasCommandFn = origHasCommand
	}()

	goosFn = func() string { return "linux" }
	hasCommandFn = func(name string) bool { return name == "brew" || name == "apt-get" }
	if got := detectPackageManager(); got != PkgApt {
		t.Fatalf("detectPackageManager linux = %v, want apt", got)
	}

	goosFn = func() string { return "darwin" }
	hasCommandFn = func(name string) bool { return name == "brew" || name == "apt-get" }
	if got := detectPackageManager(); got != PkgBrew {
		t.Fatalf("detectPackageManager darwin = %v, want brew", got)
	}
}
