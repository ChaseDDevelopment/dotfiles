package pkgmgr

import (
	"context"
	"fmt"

	"github.com/chaseddevelopment/dotfiles/installer/internal/executor"
	"github.com/chaseddevelopment/dotfiles/installer/internal/platform"
)

// PackageManager abstracts system package manager operations.
type PackageManager interface {
	// Name returns the manager identifier ("brew", "apt", "pacman", etc.).
	Name() string

	// Install installs one or more packages by generic name.
	Install(ctx context.Context, genericNames ...string) error

	// IsInstalled checks if a package is installed.
	IsInstalled(genericName string) bool

	// UpdateAll runs the system-wide update/upgrade command.
	UpdateAll(ctx context.Context) error

	// MapName translates a generic package name to platform-specific
	// names. May return multiple (e.g., "nodejs" -> ["nodejs", "npm"]).
	MapName(generic string) []string
}

// New creates the appropriate PackageManager for the detected platform.
func New(p *platform.Platform, runner *executor.Runner) (PackageManager, error) {
	switch p.PackageManager {
	case platform.PkgBrew:
		return &Brew{runner: runner}, nil
	case platform.PkgApt:
		return &Apt{runner: runner, useNala: p.HasNala}, nil
	case platform.PkgPacman:
		return newPacman(runner), nil
	case platform.PkgDnf:
		return newDnf(runner), nil
	case platform.PkgYum:
		return newYum(runner), nil
	case platform.PkgZypper:
		return newZypper(runner), nil
	default:
		return nil, fmt.Errorf("no supported package manager found")
	}
}
