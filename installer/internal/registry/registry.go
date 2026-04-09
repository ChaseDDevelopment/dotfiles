package registry

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/chaseddevelopment/dotfiles/installer/internal/github"
	"github.com/chaseddevelopment/dotfiles/installer/internal/platform"
)

// AllTools returns every tool the installer manages, in install order.
func AllTools() []Tool {
	var all []Tool
	all = append(all, coreTools()...)
	all = append(all, rustToolchain()...)
	all = append(all, cliTools()...)
	all = append(all, devTools()...)
	all = append(all, officialInstallerTools()...)
	return all
}

// Lookup finds a tool by command name.
func Lookup(command string) *Tool {
	for _, t := range AllTools() {
		if t.Command == command {
			return &t
		}
	}
	return nil
}

// ShouldInstall checks whether a tool applies to the current platform.
func ShouldInstall(t *Tool, p *platform.Platform) bool {
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

// IsInstalled checks if a tool is already present on the system.
// Uses IsInstalledFunc if set, otherwise falls back to exec.LookPath.
func IsInstalled(t *Tool) bool {
	if t.IsInstalledFunc != nil {
		return t.IsInstalledFunc()
	}
	_, err := exec.LookPath(t.Command)
	return err == nil
}

// ExecuteInstall tries each strategy in order until one succeeds.
func ExecuteInstall(ctx context.Context, t *Tool, ic *InstallContext, p *platform.Platform) error {
	mgrName := ic.PkgMgr.Name()

	for _, strategy := range t.Strategies {
		if !strategy.AppliesTo(mgrName) {
			continue
		}

		err := executeStrategy(ctx, &strategy, ic, p)
		if err == nil {
			for _, pa := range strategy.PostInstall {
				if paErr := executePostAction(ctx, &pa, ic); paErr != nil {
					ic.Runner.Log.Write(fmt.Sprintf("WARNING: %v", paErr))
				}
			}
			return nil
		}
		ic.Runner.Log.Write(fmt.Sprintf(
			"Strategy %d failed for %s: %v, trying next",
			strategy.Method, t.Name, err,
		))
	}
	return fmt.Errorf("all install strategies failed for %s", t.Name)
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
		version, err := github.LatestVersion(s.GitHub.Repo, s.GitHub.StripV)
		if err != nil {
			return err
		}
		url, isTarball := github.BuildURL(s.GitHub, p, version)
		return github.DownloadAndInstall(ctx, url, s.GitHub.Binary, isTarball)

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
		shell = "sh"
	}
	args := append([]string{tmpFile}, cfg.Args...)
	return ic.Runner.Run(ctx, shell, args...)
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
