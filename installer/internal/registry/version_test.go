package registry

import "testing"

func TestParseVersion(t *testing.T) {
	tests := []struct {
		input   string
		want    [3]int
		wantPre bool
		ok      bool
	}{
		{"NVIM v0.12.1", [3]int{0, 12, 1}, false, true},
		{"go version go1.22.0 darwin/arm64", [3]int{1, 22, 0}, false, true},
		{"tmux 3.4", [3]int{3, 4, 0}, false, true},
		{"v20.11.0", [3]int{20, 11, 0}, false, true},
		{"bat 0.24.0 (b1e116c)", [3]int{0, 24, 0}, false, true},
		{"starship 1.17.1", [3]int{1, 17, 1}, false, true},
		{"zoxide 0.9.4", [3]int{0, 9, 4}, false, true},
		{"1.0.0", [3]int{1, 0, 0}, false, true},
		{"3.4", [3]int{3, 4, 0}, false, true},
		{"", [3]int{}, false, false},
		{"no version here", [3]int{}, false, false},
		// Pre-release versions: triple is still extracted, pre flag set.
		{"NVIM v0.12.1-rc1", [3]int{0, 12, 1}, true, true},
		{"1.0.0-alpha.3", [3]int{1, 0, 0}, true, true},
		{"v2.3.4-beta1", [3]int{2, 3, 4}, true, true},
	}
	for _, tt := range tests {
		got, pre, ok := parseVersion(tt.input)
		if ok != tt.ok || got != tt.want || pre != tt.wantPre {
			t.Errorf(
				"parseVersion(%q) = %v pre=%v ok=%v; want %v pre=%v ok=%v",
				tt.input, got, pre, ok, tt.want, tt.wantPre, tt.ok,
			)
		}
	}
}

func TestVersionAtLeast(t *testing.T) {
	tests := []struct {
		have    [3]int
		want    [3]int
		havePre bool
		result  bool
	}{
		{[3]int{0, 12, 1}, [3]int{0, 12, 0}, false, true},
		{[3]int{0, 12, 0}, [3]int{0, 12, 0}, false, true},
		{[3]int{0, 10, 0}, [3]int{0, 12, 0}, false, false},
		{[3]int{1, 0, 0}, [3]int{0, 99, 99}, false, true},
		{[3]int{0, 12, 0}, [3]int{0, 12, 1}, false, false},
		{[3]int{2, 0, 0}, [3]int{1, 99, 99}, false, true},
		{[3]int{0, 0, 1}, [3]int{0, 0, 2}, false, false},
		// Pre-release demotion: 0.12.1-rc1 fails MinVersion 0.12.1
		// even though the numeric triples are equal.
		{[3]int{0, 12, 1}, [3]int{0, 12, 1}, true, false},
		{[3]int{0, 12, 2}, [3]int{0, 12, 1}, true, true},
	}
	for _, tt := range tests {
		got := versionAtLeast(tt.have, tt.want, tt.havePre)
		if got != tt.result {
			t.Errorf(
				"versionAtLeast(%v, %v, pre=%v) = %v; want %v",
				tt.have, tt.want, tt.havePre, got, tt.result,
			)
		}
	}
}
