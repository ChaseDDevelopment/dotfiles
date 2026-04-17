// Package engine provides a DAG-based parallel task scheduler for tool
// installation. Tasks declare dependencies and resource locks; the
// scheduler runs independent tasks concurrently while serializing
// access to shared resources (e.g., apt dpkg lock, cargo build lock).
package engine

import (
	"context"
	"time"
)

// Resource identifies a shared system resource that must be accessed
// exclusively. Tasks holding different resources run concurrently up
// to the worker-pool cap — so cross-manager parallelism (apt + cargo,
// brew + cargo) falls out naturally.
//
//	Resource     Serializes                          Reason
//	ResDpkg      apt, apt-get, nala, dpkg -i         Shared /var/lib/dpkg/lock-frontend
//	ResRpm       dnf, yum, zypper                    Shared rpm database lock
//	ResPacman    pacman                              Shared pacman db lock
//	ResBrew      brew                                Shared brew cellar lock
//	ResCargo     cargo                               Shared ~/.cargo/registry lock
//
// Adding a new package manager means (a) a new Resource constant
// here, (b) a slot in scheduler.resSems, and (c) orchestrator
// resource-assignment logic mapping the manager to that Resource.
type Resource string

const (
	ResDpkg   Resource = "dpkg"
	ResRpm    Resource = "rpm"
	ResPacman Resource = "pacman"
	ResBrew   Resource = "brew"
	ResCargo  Resource = "cargo"
)

// TaskState tracks the lifecycle of a single task.
type TaskState int

const (
	Queued TaskState = iota
	Running
	Succeeded
	Failed
	Skipped
)

// Task describes a single installable unit with dependencies and
// resource requirements.
type Task struct {
	ID        string
	Label     string
	Critical  bool
	DependsOn []string   // task IDs that must complete first
	Resources []Resource // exclusive resources needed during execution
	// Timeout caps how long Run may execute. Zero means use the
	// scheduler default (long enough for package-manager installs
	// but shorter than a cold cargo build). Override per-task for
	// known slow work — cargo crates, headless nvim plugin syncs —
	// so a 10-minute default doesn't kill them with an opaque
	// "signal: killed".
	Timeout time.Duration
	Run     func(ctx context.Context) error
	// BatchPeers lists task IDs that are installed together with this
	// task in a single shared package-manager invocation (e.g., one
	// `brew install a b c` for several tools). When a task in a batch
	// acquires its resource and starts, the scheduler also emits
	// TaskStartedMsg for every peer so the UI reflects that every
	// member of the batch is actively being installed — not just the
	// one whose goroutine won the resource race. Each task ID is
	// announced at most once per run.
	BatchPeers []string
}

// Event is the sealed interface implemented by every scheduler
// message. Using a typed channel instead of `chan any` lets the TUI
// switch be compile-time exhaustive (switch over Event will error
// if a new variant is added without handling it).
type Event interface {
	isEngineEvent()
}

// TaskStartedMsg is sent when a task begins execution.
type TaskStartedMsg struct {
	ID    string
	Label string
}

// The four isEngineEvent marker methods below report 0% in `go test
// -cover` because empty function bodies have no countable statements,
// even though the runtime dispatches them (verified by
// scheduler_invariants_test.go::TestEventMarkers). This is a Go
// cover-tool quirk, not missing test coverage — do not "fix" it by
// adding a body just to bump the number.
func (TaskStartedMsg) isEngineEvent() {}

// TaskDoneMsg is sent when a task finishes (success or failure).
type TaskDoneMsg struct {
	ID       string
	Label    string
	Err      error
	Critical bool
}

func (TaskDoneMsg) isEngineEvent() {}

// TaskSkippedMsg is sent when a task is skipped due to a failed dependency.
type TaskSkippedMsg struct {
	ID     string
	Label  string
	Reason string
}

func (TaskSkippedMsg) isEngineEvent() {}

// AllDoneMsg is sent after all tasks have completed, failed, or been skipped.
type AllDoneMsg struct{}

func (AllDoneMsg) isEngineEvent() {}

// BatchProgressMsg reports per-item progress inside a shared
// package-manager batch (e.g., brew `==> Pouring <name>` lines from
// one `brew install a b c …` invocation). The scheduler does not
// generate these itself; Task.Run closures emit them via the
// context-bound emitter so the UI can flip individual tools to done
// while the shared command is still running, instead of waiting for
// the whole batch to finish. ID is the engine task ID of the
// completed item (same namespace as TaskDoneMsg.ID).
type BatchProgressMsg struct {
	ID    string
	Label string
	Phase string // "done" is currently the only phase emitted
}

func (BatchProgressMsg) isEngineEvent() {}

// emitterKey is the context key used to carry the scheduler's event
// emitter into Task.Run closures. Unexported so callers cannot
// inject a fake emitter and bypass the scheduler's send-with-cancel
// semantics; use EmitFromContext to retrieve.
type emitterKey struct{}

// WithEmitter returns a child context carrying emit, which Task.Run
// closures can retrieve via EmitFromContext to send custom events
// (BatchProgressMsg today) on the same channel the scheduler uses
// for TaskStartedMsg / TaskDoneMsg. Called by the scheduler;
// external callers should not invoke this directly.
func WithEmitter(ctx context.Context, emit func(Event)) context.Context {
	if emit == nil {
		return ctx
	}
	return context.WithValue(ctx, emitterKey{}, emit)
}

// EmitFromContext retrieves the event emitter attached by the
// scheduler. Returns a no-op when absent so call sites never need
// a nil check — calling the returned func is always safe.
func EmitFromContext(ctx context.Context) func(Event) {
	if ctx == nil {
		return func(Event) {}
	}
	if emit, ok := ctx.Value(emitterKey{}).(func(Event)); ok && emit != nil {
		return emit
	}
	return func(Event) {}
}
