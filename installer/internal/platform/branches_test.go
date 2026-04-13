package platform

import (
	"fmt"
	"testing"
)

// TestDetectMacOSVersionWarnsNonFatal covers the darwin branch where
// macOSVersion returns an error — the Detect call should still succeed
// and append a warning.
func TestDetectMacOSVersionWarnsNonFatal(t *testing.T) {
	origGOOS := goosFn
	origGOARCH := goarchFn
	origMac := macOSVersionFn
	origDetectPkg := detectPkgMgrFn
	origHasCommand := hasCommandFn
	defer func() {
		goosFn = origGOOS
		goarchFn = origGOARCH
		macOSVersionFn = origMac
		detectPkgMgrFn = origDetectPkg
		hasCommandFn = origHasCommand
	}()

	goosFn = func() string { return "darwin" }
	goarchFn = func() string { return "arm64" }
	macOSVersionFn = func() (string, error) {
		return "", fmt.Errorf("sw_vers missing")
	}
	detectPkgMgrFn = func() PkgManagerType { return PkgBrew }
	hasCommandFn = func(string) bool { return false }

	p, err := Detect()
	if err != nil {
		t.Fatalf("Detect should not fail on macOS version warning: %v", err)
	}
	if len(p.Warnings) == 0 {
		t.Fatal("expected macOS version warning to be appended")
	}
}

// TestDetectLinuxDistroWarnsNonFatal covers the linux branch where
// linuxDistroFn returns an error but Detect still succeeds with a
// warning populated.
func TestDetectLinuxDistroWarnsNonFatal(t *testing.T) {
	origGOOS := goosFn
	origGOARCH := goarchFn
	origLinux := linuxDistroFn
	origDetectPkg := detectPkgMgrFn
	origHasCommand := hasCommandFn
	defer func() {
		goosFn = origGOOS
		goarchFn = origGOARCH
		linuxDistroFn = origLinux
		detectPkgMgrFn = origDetectPkg
		hasCommandFn = origHasCommand
	}()

	goosFn = func() string { return "linux" }
	goarchFn = func() string { return "amd64" }
	linuxDistroFn = func() (string, string, error) {
		return "Linux", "", fmt.Errorf("no os-release")
	}
	detectPkgMgrFn = func() PkgManagerType { return PkgNone }
	hasCommandFn = func(string) bool { return false }

	p, err := Detect()
	if err != nil {
		t.Fatalf("Detect should not fail on linuxDistro warning: %v", err)
	}
	if len(p.Warnings) == 0 {
		t.Fatal("expected linux distro warning to be appended")
	}
}

// TestDetectPackageManagerEachManager iterates every branch of the
// detectPackageManager switch.
func TestDetectPackageManagerEachManager(t *testing.T) {
	origGOOS := goosFn
	origHasCommand := hasCommandFn
	defer func() {
		goosFn = origGOOS
		hasCommandFn = origHasCommand
	}()

	// Linux: each manager name in order of precedence.
	goosFn = func() string { return "linux" }
	cases := []struct {
		present []string
		want    PkgManagerType
	}{
		{[]string{"dnf"}, PkgDnf},
		{[]string{"yum"}, PkgYum},
		{[]string{"pacman"}, PkgPacman},
		{[]string{"zypper"}, PkgZypper},
		{[]string{"brew"}, PkgBrew}, // linuxbrew fallback
		{nil, PkgNone},
	}
	for _, tc := range cases {
		set := map[string]bool{}
		for _, n := range tc.present {
			set[n] = true
		}
		hasCommandFn = func(name string) bool { return set[name] }
		if got := detectPackageManager(); got != tc.want {
			t.Fatalf("detectPackageManager(%v) = %v, want %v", tc.present, got, tc.want)
		}
	}

	// Darwin: brew is only picked when goosFn=="darwin".
	goosFn = func() string { return "darwin" }
	hasCommandFn = func(name string) bool { return name == "brew" }
	if got := detectPackageManager(); got != PkgBrew {
		t.Fatalf("darwin+brew = %v, want brew", got)
	}
}

// TestMacOSVersionRealFails when sw_vers isn't on PATH — covers the
// error path of the default macOSVersion function.
func TestMacOSVersionRealFails(t *testing.T) {
	t.Setenv("PATH", "")
	if _, err := macOSVersion(); err == nil {
		t.Fatal("expected sw_vers-missing error")
	}
}
