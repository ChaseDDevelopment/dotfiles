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
		{name: "cached sudo note", scenario: "hassudo", wantSub: "Credentials already available", wantError: false},
		{name: "log file failure", scenario: "logfile", wantSub: "log open failed", wantError: true},
		{name: "pkg manager failure", scenario: "pkgmgr", wantSub: "pkg manager unavailable", wantError: true},
		{name: "state load failure", scenario: "state-load", wantSub: "load state: state read failed", wantError: true},
		{name: "program run failure", scenario: "program", wantSub: "program failed", wantError: true},
		{name: "corrupt state recovery", scenario: "recover-corrupt", wantSub: "starting fresh", wantError: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			cmd := exec.Command(os.Args[0], "-test.run=TestMainHelperProcess", "--")
			cmd.Env = append(
				os.Environ(),
				"GO_WANT_MAIN_HELPER=1",
				"HOME="+home,
				"GO_MAIN_SCENARIO="+tc.scenario,
				"GO_MAIN_TMP="+t.TempDir(),
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
		needsSudoFn = func() bool { return false }
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
			needsSudoFn = func() bool { return true }
			preAuthFn = func() error { return fmt.Errorf("prompt failed") }
		case "hassudo":
			hasSudoFn = func() bool { return true }
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
