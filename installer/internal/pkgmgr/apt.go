package pkgmgr

import (
	"context"
	"fmt"

	"github.com/chaseddevelopment/dotfiles/installer/internal/executor"
)

// Apt implements PackageManager for APT-based systems (Debian, Ubuntu).
// Prefers nala as a frontend when available.
type Apt struct {
	runner     *executor.Runner
	useNala    bool
	didUpdate  bool
}

func (a *Apt) Name() string { return "apt" }

func (a *Apt) cmd() string {
	if a.useNala {
		return "nala"
	}
	return "apt-get"
}

// ensureUpdated runs apt-get update once per session before the
// first install to ensure the package cache is fresh.
func (a *Apt) ensureUpdated(ctx context.Context) error {
	if a.didUpdate {
		return nil
	}
	if err := a.runner.Run(
		ctx, "sudo", a.cmd(), "update",
	); err != nil {
		return fmt.Errorf("%s update: %w", a.cmd(), err)
	}
	a.didUpdate = true
	return nil
}

func (a *Apt) Install(ctx context.Context, genericNames ...string) error {
	if err := a.ensureUpdated(ctx); err != nil {
		return err
	}
	for _, generic := range genericNames {
		names := a.MapName(generic)
		for _, pkg := range names {
			if err := a.runner.Run(ctx, "sudo", a.cmd(), "install", "-y", pkg); err != nil {
				return fmt.Errorf("%s install %s: %w", a.cmd(), pkg, err)
			}
		}
	}
	return nil
}

func (a *Apt) IsInstalled(genericName string) bool {
	names := a.MapName(genericName)
	for _, pkg := range names {
		if err := a.runner.Run(context.Background(), "dpkg", "-l", pkg); err != nil {
			return false
		}
	}
	return len(names) > 0
}

func (a *Apt) UpdateAll(ctx context.Context) error {
	cmd := a.cmd()
	script := fmt.Sprintf("sudo %s update && sudo %s upgrade -y", cmd, cmd)
	return a.runner.RunShell(ctx, script)
}

func (a *Apt) MapName(generic string) []string {
	m := map[string][]string{
		"nodejs":          {"nodejs", "npm"},
		"build-essential": {"build-essential"},
		"fd":              {"fd-find"},
		"bat":             {"bat"},
	}
	if names, ok := m[generic]; ok {
		return names
	}
	return []string{generic}
}
