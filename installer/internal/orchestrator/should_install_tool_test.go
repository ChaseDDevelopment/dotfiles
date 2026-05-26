package orchestrator

import (
	"testing"

	"github.com/chaseddevelopment/dotfiles/installer/internal/platform"
	"github.com/chaseddevelopment/dotfiles/installer/internal/registry"
)

func TestShouldInstallToolFiltersByManagerApplicability(t *testing.T) {
	bc := &BuildConfig{}
	pacman := &platform.Platform{
		OS: platform.Linux, Arch: platform.AMD64,
		PackageManager: platform.PkgPacman,
	}

	// apt-only tool (OSFilter passes on Linux) must NOT be considered on
	// pacman — otherwise it fails at execution with "no applicable
	// install strategies" and degrades the whole run.
	aptOnly := registry.Tool{
		Name: "nala", Command: "nala",
		OSFilter: []string{"linux"},
		Strategies: []registry.InstallStrategy{
			{
				Managers: []string{"apt"},
				Method:   registry.MethodPackageManager, Package: "nala",
			},
		},
	}
	if bc.shouldInstallTool(&aptOnly, pacman) {
		t.Error("apt-only tool should not be considered on pacman")
	}

	// A tool with a generic (manager-agnostic) strategy still applies.
	crossPlatform := registry.Tool{
		Name: "tmux", Command: "tmux",
		Strategies: []registry.InstallStrategy{
			{Method: registry.MethodPackageManager, Package: "tmux"},
		},
	}
	if !bc.shouldInstallTool(&crossPlatform, pacman) {
		t.Error("cross-platform tool should be considered on pacman")
	}
}
