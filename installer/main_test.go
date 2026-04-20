package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/chaseddevelopment/dotfiles/installer/internal/executor"
	"github.com/chaseddevelopment/dotfiles/installer/internal/pkgmgr"
	"github.com/chaseddevelopment/dotfiles/installer/internal/platform"
	"github.com/chaseddevelopment/dotfiles/installer/internal/state"
	"github.com/chaseddevelopment/dotfiles/installer/internal/tui"
)

type mainTestPkgMgr struct{}

func (mainTestPkgMgr) Name() string                             { return "brew" }
func (mainTestPkgMgr) Install(context.Context, ...string) error { return nil }
func (mainTestPkgMgr) IsInstalled(string) bool                  { return false }
func (mainTestPkgMgr) UpdateAll(context.Context) error          { return nil }
func (mainTestPkgMgr) MapName(name string) []string             { return []string{name} }

type fakeModel struct{}

func (fakeModel) Init() tea.Cmd                           { return nil }
func (fakeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) { return fakeModel{}, nil }
func (fakeModel) View() tea.View                          { return tea.NewView("") }

type fakeProgram struct {
	model tea.Model
	err   error
}

func (p fakeProgram) Run() (tea.Model, error) { return p.model, p.err }

func TestHasConfigsAndAugmentPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	origPath := os.Getenv("PATH")
	t.Setenv("PATH", origPath)

	if err := os.MkdirAll(filepath.Join(home, ".cargo", "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(home, ".local", "bin"), 0o755); err != nil {
		t.Fatal(err)
	}

	augmentPath()
	path := os.Getenv("PATH")
	for _, want := range []string{
		filepath.Join(home, ".cargo", "bin"),
		filepath.Join(home, ".local", "bin"),
	} {
		if !strings.Contains(path, want) {
			t.Fatalf("PATH missing %q: %s", want, path)
		}
	}

	repo := t.TempDir()
	if hasConfigs(repo) {
		t.Fatal("expected false before configs exists")
	}
	if err := os.MkdirAll(filepath.Join(repo, "configs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if !hasConfigs(repo) {
		t.Fatal("expected true after configs dir exists")
	}
}

func TestFindRootDirFromEnvAndDefaults(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	envRepo := filepath.Join(home, "env-repo")
	if err := os.MkdirAll(filepath.Join(envRepo, "configs"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DOTFILES_DIR", envRepo)
	got, err := findRootDir()
	if err != nil {
		t.Fatalf("findRootDir env: %v", err)
	}
	if got != envRepo {
		t.Fatalf("findRootDir env = %q, want %q", got, envRepo)
	}

	t.Setenv("DOTFILES_DIR", filepath.Join(home, "missing"))
	defaultRepo := filepath.Join(home, "dotfiles")
	if err := os.MkdirAll(filepath.Join(defaultRepo, "configs"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(home)
	got, err = findRootDir()
	if err != nil {
		t.Fatalf("findRootDir default: %v", err)
	}
	if got != defaultRepo {
		t.Fatalf("findRootDir default = %q, want %q", got, defaultRepo)
	}
}

// TestFindRootDir_HomeSymlinkDefault verifies the strategy-4
// default-locations scan resolves a symlinked `~/dotfiles` to its
// real path. Regression guard: if the implementation switched to a
// Lstat-based check, symlinked dotfiles directories (a common
// multi-machine setup) would silently fall through to the
// not-found branch.
func TestFindRootDir_HomeSymlinkDefault(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("DOTFILES_DIR", "")

	// Real repo sits outside HOME; ~/dotfiles is a symlink to it.
	realRepo := filepath.Join(t.TempDir(), "real-dotfiles")
	if err := os.MkdirAll(filepath.Join(realRepo, "configs"), 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(home, "dotfiles")
	if err := os.Symlink(realRepo, link); err != nil {
		t.Skipf("symlink not supported on this fs: %v", err)
	}

	// Chdir to a leaf with no configs/ ancestors so strategies 2/3
	// definitely miss and we exercise strategy 4.
	leaf := t.TempDir()
	t.Chdir(leaf)

	got, err := findRootDir()
	if err != nil {
		t.Fatalf("findRootDir: %v", err)
	}
	// hasConfigs follows the symlink via os.Stat — the returned
	// path is the symlink path itself (strategy 4 does not call
	// EvalSymlinks). The important behavior is that the symlinked
	// dotfiles directory is discovered.
	if got != link {
		t.Fatalf("findRootDir = %q, want %q (symlink path)", got, link)
	}
	resolved, err := filepath.EvalSymlinks(got)
	if err != nil {
		t.Fatalf("EvalSymlinks(%q): %v", got, err)
	}
	// macOS prefixes tmpdirs with /private; resolve both sides
	// before comparing so the test isn't platform-brittle.
	wantResolved, err := filepath.EvalSymlinks(realRepo)
	if err != nil {
		t.Fatalf("EvalSymlinks(%q): %v", realRepo, err)
	}
	if resolved != wantResolved {
		t.Fatalf("resolved = %q, want %q", resolved, wantResolved)
	}
}

// TestFindRootDir_CWDWalkStrategy3 covers strategy 3 (main.go:
// 230–237): when HOME is empty (defaults skip), DOTFILES_DIR is
// unset, and the binary's directory has no configs/, findRootDir
// must walk up from CWD until it finds a configs/-bearing parent.
func TestFindRootDir_CWDWalkStrategy3(t *testing.T) {
	// Empty HOME so the strategy-4 default scan finds nothing.
	emptyHome := t.TempDir()
	t.Setenv("HOME", emptyHome)
	t.Setenv("DOTFILES_DIR", "")

	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, "configs"), 0o755); err != nil {
		t.Fatal(err)
	}
	deep := filepath.Join(repo, "a", "b", "c", "d")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(deep)

	got, err := findRootDir()
	if err != nil {
		t.Fatalf("findRootDir: %v", err)
	}
	// The CWD walk on macOS may surface either the real or
	// /private-prefixed temp path depending on getcwd resolution.
	gotResolved, _ := filepath.EvalSymlinks(got)
	wantResolved, _ := filepath.EvalSymlinks(repo)
	if gotResolved != wantResolved {
		t.Fatalf("findRootDir = %q (resolved %q), want %q (resolved %q)",
			got, gotResolved, repo, wantResolved)
	}
}

// TestFindRootDir_NotFound exercises the final error return at
// main.go:252–255 when every strategy whiffs: HOME points to an
// empty tmpdir (no defaults match), DOTFILES_DIR is unset, and CWD
// has no configs/-bearing ancestor.
func TestFindRootDir_NotFound(t *testing.T) {
	emptyHome := t.TempDir()
	t.Setenv("HOME", emptyHome)
	t.Setenv("DOTFILES_DIR", "")

	// Chdir into a tmpdir tree with zero configs/ ancestors.
	leaf := t.TempDir()
	t.Chdir(leaf)

	got, err := findRootDir()
	if err == nil {
		t.Fatalf("expected error, got %q", got)
	}
	if got != "" {
		t.Fatalf("expected empty path on error, got %q", got)
	}
	if !strings.Contains(err.Error(), "cannot find dotfiles root") {
		t.Fatalf("unexpected error message: %v", err)
	}
	if !strings.Contains(err.Error(), "DOTFILES_DIR") {
		t.Fatalf("error should mention DOTFILES_DIR escape hatch: %v", err)
	}
}

func TestMainProcessVersionAndHomeError(t *testing.T) {
	t.Run("version exits zero", func(t *testing.T) {
		cmd := exec.Command(os.Args[0], "-test.run=TestMainHelperProcess", "--", "-version")
		cmd.Env = append(os.Environ(), "GO_WANT_MAIN_HELPER=1", "HOME="+t.TempDir())
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("version helper failed: %v\n%s", err, out)
		}
		if !strings.Contains(string(out), "dotsetup") {
			t.Fatalf("unexpected version output: %s", out)
		}
	})

	t.Run("missing home exits non-zero", func(t *testing.T) {
		cmd := exec.Command(os.Args[0], "-test.run=TestMainHelperProcess", "--")
		cmd.Env = append(os.Environ(), "GO_WANT_MAIN_HELPER=1", "HOME=")
		out, err := cmd.CombinedOutput()
		if err == nil {
			t.Fatalf("expected failure when HOME unset, output=%s", out)
		}
		if !strings.Contains(string(out), "$HOME is not set") {
			t.Fatalf("unexpected missing HOME output: %s", out)
		}
	})
}

func TestMainProcessStartupErrorsAndRecovery(t *testing.T) {
	tests := []struct {
		name      string
		scenario  string
		wantSub   string
		wantError bool
	}{
		{name: "non interactive terminal", scenario: "noninteractive", wantSub: "requires an interactive terminal", wantError: true},
		{name: "find root failure", scenario: "findroot", wantSub: "cannot find repo", wantError: true},
		{name: "platform detect failure", scenario: "platform", wantSub: "platform detect failed", wantError: true},
		{name: "sudo auth failure", scenario: "sudoauth", wantSub: "sudo authentication failed", wantError: true},
		{name: "sudo priming banner", scenario: "hassudo", wantSub: "Priming credentials for this session", wantError: false},
		{name: "log file failure", scenario: "logfile", wantSub: "log open failed", wantError: true},
		{name: "pkg manager failure", scenario: "pkgmgr", wantSub: "pkg manager unavailable", wantError: true},
		{name: "state load failure", scenario: "state-load", wantSub: "load state: state read failed", wantError: true},
		{name: "program run failure", scenario: "program", wantSub: "program failed", wantError: true},
		{name: "corrupt state recovery", scenario: "recover-corrupt", wantSub: "starting fresh", wantError: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			scratch := t.TempDir()
			cmd := exec.Command(os.Args[0], "-test.run=TestMainHelperProcess", "--")
			cmd.Env = append(
				os.Environ(),
				"GO_WANT_MAIN_HELPER=1",
				"HOME="+home,
				"GO_MAIN_SCENARIO="+tc.scenario,
				"GO_MAIN_TMP="+scratch,
			)
			out, err := cmd.CombinedOutput()
			if tc.wantError && err == nil {
				t.Fatalf("expected failure, output=%s", out)
			}
			if !tc.wantError && err != nil {
				t.Fatalf("expected success, err=%v output=%s", err, out)
			}
			if !strings.Contains(string(out), tc.wantSub) {
				t.Fatalf("output %q missing %q", out, tc.wantSub)
			}

			// For success scenarios assert observable recovery side
			// effects (audit complaint: original test only checked
			// stdout substrings).
			if !tc.wantError {
				logPath := filepath.Join(scratch, "helper.log")
				logBytes, err := os.ReadFile(logPath)
				if err != nil {
					t.Fatalf("scenario %q: log file not created: %v", tc.scenario, err)
				}
				if len(logBytes) == 0 {
					t.Fatalf("scenario %q: log file empty (no startup writes)", tc.scenario)
				}

				if tc.scenario == "hassudo" {
					// PreAuth is unconditional now — assert the
					// priming banner landed on stderr and the
					// install.log captured the startup banner so
					// downstream control flow is proven to have
					// continued past the sudo step.
					if !strings.Contains(string(logBytes), "dotsetup version=") {
						t.Fatalf("install.log missing version banner: %q", logBytes)
					}
				}

				if tc.scenario == "recover-corrupt" {
					// Recovery should have renamed the corrupt
					// state.json to a .bak-* sibling and logged a
					// WARNING line so the user can inspect later.
					entries, err := os.ReadDir(scratch)
					if err != nil {
						t.Fatalf("read scratch: %v", err)
					}
					var foundBak, foundFresh bool
					for _, e := range entries {
						if strings.HasPrefix(e.Name(), "state.json.bak-") {
							foundBak = true
							raw, err := os.ReadFile(filepath.Join(scratch, e.Name()))
							if err != nil {
								t.Fatalf("read backup: %v", err)
							}
							if !strings.Contains(string(raw), "{invalid") {
								t.Fatalf("recover-corrupt: backup did not preserve original payload: %q", raw)
							}
						}
						if e.Name() == "state.json" {
							foundFresh = true
						}
					}
					if !foundBak {
						t.Fatalf("recover-corrupt: no state.json.bak-* file produced; entries=%v", entries)
					}
					if foundFresh {
						t.Fatalf("recover-corrupt: stale state.json still present (rename did not happen)")
					}
					if !strings.Contains(string(logBytes), "WARNING: state file corrupt") {
						t.Fatalf("recover-corrupt: log missing WARNING line; got=%q", logBytes)
					}
				}
			}
		})
	}
}

func TestMainHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_MAIN_HELPER") != "1" {
		return
	}
	tmp := os.Getenv("GO_MAIN_TMP")
	if tmp != "" {
		repo := filepath.Join(tmp, "repo")
		_ = os.MkdirAll(filepath.Join(repo, "configs"), 0o755)
		logPath := filepath.Join(tmp, "helper.log")
		statePath := filepath.Join(tmp, "state.json")

		termIsTerminalFn = func(int) bool { return true }
		findRootDirFn = func() (string, error) { return repo, nil }
		detectPlatformFn = func() (*platform.Platform, error) {
			return &platform.Platform{OS: platform.MacOS, Arch: platform.ARM64, PackageManager: platform.PkgBrew}, nil
		}
		preAuthFn = func() error { return nil }
		hasSudoFn = func() bool { return false }
		newLogFileFn = func(_ string) (*executor.LogFile, error) { return executor.NewLogFile(logPath) }
		startKeepaliveFn = func(context.Context, *executor.LogFile) func() { return func() {} }
		pkgmgrNewFn = func(*platform.Platform, *executor.Runner) (pkgmgr.PackageManager, error) {
			return mainTestPkgMgr{}, nil
		}
		stateDefaultPathFn = func() string { return statePath }
		stateLoadFn = state.Load
		stateNewStoreFn = state.NewStore
		newAppFn = func(*tui.AppConfig) tea.Model { return fakeModel{} }
		newTeaProgramFn = func(model tea.Model) teaProgram { return fakeProgram{model: model} }

		switch os.Getenv("GO_MAIN_SCENARIO") {
		case "noninteractive":
			termIsTerminalFn = func(int) bool { return false }
		case "findroot":
			findRootDirFn = func() (string, error) { return "", fmt.Errorf("cannot find repo") }
		case "platform":
			detectPlatformFn = func() (*platform.Platform, error) { return nil, fmt.Errorf("platform detect failed") }
		case "sudoauth":
			hasSudoFn = func() bool { return true }
			preAuthFn = func() error { return fmt.Errorf("prompt failed") }
		case "hassudo":
			// hasSudoFn=true forces unconditional PreAuth. The stub
			// PreAuth returns nil (no prompt), so main continues and
			// prints the priming banner to stderr. The wantSub match
			// validates the banner arrived on combined output.
			hasSudoFn = func() bool { return true }
			preAuthFn = func() error {
				fmt.Fprintln(os.Stderr,
					"[sudo] Priming credentials for this session "+
						"(password prompt only if cache is stale):",
				)
				return nil
			}
		case "logfile":
			newLogFileFn = func(string) (*executor.LogFile, error) {
				return nil, fmt.Errorf("log open failed")
			}
		case "pkgmgr":
			pkgmgrNewFn = func(*platform.Platform, *executor.Runner) (pkgmgr.PackageManager, error) {
				return nil, fmt.Errorf("pkg manager unavailable")
			}
		case "state-load":
			stateLoadFn = func(string) (*state.Store, error) {
				return nil, fmt.Errorf("state read failed")
			}
		case "program":
			newTeaProgramFn = func(model tea.Model) teaProgram {
				return fakeProgram{model: model, err: fmt.Errorf("program failed")}
			}
		case "recover-corrupt":
			_ = os.WriteFile(statePath, []byte("{invalid"), 0o644)
		}
	}
	args := []string{os.Args[0]}
	for i, arg := range os.Args {
		if arg == "--" {
			args = append(args, os.Args[i+1:]...)
			break
		}
	}
	os.Args = args
	main()
}
