package pkgmgr

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/chaseddevelopment/dotfiles/installer/internal/executor"
)

// Typed errors returned by Apt.Install so callers can distinguish
// recoverable dpkg states from fatal apt conditions via errors.Is.
//
// ErrDpkgInterrupted / ErrDpkgLocked are recoverable: the dpkg
// doctor can run `dpkg --configure -a`, and apt's own
// DPkg::Lock::Timeout waits out a held lock. ErrAptFatal is
// actionable but NOT recoverable by the installer — the user must
// fix unmet deps, held packages, bad mirrors, etc.
var (
	ErrDpkgInterrupted = errors.New("dpkg is in an interrupted state")
	ErrDpkgLocked      = errors.New("dpkg lock is held")
	ErrAptFatal        = errors.New("apt reported a fatal, non-recoverable condition")
)

// dpkgLockTimeoutSec is passed as `-o DPkg::Lock::Timeout=N` so apt
// waits for a held dpkg lock instead of failing immediately. Nala
// forwards -o options to libapt.
const dpkgLockTimeoutSec = 60

// DpkgState describes the result of a read-only dpkg health probe.
// A Healthy=false state means the installer should NOT proceed with
// apt installs until the inconsistency is repaired (via
// RunDpkgConfigureAll, ideally under user consent).
type DpkgState struct {
	Healthy      bool
	AuditOutput  string
	StaleUpdates []string
	Reason       string
}

// Apt implements PackageManager for APT-based systems (Debian, Ubuntu).
// Prefers nala as a frontend when available.
type Apt struct {
	runner  *executor.Runner
	useNala bool

	mu        sync.Mutex
	didUpdate bool

	// healthOnce ensures the read-only dpkg health probe runs at
	// most once per installer session. Subsequent callers see the
	// cached state.
	healthOnce sync.Once
	health     DpkgState
	healthErr  error

	// repairOnce gates RunDpkgConfigureAll — at most one repair
	// attempt per session regardless of how many install tasks
	// encounter the interrupted state.
	repairOnce sync.Once
	repairErr  error
	repaired   bool

	// UserApprovedRepair gates the auto-repair path when unhealthy
	// state is detected. Defaults to true for the interim (pre-TUI
	// modal). When the TUI modal (plan Phase A2) lands, this will
	// flip to false and only be set true via explicit user choice.
	UserApprovedRepair bool
}

// NewApt constructs an Apt package manager. Exposed so higher
// layers (orchestrator, TUI) can reference the concrete type when
// wiring the dpkg doctor pseudo-task and repair-consent flag.
func NewApt(runner *executor.Runner, useNala bool) *Apt {
	return &Apt{
		runner:             runner,
		useNala:            useNala,
		UserApprovedRepair: true,
	}
}

func (a *Apt) Name() string { return "apt" }

func (a *Apt) cmd() string {
	if a.useNala {
		return "nala"
	}
	return "apt-get"
}

// installArgs builds the install command for one or more packages.
// The lock-timeout option sits AFTER the `install` subcommand —
// nala only accepts `-o` in that position (while apt-get accepts
// it anywhere). Placing it after `install` keeps both CLIs happy.
func (a *Apt) installArgs(pkgs []string) []string {
	args := []string{
		a.cmd(),
		"install",
		"-o", fmt.Sprintf("DPkg::Lock::Timeout=%d", dpkgLockTimeoutSec),
		"-y",
	}
	return append(args, pkgs...)
}

// aptEnv is the per-invocation env applied to apt/nala calls.
// TERM=dumb and NO_COLOR=1 strip the Rich/progress-bar box from
// stderr — that output historically bled into install.log and made
// grepping for failures harder. DEBIAN_FRONTEND=noninteractive is
// a belt-and-braces guard against prompts that -y doesn't cover
// (e.g. restart-services dialogs from some packages).
// NALA_FANCY=0 forces nala into plain line-oriented output so the
// orchestrator's batch progress tap can match dpkg's
// `Setting up <name>` completion markers without chasing ANSI
// cursor games. Harmless when the active command is plain apt.
func aptEnv() []string {
	return []string{
		"TERM=dumb",
		"NO_COLOR=1",
		"DEBIAN_FRONTEND=noninteractive",
		"NALA_FANCY=0",
	}
}

// ensureUpdated runs apt-get update once per session before the
// first install to ensure the package cache is fresh.
func (a *Apt) ensureUpdated(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.didUpdate {
		return nil
	}

	out, err := a.runner.RunWithEnvAndOutput(
		ctx, aptEnv(), "sudo", a.cmd(), "update",
	)
	if err != nil {
		return classifyAptErr(
			fmt.Errorf("%s update: %w", a.cmd(), err),
			out,
		)
	}

	a.didUpdate = true
	return nil
}

func (a *Apt) Install(ctx context.Context, genericNames ...string) error {
	if err := a.ensureUpdated(ctx); err != nil {
		return err
	}
	if len(genericNames) == 0 {
		return nil
	}
	pkgs := dedupeNames(a.MapName, genericNames)
	if len(pkgs) == 0 {
		return nil
	}
	args := append([]string{"sudo"}, a.installArgs(pkgs)...)
	out, err := a.runner.RunWithEnvAndOutput(
		ctx, aptEnv(), args[0], args[1:]...,
	)
	if err == nil {
		return nil
	}
	classified := classifyAptErr(
		fmt.Errorf("%s install: %w", a.cmd(), err),
		out,
	)
	return attribute(classified, genericNames, a.IsInstalled)
}

// IsInstalled checks each mapped package via dpkg-query. dpkg-query
// -W -f='${Status}' is precise (exact name match, returns a single
// field) unlike `dpkg -l` which glob-matches and returns success
// even when the package is in the "rc" (removed, config remaining)
// state. Both behaviors are pre-existing latent bugs for packages
// whose names are prefixes of unrelated packages.
func (a *Apt) IsInstalled(genericName string) bool {
	names := a.MapName(genericName)
	if len(names) == 0 {
		return false
	}
	for _, pkg := range names {
		out, err := a.runner.RunProbe(
			context.Background(),
			"dpkg-query", "-W", "-f=${Status}", pkg,
		)
		if err != nil {
			return false
		}
		// Status is "install ok installed" when the package is
		// actually installed. Anything else (including "rc") means
		// not installed for our purposes.
		if !containsInstalled(out) {
			return false
		}
	}
	return true
}

// containsInstalled parses a dpkg-query Status string. Exposed as a
// helper so the behavior is unit-testable without shelling out.
func containsInstalled(status string) bool {
	const want = "install ok installed"
	return len(status) >= len(want) && status[:len(want)] == want
}

func (a *Apt) UpdateAll(ctx context.Context) error {
	cmd := a.cmd()
	script := fmt.Sprintf("sudo %s update && sudo %s upgrade -y", cmd, cmd)
	return a.runner.RunShell(ctx, script)
}

func (a *Apt) MapName(generic string) []string {
	m := map[string][]string{
		"nodejs":          {"nodejs", "npm"},
		"build-essential": {"build-essential"},
		"fd":              {"fd-find"},
		"bat":             {"bat"},
	}
	if names, ok := m[generic]; ok {
		return names
	}
	return []string{generic}
}

// DetectDpkgHealth runs a read-only probe of dpkg state. Returns
// the cached result on subsequent calls — a session-scoped sync.Once
// prevents redundant `dpkg --audit` invocations. The probe is
// intentionally read-only; remediation (if any) happens via a
// separate consent-gated call to RunDpkgConfigureAll.
func (a *Apt) DetectDpkgHealth(ctx context.Context) (DpkgState, error) {
	a.healthOnce.Do(func() {
		a.health, a.healthErr = a.probeDpkg(ctx)
	})
	return a.health, a.healthErr
}

func (a *Apt) probeDpkg(ctx context.Context) (DpkgState, error) {
	// dpkg --audit exits 0 with empty stdout when healthy; non-empty
	// output (even with exit 0 on some versions) means at least one
	// package is in an inconsistent state.
	audit, err := a.runner.RunProbe(ctx, "dpkg", "--audit")
	if err != nil {
		// A non-zero audit exit is itself diagnostic of an
		// inconsistent state rather than a reason to abort — fall
		// through and treat the output as the reason.
		return DpkgState{
			Healthy:     false,
			AuditOutput: strings.TrimSpace(audit),
			Reason: fmt.Sprintf(
				"dpkg --audit reported errors: %v", err,
			),
		}, nil
	}
	trimmed := strings.TrimSpace(audit)
	if trimmed != "" {
		return DpkgState{
			Healthy:     false,
			AuditOutput: trimmed,
			Reason:      "dpkg --audit reported inconsistencies",
		}, nil
	}
	// Additionally check for stale transaction files. A non-empty
	// /var/lib/dpkg/updates/ indicates a prior dpkg run was
	// interrupted mid-transaction.
	stale := listStaleDpkgUpdates()
	if len(stale) > 0 {
		return DpkgState{
			Healthy:      false,
			StaleUpdates: stale,
			Reason:       "stale transaction files in /var/lib/dpkg/updates/",
		}, nil
	}
	return DpkgState{Healthy: true}, nil
}

// listStaleDpkgUpdates returns the filenames of any transaction
// journal files in /var/lib/dpkg/updates/. A healthy system has an
// empty directory; entries here mean a prior dpkg invocation died
// mid-transaction and `dpkg --configure -a` is required to replay.
//
// Errors (missing directory, permission denied, non-Debian host)
// are treated as "nothing stale" — callers should not block an
// install because they couldn't read the directory.
func listStaleDpkgUpdates() []string {
	entries, err := os.ReadDir("/var/lib/dpkg/updates")
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names
}

// RunDpkgConfigureAll invokes `sudo dpkg --configure -a` to repair
// an interrupted transaction. Guarded by sync.Once so repeated
// callers (install tasks that all hit ErrDpkgInterrupted at once)
// don't pile repair attempts on top of each other. A repair
// attempt that fails is recorded and all subsequent callers see
// the same error — subsequent attempts would fail identically.
func (a *Apt) RunDpkgConfigureAll(ctx context.Context) error {
	a.repairOnce.Do(func() {
		a.runner.Log.Write(
			"dpkg doctor: running `sudo dpkg --configure -a` to repair interrupted state",
		)
		_, err := a.runner.RunWithOutput(
			ctx, "sudo", "dpkg", "--configure", "-a",
		)
		if err != nil {
			a.repairErr = fmt.Errorf("dpkg --configure -a: %w", err)
			return
		}
		a.repaired = true
		// Invalidate the cached health result so a subsequent
		// DetectDpkgHealth re-probes rather than returning the
		// pre-repair diagnosis.
		a.healthOnce = sync.Once{}
	})
	return a.repairErr
}

// EnsureHealthy is the entry point the orchestrator's dpkg doctor
// pseudo-task calls. It probes state; if unhealthy AND
// UserApprovedRepair is set, attempts a one-shot repair. Otherwise
// returns an error describing what the user needs to fix manually.
//
// Callers are responsible for surfacing the TUI modal that flips
// UserApprovedRepair based on user choice — until that wiring
// (plan Phase A2 TUI) is in place, UserApprovedRepair defaults to
// true so tool installs aren't blocked by the interim.
func (a *Apt) EnsureHealthy(ctx context.Context) error {
	state, err := a.DetectDpkgHealth(ctx)
	if err != nil {
		return fmt.Errorf("probe dpkg health: %w", err)
	}
	if state.Healthy {
		a.runner.Log.Write("dpkg doctor: system is healthy")
		return nil
	}
	a.runner.Log.Write(fmt.Sprintf(
		"dpkg doctor: UNHEALTHY — %s (audit output: %q, stale: %v)",
		state.Reason, state.AuditOutput, state.StaleUpdates,
	))
	if !a.UserApprovedRepair {
		return fmt.Errorf(
			"dpkg is in an inconsistent state and repair was not "+
				"approved: %s (run `sudo dpkg --configure -a` manually "+
				"and re-run dotsetup)",
			state.Reason,
		)
	}
	if err := a.RunDpkgConfigureAll(ctx); err != nil {
		return fmt.Errorf("dpkg doctor repair failed: %w", err)
	}
	// Re-probe so callers see the post-repair state, and so any
	// subsequent ErrDpkgInterrupted classifier short-circuits
	// correctly instead of triggering another repair.
	state, err = a.DetectDpkgHealth(ctx)
	if err != nil {
		return fmt.Errorf("re-probe after repair: %w", err)
	}
	if !state.Healthy {
		return fmt.Errorf(
			"dpkg still inconsistent after repair: %s",
			state.Reason,
		)
	}
	a.runner.Log.Write("dpkg doctor: repair succeeded, dpkg now healthy")
	return nil
}

// classifyAptErr wraps err with the most specific typed sentinel
// when combinedOutput matches a known dpkg/apt failure pattern.
// The original error (and its exit-code chain) is preserved via
// wrapping so callers that don't care about the typed class still
// see the raw shell failure.
//
// Matches are deliberately narrow. "dpkg returned an error code" is
// an umbrella message covering real dependency failures we want to
// surface as-is rather than treat as retryable, so it is NOT matched.
func classifyAptErr(err error, combinedOutput string) error {
	if err == nil {
		return nil
	}
	lower := strings.ToLower(combinedOutput)
	switch {
	case strings.Contains(lower, "dpkg was interrupted"):
		return fmt.Errorf("%w: %w", ErrDpkgInterrupted, err)
	case strings.Contains(lower, "could not get lock"),
		strings.Contains(lower, "/var/lib/dpkg/lock-frontend"):
		return fmt.Errorf("%w: %w", ErrDpkgLocked, err)
	case strings.Contains(lower, "unmet dependencies"),
		strings.Contains(lower, "held packages"),
		strings.Contains(lower, "hash sum mismatch"),
		strings.Contains(lower, "release file"):
		return fmt.Errorf("%w: %w", ErrAptFatal, err)
	}
	return err
}
