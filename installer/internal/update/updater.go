package update

// Test-coverage note (Category C — environmental syscall paths):
//   - The AUR-helper (yay/paru) discovery branches inside
//     updateNeovim (lines ~209–224) require those binaries to be
//     installed and a real Arch package database; they cannot be
//     exercised meaningfully in the cross-platform CI matrix.
//   - The Chmod / bash-exec paths inside runDownloadedScript
//     (lines ~270–289) depend on a writable /tmp + an executable
//     bash that downloads a real Microsoft installer script. These
//     are integration-scope and intentionally not unit-tested; the
//     argv composition is inspected via code review rather than
//     stubbed shell shims.

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/chaseddevelopment/dotfiles/installer/internal/executor"
	"github.com/chaseddevelopment/dotfiles/installer/internal/github"
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
		{"Oh-My-Posh", func(ctx context.Context) error {
			if !platform.HasCommand("oh-my-posh") {
				return nil
			}
			return updateOhMyPosh(ctx, runner, mgr)
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

func updateOhMyPosh(ctx context.Context, runner *executor.Runner, mgr pkgmgr.PackageManager) error {
	// Oh-My-Posh is shipped upstream only via Homebrew and the
	// official install script. It's AUR-only on Arch and not in
	// Debian/Fedora/openSUSE repos, so every non-brew manager falls
	// through to the auditable download-then-exec helper below.
	if mgr.Name() == "brew" {
		return runner.Run(ctx, "brew", "upgrade", "oh-my-posh")
	}
	return runDownloadedScript(
		ctx, runner,
		"https://ohmyposh.dev/install.sh",
		[]string{"-d", filepath.Join(os.Getenv("HOME"), ".local", "bin")},
	)
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
	case "apt", "dnf", "yum":
		// atuin ships .deb/.rpm assets in their GitHub release —
		// but the simplest path when it's installable via cargo is
		// to upgrade that way rather than piping a remote shell
		// script through sh.
		if platform.HasCommand("cargo") {
			return runner.Run(ctx, "cargo", "install", "atuin")
		}
	}
	return fmt.Errorf(
		"no safe Atuin update path for package manager %q; "+
			"install cargo or use a supported package manager",
		mgrName,
	)
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
	}
	// No package-manager path: fall back to Microsoft's official
	// dotnet-install.sh, but download to disk first (auditable), log
	// the resulting sha256 so incident responders can cross-check,
	// and invoke the saved file with argv — never through `sh -c` or
	// `curl | sh`. This is a strict improvement over the previous
	// implementation. Pinning a sha is TODO; Microsoft's script URL
	// is versioned but the content churns.
	return runDownloadedScript(
		ctx, runner,
		"https://dot.net/v1/dotnet-install.sh",
		[]string{
			"--channel", "LTS",
			"--install-dir", filepath.Join(os.Getenv("HOME"), ".dotnet"),
		},
	)
}

// runDownloadedScript fetches a shell script to a temp file, logs
// its sha256 for post-hoc verification, executes it with the given
// args (no shell interpolation), and removes the file. This replaces
// the `curl | sh` pattern, which executes bytes as they arrive from
// the network — an attacker able to inject mid-stream could splice
// arbitrary commands that curl wouldn't otherwise see.
func runDownloadedScript(
	ctx context.Context,
	runner *executor.Runner,
	url string,
	args []string,
) error {
	f, err := os.CreateTemp("", "dotsetup-update-*.sh")
	if err != nil {
		return fmt.Errorf("create temp script: %w", err)
	}
	scriptPath := f.Name()
	f.Close()
	defer os.Remove(scriptPath)

	if err := runner.Run(
		ctx, "curl", "-fsSL", url, "-o", scriptPath,
	); err != nil {
		return fmt.Errorf("download %s: %w", url, err)
	}
	if sum, err := github.Sha256File(scriptPath); err == nil {
		runner.Log.Write(fmt.Sprintf(
			"downloaded %s sha256=%s", url, sum,
		))
	}
	if err := os.Chmod(scriptPath, 0o755); err != nil {
		return fmt.Errorf("chmod script: %w", err)
	}
	argv := append([]string{scriptPath}, args...)
	return runner.Run(ctx, "bash", argv...)
}
