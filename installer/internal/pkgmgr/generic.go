package pkgmgr

import (
	"context"
	"fmt"

	"github.com/chaseddevelopment/dotfiles/installer/internal/executor"
)

// genericMgr is a data-driven PackageManager for simple
// distributions that follow the same install/check/update pattern.
type genericMgr struct {
	runner    *executor.Runner
	name      string
	installFn func(ctx context.Context, r *executor.Runner, pkg string) error
	checkFn   func(r *executor.Runner, pkg string) bool
	updateFn  func(ctx context.Context, r *executor.Runner) error
	nameMap   map[string][]string
}

func (g *genericMgr) Name() string { return g.name }

func (g *genericMgr) Install(
	ctx context.Context,
	genericNames ...string,
) error {
	for _, generic := range genericNames {
		for _, pkg := range g.MapName(generic) {
			if err := g.installFn(ctx, g.runner, pkg); err != nil {
				return fmt.Errorf(
					"%s install %s: %w", g.name, pkg, err,
				)
			}
		}
	}
	return nil
}

func (g *genericMgr) IsInstalled(genericName string) bool {
	names := g.MapName(genericName)
	for _, pkg := range names {
		if !g.checkFn(g.runner, pkg) {
			return false
		}
	}
	return len(names) > 0
}

func (g *genericMgr) UpdateAll(ctx context.Context) error {
	return g.updateFn(ctx, g.runner)
}

func (g *genericMgr) MapName(generic string) []string {
	if names, ok := g.nameMap[generic]; ok {
		return names
	}
	return []string{generic}
}

// newPacman creates a PackageManager for Arch Linux.
func newPacman(runner *executor.Runner) PackageManager {
	return &genericMgr{
		runner: runner,
		name:   "pacman",
		installFn: func(ctx context.Context, r *executor.Runner, pkg string) error {
			return r.Run(ctx, "sudo", "pacman", "-S", "--noconfirm", pkg)
		},
		checkFn: func(r *executor.Runner, pkg string) bool {
			return r.Run(context.Background(), "pacman", "-Q", pkg) == nil
		},
		updateFn: func(ctx context.Context, r *executor.Runner) error {
			return r.Run(ctx, "sudo", "pacman", "-Syu", "--noconfirm")
		},
		nameMap: map[string][]string{
			"nodejs":          {"nodejs", "npm"},
			"build-essential": {"base-devel"},
			"git-delta":       {"git-delta"},
			"fd":              {"fd"},
		},
	}
}

// newDnf creates a PackageManager for Fedora/RHEL (dnf).
func newDnf(runner *executor.Runner) PackageManager {
	return &genericMgr{
		runner: runner,
		name:   "dnf",
		installFn: func(ctx context.Context, r *executor.Runner, pkg string) error {
			return r.Run(ctx, "sudo", "dnf", "install", "-y", pkg)
		},
		checkFn: func(r *executor.Runner, pkg string) bool {
			return r.Run(context.Background(), "dnf", "list", "installed", pkg) == nil
		},
		updateFn: func(ctx context.Context, r *executor.Runner) error {
			return r.Run(ctx, "sudo", "dnf", "update", "-y")
		},
		nameMap: map[string][]string{
			"nodejs":          {"nodejs", "npm"},
			"build-essential": {"gcc", "gcc-c++", "make"},
			"fd":              {"fd-find"},
			"git-delta":       {"git-delta"},
		},
	}
}

// newYum creates a PackageManager for older RHEL/CentOS systems.
func newYum(runner *executor.Runner) PackageManager {
	return &genericMgr{
		runner: runner,
		name:   "yum",
		installFn: func(ctx context.Context, r *executor.Runner, pkg string) error {
			return r.Run(ctx, "sudo", "yum", "install", "-y", pkg)
		},
		checkFn: func(r *executor.Runner, pkg string) bool {
			return r.Run(context.Background(), "yum", "list", "installed", pkg) == nil
		},
		updateFn: func(ctx context.Context, r *executor.Runner) error {
			return r.Run(ctx, "sudo", "yum", "update", "-y")
		},
		nameMap: map[string][]string{
			"nodejs":          {"nodejs", "npm"},
			"build-essential": {"gcc", "gcc-c++", "make"},
			"fd":              {"fd-find"},
			"git-delta":       {"git-delta"},
		},
	}
}

// newZypper creates a PackageManager for openSUSE.
func newZypper(runner *executor.Runner) PackageManager {
	return &genericMgr{
		runner: runner,
		name:   "zypper",
		installFn: func(ctx context.Context, r *executor.Runner, pkg string) error {
			return r.Run(ctx, "sudo", "zypper", "install", "-y", pkg)
		},
		checkFn: func(r *executor.Runner, pkg string) bool {
			return r.Run(context.Background(), "zypper", "se", "--installed-only", pkg) == nil
		},
		updateFn: func(ctx context.Context, r *executor.Runner) error {
			return r.Run(ctx, "sudo", "zypper", "update", "-y")
		},
		nameMap: map[string][]string{
			"nodejs":          {"nodejs", "npm"},
			"build-essential": {"gcc", "gcc-c++", "make"},
		},
	}
}
