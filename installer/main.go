package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	tea "charm.land/bubbletea/v2"

	"github.com/chaseddevelopment/dotfiles/installer/internal/executor"
	"github.com/chaseddevelopment/dotfiles/installer/internal/platform"
	"github.com/chaseddevelopment/dotfiles/installer/internal/pkgmgr"
	"github.com/chaseddevelopment/dotfiles/installer/internal/state"
	"github.com/chaseddevelopment/dotfiles/installer/internal/tui"

	"golang.org/x/term"
)

// Version is set at build time via -ldflags.
var Version = "dev"

func main() {
	if os.Getenv("HOME") == "" {
		fmt.Fprintln(os.Stderr, "Error: $HOME is not set")
		os.Exit(1)
	}

	augmentPath()

	dryRun := flag.Bool("dry-run", false, "Preview changes without making them")
	version := flag.Bool("version", false, "Print version and exit")
	flag.Parse()

	if *version {
		fmt.Printf("dotsetup %s (%s/%s)\n", Version, runtime.GOOS, runtime.GOARCH)
		os.Exit(0)
	}

	if !term.IsTerminal(int(os.Stdin.Fd())) {
		fmt.Fprintln(os.Stderr, "Error: dotsetup requires an interactive terminal.")
		os.Exit(1)
	}

	rootDir, err := findRootDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	plat, err := platform.Detect()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Pre-authenticate sudo before the TUI takes ownership of
	// stdin. The keepalive goroutine refreshes the credential
	// cache so long-running installs don't hit timeouts.
	if !*dryRun {
		if executor.NeedsSudo() {
			if err := executor.PreAuth(); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
			}
		} else if executor.HasSudo() {
			fmt.Fprintln(os.Stderr,
				"[sudo] Credentials already available.",
			)
		}
	}
	logFile, err := executor.NewLogFile(filepath.Join(rootDir, "install.log"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer logFile.Close()

	sudoCtx, cancelSudo := context.WithCancel(
		context.Background(),
	)
	defer cancelSudo()
	stopSudo := executor.StartKeepalive(sudoCtx, logFile)
	defer stopSudo()

	runner := executor.NewRunner(logFile, *dryRun)

	// Open /dev/tty so child processes can identify the
	// controlling terminal. sudo needs this to match cached
	// credentials when tty_tickets is enabled (default).
	if ttyFile, err := os.Open("/dev/tty"); err == nil {
		runner.Stdin = ttyFile
		defer ttyFile.Close()
	}

	mgr, err := pkgmgr.New(plat, runner)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	installState, err := state.Load(state.DefaultPath())
	if err != nil {
		logFile.Write(fmt.Sprintf(
			"WARNING: load state: %v (starting fresh)", err,
		))
		installState = state.NewStore(state.DefaultPath())
	}

	cfg := &tui.AppConfig{
		DryRun:   *dryRun,
		Platform: plat,
		PkgMgr:   mgr,
		RootDir:  rootDir,
		LogFile:  logFile,
		Runner:   runner,
		State:    installState,
	}

	tui.Version = Version

	app := tui.NewApp(cfg)
	p := tea.NewProgram(app)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
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
	os.Setenv("PATH", path)
}

func hasConfigs(dir string) bool {
	info, err := os.Stat(filepath.Join(dir, "configs"))
	return err == nil && info.IsDir()
}
