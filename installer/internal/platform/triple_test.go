package platform

import "testing"

func TestTargetTriple(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		plat Platform
		libc string
		want string
	}{
		{
			name: "macOS ARM64",
			plat: Platform{OS: MacOS, Arch: ARM64},
			libc: "gnu",
			want: "aarch64-apple-darwin",
		},
		{
			name: "macOS AMD64",
			plat: Platform{OS: MacOS, Arch: AMD64},
			libc: "gnu",
			want: "x86_64-apple-darwin",
		},
		{
			name: "Linux ARM64 musl",
			plat: Platform{OS: Linux, Arch: ARM64},
			libc: "musl",
			want: "aarch64-unknown-linux-musl",
		},
		{
			name: "Linux AMD64 gnu",
			plat: Platform{OS: Linux, Arch: AMD64},
			libc: "gnu",
			want: "x86_64-unknown-linux-gnu",
		},
		{
			name: "Linux ARM64 gnu",
			plat: Platform{OS: Linux, Arch: ARM64},
			libc: "gnu",
			want: "aarch64-unknown-linux-gnu",
		},
		{
			name: "Linux AMD64 musl",
			plat: Platform{OS: Linux, Arch: AMD64},
			libc: "musl",
			want: "x86_64-unknown-linux-musl",
		},
		{
			name: "unknown OS",
			plat: Platform{OS: OS(99), Arch: AMD64},
			libc: "gnu",
			want: "unknown",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tt.plat.TargetTriple(tt.libc)
			if got != tt.want {
				t.Errorf(
					"TargetTriple(%q) = %q, want %q",
					tt.libc, got, tt.want,
				)
			}
		})
	}
}

func TestGoStyle(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		plat     Platform
		wantOS   string
		wantArch string
	}{
		{
			name:     "macOS ARM64",
			plat:     Platform{OS: MacOS, Arch: ARM64},
			wantOS:   "darwin",
			wantArch: "arm64",
		},
		{
			name:     "macOS AMD64",
			plat:     Platform{OS: MacOS, Arch: AMD64},
			wantOS:   "darwin",
			wantArch: "amd64",
		},
		{
			name:     "Linux ARM64",
			plat:     Platform{OS: Linux, Arch: ARM64},
			wantOS:   "linux",
			wantArch: "arm64",
		},
		{
			name:     "Linux AMD64",
			plat:     Platform{OS: Linux, Arch: AMD64},
			wantOS:   "linux",
			wantArch: "amd64",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotOS, gotArch := tt.plat.GoStyle()
			if gotOS != tt.wantOS {
				t.Errorf("GoStyle() OS = %q, want %q", gotOS, tt.wantOS)
			}
			if gotArch != tt.wantArch {
				t.Errorf(
					"GoStyle() Arch = %q, want %q",
					gotArch, tt.wantArch,
				)
			}
		})
	}
}

func TestTitleStyle(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		plat     Platform
		wantOS   string
		wantArch string
	}{
		{
			name:     "macOS ARM64",
			plat:     Platform{OS: MacOS, Arch: ARM64},
			wantOS:   "darwin",
			wantArch: "arm64",
		},
		{
			name:     "macOS AMD64",
			plat:     Platform{OS: MacOS, Arch: AMD64},
			wantOS:   "darwin",
			wantArch: "x86_64",
		},
		{
			name:     "Linux ARM64",
			plat:     Platform{OS: Linux, Arch: ARM64},
			wantOS:   "linux",
			wantArch: "arm64",
		},
		{
			name:     "Linux AMD64",
			plat:     Platform{OS: Linux, Arch: AMD64},
			wantOS:   "linux",
			wantArch: "x86_64",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotOS, gotArch := tt.plat.TitleStyle()
			if gotOS != tt.wantOS {
				t.Errorf(
					"TitleStyle() OS = %q, want %q",
					gotOS, tt.wantOS,
				)
			}
			if gotArch != tt.wantArch {
				t.Errorf(
					"TitleStyle() Arch = %q, want %q",
					gotArch, tt.wantArch,
				)
			}
		})
	}
}
