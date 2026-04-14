package registry

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/chaseddevelopment/dotfiles/installer/internal/github"
)

var latestVersionFn = github.LatestVersion

func officialInstallerTools() []Tool {
	return []Tool{
		// nvm — Node Version Manager (shell function, not a binary)
		{
			Name: "nvm", Command: "nvm", Description: "Node Version Manager",
			IsInstalledFunc: func() bool {
				home := os.Getenv("HOME")
				for _, p := range []string{
					filepath.Join(home, ".config", "nvm", "nvm.sh"),
					filepath.Join(home, ".nvm", "nvm.sh"),
				} {
					if _, err := os.Stat(p); err == nil {
						return true
					}
				}
				return false
			},
			Strategies: []InstallStrategy{
				{Method: MethodCustom, CustomFunc: installNvm, Requires: []string{"curl"}},
			},
		},
		// Node.js LTS (installed via package manager as base)
		{
			Name: "nodejs", Command: "node", Description: "Node.js runtime",
			Strategies: []InstallStrategy{
				{Method: MethodPackageManager, Package: "nodejs"},
			},
		},
		// Atuin — shell history
		{
			Name: "atuin", Command: "atuin", Description: "Magical shell history",
			Strategies: []InstallStrategy{
				{Managers: []string{"brew"}, Method: MethodPackageManager, Package: "atuin"},
				{Managers: []string{"pacman"}, Method: MethodPackageManager, Package: "atuin"},
				{Method: MethodCustom, CustomFunc: installAtuin, Requires: []string{"curl"}},
			},
		},
		// TPM — Tmux Plugin Manager (git repo, not a binary)
		{
			Name: "tpm", Command: "tpm", Description: "Tmux Plugin Manager",
			IsInstalledFunc: func() bool {
				tpmDir := filepath.Join(os.Getenv("HOME"), ".tmux", "plugins", "tpm")
				_, err := os.Stat(tpmDir)
				return err == nil
			},
			Strategies: []InstallStrategy{
				{Method: MethodCustom, CustomFunc: installTPM, Requires: []string{"git"}},
			},
		},
	}
}

func installNvm(ctx context.Context, ic *InstallContext) error {
	nvmDir := filepath.Join(os.Getenv("HOME"), ".config", "nvm")

	// Skip if already installed (unless force reinstall).
	if !ic.ForceReinstall {
		if _, err := os.Stat(nvmDir); err == nil {
			return nil
		}
		altDir := filepath.Join(os.Getenv("HOME"), ".nvm")
		if _, err := os.Stat(altDir); err == nil {
			return nil
		}
	}

	// Set NVM_DIR before running the installer so it installs there.
	ic.Runner.AddEnv("NVM_DIR", nvmDir)

	// Fetch latest nvm version dynamically. A silent fallback to a
	// hardcoded tag would violate the "always latest" policy and
	// hide rate-limits or network issues from the user.
	nvmTag, err := latestVersionFn("nvm-sh/nvm", false)
	if err != nil {
		return fmt.Errorf("resolve latest nvm version: %w", err)
	}
	nvmURL := fmt.Sprintf(
		"https://raw.githubusercontent.com/nvm-sh/nvm/%s/install.sh",
		nvmTag,
	)
	// Download the installer to disk, log its sha256 for audit, and
	// exec from the saved file. Replaces the previous `curl | bash`
	// pipe so bytes from the network are never executed as they
	// arrive — an in-flight MITM can't splice commands curl didn't
	// see.
	f, err := os.CreateTemp("", "dotsetup-nvm-install-*.sh")
	if err != nil {
		return fmt.Errorf("create temp nvm installer: %w", err)
	}
	scriptPath := f.Name()
	f.Close()
	defer os.Remove(scriptPath)

	if err := ic.Runner.Run(
		ctx, "curl", "-fsSL", nvmURL, "-o", scriptPath,
	); err != nil {
		return fmt.Errorf("download nvm installer: %w", err)
	}
	if sum, err := github.Sha256File(scriptPath); err == nil {
		ic.Runner.Log.Write(fmt.Sprintf(
			"downloaded nvm %s installer sha256=%s", nvmTag, sum,
		))
	}
	if err := os.Chmod(scriptPath, 0o755); err != nil {
		return fmt.Errorf("chmod nvm installer: %w", err)
	}
	// PROFILE=/dev/null tells nvm's install.sh to skip its
	// .zshrc/.bashrc append step. configs/zsh/tools/nvm.zsh already
	// sets NVM_DIR and lazy-loads nvm.sh, so the append would only
	// duplicate what our repo already provides — and mutate the
	// symlinked .zshrc in the process.
	if err := ic.Runner.RunWithEnv(
		ctx, noProfileEnv(), "bash", scriptPath,
	); err != nil {
		return fmt.Errorf("install nvm: %w", err)
	}

	// Source nvm and install LTS node.
	script := fmt.Sprintf(
		`export NVM_DIR="%s" && [ -s "$NVM_DIR/nvm.sh" ] && . "$NVM_DIR/nvm.sh" && nvm install --lts && nvm alias default lts/*`,
		nvmDir,
	)
	return ic.Runner.RunShell(ctx, script)
}

func installAtuin(ctx context.Context, ic *InstallContext) error {
	// Fallback for non-brew/pacman: use official installer. Unlike
	// the previous `curl | sh` pipe, we download the script to a
	// temp file, log its sha256 for audit, and exec from disk so no
	// bytes are executed as they arrive from the network.
	f, err := os.CreateTemp("", "dotsetup-atuin-install-*.sh")
	if err != nil {
		return fmt.Errorf("create temp atuin installer: %w", err)
	}
	scriptPath := f.Name()
	f.Close()
	defer os.Remove(scriptPath)

	if err := ic.Runner.Run(
		ctx, "curl", "--proto", "=https", "--tlsv1.2",
		"-fsSL", "https://setup.atuin.sh", "-o", scriptPath,
	); err != nil {
		return fmt.Errorf("download atuin installer: %w", err)
	}
	if sum, err := github.Sha256File(scriptPath); err == nil {
		ic.Runner.Log.Write(fmt.Sprintf(
			"downloaded atuin installer sha256=%s", sum,
		))
	}
	if err := os.Chmod(scriptPath, 0o755); err != nil {
		return fmt.Errorf("chmod atuin installer: %w", err)
	}
	// SHELL=/bin/sh makes atuin's setup.sh fall through its shell
	// detection (bash/zsh/fish) and skip the rc-append step.
	// configs/zsh/.zshrc already runs `_cached_init atuin init zsh`,
	// so the installer's eval line would be redundant.
	if err := ic.Runner.RunWithEnv(
		ctx, noProfileEnv(), "sh", scriptPath,
	); err != nil {
		return fmt.Errorf("install atuin: %w", err)
	}
	// Add atuin to PATH for the current session so subsequent
	// tasks and exec.LookPath can find the binary.
	atunBin := filepath.Join(os.Getenv("HOME"), ".atuin", "bin")
	newPath := atunBin + ":" + os.Getenv("PATH")
	ic.Runner.AddEnv("PATH", newPath)
	return nil
}

func installTPM(ctx context.Context, ic *InstallContext) error {
	tpmDir := filepath.Join(os.Getenv("HOME"), ".tmux", "plugins", "tpm")

	// Skip if already installed (unless force reinstall).
	if !ic.ForceReinstall {
		if _, err := os.Stat(tpmDir); err == nil {
			return nil
		}
	} else {
		os.RemoveAll(tpmDir)
	}

	return ic.Runner.Run(ctx, "git", "clone",
		"https://github.com/tmux-plugins/tpm", tpmDir)
}
