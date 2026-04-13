package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/chaseddevelopment/dotfiles/installer/internal/executor"
	"github.com/chaseddevelopment/dotfiles/installer/internal/pkgmgr"
	"github.com/chaseddevelopment/dotfiles/installer/internal/platform"
	"github.com/chaseddevelopment/dotfiles/installer/internal/state"
	"github.com/chaseddevelopment/dotfiles/installer/internal/tui"

	"golang.org/x/term"
)

// Version is set at build time via -ldflags.
var Version = "dev"

// Commit is the git SHA this binary was built from, injected via
// -ldflags. Empty in dev builds. Logged at session start so users
// (and incident responders) can tell exactly which installer ran.
var Commit = ""

type teaProgram interface {
	Run() (tea.Model, error)
}

var (
	termIsTerminalFn   = term.IsTerminal
	findRootDirFn      = findRootDir
	detectPlatformFn   = platform.Detect
	needsSudoFn        = executor.NeedsSudo
	preAuthFn          = executor.PreAuth
	hasSudoFn          = executor.HasSudo
	newLogFileFn       = executor.NewLogFile
	startKeepaliveFn   = executor.StartKeepalive
	pkgmgrNewFn        = pkgmgr.New
	stateDefaultPathFn = state.DefaultPath
	stateLoadFn        = state.Load
	stateNewStoreFn    = state.NewStore
	newAppFn           = func(cfg *tui.AppConfig) tea.Model { return tui.NewApp(cfg) }
	newTeaProgramFn    = func(model tea.Model) teaProgram { return tea.NewProgram(model) }
)

func main() {
	if os.Getenv("HOME") == "" {
		fmt.Fprintln(os.Stderr, "Error: $HOME is not set")
		os.Exit(1)
	}

	augmentPath()

	// --version is diagnostic only. Dry-run is a runtime mode set
	// inside the TUI options screen so it stays consistent across
	// navigation; a CLI flag would silently desync.
	version := flag.Bool("version", false, "Print version and exit")
	flag.Parse()

	if *version {
		fmt.Printf("dotsetup %s (%s/%s)\n", Version, runtime.GOOS, runtime.GOARCH)
		os.Exit(0)
	}

	if !termIsTerminalFn(int(os.Stdin.Fd())) {
		fmt.Fprintln(os.Stderr, "Error: dotsetup requires an interactive terminal.")
		os.Exit(1)
	}

	rootDir, err := findRootDirFn()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	plat, err := detectPlatformFn()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Pre-authenticate sudo before the TUI takes ownership of
	// stdin. The keepalive goroutine refreshes the credential
	// cache so long-running installs don't hit timeouts. Fail
	// fast if auth doesn't succeed — sudo prompts inside the alt
	// screen are hidden and every sudo task would silently error.
	if needsSudoFn() {
		if err := preAuthFn(); err != nil {
			fmt.Fprintf(os.Stderr,
				"Error: sudo authentication failed: %v\n", err,
			)
			os.Exit(1)
		}
	} else if hasSudoFn() {
		fmt.Fprintln(os.Stderr,
			"[sudo] Credentials already available.",
		)
	}
	logFile, err := newLogFileFn(filepath.Join(rootDir, "install.log"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer logFile.Close()

	// Record exactly which binary ran. After the dock incident where a
	// stale installer silently handled the run, this lets anyone
	// cross-reference the log against `git log` without guessing.
	commitLabel := Commit
	if commitLabel == "" {
		commitLabel = "unknown"
	}
	logFile.Write(fmt.Sprintf(
		"dotsetup version=%s commit=%s platform=%s/%s",
		Version, commitLabel, runtime.GOOS, runtime.GOARCH,
	))

	sudoCtx, cancelSudo := context.WithCancel(
		context.Background(),
	)
	defer cancelSudo()
	stopSudo := startKeepaliveFn(sudoCtx, logFile)
	defer stopSudo()

	runner := executor.NewRunner(logFile, false)

	// Open /dev/tty so child processes can identify the
	// controlling terminal. sudo needs this to match cached
	// credentials when tty_tickets is enabled (default).
	if ttyFile, err := os.Open("/dev/tty"); err == nil {
		runner.Stdin = ttyFile
		defer ttyFile.Close()
	}

	mgr, err := pkgmgrNewFn(plat, runner)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	statePath := stateDefaultPathFn()
	installState, err := stateLoadFn(statePath)
	if err != nil {
		if errors.Is(err, state.ErrCorrupt) {
			// Preserve the corrupt file so the user can inspect it
			// instead of losing their install history silently.
			backupPath := fmt.Sprintf(
				"%s.bak-%s", statePath,
				time.Now().Format("20060102-150405"),
			)
			if renameErr := os.Rename(statePath, backupPath); renameErr != nil {
				fmt.Fprintf(os.Stderr,
					"Error: state file is corrupt and backup failed: "+
						"load=%v rename=%v\n",
					err, renameErr,
				)
				os.Exit(1)
			}
			msg := fmt.Sprintf(
				"WARNING: state file corrupt (%v); "+
					"moved to %s, starting fresh",
				err, backupPath,
			)
			fmt.Fprintln(os.Stderr, msg)
			logFile.Write(msg)
			installState = stateNewStoreFn(statePath)
		} else {
			fmt.Fprintf(os.Stderr, "Error: load state: %v\n", err)
			os.Exit(1)
		}
	}

	cfg := &tui.AppConfig{
		DryRun:   false,
		Platform: plat,
		PkgMgr:   mgr,
		RootDir:  rootDir,
		LogFile:  logFile,
		Runner:   runner,
		State:    installState,
	}

	tui.Version = Version
	tui.Commit = Commit

	app := newAppFn(cfg)
	p := newTeaProgramFn(app)
	finalModel, err := p.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	// Propagate critical install failures as non-zero exit — CI
	// wrappers and shell scripts need this to stop silently lying.
	if m, ok := finalModel.(tui.AppModel); ok && m.CriticalFailure() {
		os.Exit(2)
	}
}

// findRootDir locates the dotfiles repository root (the directory
// containing configs/).
func findRootDir() (string, error) {
	// Strategy 1: DOTFILES_DIR environment variable.
	if dir := os.Getenv("DOTFILES_DIR"); dir != "" {
		if hasConfigs(dir) {
			return dir, nil
		}
	}

	// Strategy 2: Walk up from the binary's location.
	if exe, err := os.Executable(); err == nil {
		if resolved, err := filepath.EvalSymlinks(exe); err == nil {
			dir := filepath.Dir(resolved)
			// Binary may be in installer/ subdirectory.
			for _, candidate := range []string{dir, filepath.Dir(dir)} {
				if hasConfigs(candidate) {
					return candidate, nil
				}
			}
		}
	}

	// Strategy 3: Walk up from CWD.
	if cwd, err := os.Getwd(); err == nil {
		for d := cwd; d != "/" && d != "."; d = filepath.Dir(d) {
			if hasConfigs(d) {
				return d, nil
			}
		}
	}

	// Strategy 4: Common default locations.
	home := os.Getenv("HOME")
	defaults := []string{
		filepath.Join(home, "dotfiles"),
		filepath.Join(home, ".dotfiles"),
		filepath.Join(home, "Documents", "GitHub", "dotfiles"),
	}
	for _, candidate := range defaults {
		if hasConfigs(candidate) {
			return candidate, nil
		}
	}

	return "", fmt.Errorf(
		"cannot find dotfiles root (expected a directory containing configs/). " +
			"Set DOTFILES_DIR or run from within the dotfiles repo",
	)
}

// augmentPath prepends common tool install directories to PATH so
// exec.LookPath and exec.CommandContext can find binaries that live
// outside the default system PATH (e.g., ~/.cargo/bin, ~/.local/bin).
func augmentPath() {
	home := os.Getenv("HOME")
	dirs := []string{
		filepath.Join(home, ".cargo", "bin"),
		filepath.Join(home, ".local", "bin"),
		filepath.Join(home, ".bun", "bin"),
		filepath.Join(home, ".atuin", "bin"),
		filepath.Join(home, ".dotnet"),
		"/usr/local/go/bin",
	}
	path := os.Getenv("PATH")
	for _, d := range dirs {
		if _, err := os.Stat(d); err == nil {
			path = d + string(filepath.ListSeparator) + path
		}
	}
	if err := os.Setenv("PATH", path); err != nil {
		fmt.Fprintf(os.Stderr,
			"Error: setenv PATH: %v\n", err,
		)
		os.Exit(1)
	}
}

func hasConfigs(dir string) bool {
	info, err := os.Stat(filepath.Join(dir, "configs"))
	return err == nil && info.IsDir()
}
