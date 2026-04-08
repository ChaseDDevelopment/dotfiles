package pkgmgr

import (
	"context"
	"fmt"

	"github.com/chaseddevelopment/dotfiles/installer/internal/executor"
)

// Dnf implements PackageManager for Fedora/RHEL (dnf).
type Dnf struct {
	runner *executor.Runner
}

func (d *Dnf) Name() string { return "dnf" }

func (d *Dnf) Install(ctx context.Context, genericNames ...string) error {
	for _, generic := range genericNames {
		names := d.MapName(generic)
		for _, pkg := range names {
			if err := d.runner.Run(ctx, "sudo", "dnf", "install", "-y", pkg); err != nil {
				return fmt.Errorf("dnf install %s: %w", pkg, err)
			}
		}
	}
	return nil
}

func (d *Dnf) IsInstalled(genericName string) bool {
	names := d.MapName(genericName)
	for _, pkg := range names {
		if err := d.runner.Run(context.Background(), "dnf", "list", "installed", pkg); err != nil {
			return false
		}
	}
	return len(names) > 0
}

func (d *Dnf) UpdateAll(ctx context.Context) error {
	return d.runner.Run(ctx, "sudo", "dnf", "update", "-y")
}

func (d *Dnf) MapName(generic string) []string {
	m := map[string][]string{
		"nodejs":          {"nodejs", "npm"},
		"build-essential": {"gcc", "gcc-c++", "make"},
		"fd":              {"fd-find"},
		"git-delta":       {"git-delta"},
	}
	if names, ok := m[generic]; ok {
		return names
	}
	return []string{generic}
}

// Yum implements PackageManager for older RHEL/CentOS systems.
type Yum struct {
	runner *executor.Runner
}

func (y *Yum) Name() string { return "yum" }

func (y *Yum) Install(ctx context.Context, genericNames ...string) error {
	for _, generic := range genericNames {
		names := y.MapName(generic)
		for _, pkg := range names {
			if err := y.runner.Run(ctx, "sudo", "yum", "install", "-y", pkg); err != nil {
				return fmt.Errorf("yum install %s: %w", pkg, err)
			}
		}
	}
	return nil
}

func (y *Yum) IsInstalled(genericName string) bool {
	names := y.MapName(genericName)
	for _, pkg := range names {
		if err := y.runner.Run(context.Background(), "yum", "list", "installed", pkg); err != nil {
			return false
		}
	}
	return len(names) > 0
}

func (y *Yum) UpdateAll(ctx context.Context) error {
	return y.runner.Run(ctx, "sudo", "yum", "update", "-y")
}

func (y *Yum) MapName(generic string) []string {
	// Same mappings as DNF.
	m := map[string][]string{
		"nodejs":          {"nodejs", "npm"},
		"build-essential": {"gcc", "gcc-c++", "make"},
		"fd":              {"fd-find"},
		"git-delta":       {"git-delta"},
	}
	if names, ok := m[generic]; ok {
		return names
	}
	return []string{generic}
}
