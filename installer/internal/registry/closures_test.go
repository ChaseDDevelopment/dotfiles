package registry

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/chaseddevelopment/dotfiles/installer/internal/executor"
	"github.com/chaseddevelopment/dotfiles/installer/internal/platform"
)

// stubBin writes a shell stub into dir that logs invocation and
// exits 0. The stub tolerates "-o DEST" so curl-style commands
// materialize an empty file at DEST.
func stubBin(t *testing.T, dir, name, logEnv string) {
	t.Helper()
	body := `#!/bin/sh
printf '` + name + ` %s\n' "$*" >> "$` + logEnv + `"
dest=""
prev=""
for arg in "$@"; do
  if [ "$prev" = "-o" ] || [ "$prev" = "-O" ]; then
    dest="$arg"
  fi
  prev="$arg"
done
if [ -n "$dest" ]; then
  : > "$dest" 2>/dev/null || true
fi
exit 0
`
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

// setupClosureEnv sets up an InstallContext and PATH full of stubs
// so every tool closure can run its happy path without real shelling.
func setupClosureEnv(t *testing.T, mgrName string) (*InstallContext, string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	bin := filepath.Join(home, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	logFile := filepath.Join(home, "stubs.log")
	t.Setenv("CLOSURE_LOG", logFile)

	// Every command a closure might call.
	for _, name := range []string{
		"brew", "pacman", "yay", "paru", "sudo", "cargo", "curl",
		"tar", "bash", "sh", "git", "dpkg", "tree-sitter", "uv",
		"unzip", "fc-cache", "ln", "chmod", "install", "dd",
		"dnf", "apt-get", "nala", "tee", "go",
	} {
		stubBin(t, bin, name, "CLOSURE_LOG")
	}

	log, err := executor.NewLogFile(filepath.Join(home, "test.log"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { log.Close() })

	return &InstallContext{
		Runner:   executor.NewRunner(log, false),
		PkgMgr:   &stubPkgMgr{name: mgrName},
		Platform: &platform.Platform{OS: platform.Linux, Arch: platform.AMD64},
	}, home
}

// TestInlineCustomClosures_InvokeExpectedCommands replaces the former
// fire-and-forget "exercise every closure" test. Registry catalogs
// embed inline CustomFunc lambdas (e.g. neovim-brew, ghostty-brew,
// ghostty-dnf, yazi-pacman, yazi-brew, nerd-font-brew); this test
// locks in the exact argv those closures invoke via ic.Runner. Named
// closures (installGhCLI, installNvm, installTPM, installAtuin,
// installNeovimPacman, installYaziApt, installTailspin,
// installTreeSitterCLI, installNerdFontLinux, InstallNeovimApt) are
// covered per-closure in installers_test.go and closures_test.go's
// other tests — this one only exercises what was previously tested
// with discarded errors.
//
// Each case declares:
//   - mgr: the pkgmgr name so inline lambdas whose surrounding strategy
//     has Managers: []string{"brew"} are dispatched under the right
//     context.
//   - cmd: the exact binary the closure must invoke (PATH stubs
//     record a single line per invocation).
//   - wantArgs: the trailing substring the log line must contain —
//     captures argv without locking in the full "brew %s" prefix.
func TestInlineCustomClosures_InvokeExpectedCommands(t *testing.T) {
	type closureCase struct {
		tool     string
		mgr      string
		findFn   func(tools []Tool) func(context.Context, *InstallContext) error
		wantArgs string // exact trailing substring expected in the log
	}

	pickCustom := func(toolName, mgr string) func(tools []Tool) func(context.Context, *InstallContext) error {
		return func(tools []Tool) func(context.Context, *InstallContext) error {
			for _, tool := range tools {
				if tool.Name != toolName {
					continue
				}
				for i := range tool.Strategies {
					s := &tool.Strategies[i]
					if s.Method != MethodCustom || s.CustomFunc == nil {
						continue
					}
					if !s.AppliesTo(mgr) {
						continue
					}
					return s.CustomFunc
				}
			}
			return nil
		}
	}

	cases := []closureCase{
		{
			tool:     "neovim",
			mgr:      "brew",
			findFn:   pickCustom("neovim", "brew"),
			wantArgs: "brew install --HEAD neovim",
		},
		{
			tool:     "yazi",
			mgr:      "brew",
			findFn:   pickCustom("yazi", "brew"),
			wantArgs: "brew install yazi ffmpeg sevenzip jq poppler resvg imagemagick",
		},
		{
			tool:     "yazi",
			mgr:      "pacman",
			findFn:   pickCustom("yazi", "pacman"),
			wantArgs: "sudo pacman -S --noconfirm yazi ffmpeg 7zip jq poppler resvg imagemagick",
		},
		{
			tool:     "ghostty",
			mgr:      "brew",
			findFn:   pickCustom("ghostty", "brew"),
			wantArgs: "brew install --cask ghostty",
		},
		{
			tool:     "ghostty",
			mgr:      "dnf",
			findFn:   pickCustom("ghostty", "dnf"),
			wantArgs: "sudo dnf install -y ghostty",
		},
		{
			tool:     "nerd-font",
			mgr:      "brew",
			findFn:   pickCustom("nerd-font", "brew"),
			wantArgs: "brew install --cask font-jetbrains-mono-nerd-font",
		},
		{
			tool:     "ruff",
			mgr:      "brew", // no Managers filter, matches any
			findFn:   pickCustom("ruff", "brew"),
			wantArgs: "uv tool install ruff",
		},
		{
			tool:     "gopls",
			mgr:      "brew", // no Managers filter, matches any
			findFn:   pickCustom("gopls", "brew"),
			wantArgs: "go install golang.org/x/tools/gopls@latest",
		},
	}

	toolsByCat := [][]Tool{cliTools(), devTools(), officialInstallerTools()}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.tool+"/"+tc.mgr, func(t *testing.T) {
			ic, home := setupClosureEnv(t, tc.mgr)

			var fn func(context.Context, *InstallContext) error
			for _, cat := range toolsByCat {
				if got := tc.findFn(cat); got != nil {
					fn = got
					break
				}
			}
			if fn == nil {
				t.Fatalf("no inline CustomFunc for %s under %s",
					tc.tool, tc.mgr)
			}

			if err := fn(context.Background(), ic); err != nil {
				t.Fatalf("closure returned error: %v", err)
			}

			logBytes, err := os.ReadFile(filepath.Join(home, "stubs.log"))
			if err != nil {
				t.Fatalf("read stub log: %v", err)
			}
			got := string(logBytes)
			if !strings.Contains(got, tc.wantArgs) {
				t.Fatalf(
					"stub log missing expected argv.\n want: %q\n got log:\n%s",
					tc.wantArgs, got,
				)
			}
		})
	}
}

// TestAllCatalogCustomFuncs_AreReachable is a compile-time-ish guard
// that every Tool's CustomFunc is either (a) a named package-level
// function covered by a dedicated test in installers_test.go /
// closures_test.go, or (b) an inline anonymous lambda whose tool
// appears in the inlineCoveredTools set and is asserted by
// TestInlineCustomClosures_InvokeExpectedCommands above.
//
// When a new CustomFunc is added to the catalog without matching
// coverage, this test fails and tells the author to extend the
// table — preventing the class of drift that produced the original
// fire-and-forget "exercise every closure" test.
func TestAllCatalogCustomFuncs_AreReachable(t *testing.T) {
	// Named package-level helpers with their own dedicated tests.
	// Detected by pulling the function's runtime name via reflection;
	// anonymous closures render as "<pkg>.<parent>.func1" and fail
	// this match, while named ones render as "<pkg>.installXxx".
	namedCovered := map[string]struct{}{
		"installNvm":           {},
		"installAtuin":         {},
		"installTPM":           {},
		"installGhCLI":         {},
		"installTailspin":      {},
		"installTreeSitterCLI": {},
		"installNeovimPacman":  {},
		"InstallNeovimApt":     {},
		"installYaziApt":       {},
		"installNerdFontLinux":    {},
		"installJlessFromSource":  {},
	}
	// Tools whose anonymous closures are asserted by
	// TestInlineCustomClosures_InvokeExpectedCommands above.
	inlineCoveredTools := map[string]struct{}{
		"neovim":    {},
		"yazi":      {},
		"ghostty":   {},
		"nerd-font": {},
		"ruff":      {},
		"gopls":     {},
	}

	for _, cat := range [][]Tool{
		cliTools(), devTools(), officialInstallerTools(),
	} {
		for _, tool := range cat {
			for _, s := range tool.Strategies {
				if s.Method != MethodCustom || s.CustomFunc == nil {
					continue
				}
				fnName := runtimeFuncName(s.CustomFunc)
				if _, ok := namedCovered[fnName]; ok {
					continue
				}
				if _, ok := inlineCoveredTools[tool.Name]; ok {
					continue
				}
				t.Errorf(
					"tool %q has an un-covered CustomFunc (%s, "+
						"managers=%v); add it to "+
						"TestInlineCustomClosures_InvokeExpectedCommands "+
						"or give it a dedicated test",
					tool.Name, fnName, s.Managers,
				)
			}
		}
	}
}

// runtimeFuncName returns the short name of a function value via
// reflection, stripping the package path. Anonymous closures appear
// as "<parent>.func<N>" which deliberately doesn't match named-helper
// entries in namedCovered.
func runtimeFuncName(fn any) string {
	rv := reflect.ValueOf(fn)
	if rv.Kind() != reflect.Func {
		return ""
	}
	rf := runtime.FuncForPC(rv.Pointer())
	if rf == nil {
		return ""
	}
	full := rf.Name()
	if i := strings.LastIndex(full, "."); i >= 0 {
		return full[i+1:]
	}
	return full
}

// TestIsInstalledFuncs_HappyPath exercises every inline IsInstalledFunc
// in the catalog against a prepared HOME with the tool's detection
// target planted. Previously these were called with `_ = fn()` and
// the bool was discarded — a regression that flipped true/false would
// have been invisible. Now we set the detection substrate and assert
// true.
//
// Tools that can only be detected via a real binary (e.g. pkg-config
// for tree-sitter-lib) are tested via their named helpers elsewhere;
// nvm, tpm, and nerd-font each have a filesystem-based probe easily
// simulated with a planted directory/file.
func TestIsInstalledFuncs_HappyPath(t *testing.T) {
	cases := []struct {
		tool  string
		prep  func(t *testing.T, home string)
		slice func() []Tool
	}{
		{
			tool:  "nvm",
			slice: officialInstallerTools,
			prep: func(t *testing.T, home string) {
				dir := filepath.Join(home, ".config", "nvm")
				if err := os.MkdirAll(dir, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(
					filepath.Join(dir, "nvm.sh"),
					[]byte("# stub"), 0o644,
				); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			tool:  "tpm",
			slice: officialInstallerTools,
			prep: func(t *testing.T, home string) {
				if err := os.MkdirAll(
					filepath.Join(home, ".tmux", "plugins", "tpm"),
					0o755,
				); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			tool:  "nerd-font",
			slice: cliTools,
			prep: func(t *testing.T, home string) {
				fontDir := filepath.Join(
					home, ".local", "share", "fonts", "NerdFonts",
				)
				if err := os.MkdirAll(fontDir, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(
					filepath.Join(fontDir, "JetBrainsMono-Nerd.ttf"),
					[]byte("font"), 0o644,
				); err != nil {
					t.Fatal(err)
				}
			},
		},
	}

	// tree-sitter-lib's IsInstalledFunc tries pkg-config + filesystem
	// probes at /opt/homebrew/... paths we can't plant inside
	// t.TempDir without root. It's intentionally skipped — documented
	// here, not fake-covered.

	for _, tc := range cases {
		tc := tc
		t.Run(tc.tool, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			tc.prep(t, home)

			var fn func() bool
			for _, tool := range tc.slice() {
				if tool.Name == tc.tool && tool.IsInstalledFunc != nil {
					fn = tool.IsInstalledFunc
					break
				}
			}
			if fn == nil {
				t.Fatalf("tool %q has no IsInstalledFunc", tc.tool)
			}
			if !fn() {
				t.Fatalf("expected IsInstalledFunc(%q)=true after prep",
					tc.tool)
			}
		})
	}
}

// TestInstallNeovimPacmanFallsBackToSudo drives the "yay -S failed,
// try next helper, then fall back to sudo pacman" branch, which the
// happy-path stubs can't reach.
func TestInstallNeovimPacmanFallsBackToSudo(t *testing.T) {
	ic, home := setupClosureEnv(t, "pacman")
	bin := filepath.Join(home, "bin")
	// Make yay and paru return non-zero so the loop continues.
	body := `#!/bin/sh
printf '%s %s\n' "` + "FAIL" + `" "$*" >> "$CLOSURE_LOG"
exit 1
`
	for _, name := range []string{"yay", "paru"} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// sudo still succeeds; that's the fallback path.
	if err := installNeovimPacman(context.Background(), ic); err != nil {
		t.Fatalf("installNeovimPacman: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(home, "stubs.log"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	if !strings.Contains(got, "sudo pacman -S --noconfirm neovim") {
		t.Fatalf("expected sudo pacman fallback to run, log:\n%s", got)
	}
}

// TestNvmSkipsWhenAlreadyInstalled hits the early-return path the
// happy-path test doesn't (it unconditionally installs).
func TestNvmSkipsWhenAlreadyInstalled(t *testing.T) {
	ic, home := setupClosureEnv(t, "brew")
	nvmDir := filepath.Join(home, ".config", "nvm")
	if err := os.MkdirAll(nvmDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := installNvm(context.Background(), ic); err != nil {
		t.Fatalf("installNvm skip: %v", err)
	}

	// Also exercise the fallback-altDir skip.
	_ = os.RemoveAll(nvmDir)
	altDir := filepath.Join(home, ".nvm")
	if err := os.MkdirAll(altDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := installNvm(context.Background(), ic); err != nil {
		t.Fatalf("installNvm altDir skip: %v", err)
	}
}

// TestTpmSkipsWhenAlreadyInstalled and its force-reinstall inverse
// cover both branches of installTPM's existence check.
func TestTpmSkipsWhenAlreadyInstalled(t *testing.T) {
	ic, home := setupClosureEnv(t, "brew")
	tpmDir := filepath.Join(home, ".tmux", "plugins", "tpm")
	if err := os.MkdirAll(tpmDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := installTPM(context.Background(), ic); err != nil {
		t.Fatalf("installTPM skip: %v", err)
	}
	ic.ForceReinstall = true
	if err := installTPM(context.Background(), ic); err != nil {
		t.Fatalf("installTPM force: %v", err)
	}
}

// TestIsNerdFontInstalled_LinuxFontDir verifies isNerdFontInstalled()
// returns true when a JetBrains Nerd Font file is present in the
// user's ~/.local/share/fonts/NerdFonts directory.
//
// This is intentionally not a darwin test — there is no goosFn seam
// in the registry package, and runtime.GOOS is used directly at
// cli_tools.go:434. The brew-cask probe inside the darwin branch
// cannot be exercised from a non-darwin runtime without forking the
// production code or shelling out to a fake brew (which would also
// need runtime.GOOS=="darwin" to even be reached). The filesystem
// branch below runs on any OS and is the assertion that actually
// catches a regression — if someone swaps the glob pattern or drops
// the NerdFonts subdirectory from the scan list, this fails.
//
// Adding a goosFn seam to registry is out of scope for this test;
// see installer/internal/platform/detect.go for the existing seam.
func TestIsNerdFontInstalled_LinuxFontDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	fontDir := filepath.Join(home, ".local", "share", "fonts", "NerdFonts")
	if err := os.MkdirAll(fontDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fontDir, "JetBrainsMono-Nerd.ttf"),
		[]byte("font"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !isNerdFontInstalled() {
		t.Fatal("expected nerd font detection via NerdFonts directory")
	}
}

// TestIsNerdFontInstalled_NoFontsReturnsFalse locks in the inverse:
// on a clean HOME with no fonts anywhere, the probe must return
// false. A regression that always-returned-true (e.g. a refactor
// that mistakenly returned after the loop) would let the installer
// skip the font install silently. Non-darwin only — on macOS the
// test would need a brew stub in PATH to force the cask probe to
// fail cleanly.
func TestIsNerdFontInstalled_NoFontsReturnsFalse(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip(
			"darwin: brew-list cask probe can succeed from user env; " +
				"registry has no goosFn seam to override runtime.GOOS",
		)
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	// Also pin PATH so brew is definitely not found even on a linux
	// host that happens to have linuxbrew installed.
	bin := filepath.Join(home, "empty-bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	if isNerdFontInstalled() {
		t.Fatal("expected isNerdFontInstalled()=false on empty HOME")
	}
}
