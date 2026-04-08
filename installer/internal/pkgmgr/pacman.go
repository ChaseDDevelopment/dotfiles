package pkgmgr

import (
	"context"
	"fmt"

	"github.com/chaseddevelopment/dotfiles/installer/internal/executor"
)

// Pacman implements PackageManager for Arch Linux.
type Pacman struct {
	runner *executor.Runner
}

func (p *Pacman) Name() string { return "pacman" }

func (p *Pacman) Install(ctx context.Context, genericNames ...string) error {
	for _, generic := range genericNames {
		names := p.MapName(generic)
		for _, pkg := range names {
			if err := p.runner.Run(ctx, "sudo", "pacman", "-S", "--noconfirm", pkg); err != nil {
				return fmt.Errorf("pacman install %s: %w", pkg, err)
			}
		}
	}
	return nil
}

func (p *Pacman) IsInstalled(genericName string) bool {
	names := p.MapName(genericName)
	for _, pkg := range names {
		if err := p.runner.Run(context.Background(), "pacman", "-Q", pkg); err != nil {
			return false
		}
	}
	return len(names) > 0
}

func (p *Pacman) UpdateAll(ctx context.Context) error {
	return p.runner.Run(ctx, "sudo", "pacman", "-Syu", "--noconfirm")
}

func (p *Pacman) MapName(generic string) []string {
	m := map[string][]string{
		"nodejs":          {"nodejs", "npm"},
		"build-essential": {"base-devel"},
		"git-delta":       {"git-delta"},
		"fd":              {"fd"},
	}
	if names, ok := m[generic]; ok {
		return names
	}
	return []string{generic}
}
