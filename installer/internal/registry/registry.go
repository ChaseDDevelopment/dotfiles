package registry

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	"github.com/chaseddevelopment/dotfiles/installer/internal/github"
	"github.com/chaseddevelopment/dotfiles/installer/internal/pkgmgr"
	"github.com/chaseddevelopment/dotfiles/installer/internal/platform"
)

var (
	toolsOnce sync.Once
	cachedAll []Tool
	lookupMap map[string]*Tool
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
	return executeInstallFiltered(ctx, t, ic, p, nil)
}

// ExecuteInstallSkippingPkgMgr runs the strategy loop but skips any
// MethodPackageManager entries. Used by the orchestrator's batch
// fan-out when the pkgmgr install already ran (successfully or not)
// as part of a per-manager batch task — re-running it per tool would
// duplicate work for successes and re-fail identically for failures.
func ExecuteInstallSkippingPkgMgr(ctx context.Context, t *Tool, ic *InstallContext, p *platform.Platform) error {
	return executeInstallFiltered(ctx, t, ic, p, func(s *InstallStrategy) bool {
		return s.Method == MethodPackageManager
	})
}

// dpkgHealer is the capability interface registry uses to trigger a
// dpkg repair between strategy attempts when the underlying error
// is a recoverable dpkg state. Only *pkgmgr.Apt implements it.
type dpkgHealer interface {
	RunDpkgConfigureAll(ctx context.Context) error
}

// executeInstallFiltered is the shared strategy-loop body with an
// optional skip predicate.
//
// Error classification (Phase A4 of the robustness plan):
//
//   - errors.Is(err, pkgmgr.ErrDpkgInterrupted) or ErrDpkgLocked:
//     attempt a one-shot repair via the PackageManager's healer
//     capability, then retry the SAME strategy exactly once.
//     A per-strategy attempt counter caps retries so a persistently
//     broken dpkg surfaces as terminal rather than looping.
//   - errors.Is(err, pkgmgr.ErrAptFatal): FAIL the tool immediately.
//     Falling through to cargo/GitHub on fatal apt conditions
//     (held packages, hash mismatch, …) silently changes the
//     user's binary provenance — they picked apt, they get apt,
//     or they get a diagnostic.
//   - Any other error: preserve existing fallthrough to the next
//     applicable strategy.
func executeInstallFiltered(
	ctx context.Context,
	t *Tool,
	ic *InstallContext,
	p *platform.Platform,
	skip func(*InstallStrategy) bool,
) error {
	mgrName := ic.PkgMgr.Name()

	var lastErr error
	for i := range t.Strategies {
		strategy := t.Strategies[i]
		if !strategy.AppliesTo(mgrName) {
			continue
		}
		if skip != nil && skip(&strategy) {
			continue
		}

		var err error
		for attempt := 1; attempt <= 2; attempt++ {
			err = executeStrategy(ctx, &strategy, ic, p)
			if err == nil {
				break
			}
			// Recoverable dpkg state → heal + retry once.
			if attempt == 1 && isRecoverableDpkg(err) {
				if healer, ok := ic.PkgMgr.(dpkgHealer); ok {
					ic.Runner.Log.Write(fmt.Sprintf(
						"Strategy %d hit recoverable dpkg error for %s: %v; invoking dpkg doctor",
						strategy.Method, t.Name, err,
					))
					if healErr := healer.RunDpkgConfigureAll(ctx); healErr != nil {
						ic.Runner.Log.Write(fmt.Sprintf(
							"dpkg repair failed for %s: %v",
							t.Name, healErr,
						))
						break
					}
					continue // retry same strategy
				}
			}
			break
		}

		if err != nil {
			lastErr = err
			// ErrAptFatal is terminal — no fallthrough.
			if errors.Is(err, pkgmgr.ErrAptFatal) {
				return fmt.Errorf(
					"%s: apt reported a fatal condition; "+
						"not attempting fallback strategies: %w",
					t.Name, err,
				)
			}
			ic.Runner.Log.Write(fmt.Sprintf(
				"Strategy %d failed for %s: %v, trying next",
				strategy.Method, t.Name, err,
			))
			continue
		}

		// Strategy succeeded. Post-install errors are terminal for
		// this tool — the binary is already in place, so falling
		// through would double-install.
		if err := RunPostInstall(ctx, &strategy, ic); err != nil {
			ic.Runner.Log.Write(fmt.Sprintf(
				"Strategy %d post-install failed for %s: %v",
				strategy.Method, t.Name, err,
			))
			return fmt.Errorf(
				"%s: post-install after strategy %d: %w",
				t.Name, strategy.Method, err,
			)
		}
		return nil
	}
	if lastErr != nil {
		return fmt.Errorf("all install strategies failed for %s: %w", t.Name, lastErr)
	}
	return fmt.Errorf("no applicable install strategies for %s", t.Name)
}

// isRecoverableDpkg reports whether err indicates a dpkg state the
// doctor can repair (interrupted transaction or held lock) —
// distinct from fatal apt conditions (unmet deps, hash mismatch)
// which require human intervention.
func isRecoverableDpkg(err error) bool {
	return errors.Is(err, pkgmgr.ErrDpkgInterrupted) ||
		errors.Is(err, pkgmgr.ErrDpkgLocked)
}

// RunPostInstall runs every PostAction declared on the strategy.
// Exposed so the orchestrator's batch fan-out can invoke post-install
// for each tool in a successfully batched bucket without re-running
// the strategy itself.
func RunPostInstall(ctx context.Context, s *InstallStrategy, ic *InstallContext) error {
	for _, pa := range s.PostInstall {
		if err := executePostAction(ctx, &pa, ic); err != nil {
			return err
		}
	}
	return nil
}

// FirstPkgMgrStrategy returns the first strategy applicable under
// mgrName whose method is MethodPackageManager, or nil if the tool
// has no batch-eligible strategy. Used by the orchestrator to
// partition tools into per-manager batch buckets.
func FirstPkgMgrStrategy(t *Tool, mgrName string) *InstallStrategy {
	for i := range t.Strategies {
		s := &t.Strategies[i]
		if !s.AppliesTo(mgrName) {
			continue
		}
		if s.Method == MethodPackageManager {
			return s
		}
		// A non-pkgmgr strategy comes first — not batch-eligible.
		return nil
	}
	return nil
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
		return ic.Runner.Run(ctx, resolveCargo(), "install", s.Crate)

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
	return executeScriptFile(ctx, cfg, tmpFile, ic)
}

func executeScriptFile(
	ctx context.Context,
	cfg *ScriptConfig,
	scriptPath string,
	ic *InstallContext,
) error {
	shell := cfg.Shell
	if shell == "" {
		shell = "bash" // dash on Debian/Ubuntu can't handle bash syntax
	}
	args := make([]string, 0, len(cfg.Args)+1)
	args = append(args, scriptPath)
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

// resolveCargo returns the cargo binary to exec. It prefers PATH,
// falling back to $HOME/.cargo/bin/cargo — rustup installs there
// regardless of --no-modify-path, so the binary exists even when
// the installer's PATH was snapshotted (in main.augmentPath) before
// rust's install strategy ran in this session.
func resolveCargo() string {
	if p, err := exec.LookPath("cargo"); err == nil {
		return p
	}
	if home, err := os.UserHomeDir(); err == nil {
		cand := filepath.Join(home, ".cargo", "bin", "cargo")
		if _, err := os.Stat(cand); err == nil {
			return cand
		}
	}
	return "cargo" // let os/exec produce the real error
}
