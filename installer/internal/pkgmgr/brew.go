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
	for _, generic := range genericNames {
		names := b.MapName(generic)
		if len(names) == 0 {
			continue // skip packages not relevant to brew
		}
		for _, pkg := range names {
			if err := b.runner.Run(ctx, "brew", "install", pkg); err != nil {
				return fmt.Errorf("brew install %s: %w", pkg, err)
			}
		}
	}
	return nil
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
