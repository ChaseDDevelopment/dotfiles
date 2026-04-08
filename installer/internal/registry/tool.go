package registry

import (
	"context"

	"github.com/chaseddevelopment/dotfiles/installer/internal/executor"
	"github.com/chaseddevelopment/dotfiles/installer/internal/github"
	"github.com/chaseddevelopment/dotfiles/installer/internal/pkgmgr"
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
type ScriptConfig struct {
	URL   string   // URL to download the install script
	Args  []string // arguments to pass to the script
	Shell string   // "sh" or "bash" (default "sh")
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
	Runner *executor.Runner
	PkgMgr pkgmgr.PackageManager
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
