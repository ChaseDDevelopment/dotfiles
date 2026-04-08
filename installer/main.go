package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/chaseddevelopment/dotfiles/installer/internal/executor"
	"github.com/chaseddevelopment/dotfiles/installer/internal/platform"
	"github.com/chaseddevelopment/dotfiles/installer/internal/pkgmgr"
	"github.com/chaseddevelopment/dotfiles/installer/internal/tui"

	"golang.org/x/term"
)

// Version is set at build time via -ldflags.
var Version = "dev"

func main() {
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

	logFile, err := executor.NewLogFile(filepath.Join(rootDir, "install.log"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer logFile.Close()

	runner := executor.NewRunner(logFile, *dryRun)

	mgr, err := pkgmgr.New(plat, runner)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	cfg := &tui.AppConfig{
		DryRun:   *dryRun,
		Platform: plat,
		PkgMgr:   mgr,
		RootDir:  rootDir,
		LogFile:  logFile,
		Runner:   runner,
	}

	tui.Version = Version

	app := tui.NewApp(cfg)
	p := tea.NewProgram(app, tea.WithAltScreen())
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

func hasConfigs(dir string) bool {
	info, err := os.Stat(filepath.Join(dir, "configs"))
	return err == nil && info.IsDir()
}
