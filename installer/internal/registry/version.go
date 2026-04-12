package registry

import (
	"context"
	"os/exec"
	"regexp"
	"strconv"
	"time"
)

// semverRe matches the first semver-like pattern in arbitrary text.
var semverRe = regexp.MustCompile(`(\d+)\.(\d+)(?:\.(\d+))?`)

// parseVersion extracts a [major, minor, patch] triplet from a
// string. Missing patch defaults to 0.
func parseVersion(s string) ([3]int, bool) {
	m := semverRe.FindStringSubmatch(s)
	if m == nil {
		return [3]int{}, false
	}
	major, _ := strconv.Atoi(m[1])
	minor, _ := strconv.Atoi(m[2])
	patch := 0
	if m[3] != "" {
		patch, _ = strconv.Atoi(m[3])
	}
	return [3]int{major, minor, patch}, true
}

// versionAtLeast returns true if have >= want.
func versionAtLeast(have, want [3]int) bool {
	for i := 0; i < 3; i++ {
		if have[i] > want[i] {
			return true
		}
		if have[i] < want[i] {
			return false
		}
	}
	return true
}

// TODO(#29): this helper collapses three distinct outcomes
// (binary missing, --version changed format, probe hang) into a
// single `false`. Distinguishing them requires threading a logger
// through InstallContext/CheckInstalled — safe once TrackedFailures
// lands (#23) so the diagnostic has somewhere to surface.

// getInstalledVersion runs the tool's version command and extracts
// the version triplet. Returns the raw version string and parsed
// triplet. Returns ("", [3]int{}, false) on any failure.
func getInstalledVersion(t *Tool) (string, [3]int, bool) {
	if t.Command == "" || t.MinVersion == "" {
		return "", [3]int{}, false
	}
	path, err := exec.LookPath(t.Command)
	if err != nil {
		return "", [3]int{}, false
	}
	args := t.VersionArgs
	if len(args) == 0 {
		args = []string{"--version"}
	}
	ctx, cancel := context.WithTimeout(
		context.Background(), 2*time.Second,
	)
	defer cancel()
	out, err := exec.CommandContext(ctx, path, args...).
		CombinedOutput()
	if err != nil {
		return "", [3]int{}, false
	}
	ver, ok := parseVersion(string(out))
	if !ok {
		return "", [3]int{}, false
	}
	raw := semverRe.FindString(string(out))
	return raw, ver, true
}

// CheckVersion returns true if the installed version meets the
// MinVersion requirement. Always returns true when MinVersion is
// empty.
func CheckVersion(t *Tool) bool {
	if t.MinVersion == "" {
		return true
	}
	want, ok := parseVersion(t.MinVersion)
	if !ok {
		return true
	}
	_, have, ok := getInstalledVersion(t)
	if !ok {
		return false
	}
	return versionAtLeast(have, want)
}

// InstalledVersion returns the detected version string for a tool,
// or "" if the version cannot be determined.
func InstalledVersion(t *Tool) string {
	raw, _, _ := getInstalledVersion(t)
	return raw
}
