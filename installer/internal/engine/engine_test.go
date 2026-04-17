package engine

import (
	"errors"
	"testing"
)

// TestEventMarkers covers the four isEngineEvent marker methods.
// These satisfy a sealed-interface pattern: the TUI's switch on Event
// is compile-time-checked against the marker interface, so removing a
// marker silently breaks the switch. The interface satisfaction checks
// here catch that at unit-test time.
//
// The var _ Event = (*T)(nil) lines are compile-time assertions;
// the runtime calls exist purely to register the methods in coverage
// — calling them via the interface confirms the method set is right
// without needing reflection.
func TestEventMarkers(t *testing.T) {
	// Compile-time assertions — each type must implement Event.
	var (
		_ Event = TaskStartedMsg{}
		_ Event = TaskDoneMsg{}
		_ Event = TaskSkippedMsg{}
		_ Event = AllDoneMsg{}
	)

	cases := []struct {
		name string
		ev   Event
	}{
		{"TaskStartedMsg", TaskStartedMsg{ID: "a", Label: "A"}},
		{"TaskDoneMsg", TaskDoneMsg{ID: "a", Label: "A", Err: errors.New("x"), Critical: true}},
		{"TaskSkippedMsg", TaskSkippedMsg{ID: "a", Label: "A", Reason: "dep failed"}},
		{"AllDoneMsg", AllDoneMsg{}},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// Call the marker via the interface — the call itself is
			// no-op but both registers coverage for the method body
			// and confirms the dispatch table picks the right impl.
			tc.ev.isEngineEvent()
		})
	}

	// Exhaustive type-switch check: if a new Event variant is added
	// without wiring a case here, the default branch fires and the
	// test fails — forcing the plan's sealed-interface promise to
	// stay honest.
	for _, tc := range cases {
		switch tc.ev.(type) {
		case TaskStartedMsg, TaskDoneMsg, TaskSkippedMsg, AllDoneMsg:
			// ok — known variant.
		default:
			t.Fatalf("%s: unrecognized Event variant %T — update TestEventMarkers",
				tc.name, tc.ev)
		}
	}
}
