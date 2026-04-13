package pkgmgr

import (
	"context"
	"fmt"

	"github.com/chaseddevelopment/dotfiles/installer/internal/executor"
)

// genericMgr is a data-driven PackageManager for simple
// distributions that follow the same install/check/update pattern.
// installFn now receives the full package slice so each manager can
// emit a single multi-pkg shell invocation (one `pacman -S a b c`,
// one `dnf install a b c`, etc.) rather than looping one-per-call.
type genericMgr struct {
	runner    *executor.Runner
	name      string
	installFn func(ctx context.Context, r *executor.Runner, pkgs []string) error
	checkFn   func(r *executor.Runner, pkg string) bool
	updateFn  func(ctx context.Context, r *executor.Runner) error
	nameMap   map[string][]string
}

func (g *genericMgr) Name() string { return g.name }

func (g *genericMgr) Install(
	ctx context.Context,
	genericNames ...string,
) error {
	if len(genericNames) == 0 {
		return nil
	}
	pkgs := dedupeNames(g.MapName, genericNames)
	if len(pkgs) == 0 {
		return nil
	}
	if err := g.installFn(ctx, g.runner, pkgs); err != nil {
		return attribute(
			fmt.Errorf("%s install: %w", g.name, err),
			genericNames,
			g.IsInstalled,
		)
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
		installFn: func(ctx context.Context, r *executor.Runner, pkgs []string) error {
			args := append([]string{"sudo", "pacman", "-S", "--needed", "--noconfirm"}, pkgs...)
			return r.Run(ctx, args[0], args[1:]...)
		},
		checkFn: func(r *executor.Runner, pkg string) bool {
			_, err := r.RunProbe(context.Background(), "pacman", "-Q", pkg)
			return err == nil
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
		installFn: func(ctx context.Context, r *executor.Runner, pkgs []string) error {
			args := append([]string{"sudo", "dnf", "install", "-y"}, pkgs...)
			return r.Run(ctx, args[0], args[1:]...)
		},
		checkFn: func(r *executor.Runner, pkg string) bool {
			_, err := r.RunProbe(context.Background(), "dnf", "list", "installed", pkg)
			return err == nil
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
		installFn: func(ctx context.Context, r *executor.Runner, pkgs []string) error {
			args := append([]string{"sudo", "yum", "install", "-y"}, pkgs...)
			return r.Run(ctx, args[0], args[1:]...)
		},
		checkFn: func(r *executor.Runner, pkg string) bool {
			_, err := r.RunProbe(context.Background(), "yum", "list", "installed", pkg)
			return err == nil
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
		installFn: func(ctx context.Context, r *executor.Runner, pkgs []string) error {
			args := append([]string{"sudo", "zypper", "--non-interactive", "install"}, pkgs...)
			return r.Run(ctx, args[0], args[1:]...)
		},
		checkFn: func(r *executor.Runner, pkg string) bool {
			_, err := r.RunProbe(context.Background(), "zypper", "se", "--installed-only", pkg)
			return err == nil
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
