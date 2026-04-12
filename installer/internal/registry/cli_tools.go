package registry

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/chaseddevelopment/dotfiles/installer/internal/github"
)

func coreTools() []Tool {
	return []Tool{
		// Homebrew — must come first so brew is available for
		// subsequent tools on fresh macOS machines.
		{
			Name: "homebrew", Command: "brew",
			Description: "macOS package manager",
			Critical:    true,
			OSFilter:    []string{"darwin"},
			Strategies: []InstallStrategy{
				{Method: MethodScript, Script: &ScriptConfig{
					URL:   "https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh",
					Shell: "bash",
				}},
			},
		},
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
					Binary: "eza", StripVPrefix:true, LibC: "gnu",
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
					Binary: "delta", StripVPrefix:false, LibC: "musl",
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
					Binary: "lazygit", StripVPrefix:true,
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
					Binary: "xh", StripVPrefix:false, LibC: "musl",
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
		// Ghostty — GPU-accelerated terminal
		{
			Name: "ghostty", Command: "ghostty",
			Description: "GPU-accelerated terminal",
			DesktopOnly: true,
			Strategies: []InstallStrategy{
				{Managers: []string{"brew"}, Method: MethodCustom,
					CustomFunc: func(ctx context.Context, ic *InstallContext) error {
						return ic.Runner.Run(ctx, "brew", "install", "--cask", "ghostty")
					},
				},
				{Managers: []string{"pacman"}, Method: MethodPackageManager, Package: "ghostty"},
				{Managers: []string{"dnf"}, Method: MethodCustom,
					CustomFunc: func(ctx context.Context, ic *InstallContext) error {
						return ic.Runner.Run(ctx, "sudo", "dnf", "install", "-y", "ghostty")
					},
				},
			},
		},
		// gh — GitHub CLI
		{
			Name: "gh", Command: "gh", Description: "GitHub CLI",
			Strategies: []InstallStrategy{
				{Managers: []string{"brew"}, Method: MethodPackageManager, Package: "gh"},
				{Managers: []string{"pacman"}, Method: MethodPackageManager, Package: "github-cli"},
				{Managers: []string{"apt"}, Method: MethodCustom, CustomFunc: installGhCLI},
				{Managers: []string{"dnf", "yum"}, Method: MethodPackageManager, Package: "gh"},
			},
		},
		// jq — JSON processor
		{
			Name: "jq", Command: "jq", Description: "JSON processor",
			Strategies: []InstallStrategy{
				{Managers: []string{"brew"}, Method: MethodPackageManager, Package: "jq"},
				{Managers: []string{"apt"}, Method: MethodPackageManager, Package: "jq"},
				{Managers: []string{"pacman"}, Method: MethodPackageManager, Package: "jq"},
			},
		},
		// dust — modern disk usage analyzer
		{
			Name: "dust", Command: "dust", Description: "Modern disk usage analyzer",
			Strategies: []InstallStrategy{
				{Managers: []string{"brew"}, Method: MethodPackageManager, Package: "dust"},
				{Managers: []string{"pacman"}, Method: MethodPackageManager, Package: "dust"},
				{Method: MethodCargo, Crate: "du-dust"},
			},
			CargoCrate: "du-dust",
		},
		// btop — system monitor
		{
			Name: "btop", Command: "btop", Description: "System monitor",
			Strategies: []InstallStrategy{
				{Managers: []string{"brew"}, Method: MethodPackageManager, Package: "btop"},
				{Managers: []string{"apt"}, Method: MethodPackageManager, Package: "btop"},
				{Managers: []string{"pacman"}, Method: MethodPackageManager, Package: "btop"},
			},
		},
		// hyperfine — CLI benchmarking tool
		{
			Name: "hyperfine", Command: "hyperfine", Description: "CLI benchmarking tool",
			Strategies: []InstallStrategy{
				{Managers: []string{"brew"}, Method: MethodPackageManager, Package: "hyperfine"},
				{Managers: []string{"apt"}, Method: MethodPackageManager, Package: "hyperfine"},
				{Managers: []string{"pacman"}, Method: MethodPackageManager, Package: "hyperfine"},
				{Method: MethodCargo, Crate: "hyperfine"},
			},
			CargoCrate: "hyperfine",
		},
		// nala — prettier apt frontend
		{
			Name: "nala", Command: "nala", Description: "Prettier apt frontend",
			OSFilter: []string{"linux"},
			Strategies: []InstallStrategy{
				{Managers: []string{"apt"}, Method: MethodPackageManager, Package: "nala"},
			},
		},
		// ffmpeg — media processor
		{
			Name: "ffmpeg", Command: "ffmpeg", Description: "Media processor",
			Strategies: []InstallStrategy{
				{Managers: []string{"brew"}, Method: MethodPackageManager, Package: "ffmpeg"},
				{Managers: []string{"apt"}, Method: MethodPackageManager, Package: "ffmpeg"},
				{Managers: []string{"pacman"}, Method: MethodPackageManager, Package: "ffmpeg"},
			},
		},
		// imagemagick — image processor
		{
			Name: "imagemagick", Command: "magick", Description: "Image processor",
			Strategies: []InstallStrategy{
				{Managers: []string{"brew"}, Method: MethodPackageManager, Package: "imagemagick"},
				{Managers: []string{"apt"}, Method: MethodPackageManager, Package: "imagemagick"},
				{Managers: []string{"pacman"}, Method: MethodPackageManager, Package: "imagemagick"},
			},
		},
		// poppler — PDF rendering tools
		{
			Name: "poppler", Command: "pdftotext", Description: "PDF rendering tools",
			Strategies: []InstallStrategy{
				{Managers: []string{"brew"}, Method: MethodPackageManager, Package: "poppler"},
				{Managers: []string{"apt"}, Method: MethodPackageManager, Package: "poppler-utils"},
				{Managers: []string{"pacman"}, Method: MethodPackageManager, Package: "poppler"},
			},
		},
		// 7zip — archive tool
		{
			Name: "7zip", Command: "7zz", Description: "Archive tool",
			Strategies: []InstallStrategy{
				{Managers: []string{"brew"}, Method: MethodPackageManager, Package: "sevenzip"},
				{Managers: []string{"apt"}, Method: MethodPackageManager, Package: "7zip"},
				{Managers: []string{"pacman"}, Method: MethodPackageManager, Package: "7zip"},
			},
		},
		// Nerd Font — required for icons in eza, starship, tmux, yazi
		{
			Name: "nerd-font", Command: "nerd-font",
			Description: "JetBrains Mono Nerd Font",
			IsInstalledFunc: func() bool {
				return isNerdFontInstalled()
			},
			Strategies: []InstallStrategy{
				{Managers: []string{"brew"}, Method: MethodCustom,
					CustomFunc: func(ctx context.Context, ic *InstallContext) error {
						return ic.Runner.Run(ctx, "brew", "install", "--cask", "font-jetbrains-mono-nerd-font")
					},
				},
				{Method: MethodCustom, CustomFunc: installNerdFontLinux},
			},
		},
	}
}

func installGhCLI(ctx context.Context, ic *InstallContext) error {
	script := `type -p curl >/dev/null || (sudo apt update && sudo apt install curl -y)
curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg | sudo dd of=/usr/share/keyrings/githubcli-archive-keyring.gpg
sudo chmod go+r /usr/share/keyrings/githubcli-archive-keyring.gpg
echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" | sudo tee /etc/apt/sources.list.d/github-cli.list > /dev/null
sudo apt update
sudo apt install gh -y`
	return ic.Runner.RunShell(ctx, script)
}

func isNerdFontInstalled() bool {
	// macOS: check brew cask (authoritative — this is how we install it).
	if runtime.GOOS == "darwin" {
		if err := exec.Command("brew", "list", "--cask",
			"font-jetbrains-mono-nerd-font").Run(); err == nil {
			return true
		}
	}
	// Check font directories for JetBrains Nerd Font files.
	home := os.Getenv("HOME")
	for _, dir := range []string{
		filepath.Join(home, "Library", "Fonts"),
		filepath.Join(home, ".local", "share", "fonts"),
		filepath.Join(home, ".local", "share", "fonts", "NerdFonts"),
		"/usr/local/share/fonts",
		"/usr/share/fonts",
	} {
		matches, _ := filepath.Glob(
			filepath.Join(dir, "*JetBrains*Nerd*"),
		)
		if len(matches) > 0 {
			return true
		}
	}
	return false
}

func installNerdFontLinux(ctx context.Context, ic *InstallContext) error {
	version, err := github.LatestVersion("ryanoasis/nerd-fonts", true)
	if err != nil {
		return fmt.Errorf("resolve nerd-fonts latest version: %w", err)
	}
	url := fmt.Sprintf(
		"https://github.com/ryanoasis/nerd-fonts/releases/download/v%s/JetBrainsMono.tar.xz",
		version,
	)
	home := os.Getenv("HOME")
	fontDir := filepath.Join(home, ".local", "share", "fonts", "NerdFonts")
	if err := os.MkdirAll(fontDir, 0o755); err != nil {
		return fmt.Errorf("create font dir: %w", err)
	}

	tmpFile := filepath.Join(os.TempDir(), "JetBrainsMono.tar.xz")
	if err := ic.Runner.Run(
		ctx, "curl", "-fsSL", url, "-o", tmpFile,
	); err != nil {
		return fmt.Errorf("download nerd font: %w", err)
	}
	defer os.Remove(tmpFile)

	if err := ic.Runner.Run(
		ctx, "tar", "-xJf", tmpFile, "-C", fontDir,
	); err != nil {
		return fmt.Errorf("extract nerd font: %w", err)
	}

	// Refresh font cache.
	_ = ic.Runner.Run(ctx, "fc-cache", "-fv")
	return nil
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
