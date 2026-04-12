package update

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/chaseddevelopment/dotfiles/installer/internal/executor"
	"github.com/chaseddevelopment/dotfiles/installer/internal/platform"
	"github.com/chaseddevelopment/dotfiles/installer/internal/pkgmgr"
	"github.com/chaseddevelopment/dotfiles/installer/internal/registry"
)

// Step describes a single update operation.
type Step struct {
	Name string
	Fn   func(ctx context.Context) error
}

// AllSteps returns the ordered list of update operations,
// matching the current update-packages.sh behavior.
func AllSteps(runner *executor.Runner, mgr pkgmgr.PackageManager, plat *platform.Platform) []Step {
	mgrName := mgr.Name()
	return []Step{
		{"System packages", func(ctx context.Context) error {
			return mgr.UpdateAll(ctx)
		}},
		{"Rust toolchain", func(ctx context.Context) error {
			if !platform.HasCommand("rustup") {
				return nil
			}
			return runner.Run(ctx, "rustup", "update")
		}},
		{"Cargo binaries", func(ctx context.Context) error {
			return updateCargoBinaries(ctx, runner)
		}},
		{"uv ecosystem", func(ctx context.Context) error {
			if !platform.HasCommand("uv") {
				return nil
			}
			if err := runner.Run(ctx, "uv", "self", "update"); err != nil {
				return err
			}
			return runner.Run(ctx, "uv", "tool", "upgrade", "--all")
		}},
		{"Bun", func(ctx context.Context) error {
			if !platform.HasCommand("bun") {
				return nil
			}
			return runner.Run(ctx, "bun", "upgrade")
		}},
		{"Node.js (nvm)", func(ctx context.Context) error {
			return updateNvm(ctx, runner)
		}},
		{"Starship", func(ctx context.Context) error {
			if !platform.HasCommand("starship") {
				return nil
			}
			switch mgrName {
			case "brew":
				return runner.Run(ctx, "brew", "upgrade", "starship")
			case "pacman":
				return runner.Run(ctx, "sudo", "pacman", "-S", "--noconfirm", "starship")
			default:
				return runner.RunShell(ctx,
					`curl -sS https://starship.rs/install.sh -o /tmp/starship-install.sh && sh /tmp/starship-install.sh --yes && rm -f /tmp/starship-install.sh`)
			}
		}},
		{"Atuin", func(ctx context.Context) error {
			return updateAtuin(ctx, runner, mgrName)
		}},
		{"Neovim", func(ctx context.Context) error {
			return updateNeovim(ctx, runner, mgr, plat)
		}},
		{".NET SDK", func(ctx context.Context) error {
			return updateDotnet(ctx, runner, mgrName)
		}},
		{"Yazi plugins", func(ctx context.Context) error {
			if !platform.HasCommand("ya") {
				return nil
			}
			return runner.Run(ctx, "ya", "pkg", "upgrade")
		}},
		{"Tmux plugins", func(ctx context.Context) error {
			script := filepath.Join(os.Getenv("HOME"), ".tmux", "plugins", "tpm", "scripts", "update_plugin.sh")
			if _, err := os.Stat(script); os.IsNotExist(err) {
				return nil
			}
			return runner.Run(ctx, script, "all")
		}},
	}
}

// SelfUpdateStep returns an update step that checks for and
// installs a newer dotsetup binary. Returns nil if the current
// version is "dev".
func SelfUpdateStep(
	runner *executor.Runner,
	currentVersion string,
) *Step {
	if currentVersion == "dev" || currentVersion == "" {
		return nil
	}
	return &Step{
		Name: "dotsetup self-update",
		Fn: func(ctx context.Context) error {
			return SelfUpdate(ctx, runner, currentVersion)
		},
	}
}

func updateCargoBinaries(
	ctx context.Context,
	runner *executor.Runner,
) error {
	if !platform.HasCommand("cargo") {
		return nil
	}
	var errs []error
	// Derive the cargo tool list from the registry instead of
	// maintaining a separate hardcoded list.
	for _, t := range registry.AllTools() {
		if t.CargoCrate == "" {
			continue
		}
		if !platform.HasCommand(t.Command) {
			continue
		}
		if err := runner.Run(
			ctx, "cargo", "install", t.CargoCrate,
		); err != nil {
			errs = append(errs, fmt.Errorf(
				"cargo install %s: %w", t.CargoCrate, err,
			))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf(
			"cargo update failures: %v", errs,
		)
	}
	return nil
}

func updateNvm(ctx context.Context, runner *executor.Runner) error {
	nvmDir := filepath.Join(os.Getenv("HOME"), ".config", "nvm")
	if _, err := os.Stat(nvmDir); os.IsNotExist(err) {
		nvmDir = filepath.Join(os.Getenv("HOME"), ".nvm")
	}
	if _, err := os.Stat(nvmDir); os.IsNotExist(err) {
		return nil
	}
	script := fmt.Sprintf(
		`export NVM_DIR="%s" && [ -s "$NVM_DIR/nvm.sh" ] && . "$NVM_DIR/nvm.sh" && nvm install --lts && nvm alias default lts/*`,
		nvmDir,
	)
	return runner.RunShell(ctx, script)
}

func updateAtuin(ctx context.Context, runner *executor.Runner, mgrName string) error {
	if !platform.HasCommand("atuin") {
		return nil
	}
	switch mgrName {
	case "brew":
		return runner.Run(ctx, "brew", "upgrade", "atuin")
	case "pacman":
		return runner.Run(ctx, "sudo", "pacman", "-S", "--noconfirm", "atuin")
	default:
		return runner.RunShell(ctx,
			`curl --proto '=https' --tlsv1.2 -LsSf https://setup.atuin.sh | sh`)
	}
}

func updateNeovim(ctx context.Context, runner *executor.Runner, mgr pkgmgr.PackageManager, plat *platform.Platform) error {
	if !platform.HasCommand("nvim") {
		return nil
	}
	switch mgr.Name() {
	case "brew":
		return runner.Run(ctx, "brew", "upgrade", "neovim")
	case "pacman":
		for _, helper := range []string{"yay", "paru"} {
			if _, err := exec.LookPath(helper); err != nil {
				continue
			}
			if err := runner.Run(ctx, helper, "-S", "--noconfirm", "neovim-git"); err != nil {
				runner.Log.Write(fmt.Sprintf(
					"NOTE: %s neovim-git update failed: %v", helper, err,
				))
				continue
			}
			return nil
		}
		return runner.Run(ctx, "sudo", "pacman", "-S", "--noconfirm", "neovim")
	case "apt":
		// Reuse the same GitHub release install logic.
		ic := &registry.InstallContext{Runner: runner, PkgMgr: mgr, Platform: plat}
		return registry.InstallNeovimApt(ctx, ic)
	default:
		return mgr.Install(ctx, "neovim")
	}
}

func updateDotnet(ctx context.Context, runner *executor.Runner, mgrName string) error {
	if !platform.HasCommand("dotnet") {
		return nil
	}
	switch mgrName {
	case "brew":
		return runner.Run(ctx, "brew", "upgrade", "dotnet-sdk")
	case "pacman":
		return runner.Run(ctx, "sudo", "pacman", "-S", "--noconfirm", "dotnet-sdk")
	default:
		return runner.RunShell(ctx,
			`curl -sSL https://dot.net/v1/dotnet-install.sh -o /tmp/dotnet-install.sh && chmod +x /tmp/dotnet-install.sh && /tmp/dotnet-install.sh --channel LTS --install-dir "$HOME/.dotnet" && rm -f /tmp/dotnet-install.sh`)
	}
}
