package pkgmgr

import "testing"

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
