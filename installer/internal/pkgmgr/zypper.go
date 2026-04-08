package pkgmgr

import (
	"context"
	"fmt"

	"github.com/chaseddevelopment/dotfiles/installer/internal/executor"
)

// Zypper implements PackageManager for openSUSE.
type Zypper struct {
	runner *executor.Runner
}

func (z *Zypper) Name() string { return "zypper" }

func (z *Zypper) Install(ctx context.Context, genericNames ...string) error {
	for _, generic := range genericNames {
		names := z.MapName(generic)
		for _, pkg := range names {
			if err := z.runner.Run(ctx, "sudo", "zypper", "install", "-y", pkg); err != nil {
				return fmt.Errorf("zypper install %s: %w", pkg, err)
			}
		}
	}
	return nil
}

func (z *Zypper) IsInstalled(genericName string) bool {
	names := z.MapName(genericName)
	for _, pkg := range names {
		if err := z.runner.Run(context.Background(), "zypper", "se", "--installed-only", pkg); err != nil {
			return false
		}
	}
	return len(names) > 0
}

func (z *Zypper) UpdateAll(ctx context.Context) error {
	return z.runner.Run(ctx, "sudo", "zypper", "update", "-y")
}

func (z *Zypper) MapName(generic string) []string {
	m := map[string][]string{
		"nodejs":          {"nodejs", "npm"},
		"build-essential": {"gcc", "gcc-c++", "make"},
	}
	if names, ok := m[generic]; ok {
		return names
	}
	return []string{generic}
}
