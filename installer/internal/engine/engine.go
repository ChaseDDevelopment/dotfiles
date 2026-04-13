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
