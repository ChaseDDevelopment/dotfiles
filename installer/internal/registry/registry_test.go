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

// TestActiveStrategy verifies that ActiveStrategy returns the first
// strategy applicable under a given manager — the install method that
// "owns" the tool there. This is what the update pass uses to decide
// whether a tool is package-manager-owned (covered by the system
// upgrade) or needs its own update path (cargo/github/script).
func TestActiveStrategy(t *testing.T) {
	byName := func(name string) *Tool {
		for _, tool := range AllTools() {
			if tool.Name == name {
				tt := tool
				return &tt
			}
		}
		t.Fatalf("tool %q not found in registry", name)
		return nil
	}

	eza := byName("eza")
	dust := byName("dust")

	tests := []struct {
		name    string
		tool    *Tool
		mgrName string
		want    InstallMethod
		wantNil bool
	}{
		// eza is pacman-package-owned on pacman.
		{"eza on pacman → pkgmgr", eza, "pacman", MethodPackageManager, false},
		// eza falls to the github-release strategy on apt (cargo only
		// applies to apt/dnf/yum *after* the any-manager github-release
		// strategy, so github-release wins).
		{"eza on apt → github-release", eza, "apt", MethodGitHubRelease, false},
		// dust is cargo-owned on apt (its only apt-applicable strategy
		// is the unrestricted MethodCargo entry).
		{"dust on apt → cargo", dust, "apt", MethodCargo, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ActiveStrategy(tt.tool, tt.mgrName)
			if tt.wantNil {
				if got != nil {
					t.Fatalf("expected nil strategy, got %+v", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("expected a strategy, got nil")
			}
			if got.Method != tt.want {
				t.Fatalf("method mismatch: got %d, want %d", got.Method, tt.want)
			}
		})
	}

	// A tool whose only strategy is restricted to a different manager
	// has no applicable strategy → nil.
	aptOnly := &Tool{
		Name:    "apt-only",
		Command: "apt-only",
		Strategies: []InstallStrategy{
			{Managers: []string{"apt"}, Method: MethodPackageManager, Package: "apt-only"},
		},
	}
	if got := ActiveStrategy(aptOnly, "pacman"); got != nil {
		t.Fatalf("expected nil for manager-restricted tool, got %+v", got)
	}
	// A tool with no strategies at all → nil.
	if got := ActiveStrategy(&Tool{Name: "bare"}, "apt"); got != nil {
		t.Fatalf("expected nil for tool with no strategies, got %+v", got)
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
				Binary: "eza", StripVPrefix:true, LibC: "gnu",
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
				Binary: "lazygit", StripVPrefix:true,
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
