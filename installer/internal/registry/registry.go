package registry

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync"

	"github.com/chaseddevelopment/dotfiles/installer/internal/github"
	"github.com/chaseddevelopment/dotfiles/installer/internal/platform"
)

var (
	toolsOnce  sync.Once
	cachedAll  []Tool
	lookupMap  map[string]*Tool
)

func initTools() {
	toolsOnce.Do(func() {
		cachedAll = append(cachedAll, coreTools()...)
		cachedAll = append(cachedAll, rustToolchain()...)
		cachedAll = append(cachedAll, cliTools()...)
		cachedAll = append(cachedAll, devTools()...)
		cachedAll = append(cachedAll, officialInstallerTools()...)

		// Build lookup + panic on duplicate Command entries. Before
		// this check, the map silently last-wrote a duplicate and
		// Lookup returned whichever entry sorted last — impossible
		// to diagnose from symptoms. Bad registry wiring is now a
		// hard build failure (via tests) instead of a runtime
		// mystery.
		lookupMap = make(map[string]*Tool, len(cachedAll))
		for i := range cachedAll {
			cmd := cachedAll[i].Command
			if cmd == "" {
				continue
			}
			if existing, dup := lookupMap[cmd]; dup {
				panic(fmt.Sprintf(
					"registry: duplicate Command %q (tools %q and %q); "+
						"rename one of them",
					cmd, existing.Name, cachedAll[i].Name,
				))
			}
			lookupMap[cmd] = &cachedAll[i]
		}
	})
}

// AllTools returns every tool the installer manages, in install order.
func AllTools() []Tool {
	initTools()
	return cachedAll
}

// Lookup finds a tool by command name.
func Lookup(command string) *Tool {
	initTools()
	return lookupMap[command]
}

// ShouldInstall checks whether a tool applies to the current platform.
func ShouldInstall(t *Tool, p *platform.Platform) bool {
	if t.DesktopOnly && !p.IsDesktopEnvironment() {
		return false
	}
	if len(t.OSFilter) == 0 {
		return true
	}
	osStr, _ := p.GoStyle()
	for _, f := range t.OSFilter {
		if f == osStr {
			return true
		}
	}
	return false
}

// ToolStatus represents the result of checking a tool's install state.
type ToolStatus int

const (
	// StatusNotInstalled means the tool binary is not found.
	StatusNotInstalled ToolStatus = iota
	// StatusInstalled means the tool is present and meets version
	// requirements.
	StatusInstalled
	// StatusOutdated means the binary exists but the version is
	// below MinVersion.
	StatusOutdated
)

// IsInstalled checks if a tool is already present on the system
// and meets version requirements.
func IsInstalled(t *Tool) bool {
	return CheckInstalled(t) == StatusInstalled
}

// CheckInstalled returns the detailed installation status of a
// tool, distinguishing between not installed, installed, and
// outdated.
func CheckInstalled(t *Tool) ToolStatus {
	if t.IsInstalledFunc != nil {
		if !t.IsInstalledFunc() {
			return StatusNotInstalled
		}
		if t.MinVersion != "" && t.Command != "" {
			if _, err := exec.LookPath(t.Command); err == nil {
				if !CheckVersion(t) {
					return StatusOutdated
				}
			}
		}
		return StatusInstalled
	}
	if _, err := exec.LookPath(t.Command); err != nil {
		return StatusNotInstalled
	}
	if !CheckVersion(t) {
		return StatusOutdated
	}
	return StatusInstalled
}

// ExecuteInstall tries each strategy in order until one succeeds.
//
// Error classes are handled distinctly:
//   - A strategy failure (the install step itself didn't complete)
//     falls through to the next applicable strategy.
//   - A post-install failure (install succeeded but a follow-up step
//     like a symlink or PATH addition errored) is terminal for this
//     tool. Falling through would run the next strategy and
//     potentially overwrite a successful install with a second
//     binary of different provenance (e.g., brew-installed tool
//     clobbered by a GitHub release tarball).
func ExecuteInstall(ctx context.Context, t *Tool, ic *InstallContext, p *platform.Platform) error {
	mgrName := ic.PkgMgr.Name()

	var lastErr error
	for _, strategy := range t.Strategies {
		if !strategy.AppliesTo(mgrName) {
			continue
		}

		if err := executeStrategy(ctx, &strategy, ic, p); err != nil {
			lastErr = err
			ic.Runner.Log.Write(fmt.Sprintf(
				"Strategy %d failed for %s: %v, trying next",
				strategy.Method, t.Name, err,
			))
			continue
		}

		// Strategy succeeded. Post-install errors are terminal for
		// this tool — the binary is already in place, so falling
		// through would double-install.
		for _, pa := range strategy.PostInstall {
			if paErr := executePostAction(ctx, &pa, ic); paErr != nil {
				ic.Runner.Log.Write(fmt.Sprintf(
					"Strategy %d post-install failed for %s: %v",
					strategy.Method, t.Name, paErr,
				))
				return fmt.Errorf(
					"%s: post-install after strategy %d: %w",
					t.Name, strategy.Method, paErr,
				)
			}
		}
		return nil
	}
	if lastErr != nil {
		return fmt.Errorf("all install strategies failed for %s: %w", t.Name, lastErr)
	}
	return fmt.Errorf("no applicable install strategies for %s", t.Name)
}

func executeStrategy(ctx context.Context, s *InstallStrategy, ic *InstallContext, p *platform.Platform) error {
	switch s.Method {
	case MethodPackageManager:
		pkg := s.Package
		if pkg == "" {
			return fmt.Errorf("no package name specified")
		}
		return ic.PkgMgr.Install(ctx, pkg)

	case MethodCargo:
		return ic.Runner.Run(ctx, "cargo", "install", s.Crate)

	case MethodGitHubRelease:
		if s.GitHub == nil {
			return fmt.Errorf("missing GitHub config")
		}
		version := s.GitHub.PinVersion
		if version == "" {
			var err error
			version, err = github.LatestVersion(s.GitHub.Repo, s.GitHub.StripVPrefix)
			if err != nil {
				return err
			}
		}
		url, isTarball := github.BuildURL(s.GitHub, p, version)
		return github.DownloadAndInstall(ctx, url, s.GitHub.Binary, isTarball, ic.Runner)

	case MethodScript:
		if s.Script == nil {
			return fmt.Errorf("missing script config")
		}
		return executeScript(ctx, s.Script, ic)

	case MethodGitClone:
		if s.GitClone == nil {
			return fmt.Errorf("missing git clone config")
		}
		dest := os.ExpandEnv(s.GitClone.Dest)
		args := []string{"clone"}
		if s.GitClone.Depth > 0 {
			args = append(args, fmt.Sprintf("--depth=%d", s.GitClone.Depth))
		}
		args = append(args, s.GitClone.Repo, dest)
		return ic.Runner.Run(ctx, "git", args...)

	case MethodCustom:
		if s.CustomFunc == nil {
			return fmt.Errorf("missing custom function")
		}
		return s.CustomFunc(ctx, ic)

	default:
		return fmt.Errorf("unknown install method: %d", s.Method)
	}
}

func executeScript(
	ctx context.Context,
	cfg *ScriptConfig,
	ic *InstallContext,
) error {
	f, err := os.CreateTemp("", "dotsetup-script-*.sh")
	if err != nil {
		return fmt.Errorf("create temp script: %w", err)
	}
	tmpFile := f.Name()
	f.Close()
	defer os.Remove(tmpFile)

	if err := ic.Runner.Run(
		ctx, "curl", "-fsSL", cfg.URL, "-o", tmpFile,
	); err != nil {
		return fmt.Errorf("download script: %w", err)
	}

	shell := cfg.Shell
	if shell == "" {
		shell = "bash" // dash on Debian/Ubuntu can't handle bash syntax
	}
	args := make([]string, 0, len(cfg.Args)+1)
	args = append(args, tmpFile)
	for _, a := range cfg.Args {
		args = append(args, os.ExpandEnv(a))
	}
	if cfg.NoProfileModify {
		// Every upstream installer we register has its own knob
		// for disabling rc-file appends. We set all three knobs
		// unconditionally — each installer ignores the ones it
		// doesn't recognize, and that's cheaper than per-tool
		// branching.
		extraEnv := noProfileEnv()
		return ic.Runner.RunWithEnv(ctx, extraEnv, shell, args...)
	}
	return ic.Runner.Run(ctx, shell, args...)
}

// noProfileEnv returns the env vars that tell upstream tool
// installers not to edit the user's shell rc files. Exported via
// helper so both executeScript and the one-off installers in
// official_tools.go share the same policy.
func noProfileEnv() []string {
	return []string{
		"PROFILE=/dev/null",          // nvm
		"SHELL=/bin/sh",              // bun, atuin setup.sh
		"INSTALLER_NO_MODIFY_PATH=1", // uv (astral)
	}
}

func executePostAction(ctx context.Context, pa *PostAction, ic *InstallContext) error {
	switch pa.Type {
	case PostSymlink:
		src := os.ExpandEnv(pa.Source)
		tgt := os.ExpandEnv(pa.Target)
		if err := ic.Runner.Run(ctx, "sudo", "ln", "-sf", src, tgt); err != nil {
			return fmt.Errorf("post-install symlink %s -> %s: %w", src, tgt, err)
		}
	case PostAddToPath:
		// PATH additions are handled by the runner's Env.
		ic.Runner.AddEnv("PATH", os.ExpandEnv(pa.Target)+":"+os.Getenv("PATH"))
	}
	return nil
}
