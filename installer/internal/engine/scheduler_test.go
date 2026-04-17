package engine

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// collectEvents drains the event channel and returns all messages
// (excluding the final AllDoneMsg).
func collectEvents(ch <-chan Event) []Event {
	var msgs []Event
	for msg := range ch {
		if _, ok := msg.(AllDoneMsg); ok {
			continue
		}
		msgs = append(msgs, msg)
	}
	return msgs
}

func TestEmptyTaskList(t *testing.T) {
	ch := Run(context.Background(), nil, 5)
	msgs := collectEvents(ch)
	if len(msgs) != 0 {
		t.Errorf("expected no events, got %d", len(msgs))
	}
}

func TestSingleTask(t *testing.T) {
	tasks := []Task{
		{
			ID:    "a",
			Label: "Task A",
			Run:   func(_ context.Context) error { return nil },
		},
	}
	ch := Run(context.Background(), tasks, 5)
	msgs := collectEvents(ch)

	var started, done int
	for _, msg := range msgs {
		switch msg.(type) {
		case TaskStartedMsg:
			started++
		case TaskDoneMsg:
			done++
		}
	}
	if started != 1 || done != 1 {
		t.Errorf("expected 1 started + 1 done, got %d + %d",
			started, done)
	}
}

func TestLinearDependencyChain(t *testing.T) {
	// A → B → C: must execute in order.
	var order []string
	var mu sync.Mutex

	record := func(name string) func(context.Context) error {
		return func(_ context.Context) error {
			mu.Lock()
			order = append(order, name)
			mu.Unlock()
			return nil
		}
	}

	tasks := []Task{
		{ID: "a", Label: "A", Run: record("a")},
		{ID: "b", Label: "B", DependsOn: []string{"a"}, Run: record("b")},
		{ID: "c", Label: "C", DependsOn: []string{"b"}, Run: record("c")},
	}

	ch := Run(context.Background(), tasks, 5)
	collectEvents(ch)

	mu.Lock()
	defer mu.Unlock()
	if len(order) != 3 || order[0] != "a" || order[1] != "b" || order[2] != "c" {
		t.Errorf("expected [a b c], got %v", order)
	}
}

func TestDiamondDependency(t *testing.T) {
	// A → B, A → C, B+C → D
	var dStart, dEnd atomic.Int32
	var bDone, cDone atomic.Bool

	tasks := []Task{
		{ID: "a", Label: "A", Run: func(_ context.Context) error { return nil }},
		{
			ID: "b", Label: "B", DependsOn: []string{"a"},
			Run: func(_ context.Context) error {
				bDone.Store(true)
				return nil
			},
		},
		{
			ID: "c", Label: "C", DependsOn: []string{"a"},
			Run: func(_ context.Context) error {
				cDone.Store(true)
				return nil
			},
		},
		{
			ID: "d", Label: "D", DependsOn: []string{"b", "c"},
			Run: func(_ context.Context) error {
				dStart.Add(1)
				if !bDone.Load() || !cDone.Load() {
					return errors.New("D ran before B or C completed")
				}
				dEnd.Add(1)
				return nil
			},
		},
	}

	ch := Run(context.Background(), tasks, 5)
	msgs := collectEvents(ch)

	if dStart.Load() != 1 {
		t.Errorf("D should have started exactly once, got %d", dStart.Load())
	}
	if dEnd.Load() != 1 {
		t.Errorf("D should have completed, got %d", dEnd.Load())
	}

	// Check no errors in done messages.
	for _, msg := range msgs {
		if dm, ok := msg.(TaskDoneMsg); ok && dm.Err != nil {
			t.Errorf("task %s failed: %v", dm.ID, dm.Err)
		}
	}
}

func TestFailedTaskSkipsDependents(t *testing.T) {
	tasks := []Task{
		{
			ID: "a", Label: "A",
			Run: func(_ context.Context) error {
				return errors.New("fail")
			},
		},
		{
			ID: "b", Label: "B", DependsOn: []string{"a"},
			Run: func(_ context.Context) error {
				t.Error("B should not have run")
				return nil
			},
		},
		{
			ID: "c", Label: "C", DependsOn: []string{"b"},
			Run: func(_ context.Context) error {
				t.Error("C should not have run")
				return nil
			},
		},
	}

	ch := Run(context.Background(), tasks, 5)
	msgs := collectEvents(ch)

	var skipped int
	for _, msg := range msgs {
		if _, ok := msg.(TaskSkippedMsg); ok {
			skipped++
		}
	}
	if skipped != 2 {
		t.Errorf("expected 2 skipped tasks, got %d", skipped)
	}
}

func TestUnknownDependencyReportedLoudly(t *testing.T) {
	// Task B depends on "nonexistent" — used to be silently stripped,
	// now must emit a critical TaskDoneMsg so bad task wiring is
	// visible instead of producing wrong task ordering.
	done := make(chan struct{})
	var events []Event
	go func() {
		tasks := []Task{
			{
				ID: "b", Label: "B", DependsOn: []string{"nonexistent"},
				Run: func(_ context.Context) error {
					t.Error("task b must not run when graph is invalid")
					return nil
				},
			},
		}
		ch := Run(context.Background(), tasks, 5)
		events = collectEvents(ch)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("scheduler did not return on invalid task graph")
	}

	var sawCritical bool
	for _, ev := range events {
		if msg, ok := ev.(TaskDoneMsg); ok && msg.Critical && msg.Err != nil {
			sawCritical = true
			break
		}
	}
	if !sawCritical {
		t.Errorf("expected critical TaskDoneMsg about invalid graph, got %v", events)
	}
}

func TestContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	started := make(chan struct{})
	tasks := []Task{
		{
			ID: "slow", Label: "Slow",
			Run: func(ctx context.Context) error {
				close(started)
				<-ctx.Done()
				return ctx.Err()
			},
		},
		{
			ID: "never", Label: "Never", DependsOn: []string{"slow"},
			Run: func(_ context.Context) error {
				t.Error("should not have run after cancellation")
				return nil
			},
		},
	}

	ch := Run(ctx, tasks, 5)

	// Wait for the slow task to start, then cancel.
	<-started
	cancel()

	// Drain events — should complete promptly.
	done := make(chan struct{})
	go func() {
		collectEvents(ch)
		close(done)
	}()

	select {
	case <-done:
		// Success.
	case <-time.After(5 * time.Second):
		t.Fatal("engine did not shut down after context cancellation")
	}
}

func TestResourceSemaphoreSerializes(t *testing.T) {
	// Two tasks sharing ResDpkg should not run concurrently.
	var concurrent atomic.Int32
	var maxConcurrent atomic.Int32

	run := func(_ context.Context) error {
		cur := concurrent.Add(1)
		for {
			old := maxConcurrent.Load()
			if cur <= old || maxConcurrent.CompareAndSwap(old, cur) {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
		concurrent.Add(-1)
		return nil
	}

	tasks := []Task{
		{ID: "a", Label: "A", Resources: []Resource{ResDpkg}, Run: run},
		{ID: "b", Label: "B", Resources: []Resource{ResDpkg}, Run: run},
		{ID: "c", Label: "C", Resources: []Resource{ResDpkg}, Run: run},
	}

	ch := Run(context.Background(), tasks, 5)
	collectEvents(ch)

	if maxConcurrent.Load() > 1 {
		t.Errorf(
			"expected max 1 concurrent apt task, got %d",
			maxConcurrent.Load(),
		)
	}
}

func TestParallelIndependentTasks(t *testing.T) {
	// Three independent tasks with maxWorkers=3 should run in parallel.
	var concurrent atomic.Int32
	var maxConcurrent atomic.Int32
	barrier := make(chan struct{})

	run := func(_ context.Context) error {
		cur := concurrent.Add(1)
		for {
			old := maxConcurrent.Load()
			if cur <= old || maxConcurrent.CompareAndSwap(old, cur) {
				break
			}
		}
		<-barrier
		concurrent.Add(-1)
		return nil
	}

	tasks := []Task{
		{ID: "a", Label: "A", Run: run},
		{ID: "b", Label: "B", Run: run},
		{ID: "c", Label: "C", Run: run},
	}

	ch := Run(context.Background(), tasks, 3)

	// Wait for all tasks to start.
	time.Sleep(100 * time.Millisecond)
	close(barrier)
	collectEvents(ch)

	if maxConcurrent.Load() < 2 {
		t.Errorf(
			"expected >=2 concurrent tasks, got %d",
			maxConcurrent.Load(),
		)
	}
}

func TestDiamondFailureRace(t *testing.T) {
	// Diamond DAG: A → B (fails), A → C (succeeds), B+C → D.
	// D should be skipped exactly once with no panic.
	// Run multiple iterations to maximize race window.
	for i := 0; i < 50; i++ {
		gate := make(chan struct{})
		tasks := []Task{
			{
				ID: "a", Label: "A",
				Run: func(_ context.Context) error { return nil },
			},
			{
				ID: "b", Label: "B", DependsOn: []string{"a"},
				Run: func(_ context.Context) error {
					<-gate
					return errors.New("fail")
				},
			},
			{
				ID: "c", Label: "C", DependsOn: []string{"a"},
				Run: func(_ context.Context) error {
					<-gate
					return nil
				},
			},
			{
				ID: "d", Label: "D", DependsOn: []string{"b", "c"},
				Run: func(_ context.Context) error {
					t.Error("D should not have run")
					return nil
				},
			},
		}

		ch := Run(context.Background(), tasks, 4)
		// Release B and C simultaneously to maximize race window.
		close(gate)
		msgs := collectEvents(ch)

		var dSkipped, dDone int
		for _, msg := range msgs {
			switch m := msg.(type) {
			case TaskSkippedMsg:
				if m.ID == "d" {
					dSkipped++
				}
			case TaskDoneMsg:
				if m.ID == "d" {
					dDone++
				}
			}
		}
		if dSkipped != 1 {
			t.Fatalf("iter %d: D skipped %d times (want 1)",
				i, dSkipped)
		}
		if dDone != 0 {
			t.Fatalf("iter %d: D ran %d times (want 0)",
				i, dDone)
		}
	}
}

func TestCycleDetection(t *testing.T) {
	// A depends on B, B depends on A — mutual cycle.
	tasks := []Task{
		{
			ID: "a", Label: "A", DependsOn: []string{"b"},
			Run: func(_ context.Context) error { return nil },
		},
		{
			ID: "b", Label: "B", DependsOn: []string{"a"},
			Run: func(_ context.Context) error { return nil },
		},
	}

	done := make(chan struct{})
	go func() {
		ch := Run(context.Background(), tasks, 5)
		msgs := collectEvents(ch)

		var foundCycleErr bool
		for _, msg := range msgs {
			if dm, ok := msg.(TaskDoneMsg); ok && dm.Err != nil {
				if strings.Contains(dm.Err.Error(), "cycle") {
					foundCycleErr = true
				}
			}
		}
		if !foundCycleErr {
			t.Error("expected cycle detection error")
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("cycle caused deadlock instead of error")
	}
}

func TestSelfCycleDetection(t *testing.T) {
	tasks := []Task{
		{
			ID: "a", Label: "A", DependsOn: []string{"a"},
			Run: func(_ context.Context) error { return nil },
		},
	}

	done := make(chan struct{})
	go func() {
		ch := Run(context.Background(), tasks, 5)
		msgs := collectEvents(ch)

		var foundCycleErr bool
		for _, msg := range msgs {
			if dm, ok := msg.(TaskDoneMsg); ok && dm.Err != nil {
				if strings.Contains(dm.Err.Error(), "cycle") {
					foundCycleErr = true
				}
			}
		}
		if !foundCycleErr {
			t.Error("expected cycle detection error for self-cycle")
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("self-cycle caused deadlock instead of error")
	}
}

func TestResourceSemaphoreNoLeakOnCancel(t *testing.T) {
	// "blocker" holds ResCargo. "multi" needs ResDpkg + ResCargo.
	// "setup" gates "multi" so it only dispatches after blocker
	// has acquired ResCargo. Cancel context while "multi" is
	// blocked waiting for ResCargo.
	ctx, cancel := context.WithCancel(context.Background())

	blockerRunning := make(chan struct{})
	tasks := []Task{
		{
			ID: "blocker", Label: "Blocker",
			Resources: []Resource{ResCargo},
			Run: func(ctx context.Context) error {
				close(blockerRunning)
				<-ctx.Done()
				return ctx.Err()
			},
		},
		{
			ID: "setup", Label: "Setup",
			Run: func(_ context.Context) error {
				// Wait until blocker is running (has ResCargo).
				<-blockerRunning
				return nil
			},
		},
		{
			ID: "multi", Label: "Multi",
			DependsOn: []string{"setup"},
			Resources: []Resource{ResDpkg, ResCargo},
			Run: func(_ context.Context) error {
				t.Error("multi should not have run")
				return nil
			},
		},
	}

	ch := Run(ctx, tasks, 5)
	// Wait for blocker to confirm it holds ResCargo.
	<-blockerRunning
	// Give multi time to acquire ResDpkg and block on ResCargo.
	time.Sleep(50 * time.Millisecond)
	cancel()

	done := make(chan struct{})
	go func() {
		collectEvents(ch)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("engine did not shut down — possible semaphore leak")
	}
}
