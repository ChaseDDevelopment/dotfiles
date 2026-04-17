package registry

import (
	"context"
	"os/exec"
	"regexp"
	"strconv"
	"time"
)

// semverRe matches the first semver-like pattern in arbitrary text,
// capturing optional pre-release metadata after the patch digit.
// Pre-release text is used only to demote equal numeric triples:
// 1.2.3-rc1 < 1.2.3, per semver ordering rules. Full precedence
// across multiple pre-release tags ("rc1" vs "rc2") isn't modelled
// — MinVersion checks in this codebase are always against release
// numbers, so any pre-release fails the check at equal numerics.
var semverRe = regexp.MustCompile(
	`(\d+)\.(\d+)(?:\.(\d+))?(-[0-9A-Za-z.-]+)?`,
)

// parseVersion extracts a [major, minor, patch] triplet plus a
// pre-release flag from a version string.
func parseVersion(s string) (triplet [3]int, pre bool, ok bool) {
	m := semverRe.FindStringSubmatch(s)
	if m == nil {
		return [3]int{}, false, false
	}
	major, _ := strconv.Atoi(m[1])
	minor, _ := strconv.Atoi(m[2])
	patch := 0
	if m[3] != "" {
		patch, _ = strconv.Atoi(m[3])
	}
	pre = m[4] != ""
	return [3]int{major, minor, patch}, pre, true
}

// versionAtLeast returns true if have >= want. A pre-release
// version with equal numeric triple counts as strictly less than
// the release (per semver: 1.2.3-rc1 < 1.2.3).
func versionAtLeast(have, want [3]int, havePre bool) bool {
	for i := 0; i < 3; i++ {
		if have[i] > want[i] {
			return true
		}
		if have[i] < want[i] {
			return false
		}
	}
	// Numeric triple equal. A pre-release suffix on `have` means
	// it hasn't reached the requested version.
	return !havePre
}

// TODO(#29): this helper collapses three distinct outcomes
// (binary missing, --version changed format, probe hang) into a
// single `false`. Distinguishing them requires threading a logger
// through InstallContext/CheckInstalled — safe once TrackedFailures
// lands (#23) so the diagnostic has somewhere to surface.

// getInstalledVersion runs the tool's version command and extracts
// the version triplet plus pre-release flag. Returns the raw
// matched version string alongside for display.
func getInstalledVersion(t *Tool) (raw string, triplet [3]int, pre, ok bool) {
	if t.Command == "" || t.MinVersion == "" {
		return "", [3]int{}, false, false
	}
	path, err := exec.LookPath(t.Command)
	if err != nil {
		return "", [3]int{}, false, false
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
		return "", [3]int{}, false, false
	}
	ver, pre, ok := parseVersion(string(out))
	if !ok {
		return "", [3]int{}, false, false
	}
	raw = semverRe.FindString(string(out))
	return raw, ver, pre, true
}

// CheckVersion returns true if the installed version meets the
// MinVersion requirement. Always returns true when MinVersion is
// empty.
func CheckVersion(t *Tool) bool {
	if t.MinVersion == "" {
		return true
	}
	want, _, ok := parseVersion(t.MinVersion)
	if !ok {
		return true
	}
	_, have, pre, ok := getInstalledVersion(t)
	if !ok {
		return false
	}
	return versionAtLeast(have, want, pre)
}

// InstalledVersion returns the detected version string for a tool,
// or "" if the version cannot be determined.
func InstalledVersion(t *Tool) string {
	raw, _, _, _ := getInstalledVersion(t)
	return raw
}
