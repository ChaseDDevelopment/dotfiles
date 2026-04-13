package pkgmgr

import (
	"context"
	"fmt"

	"github.com/chaseddevelopment/dotfiles/installer/internal/executor"
)

// Brew implements PackageManager for Homebrew on macOS.
type Brew struct {
	runner *executor.Runner
}

func (b *Brew) Name() string { return "brew" }

func (b *Brew) Install(ctx context.Context, genericNames ...string) error {
	if len(genericNames) == 0 {
		return nil
	}
	pkgs := dedupeNames(b.MapName, genericNames)
	if len(pkgs) == 0 {
		// Every caller-supplied name MapName'd to an empty slice
		// (e.g. "build-essential" on macOS). Nothing to install.
		return nil
	}
	args := append([]string{"brew", "install"}, pkgs...)
	out, err := b.runner.RunWithOutput(ctx, args[0], args[1:]...)
	if err == nil {
		return nil
	}
	return attribute(
		fmt.Errorf("brew install: %w (output: %s)", err, out),
		genericNames,
		b.IsInstalled,
	)
}

// IsInstalled reports whether every mapped package exists in the
// brew prefix. A generic name that MapName deliberately resolves
// to an empty slice (e.g. "build-essential" on macOS) is treated
// as satisfied rather than not-installed — the tool is
// "not applicable" on this platform, which is the pre-install
// invariant the caller actually wants.
func (b *Brew) IsInstalled(genericName string) bool {
	names := b.MapName(genericName)
	if len(names) == 0 {
		return true
	}
	for _, pkg := range names {
		if _, err := b.runner.RunProbe(context.Background(), "brew", "list", pkg); err != nil {
			return false
		}
	}
	return true
}

func (b *Brew) UpdateAll(ctx context.Context) error {
	return b.runner.RunShell(ctx, "brew update && brew upgrade")
}

func (b *Brew) MapName(generic string) []string {
	m := map[string][]string{
		"nodejs":          {"node"},
		"build-essential": {}, // not needed on macOS
		"neovim":          {"neovim"},
		"git-delta":       {"git-delta"},
	}
	if names, ok := m[generic]; ok {
		return names
	}
	return []string{generic}
}
