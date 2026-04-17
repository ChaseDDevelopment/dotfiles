package platform

import "fmt"

// TargetTriple returns a Rust-style target triple for the current platform.
// The libc parameter controls the Linux libc variant ("musl" or "gnu").
func (p *Platform) TargetTriple(libc string) string {
	switch p.OS {
	case Linux:
		arch := "x86_64"
		if p.Arch == ARM64 {
			arch = "aarch64"
		}
		return fmt.Sprintf("%s-unknown-linux-%s", arch, libc)
	case MacOS:
		arch := "x86_64"
		if p.Arch == ARM64 {
			arch = "aarch64"
		}
		return fmt.Sprintf("%s-apple-darwin", arch)
	default:
		return "unknown"
	}
}

// GoStyle returns the GOOS/GOARCH-style identifiers: ("linux"/"darwin", "amd64"/"arm64").
func (p *Platform) GoStyle() (string, string) {
	os := "linux"
	if p.OS == MacOS {
		os = "darwin"
	}
	arch := "amd64"
	if p.Arch == ARM64 {
		arch = "arm64"
	}
	return os, arch
}

// TitleStyle returns lowercase OS and raw arch: ("linux"/"darwin", "x86_64"/"arm64").
// Used by lazygit GitHub release URLs.
func (p *Platform) TitleStyle() (string, string) {
	os := "linux"
	if p.OS == MacOS {
		os = "darwin"
	}
	arch := "x86_64"
	if p.Arch == ARM64 {
		arch = "arm64"
	}
	return os, arch
}
