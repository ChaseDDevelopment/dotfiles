package config

import (
	"fmt"
	"sync"
)

// TrackedFailure is one best-effort step that didn't succeed.
// "Best-effort" means the step's failure doesn't abort the component
// setup, but the user deserves to see it on the summary screen so
// it isn't silently buried in install.log.
type TrackedFailure struct {
	Component string
	Step      string
	Err       error
}

// TrackedFailures accumulates best-effort failures across the
// lifetime of one install/update run. Safe to use from multiple
// goroutines — the scheduler may run components in parallel.
type TrackedFailures struct {
	mu   sync.Mutex
	rows []TrackedFailure
}

// NewTrackedFailures returns an empty, thread-safe failure collector.
func NewTrackedFailures() *TrackedFailures {
	return &TrackedFailures{}
}

// Record appends a failure. Safe to call concurrently.
func (f *TrackedFailures) Record(component, step string, err error) {
	if f == nil || err == nil {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rows = append(f.rows, TrackedFailure{
		Component: component,
		Step:      step,
		Err:       err,
	})
}

// Snapshot returns a copy of recorded failures. Safe to call
// concurrently with Record.
func (f *TrackedFailures) Snapshot() []TrackedFailure {
	if f == nil {
		return nil
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := make([]TrackedFailure, len(f.rows))
	copy(cp, f.rows)
	return cp
}

// Format returns a multi-line human-readable summary suitable for
// display on the summary screen.
func (f *TrackedFailures) Format() string {
	snap := f.Snapshot()
	if len(snap) == 0 {
		return ""
	}
	var out string
	for _, row := range snap {
		out += fmt.Sprintf(
			"  • %s — %s: %v\n", row.Component, row.Step, row.Err,
		)
	}
	return out
}
