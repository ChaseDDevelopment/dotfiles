package registry

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/chaseddevelopment/dotfiles/installer/internal/github"
)

func coreTools() []Tool {
	return []Tool{
		{Name: "git", Command: "git", Critical: true, Strategies: []InstallStrategy{
			{Method: MethodPackageManager, Package: "git"},
		}},
		{Name: "curl", Command: "curl", Critical: true, Strategies: []InstallStrategy{
			{Method: MethodPackageManager, Package: "curl"},
		}},
		{Name: "wget", Command: "wget", Strategies: []InstallStrategy{
			{Method: MethodPackageManager, Package: "wget"},
		}},
		{Name: "unzip", Command: "unzip", Strategies: []InstallStrategy{
			{Method: MethodPackageManager, Package: "unzip"},
		}},
		{Name: "build-essential", Command: "make", OSFilter: []string{"linux"}, Strategies: []InstallStrategy{
			{Method: MethodPackageManager, Package: "build-essential"},
		}},
		{Name: "zsh", Command: "zsh", Critical: true, Strategies: []InstallStrategy{
			{Method: MethodPackageManager, Package: "zsh"},
		}},
		{Name: "tmux", Command: "tmux", Critical: true, Strategies: []InstallStrategy{
			{Method: MethodPackageManager, Package: "tmux"},
		}},
		{Name: "fzf", Command: "fzf", Strategies: []InstallStrategy{
			{Method: MethodPackageManager, Package: "fzf"},
		}},
		{
			Name: "powerline-fonts", Command: "powerline",
			OSFilter: []string{"linux"},
			Strategies: []InstallStrategy{
				{Managers: []string{"apt"}, Method: MethodPackageManager, Package: "fonts-powerline"},
			},
		},
	}
}

func cliTools() []Tool {
	return []Tool{
		// eza — modern ls
		{
			Name: "eza", Command: "eza", Description: "Modern ls replacement",
			Strategies: []InstallStrategy{
				{Managers: []string{"brew"}, Method: MethodPackageManager, Package: "eza"},
				{Managers: []string{"pacman"}, Method: MethodPackageManager, Package: "eza"},
				{Method: MethodGitHubRelease, GitHub: &GitHubConfig{
					Repo: "eza-community/eza", Pattern: github.PatternTargetTriple,
					Binary: "eza", StripV: true, LibC: "gnu",
				}},
				{Managers: []string{"apt", "dnf", "yum"}, Method: MethodCargo, Crate: "eza"},
			},
			CargoCrate: "eza",
		},
		// bat — modern cat
		{
			Name: "bat", Command: "bat", Description: "Cat with syntax highlighting",
			Strategies: []InstallStrategy{
				{Managers: []string{"brew", "dnf", "yum", "pacman"}, Method: MethodPackageManager, Package: "bat"},
				{Managers: []string{"apt"}, Method: MethodPackageManager, Package: "bat",
					PostInstall: []PostAction{
						{Type: PostSymlink, Source: "/usr/bin/batcat", Target: "/usr/local/bin/bat"},
					},
				},
			},
		},
		// ripgrep — modern grep
		{
			Name: "ripgrep", Command: "rg", Description: "Fast recursive grep",
			Strategies: []InstallStrategy{
				{Method: MethodPackageManager, Package: "ripgrep"},
			},
		},
		// fd — modern find
		{
			Name: "fd", Command: "fd", Description: "Fast file finder",
			Strategies: []InstallStrategy{
				{Managers: []string{"brew", "pacman"}, Method: MethodPackageManager, Package: "fd"},
				{Managers: []string{"apt"}, Method: MethodPackageManager, Package: "fd-find",
					PostInstall: []PostAction{
						{Type: PostSymlink, Source: "/usr/bin/fdfind", Target: "/usr/local/bin/fd"},
					},
				},
				{Managers: []string{"dnf", "yum"}, Method: MethodPackageManager, Package: "fd-find"},
			},
		},
		// zoxide — modern cd
		{
			Name: "zoxide", Command: "zoxide", Description: "Smarter cd command",
			Strategies: []InstallStrategy{
				{Managers: []string{"brew", "pacman"}, Method: MethodPackageManager, Package: "zoxide"},
				{Managers: []string{"apt", "dnf"}, Method: MethodPackageManager, Package: "zoxide"},
				{Method: MethodScript, Script: &ScriptConfig{
					URL: "https://raw.githubusercontent.com/ajeetdsouza/zoxide/main/install.sh",
				}},
			},
		},
		// tailspin — pretty log viewer
		{
			Name: "tailspin", Command: "tspin", Description: "Pretty log viewer",
			Strategies: []InstallStrategy{
				{Managers: []string{"brew"}, Method: MethodPackageManager, Package: "tailspin"},
				{Managers: []string{"pacman"}, Method: MethodPackageManager, Package: "tailspin"},
				{Method: MethodCustom, CustomFunc: installTailspin},
				{Method: MethodCargo, Crate: "tailspin"},
			},
			CargoCrate: "tailspin",
		},
		// delta — syntax-highlighted git diffs
		{
			Name: "delta", Command: "delta", Description: "Syntax-highlighted diffs",
			Strategies: []InstallStrategy{
				{Managers: []string{"brew"}, Method: MethodPackageManager, Package: "git-delta"},
				{Managers: []string{"pacman"}, Method: MethodPackageManager, Package: "git-delta"},
				{Managers: []string{"apt"}, Method: MethodPackageManager, Package: "git-delta"},
				{Managers: []string{"dnf", "yum"}, Method: MethodPackageManager, Package: "git-delta"},
				{Method: MethodGitHubRelease, GitHub: &GitHubConfig{
					Repo: "dandavison/delta", Pattern: github.PatternVersionPrefixed,
					Binary: "delta", StripV: false, LibC: "musl",
				}},
				{Method: MethodCargo, Crate: "git-delta"},
			},
			CargoCrate: "git-delta",
		},
		// lazygit — TUI git client
		{
			Name: "lazygit", Command: "lazygit", Description: "Terminal UI for git",
			Strategies: []InstallStrategy{
				{Managers: []string{"brew", "pacman"}, Method: MethodPackageManager, Package: "lazygit"},
				{Method: MethodGitHubRelease, GitHub: &GitHubConfig{
					Repo: "jesseduffield/lazygit", Pattern: github.PatternCustomOSArch,
					Binary: "lazygit", StripV: true,
				}},
			},
		},
		// xh — modern HTTP client
		{
			Name: "xh", Command: "xh", Description: "Friendly HTTP client",
			Strategies: []InstallStrategy{
				{Managers: []string{"brew", "pacman"}, Method: MethodPackageManager, Package: "xh"},
				{Method: MethodGitHubRelease, GitHub: &GitHubConfig{
					Repo: "ducaale/xh", Pattern: github.PatternVersionPrefixed,
					Binary: "xh", StripV: false, LibC: "musl",
				}},
				{Managers: []string{"apt", "dnf", "yum"}, Method: MethodCargo, Crate: "xh"},
			},
			CargoCrate: "xh",
		},
		// yq — YAML processor
		{
			Name: "yq", Command: "yq", Description: "YAML processor",
			Strategies: []InstallStrategy{
				{Managers: []string{"brew", "pacman"}, Method: MethodPackageManager, Package: "yq"},
				{Method: MethodGitHubRelease, GitHub: &GitHubConfig{
					Repo: "mikefarah/yq", Pattern: github.PatternRawBinary,
					Binary: "yq",
				}},
			},
		},
		// direnv — per-project env vars
		{
			Name: "direnv", Command: "direnv", Description: "Per-directory environment",
			Strategies: []InstallStrategy{
				{Method: MethodPackageManager, Package: "direnv"},
			},
		},
		// coreutils — GNU coreutils for macOS
		{
			Name: "coreutils", Command: "grm", Description: "GNU coreutils",
			OSFilter: []string{"darwin"},
			Strategies: []InstallStrategy{
				{Managers: []string{"brew"}, Method: MethodPackageManager, Package: "coreutils"},
			},
		},
		// clipboard utilities — Linux only
		{
			Name: "xclip", Command: "xclip", Description: "X11 clipboard",
			OSFilter: []string{"linux"},
			Strategies: []InstallStrategy{
				{Method: MethodPackageManager, Package: "xclip"},
			},
		},
		{
			Name: "wl-clipboard", Command: "wl-copy", Description: "Wayland clipboard",
			OSFilter: []string{"linux"},
			Strategies: []InstallStrategy{
				{Method: MethodPackageManager, Package: "wl-clipboard"},
			},
		},
	}
}

// installTailspin downloads the tailspin binary from GitHub Releases.
// Asset naming uses "tailspin-{triple}" (dash separator, project name)
// which doesn't match any standard URL pattern.
func installTailspin(ctx context.Context, ic *InstallContext) error {
	arch := "x86_64"
	if runtime.GOARCH == "arm64" {
		arch = "aarch64"
	}
	var triple string
	if runtime.GOOS == "darwin" {
		triple = arch + "-apple-darwin"
	} else {
		triple = arch + "-unknown-linux-musl"
	}
	url := fmt.Sprintf(
		"https://github.com/bensadeh/tailspin/releases/latest/download/tailspin-%s.tar.gz",
		triple,
	)

	tmpDir, err := os.MkdirTemp("", "tailspin-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	tarPath := filepath.Join(tmpDir, "tailspin.tar.gz")
	if err := ic.Runner.Run(
		ctx, "curl", "-fsSL", url, "-o", tarPath,
	); err != nil {
		return fmt.Errorf("download tailspin: %w", err)
	}

	if err := ic.Runner.Run(
		ctx, "tar", "-xzf", tarPath, "-C", tmpDir,
	); err != nil {
		return fmt.Errorf("extract tailspin: %w", err)
	}

	binPath := filepath.Join(tmpDir, "tspin")
	return ic.Runner.Run(
		ctx, "sudo", "install", "-m", "755", binPath,
		"/usr/local/bin/tspin",
	)
}
