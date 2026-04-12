package pkgmgr

import (
	"context"
	"fmt"
	"sync"

	"github.com/chaseddevelopment/dotfiles/installer/internal/executor"
)

// Apt implements PackageManager for APT-based systems (Debian, Ubuntu).
// Prefers nala as a frontend when available.
type Apt struct {
	runner    *executor.Runner
	useNala   bool
	mu        sync.Mutex
	didUpdate bool
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
	a.mu.Lock()
	defer a.mu.Unlock()

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

// IsInstalled checks each mapped package via dpkg-query. dpkg-query
// -W -f='${Status}' is precise (exact name match, returns a single
// field) unlike `dpkg -l` which glob-matches and returns success
// even when the package is in the "rc" (removed, config remaining)
// state. Both behaviors are pre-existing latent bugs for packages
// whose names are prefixes of unrelated packages.
func (a *Apt) IsInstalled(genericName string) bool {
	names := a.MapName(genericName)
	if len(names) == 0 {
		return false
	}
	for _, pkg := range names {
		out, err := a.runner.RunProbe(
			context.Background(),
			"dpkg-query", "-W", "-f=${Status}", pkg,
		)
		if err != nil {
			return false
		}
		// Status is "install ok installed" when the package is
		// actually installed. Anything else (including "rc") means
		// not installed for our purposes.
		if !containsInstalled(out) {
			return false
		}
	}
	return true
}

// containsInstalled parses a dpkg-query Status string. Exposed as a
// helper so the behavior is unit-testable without shelling out.
func containsInstalled(status string) bool {
	const want = "install ok installed"
	return len(status) >= len(want) && status[:len(want)] == want
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
