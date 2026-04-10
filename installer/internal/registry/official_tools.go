package registry

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/chaseddevelopment/dotfiles/installer/internal/github"
)

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
				{Method: MethodCustom, CustomFunc: installNvm},
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
				{Method: MethodCustom, CustomFunc: installAtuin},
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
				{Method: MethodCustom, CustomFunc: installTPM},
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

	// Fetch latest nvm version dynamically.
	nvmTag, err := github.LatestVersion("nvm-sh/nvm", false)
	if err != nil {
		nvmTag = "v0.40.4" // fallback
	}
	nvmURL := fmt.Sprintf(
		"https://raw.githubusercontent.com/nvm-sh/nvm/%s/install.sh",
		nvmTag,
	)
	if err := ic.Runner.RunShell(ctx,
		fmt.Sprintf("curl -o- %s | bash", nvmURL),
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
	// Fallback for non-brew/pacman: use official installer.
	if err := ic.Runner.RunShell(ctx,
		`curl --proto '=https' --tlsv1.2 -LsSf https://setup.atuin.sh | sh`,
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
