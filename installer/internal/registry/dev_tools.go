package registry

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/chaseddevelopment/dotfiles/installer/internal/platform"
)

func rustToolchain() []Tool {
	return []Tool{
		{
			Name: "rust", Command: "cargo", Description: "Rust toolchain via rustup",
			Strategies: []InstallStrategy{
				{Method: MethodScript, Script: &ScriptConfig{
					URL: "https://sh.rustup.rs",
					// --no-modify-path: configs/zsh/.zprofile already
					// prepends ~/.cargo/bin to PATH, so the installer
					// writing its own `source ~/.cargo/env` line into
					// .zshenv is redundant and pollutes the repo.
					Args:            []string{"-y", "--no-modify-path"},
					NoProfileModify: true,
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
			Critical:   true,
			MinVersion: "0.12.0",
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
					CustomFunc: InstallNeovimApt,
					Requires:   []string{"curl"},
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
			Strategies: []InstallStrategy{
				{Managers: []string{"brew"}, Method: MethodPackageManager, Package: "tree-sitter-cli"},
				{Managers: []string{"pacman"}, Method: MethodPackageManager, Package: "tree-sitter-cli"},
				{Method: MethodCustom, CustomFunc: installTreeSitterCLI, Requires: []string{"curl"}},
				{Method: MethodCargo, Crate: "tree-sitter-cli"},
			},
			CargoCrate: "tree-sitter-cli",
		},
		// uv — Python package manager
		{
			Name: "uv", Command: "uv", Description: "Fast Python package manager",
			DevOnly: true,
			Strategies: []InstallStrategy{
				{Method: MethodScript, Script: &ScriptConfig{
					URL:             "https://astral.sh/uv/install.sh",
					NoProfileModify: true,
				}},
			},
		},
		// ruff — Python linter/formatter
		{
			Name: "ruff", Command: "ruff", Description: "Python linter and formatter",
			DevOnly:   true,
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
			DevOnly: true,
			Strategies: []InstallStrategy{
				{Method: MethodScript, Script: &ScriptConfig{
					URL:             "https://bun.sh/install",
					Shell:           "bash",
					NoProfileModify: true,
				}},
			},
		},
		// .NET SDK
		{
			Name: "dotnet", Command: "dotnet", Description: ".NET SDK",
			DevOnly: true,
			Strategies: []InstallStrategy{
				// Homebrew: `dotnet-sdk` is a cask; the formula that ships the full SDK is just `dotnet`.
				{Managers: []string{"brew"}, Method: MethodPackageManager, Package: "dotnet"},
				{Managers: []string{"pacman"}, Method: MethodPackageManager, Package: "dotnet-sdk"},
				{Method: MethodScript, Script: &ScriptConfig{
					URL:  "https://dot.net/v1/dotnet-install.sh",
					Args: []string{"--channel", "LTS", "--install-dir", "$HOME/.dotnet"},
				}},
			},
		},
		// Oh-My-Posh prompt
		{
			Name: "oh-my-posh", Command: "oh-my-posh", Description: "Cross-shell prompt",
			Strategies: []InstallStrategy{
				// Fully qualified formula triggers brew's auto-tap of
				// jandedobbeleer/oh-my-posh on first install.
				{Managers: []string{"brew"}, Method: MethodPackageManager, Package: "jandedobbeleer/oh-my-posh/oh-my-posh"},
				{Method: MethodScript, Script: &ScriptConfig{
					// No pacman strategy: oh-my-posh is AUR-only on
					// Arch, which this installer doesn't handle, so
					// pacman users fall through to the script. Pass
					// -d to land the binary on an already-PATHed dir
					// (default ~/bin isn't on our PATH).
					URL:             "https://ohmyposh.dev/install.sh",
					Args:            []string{"-d", "$HOME/.local/bin"},
					NoProfileModify: true,
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
					Requires:   []string{"curl"},
				},
				{Method: MethodCargo, Crate: "yazi-build"},
			},
			CargoCrate: "yazi-build",
		},
		// Go
		{
			Name: "go", Command: "go", Description: "Go programming language",
			DevOnly: true,
			Strategies: []InstallStrategy{
				{Managers: []string{"brew"}, Method: MethodPackageManager, Package: "go"},
				{Managers: []string{"pacman"}, Method: MethodPackageManager, Package: "go"},
				{Managers: []string{"apt"}, Method: MethodPackageManager, Package: "golang"},
				{Managers: []string{"dnf", "yum"}, Method: MethodPackageManager, Package: "golang"},
			},
		},
		// gopls — Go language server (LSP). GOPATH/GOBIN are pinned to
		// XDG paths so gopls lands in ~/.local/bin (matching .zshenv)
		// even when ./install.sh is launched from a shell that hasn't
		// sourced .zshenv yet (e.g., system bash on a fresh box).
		{
			Name: "gopls", Command: "gopls", Description: "Go language server (LSP)",
			DevOnly:   true,
			DependsOn: []string{"go"},
			Strategies: []InstallStrategy{
				{Method: MethodCustom, CustomFunc: func(ctx context.Context, ic *InstallContext) error {
					home, err := os.UserHomeDir()
					if err != nil {
						return fmt.Errorf("resolve home dir: %w", err)
					}
					env := []string{
						"GOPATH=" + filepath.Join(home, ".local", "share", "go"),
						"GOBIN=" + filepath.Join(home, ".local", "bin"),
					}
					return ic.Runner.RunWithEnv(ctx, env, "go", "install", "golang.org/x/tools/gopls@latest")
				}},
			},
		},
	}
}

// Category C — env-bound real-shell branches.
// installNeovimPacman's "no AUR helpers found, fall through to
// `sudo pacman -S --noconfirm neovim`" tail (line 201) only triggers
// on a host where neither yay nor paru is on PATH AND `sudo pacman`
// can actually run. Unit tests can't safely exercise sudo; the
// pacman-only path is covered indirectly via the dry-run runner in
// closures_test.go. Same applies to InstallNeovimApt's
// `sudo rm -rf /opt/...` cleanup loop (lines 263-269) and the
// `sudo tar -C /opt -xzf` extraction (line 271): both require root
// in non-dry-run mode. Fakes via PATH stubs in installers_test.go
// cover the argv-shape contract, but the actual privilege-elevation
// branches stay untested by design.
func installNeovimPacman(ctx context.Context, ic *InstallContext) error {
	// Try AUR helpers first for neovim-git, fall back to pacman.
	// Log each helper failure so "fell back to stable neovim" isn't
	// a silent downgrade.
	for _, helper := range []string{"yay", "paru"} {
		if _, err := exec.LookPath(helper); err != nil {
			continue
		}
		if err := ic.Runner.Run(ctx, helper, "-S", "--noconfirm", "neovim-git"); err != nil {
			ic.Runner.Log.Write(fmt.Sprintf(
				"NOTE: %s neovim-git failed: %v", helper, err,
			))
			continue
		}
		return nil
	}
	return ic.Runner.Run(ctx, "sudo", "pacman", "-S", "--noconfirm", "neovim")
}

// InstallNeovimApt downloads Neovim from GitHub releases for
// apt-based systems where the repo version is too old. Exported
// so update logic can reuse the same install path.
func InstallNeovimApt(ctx context.Context, ic *InstallContext) error {
	// Download from GitHub releases for apt systems (repos are too old).
	arch := "x86_64"
	if ic.Platform != nil && ic.Platform.Arch == platform.ARM64 {
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

	// Peek at the archive to learn the top-level directory name
	// before extracting. Using a shell glob post-extract would
	// match any pre-existing /opt/nvim-linux-* dir, including a
	// stale or hostile one. A literal path is safer.
	listOut, err := ic.Runner.RunWithOutput(ctx, "tar", "-tzf", tarPath)
	if err != nil {
		return fmt.Errorf("list nvim tarball: %w", err)
	}
	rootDir := ""
	for _, line := range strings.Split(listOut, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if i := strings.Index(line, "/"); i > 0 {
			rootDir = line[:i]
			break
		}
	}
	if rootDir == "" || strings.ContainsAny(rootDir, "./\\") {
		return fmt.Errorf(
			"nvim tarball has no safe top-level directory: %q", rootDir,
		)
	}

	// Clean up old installs — propagate errors (permission/busy)
	// instead of silently proceeding into a broken layout.
	cleanupTargets := []string{
		"/opt/nvim",
		"/opt/nvim-linux-x86_64",
		"/opt/nvim-linux-arm64",
		"/opt/" + rootDir,
	}
	for _, old := range cleanupTargets {
		if err := ic.Runner.Run(
			ctx, "sudo", "rm", "-rf", old,
		); err != nil {
			return fmt.Errorf("clean old %s: %w", old, err)
		}
	}

	if err := ic.Runner.Run(ctx, "sudo", "tar", "-C", "/opt", "-xzf", tarPath); err != nil {
		return err
	}

	if err := ic.Runner.Run(
		ctx, "sudo", "rm", "-f", "/usr/local/bin/nvim",
	); err != nil {
		return fmt.Errorf("remove stale /usr/local/bin/nvim: %w", err)
	}
	return ic.Runner.Run(
		ctx, "sudo", "ln", "-s",
		"/opt/"+rootDir+"/bin/nvim",
		"/usr/local/bin/nvim",
	)
}

func installYaziApt(ctx context.Context, ic *InstallContext) error {
	// Install all companion packages in a single command. Errors
	// here are propagated — swallowing them masks dpkg-interrupted
	// and lock-held states that the caller's retry/classifier needs
	// to see. The yazi .deb install that follows would just fail
	// identically anyway.
	deps := []string{"ffmpeg", "p7zip-full", "jq", "poppler-utils", "imagemagick"}
	if err := ic.PkgMgr.Install(ctx, deps...); err != nil {
		return fmt.Errorf("yazi companion deps: %w", err)
	}

	// Download prebuilt deb from GitHub Releases.
	version, err := latestVersionFn("sxyazi/yazi", true)
	if err != nil {
		return fmt.Errorf("fetch yazi version: %w", err)
	}

	arch := "x86_64"
	if runtime.GOARCH == "arm64" {
		arch = "aarch64"
	}
	debName := fmt.Sprintf(
		"yazi-%s-unknown-linux-gnu.deb", arch,
	)
	url := fmt.Sprintf(
		"https://github.com/sxyazi/yazi/releases/download/v%s/%s",
		version, debName,
	)

	tmpDir, err := os.MkdirTemp("", "yazi-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	debPath := filepath.Join(tmpDir, debName)
	if err := ic.Runner.Run(
		ctx, "curl", "-fsSL", url, "-o", debPath,
	); err != nil {
		return fmt.Errorf("download yazi deb: %w", err)
	}

	return ic.Runner.Run(
		ctx, "sudo", "dpkg", "-i", debPath,
	)
}

// installTreeSitterCLI downloads the tree-sitter CLI binary from
// GitHub Releases. Assets use non-standard naming:
// tree-sitter-cli-{os}-{arch}.zip (os=macos/linux, arch=x64/arm64).
func installTreeSitterCLI(ctx context.Context, ic *InstallContext) error {
	osName := "linux"
	if runtime.GOOS == "darwin" {
		osName = "macos"
	}
	arch := "x64"
	if runtime.GOARCH == "arm64" {
		arch = "arm64"
	}

	zipName := fmt.Sprintf("tree-sitter-cli-%s-%s.zip", osName, arch)
	url := fmt.Sprintf(
		"https://github.com/tree-sitter/tree-sitter/releases/latest/download/%s",
		zipName,
	)

	tmpDir, err := os.MkdirTemp("", "tree-sitter-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	zipPath := filepath.Join(tmpDir, zipName)
	if err := ic.Runner.Run(
		ctx, "curl", "-fsSL", url, "-o", zipPath,
	); err != nil {
		return fmt.Errorf("download tree-sitter-cli: %w", err)
	}

	if err := ic.Runner.Run(
		ctx, "unzip", "-o", zipPath, "-d", tmpDir,
	); err != nil {
		return fmt.Errorf("extract tree-sitter-cli: %w", err)
	}

	binPath := filepath.Join(tmpDir, "tree-sitter")
	return ic.Runner.Run(
		ctx, "sudo", "install", "-m", "755", binPath,
		"/usr/local/bin/tree-sitter",
	)
}
