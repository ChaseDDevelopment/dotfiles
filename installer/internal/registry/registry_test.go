package registry

import (
	"fmt"
	"testing"

	"github.com/chaseddevelopment/dotfiles/installer/internal/github"
	"github.com/chaseddevelopment/dotfiles/installer/internal/platform"
)

func TestAllToolsCount(t *testing.T) {
	tools := AllTools()
	if len(tools) < 25 {
		t.Errorf("expected at least 25 tools, got %d", len(tools))
	}
	fmt.Printf("Total tools registered: %d\n", len(tools))
	for _, tool := range tools {
		fmt.Printf("  %-20s cmd=%-15s strategies=%d\n",
			tool.Name, tool.Command, len(tool.Strategies))
	}
}

func TestBuildURL(t *testing.T) {
	p := &platform.Platform{OS: platform.MacOS, Arch: platform.ARM64}

	tests := []struct {
		name     string
		cfg      *github.Config
		version  string
		wantURL  string
		wantTar  bool
	}{
		{
			name: "eza target triple",
			cfg: &github.Config{
				Repo: "eza-community/eza", Pattern: github.PatternTargetTriple,
				Binary: "eza", StripV: true, LibC: "gnu",
			},
			version: "0.20.0",
			wantURL: "https://github.com/eza-community/eza/releases/download/v0.20.0/eza_aarch64-apple-darwin.tar.gz",
			wantTar: true,
		},
		{
			name: "yq raw binary",
			cfg: &github.Config{
				Repo: "mikefarah/yq", Pattern: github.PatternRawBinary,
				Binary: "yq",
			},
			version: "",
			wantURL: "https://github.com/mikefarah/yq/releases/latest/download/yq_darwin_arm64",
			wantTar: false,
		},
		{
			name: "lazygit custom OS/arch",
			cfg: &github.Config{
				Repo: "jesseduffield/lazygit", Pattern: github.PatternCustomOSArch,
				Binary: "lazygit", StripV: true,
			},
			version: "0.40.0",
			wantURL: "https://github.com/jesseduffield/lazygit/releases/download/v0.40.0/lazygit_0.40.0_darwin_arm64.tar.gz",
			wantTar: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url, isTar := github.BuildURL(tt.cfg, p, tt.version)
			if url != tt.wantURL {
				t.Errorf("URL mismatch:\n  got:  %s\n  want: %s", url, tt.wantURL)
			}
			if isTar != tt.wantTar {
				t.Errorf("tarball mismatch: got %v, want %v", isTar, tt.wantTar)
			}
		})
	}
}
