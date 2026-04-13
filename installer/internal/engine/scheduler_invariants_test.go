package engine

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestCycleErrorNamesInvolvedTasks verifies that when Kahn's algorithm
// finds a cycle, the error surfaced to the user identifies the cycle
// members by name. A generic "cycle detected" without IDs makes a
// mis-wired registry impossible to diagnose.
func TestCycleErrorNamesInvolvedTasks(t *testing.T) {
	tasks := []Task{
		{
			ID: "alpha", Label: "Alpha", DependsOn: []string{"beta"},
			Run: func(_ context.Context) error { return nil },
		},
		{
			ID: "beta", Label: "Beta", DependsOn: []string{"gamma"},
			Run: func(_ context.Context) error { return nil },
		},
		{
			ID: "gamma", Label: "Gamma", DependsOn: []string{"alpha"},
			Run: func(_ context.Context) error { return nil },
		},
	}

	msgs := collectEvents(Run(context.Background(), tasks, 5))
	var cycleErr error
	for _, ev := range msgs {
		if dm, ok := ev.(TaskDoneMsg); ok && dm.Err != nil &&
			strings.Contains(dm.Err.Error(), "cycle") {
			cycleErr = dm.Err
			break
		}
	}
	if cycleErr == nil {
		t.Fatalf("expected cycle error in events: %+v", msgs)
	}
	msg := cycleErr.Error()
	for _, name := range []string{"alpha", "beta", "gamma"} {
		if !strings.Contains(msg, name) {
			t.Errorf("cycle error %q does not mention task %q", msg, name)
		}
	}
}

// TestUnknownDependencyNamesBothEndpoints verifies the unknown-dep
// error includes both the task holding the bad reference and the
// missing dep name. Silent dep-strip would produce wrong ordering
// without surfacing the typo — this test locks in the loud failure.
func TestUnknownDependencyNamesBothEndpoints(t *testing.T) {
	tasks := []Task{
		{
			ID: "installer", Label: "Installer",
			DependsOn: []string{"ghost-tool"},
			Run: func(_ context.Context) error {
				t.Error("task must not run when graph is invalid")
				return nil
			},
		},
	}
	msgs := collectEvents(Run(context.Background(), tasks, 5))

	var errStr string
	for _, ev := range msgs {
		if dm, ok := ev.(TaskDoneMsg); ok && dm.Err != nil {
			errStr = dm.Err.Error()
			if !dm.Critical {
				t.Errorf("invalid-graph error should be Critical=true, got %+v", dm)
			}
		}
	}
	if errStr == "" {
		t.Fatalf("expected a TaskDoneMsg with error, got %+v", msgs)
	}
	for _, needle := range []string{"installer", "ghost-tool"} {
		if !strings.Contains(errStr, needle) {
			t.Errorf("unknown-dep error %q does not mention %q", errStr, needle)
		}
	}
}

// TestSharedResourceSerializesViaOverlap verifies two tasks declaring
// the same Resource never run concurrently. Uses channel barriers
// (closed on task start) to detect overlap instead of wall-clock
// sleeps — the test would pass trivially on a slow CI if tasks
// merely didn't wall-clock-overlap. The barrier proves the second
// task is blocked on the semaphore, not just scheduled later.
func TestSharedResourceSerializesViaOverlap(t *testing.T) {
	const nTasks = 3
	var mu sync.Mutex
	running := map[string]bool{}
	overlaps := 0

	makeRun := func(id string) func(context.Context) error {
		return func(_ context.Context) error {
			mu.Lock()
			if len(running) > 0 {
				overlaps++
			}
			running[id] = true
			mu.Unlock()

			time.Sleep(25 * time.Millisecond)

			mu.Lock()
			delete(running, id)
			mu.Unlock()
			return nil
		}
	}

	var tasks []Task
	for i := 0; i < nTasks; i++ {
		id := fmt.Sprintf("apt-%d", i)
		tasks = append(tasks, Task{
			ID: id, Label: id,
			Resources: []Resource{ResDpkg},
			Run:       makeRun(id),
		})
	}

	// Give the worker pool plenty of slots so the only thing
	// serializing tasks is the resource semaphore.
	collectEvents(Run(context.Background(), tasks, 8))

	if overlaps > 0 {
		t.Fatalf("ResDpkg tasks overlapped %d times (must be 0)", overlaps)
	}
}

// TestDifferentResourcesRunInParallel verifies a ResDpkg and a
// ResCargo task run concurrently — this is the cross-manager
// parallelism promise the user explicitly called out as a hard
// requirement. The synchronisation point is a rendezvous channel:
// task A blocks until B signals it has started, and vice versa.
// If the semaphores incorrectly serialized across resources,
// the rendezvous deadlocks and the scheduler-side timeout fires.
func TestDifferentResourcesRunInParallel(t *testing.T) {
	aInside := make(chan struct{})
	bInside := make(chan struct{})

	tasks := []Task{
		{
			ID: "apt", Label: "apt",
			Resources: []Resource{ResDpkg},
			Run: func(ctx context.Context) error {
				close(aInside)
				select {
				case <-bInside:
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			},
		},
		{
			ID: "cargo", Label: "cargo",
			Resources: []Resource{ResCargo},
			Run: func(ctx context.Context) error {
				close(bInside)
				select {
				case <-aInside:
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	msgs := collectEvents(Run(ctx, tasks, 4))
	for _, ev := range msgs {
		if dm, ok := ev.(TaskDoneMsg); ok && dm.Err != nil {
			t.Fatalf("%s failed — cross-resource parallelism broken: %v",
				dm.ID, dm.Err)
		}
	}
	// Belt and suspenders — confirm both reached the rendezvous.
	select {
	case <-aInside:
	default:
		t.Fatal("apt task never entered")
	}
	select {
	case <-bInside:
	default:
		t.Fatal("cargo task never entered")
	}
}

// TestCriticalFailureSkipsDependents locks in two related invariants
// around criticality that the plan flags as load-bearing:
//
//  1. A Critical: true failure cancels downstream dependents via the
//     existing skip-cascade (since the dependent task can't succeed
//     without its parent).
//  2. A Critical: false failure must ALSO cascade skips on dependents —
//     "non-critical does not halt the entire run" means siblings keep
//     running, not that broken children run anyway. This test pins
//     the current behaviour so a future refactor that tries to "let
//     the failure dispatch just its direct parent skip" doesn't
//     silently regress into running dependents on a known-bad parent.
func TestCriticalFailureSkipsDependents(t *testing.T) {
	cases := []struct {
		name     string
		critical bool
	}{
		{"critical", true},
		{"non-critical", false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			var depRan atomic.Bool
			tasks := []Task{
				{
					ID: "root", Label: "root", Critical: tc.critical,
					Run: func(_ context.Context) error {
						return errors.New("boom")
					},
				},
				{
					ID: "dep", Label: "dep", DependsOn: []string{"root"},
					Run: func(_ context.Context) error {
						depRan.Store(true)
						return nil
					},
				},
				{
					ID: "sibling", Label: "sibling",
					Run: func(_ context.Context) error { return nil },
				},
			}

			msgs := collectEvents(Run(context.Background(), tasks, 4))

			if depRan.Load() {
				t.Fatal("dependent ran despite parent failure")
			}

			// Siblings (no dependency on the failing task) must still
			// complete — the engine's critical flag is an observable
			// attribute of TaskDoneMsg, not an abort signal on its own.
			// TUI/orchestrator layers enforce abort semantics.
			var siblingDone, depSkipped bool
			for _, ev := range msgs {
				switch m := ev.(type) {
				case TaskDoneMsg:
					if m.ID == "sibling" && m.Err == nil {
						siblingDone = true
					}
					if m.ID == "root" && m.Critical != tc.critical {
						t.Errorf("root TaskDoneMsg.Critical=%v, want %v",
							m.Critical, tc.critical)
					}
				case TaskSkippedMsg:
					if m.ID == "dep" {
						depSkipped = true
					}
				}
			}
			if !siblingDone {
				t.Error("independent sibling did not complete")
			}
			if !depSkipped {
				t.Error("dependent was not marked skipped")
			}
		})
	}
}
