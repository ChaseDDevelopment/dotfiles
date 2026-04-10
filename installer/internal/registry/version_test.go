package registry

import "testing"

func TestParseVersion(t *testing.T) {
	tests := []struct {
		input string
		want  [3]int
		ok    bool
	}{
		{"NVIM v0.12.1", [3]int{0, 12, 1}, true},
		{"go version go1.22.0 darwin/arm64", [3]int{1, 22, 0}, true},
		{"tmux 3.4", [3]int{3, 4, 0}, true},
		{"v20.11.0", [3]int{20, 11, 0}, true},
		{"bat 0.24.0 (b1e116c)", [3]int{0, 24, 0}, true},
		{"starship 1.17.1", [3]int{1, 17, 1}, true},
		{"zoxide 0.9.4", [3]int{0, 9, 4}, true},
		{"1.0.0", [3]int{1, 0, 0}, true},
		{"3.4", [3]int{3, 4, 0}, true},
		{"", [3]int{}, false},
		{"no version here", [3]int{}, false},
	}
	for _, tt := range tests {
		got, ok := parseVersion(tt.input)
		if ok != tt.ok || got != tt.want {
			t.Errorf(
				"parseVersion(%q) = %v, %v; want %v, %v",
				tt.input, got, ok, tt.want, tt.ok,
			)
		}
	}
}

func TestVersionAtLeast(t *testing.T) {
	tests := []struct {
		have, want [3]int
		result     bool
	}{
		{[3]int{0, 12, 1}, [3]int{0, 12, 0}, true},
		{[3]int{0, 12, 0}, [3]int{0, 12, 0}, true},
		{[3]int{0, 10, 0}, [3]int{0, 12, 0}, false},
		{[3]int{1, 0, 0}, [3]int{0, 99, 99}, true},
		{[3]int{0, 12, 0}, [3]int{0, 12, 1}, false},
		{[3]int{2, 0, 0}, [3]int{1, 99, 99}, true},
		{[3]int{0, 0, 1}, [3]int{0, 0, 2}, false},
	}
	for _, tt := range tests {
		got := versionAtLeast(tt.have, tt.want)
		if got != tt.result {
			t.Errorf(
				"versionAtLeast(%v, %v) = %v; want %v",
				tt.have, tt.want, got, tt.result,
			)
		}
	}
}
