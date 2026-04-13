package pkgmgr

import (
	"errors"
	"fmt"
	"testing"
)

// TestContainsInstalled covers the dpkg-query Status parse. The
// precision matters because packages in the "rc" (removed, config
// remaining) state would previously be reported as installed under
// `dpkg -l` glob-matching.
func TestContainsInstalled(t *testing.T) {
	cases := []struct {
		name   string
		status string
		want   bool
	}{
		{"installed", "install ok installed", true},
		{"installed with trailing", "install ok installed \n", true},
		{"removed config remaining", "deinstall ok config-files", false},
		{"purge pending", "purge ok not-installed", false},
		{"empty", "", false},
		{"half-installed", "install ok half-installed", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := containsInstalled(c.status)
			if got != c.want {
				t.Errorf(
					"containsInstalled(%q) = %v, want %v",
					c.status, got, c.want,
				)
			}
		})
	}
}

// TestClassifyAptErr exercises the narrow dpkg/apt error classifier
// that Phase A of the robustness plan relies on. The retry logic in
// registry.ExecuteInstall uses errors.Is on these sentinels to
// decide whether to heal + retry (ErrDpkgInterrupted / ErrDpkgLocked)
// or fail the tool outright (ErrAptFatal). Misclassification is the
// difference between a transient blip and a silent source build.
func TestClassifyAptErr(t *testing.T) {
	shellErr := fmt.Errorf("sudo nala install -y ffmpeg: exit status 100")

	cases := []struct {
		name   string
		output string
		want   error
	}{
		{
			name:   "dpkg interrupted",
			output: "E: dpkg was interrupted, you must manually run 'sudo dpkg --configure -a' to correct the problem.",
			want:   ErrDpkgInterrupted,
		},
		{
			name:   "could not get lock",
			output: "E: Could not get lock /var/lib/dpkg/lock-frontend. It is held by process 12345.",
			want:   ErrDpkgLocked,
		},
		{
			name:   "lock frontend path without could-not-get prefix",
			output: "Waiting for cache lock: Could not get lock /var/lib/dpkg/lock-frontend.",
			want:   ErrDpkgLocked,
		},
		{
			name:   "unmet dependencies",
			output: "E: Unable to correct problems, you have held broken packages.\nThe following packages have unmet dependencies:",
			want:   ErrAptFatal,
		},
		{
			name:   "hash sum mismatch",
			output: "E: Failed to fetch http://example.com/foo.deb Hash Sum mismatch",
			want:   ErrAptFatal,
		},
		{
			name:   "release file expired",
			output: "Release file for http://example.com/foo is not valid yet (invalid for another 7h 12min 33s).",
			want:   ErrAptFatal,
		},
		{
			name:   "generic package-not-found stays unclassified",
			output: "E: Unable to locate package nonexistent-package",
			want:   nil,
		},
		{
			name:   "dpkg returned an error code (umbrella) stays unclassified",
			output: "E: Sub-process /usr/bin/dpkg returned an error code (1)",
			want:   nil,
		},
		{
			name:   "empty output stays unclassified",
			output: "",
			want:   nil,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := classifyAptErr(shellErr, c.output)
			if c.want == nil {
				if !errors.Is(got, shellErr) {
					t.Fatalf("expected raw error preserved, got %v", got)
				}
				for _, sentinel := range []error{ErrDpkgInterrupted, ErrDpkgLocked, ErrAptFatal} {
					if errors.Is(got, sentinel) {
						t.Errorf("unexpected classification as %v: %v", sentinel, got)
					}
				}
				return
			}
			if !errors.Is(got, c.want) {
				t.Errorf("classifyAptErr(…, %q) = %v, want errors.Is(%v)", c.output, got, c.want)
			}
			if !errors.Is(got, shellErr) {
				t.Errorf("original shell error was not preserved in wrap chain: %v", got)
			}
		})
	}
}

// TestClassifyAptErrNilPassthrough: a nil error in must be a nil
// error out — callers rely on this to use classifyAptErr
// unconditionally without nil checks.
func TestClassifyAptErrNilPassthrough(t *testing.T) {
	if got := classifyAptErr(nil, "dpkg was interrupted"); got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}
