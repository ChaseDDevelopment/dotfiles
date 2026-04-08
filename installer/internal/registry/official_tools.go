package registry

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

func officialInstallerTools() []Tool {
	return []Tool{
		// nvm — Node Version Manager
		{
			Name: "nvm", Command: "nvm", Description: "Node Version Manager",
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
		// TPM — Tmux Plugin Manager
		{
			Name: "tpm", Command: "tpm", Description: "Tmux Plugin Manager",
			Strategies: []InstallStrategy{
				{Method: MethodCustom, CustomFunc: installTPM},
			},
		},
	}
}

func installNvm(ctx context.Context, ic *InstallContext) error {
	nvmDir := filepath.Join(os.Getenv("HOME"), ".config", "nvm")

	// Check if already installed.
	if _, err := os.Stat(nvmDir); err == nil {
		return nil
	}
	altDir := filepath.Join(os.Getenv("HOME"), ".nvm")
	if _, err := os.Stat(altDir); err == nil {
		return nil
	}

	// Set NVM_DIR before running the installer so it installs there.
	ic.Runner.AddEnv("NVM_DIR", nvmDir)

	if err := ic.Runner.RunShell(ctx,
		"curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.40.1/install.sh | bash",
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
	// Add atuin to PATH for the current session.
	atunBin := filepath.Join(os.Getenv("HOME"), ".atuin", "bin")
	ic.Runner.AddEnv("PATH", atunBin+":"+os.Getenv("PATH"))
	return nil
}

func installTPM(ctx context.Context, ic *InstallContext) error {
	tpmDir := filepath.Join(os.Getenv("HOME"), ".tmux", "plugins", "tpm")

	// Check if already installed.
	if _, err := os.Stat(tpmDir); err == nil {
		return nil
	}

	return ic.Runner.Run(ctx, "git", "clone",
		"https://github.com/tmux-plugins/tpm", tpmDir)
}
