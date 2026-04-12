package registry

import (
	"context"

	"github.com/chaseddevelopment/dotfiles/installer/internal/executor"
	"github.com/chaseddevelopment/dotfiles/installer/internal/github"
	"github.com/chaseddevelopment/dotfiles/installer/internal/pkgmgr"
	"github.com/chaseddevelopment/dotfiles/installer/internal/platform"
)

// InstallMethod describes how a tool is installed.
type InstallMethod int

const (
	// MethodPackageManager installs via the system package manager.
	MethodPackageManager InstallMethod = iota
	// MethodCargo installs via cargo install.
	MethodCargo
	// MethodGitHubRelease downloads a binary from GitHub Releases.
	MethodGitHubRelease
	// MethodScript downloads and executes an install script.
	MethodScript
	// MethodGitClone clones a git repository.
	MethodGitClone
	// MethodCustom calls a custom install function.
	MethodCustom
)


// Tool describes a single installable tool with ordered strategies.
type Tool struct {
	Name        string
	Command     string // binary name to check in PATH
	Description string
	Critical    bool   // abort on failure vs warn and continue

	// IsInstalledFunc overrides the default exec.LookPath check.
	// Use for tools that aren't binaries in PATH (e.g., shell functions,
	// git repos, libraries).
	IsInstalledFunc func() bool

	// Strategies are tried in order; the first applicable one that
	// succeeds wins. Each strategy may be restricted to specific
	// package managers.
	Strategies []InstallStrategy

	// CargoCrate is the crate name used for cargo-based updates.
	CargoCrate string

	// OSFilter restricts this tool to specific operating systems.
	// Empty means install on all platforms.
	OSFilter []string // e.g., ["darwin"] or ["linux"]

	// DependsOn lists tool Command names that must be installed before
	// this tool. Used by the parallel install engine for DAG ordering.
	DependsOn []string

	// DesktopOnly skips this tool on headless servers (no DISPLAY or
	// WAYLAND_DISPLAY). Always true on macOS.
	DesktopOnly bool

	// MinVersion is the minimum required version (e.g. "0.12.0").
	// When set, the installer treats the tool as "not installed" if
	// the detected version is older than this threshold.
	MinVersion string

	// VersionArgs overrides the default ["--version"] arguments used
	// to query the tool's version.
	VersionArgs []string
}

// InstallStrategy describes one way to install a tool.
type InstallStrategy struct {
	// Managers restricts this strategy to specific package managers.
	// Nil/empty means this strategy works with any manager.
	Managers []string

	Method  InstallMethod
	Package string // override package name for MethodPackageManager

	// MethodCargo fields
	Crate string

	// MethodGitHubRelease fields
	GitHub *GitHubConfig

	// MethodScript fields
	Script *ScriptConfig

	// MethodGitClone fields
	GitClone *GitCloneConfig

	// MethodCustom fields
	CustomFunc func(ctx context.Context, ic *InstallContext) error

	// PostInstall actions run after a successful install.
	PostInstall []PostAction
}

// GitHubConfig is an alias re-exported from the github package.
type GitHubConfig = github.Config

// ScriptConfig holds parameters for script-based installs.
//
// NoProfileModify tells executeScript to invoke the upstream
// installer with env vars (PROFILE=/dev/null, SHELL=/bin/sh,
// INSTALLER_NO_MODIFY_PATH=1) that cause the installer's
// rc-file-append branch to no-op. Use for bun/nvm/uv/rustup-init/
// atuin/starship — every PATH export and init eval they'd write
// is already covered by configs/zsh/.zprofile and the _cached_init
// scheme in configs/zsh/.zshrc, so suppressing the append keeps
// the symlinked repo files clean.
type ScriptConfig struct {
	URL             string   // URL to download the install script
	Args            []string // arguments to pass to the script
	Shell           string   // "sh" or "bash" (default "sh")
	NoProfileModify bool     // inject opt-out env to block rc edits
}

// GitCloneConfig holds parameters for git clone installs.
type GitCloneConfig struct {
	Repo  string // full git URL
	Dest  string // target directory (supports $HOME)
	Depth int    // 0 = full clone
}

// PostActionType identifies a post-install action.
type PostActionType int

const (
	PostSymlink PostActionType = iota
	PostAddToPath
)

// PostAction describes an action to perform after install.
type PostAction struct {
	Type   PostActionType
	Source string
	Target string
}

// InstallContext provides shared state to install strategies.
type InstallContext struct {
	Runner         *executor.Runner
	PkgMgr         pkgmgr.PackageManager
	Platform       *platform.Platform
	ForceReinstall bool
}

// AppliesTo returns true if this strategy is valid for the given
// package manager name.
func (s *InstallStrategy) AppliesTo(mgrName string) bool {
	if len(s.Managers) == 0 {
		return true
	}
	for _, m := range s.Managers {
		if m == mgrName {
			return true
		}
	}
	return false
}
