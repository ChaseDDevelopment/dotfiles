package registry

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func rustToolchain() []Tool {
	return []Tool{
		{
			Name: "rust", Command: "cargo", Description: "Rust toolchain via rustup",
			Strategies: []InstallStrategy{
				{Method: MethodScript, Script: &ScriptConfig{
					URL:  "https://sh.rustup.rs",
					Args: []string{"-y"},
				}},
			},
		},
	}
}

func devTools() []Tool {
	return []Tool{
		// Neovim
		{
			Name: "neovim", Command: "nvim", Description: "Hyperextensible text editor",
			Critical: true,
			Strategies: []InstallStrategy{
				{Managers: []string{"brew"}, Method: MethodCustom,
					CustomFunc: func(ctx context.Context, ic *InstallContext) error {
						return ic.Runner.Run(ctx, "brew", "install", "--HEAD", "neovim")
					},
				},
				{Managers: []string{"pacman"}, Method: MethodCustom,
					CustomFunc: installNeovimPacman,
				},
				{Managers: []string{"apt"}, Method: MethodCustom,
					CustomFunc: installNeovimApt,
				},
				{Managers: []string{"dnf", "yum"}, Method: MethodPackageManager, Package: "neovim"},
			},
		},
		// tree-sitter library (dev dependency — not a binary in PATH)
		{
			Name: "tree-sitter-lib", Command: "tree-sitter-lib", Description: "Tree-sitter parser library",
			IsInstalledFunc: func() bool {
				// Check via pkg-config (Linux) or header existence (macOS/brew).
				if err := exec.Command("pkg-config", "--exists", "tree-sitter").Run(); err == nil {
					return true
				}
				// Brew installs headers to known paths.
				for _, p := range []string{
					"/opt/homebrew/include/tree_sitter/api.h",
					"/usr/local/include/tree_sitter/api.h",
					"/usr/include/tree_sitter/api.h",
				} {
					if _, err := os.Stat(p); err == nil {
						return true
					}
				}
				return false
			},
			Strategies: []InstallStrategy{
				{Managers: []string{"brew"}, Method: MethodPackageManager, Package: "tree-sitter"},
				{Managers: []string{"pacman"}, Method: MethodPackageManager, Package: "tree-sitter"},
				{Managers: []string{"apt"}, Method: MethodPackageManager, Package: "libtree-sitter-dev"},
				{Managers: []string{"dnf", "yum"}, Method: MethodPackageManager, Package: "libtree-sitter-devel"},
			},
		},
		// tree-sitter CLI
		{
			Name: "tree-sitter-cli", Command: "tree-sitter", Description: "Tree-sitter CLI",
			DependsOn: []string{"cargo"}, // cargo fallback on apt/dnf
			Strategies: []InstallStrategy{
				{Managers: []string{"brew"}, Method: MethodPackageManager, Package: "tree-sitter-cli"},
				{Managers: []string{"pacman"}, Method: MethodPackageManager, Package: "tree-sitter-cli"},
				{Method: MethodCargo, Crate: "tree-sitter-cli"},
			},
			CargoCrate: "tree-sitter-cli",
		},
		// uv — Python package manager
		{
			Name: "uv", Command: "uv", Description: "Fast Python package manager",
			Strategies: []InstallStrategy{
				{Method: MethodScript, Script: &ScriptConfig{
					URL: "https://astral.sh/uv/install.sh",
				}},
			},
		},
		// ruff — Python linter/formatter
		{
			Name: "ruff", Command: "ruff", Description: "Python linter and formatter",
			DependsOn: []string{"uv"},
			Strategies: []InstallStrategy{
				{Method: MethodCustom, CustomFunc: func(ctx context.Context, ic *InstallContext) error {
					return ic.Runner.Run(ctx, "uv", "tool", "install", "ruff")
				}},
			},
		},
		// Bun — JavaScript runtime
		{
			Name: "bun", Command: "bun", Description: "Fast JavaScript runtime",
			Strategies: []InstallStrategy{
				{Method: MethodScript, Script: &ScriptConfig{
					URL:   "https://bun.sh/install",
					Shell: "bash",
				}},
			},
		},
		// .NET SDK
		{
			Name: "dotnet", Command: "dotnet", Description: ".NET SDK",
			Strategies: []InstallStrategy{
				{Managers: []string{"brew"}, Method: MethodPackageManager, Package: "dotnet-sdk"},
				{Managers: []string{"pacman"}, Method: MethodPackageManager, Package: "dotnet-sdk"},
				{Method: MethodScript, Script: &ScriptConfig{
					URL:  "https://dot.net/v1/dotnet-install.sh",
					Args: []string{"--channel", "LTS", "--install-dir", "$HOME/.dotnet"},
				}},
			},
		},
		// Starship prompt
		{
			Name: "starship", Command: "starship", Description: "Cross-shell prompt",
			Strategies: []InstallStrategy{
				{Managers: []string{"brew", "pacman"}, Method: MethodPackageManager, Package: "starship"},
				{Method: MethodScript, Script: &ScriptConfig{
					URL:  "https://starship.rs/install.sh",
					Args: []string{"--yes"},
				}},
			},
		},
		// yazi — terminal file manager + companion packages
		{
			Name: "yazi", Command: "yazi", Description: "Terminal file manager",
			Strategies: []InstallStrategy{
				{Managers: []string{"brew"}, Method: MethodCustom,
					CustomFunc: func(ctx context.Context, ic *InstallContext) error {
						return ic.Runner.Run(ctx, "brew", "install",
							"yazi", "ffmpeg", "sevenzip", "jq", "poppler", "resvg", "imagemagick")
					},
				},
				{Managers: []string{"pacman"}, Method: MethodCustom,
					CustomFunc: func(ctx context.Context, ic *InstallContext) error {
						return ic.Runner.Run(ctx, "sudo", "pacman", "-S", "--noconfirm",
							"yazi", "ffmpeg", "7zip", "jq", "poppler", "resvg", "imagemagick")
					},
				},
				{Managers: []string{"apt"}, Method: MethodCustom,
					CustomFunc: installYaziApt,
				},
				{Method: MethodCargo, Crate: "yazi-build"},
			},
			CargoCrate: "yazi-build",
		},
		// Go
		{
			Name: "go", Command: "go", Description: "Go programming language",
			Strategies: []InstallStrategy{
				{Managers: []string{"brew"}, Method: MethodPackageManager, Package: "go"},
				{Managers: []string{"pacman"}, Method: MethodPackageManager, Package: "go"},
				{Managers: []string{"apt"}, Method: MethodPackageManager, Package: "golang"},
				{Managers: []string{"dnf", "yum"}, Method: MethodPackageManager, Package: "golang"},
			},
		},
	}
}

func installNeovimPacman(ctx context.Context, ic *InstallContext) error {
	// Try AUR helpers first for neovim-git, fall back to pacman.
	for _, helper := range []string{"yay", "paru"} {
		if _, err := exec.LookPath(helper); err == nil {
			if err := ic.Runner.Run(ctx, helper, "-S", "--noconfirm", "neovim-git"); err == nil {
				return nil
			}
		}
	}
	return ic.Runner.Run(ctx, "sudo", "pacman", "-S", "--noconfirm", "neovim")
}

func installNeovimApt(ctx context.Context, ic *InstallContext) error {
	// Download from GitHub releases for apt systems (repos are too old).
	arch := "x86_64"
	if runtime.GOARCH == "arm64" {
		arch = "arm64"
	}

	url := fmt.Sprintf(
		"https://github.com/neovim/neovim/releases/latest/download/nvim-linux-%s.tar.gz",
		arch,
	)

	tmpDir, err := os.MkdirTemp("", "nvim-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	tarPath := filepath.Join(tmpDir, "nvim.tar.gz")
	if err := ic.Runner.Run(ctx, "curl", "-fsSL", url, "-o", tarPath); err != nil {
		return err
	}

	// Clean up old installs.
	for _, old := range []string{"/opt/nvim", "/opt/nvim-linux-x86_64", "/opt/nvim-linux-arm64"} {
		_ = ic.Runner.Run(ctx, "sudo", "rm", "-rf", old)
	}

	if err := ic.Runner.Run(ctx, "sudo", "tar", "-C", "/opt", "-xzf", tarPath); err != nil {
		return err
	}

	// Find the extracted directory and symlink.
	_ = ic.Runner.Run(ctx, "sudo", "rm", "-f", "/usr/local/bin/nvim")
	return ic.Runner.RunShell(ctx,
		"sudo ln -s /opt/nvim-linux-*/bin/nvim /usr/local/bin/nvim")
}

func installYaziApt(ctx context.Context, ic *InstallContext) error {
	// Install companion packages first (best-effort — log failures).
	deps := []string{"ffmpeg", "p7zip-full", "jq", "poppler-utils", "resvg", "imagemagick"}
	for _, dep := range deps {
		if err := ic.PkgMgr.Install(ctx, dep); err != nil {
			ic.Runner.Log.Write(fmt.Sprintf("WARNING: optional dep %s failed: %v", dep, err))
		}
	}
	// Build yazi from source via cargo.
	return ic.Runner.Run(ctx, "cargo", "install", "--force", "yazi-build")
}
