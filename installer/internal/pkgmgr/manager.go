package pkgmgr

import (
	"context"
	"fmt"
	"strings"

	"github.com/chaseddevelopment/dotfiles/installer/internal/executor"
	"github.com/chaseddevelopment/dotfiles/installer/internal/platform"
)

// PackageManager abstracts system package manager operations.
//
// Install is batch-aware: callers may pass any number of generic
// names and implementations execute a SINGLE shell invocation
// (one `sudo nala install -y …`, one `brew install …`, etc.) after
// mapping and deduplicating names. When a batch fails partially,
// implementations return a BatchFailure (wrapping the underlying
// shell error) enumerating the generic names that did NOT install
// so orchestrators can fan out to per-tool fallback strategies.
// Total-failure errors (e.g. network out, lock held) return a bare
// error without BatchFailure, since every name in the batch failed
// for the same reason.
type PackageManager interface {
	// Name returns the manager identifier ("brew", "apt", "pacman", etc.).
	Name() string

	// Install installs one or more packages by generic name in a
	// single shell invocation. See BatchFailure for partial-failure
	// semantics.
	Install(ctx context.Context, genericNames ...string) error

	// IsInstalled checks if a package is installed.
	IsInstalled(genericName string) bool

	// UpdateAll runs the system-wide update/upgrade command.
	UpdateAll(ctx context.Context) error

	// MapName translates a generic package name to platform-specific
	// names. May return multiple (e.g., "nodejs" -> ["nodejs", "npm"]).
	MapName(generic string) []string
}

// BatchFailure is returned by PackageManager.Install when a batch
// install partially failed — some packages landed, others did not.
// Callers use errors.As(err, &bf) to recover FailedNames and re-fan
// those specific generic names to fallback strategies without
// re-attempting the already-installed packages.
//
// FailedNames contains GENERIC names (pre-MapName), matching the
// arguments the caller passed. Wrapped is the underlying shell error
// so errors.Is against ErrDpkgInterrupted / ErrDpkgLocked / ErrAptFatal
// still works for classification-sensitive callers.
type BatchFailure struct {
	FailedNames []string
	Wrapped     error
}

func (e *BatchFailure) Error() string {
	return fmt.Sprintf(
		"batch install: %d package(s) failed (%s): %v",
		len(e.FailedNames),
		strings.Join(e.FailedNames, ", "),
		e.Wrapped,
	)
}

func (e *BatchFailure) Unwrap() error { return e.Wrapped }

// dedupeNames maps each generic name through mapper and returns a
// deduplicated list preserving insertion order. Order stability
// matters for mock-runner tests that assert the exact shell
// invocation.
func dedupeNames(mapper func(string) []string, generics []string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, g := range generics {
		for _, n := range mapper(g) {
			if !seen[n] {
				seen[n] = true
				out = append(out, n)
			}
		}
	}
	return out
}

// attribute converts a batch shell error into either a per-name
// BatchFailure (partial) or the raw classified error (total). Uses
// isInstalled to check each generic post-failure — packages that
// landed despite a non-zero exit code are excluded from
// FailedNames so the orchestrator's fan-out doesn't double-install.
//
// When every generic name is reported not-installed, the wrap in a
// BatchFailure would just add noise (one tool = total failure), so
// we return classified directly. This keeps errors.Is classification
// transparent for the dominant "whole batch failed for one reason"
// case (dpkg interrupted, dpkg locked, network out).
func attribute(
	classified error,
	generics []string,
	isInstalled func(string) bool,
) error {
	if classified == nil {
		return nil
	}
	var failed []string
	for _, g := range generics {
		if !isInstalled(g) {
			failed = append(failed, g)
		}
	}
	if len(failed) == 0 {
		// Shell said fail, but every package is present. This can
		// happen with tools that emit warnings to stderr and exit
		// non-zero for cosmetic reasons. Treat as success so the
		// fan-out doesn't loop.
		return nil
	}
	if len(failed) == len(generics) {
		return classified
	}
	return &BatchFailure{FailedNames: failed, Wrapped: classified}
}

// New creates the appropriate PackageManager for the detected platform.
func New(p *platform.Platform, runner *executor.Runner) (PackageManager, error) {
	switch p.PackageManager {
	case platform.PkgBrew:
		return &Brew{runner: runner}, nil
	case platform.PkgApt:
		return NewApt(runner, p.HasNala), nil
	case platform.PkgPacman:
		return newPacman(runner), nil
	case platform.PkgDnf:
		return newDnf(runner), nil
	case platform.PkgYum:
		return newYum(runner), nil
	case platform.PkgZypper:
		return newZypper(runner), nil
	default:
		return nil, fmt.Errorf("no supported package manager found")
	}
}
