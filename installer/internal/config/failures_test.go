package config

import (
	"errors"
	"strings"
	"testing"
)

func TestTrackedFailures(t *testing.T) {
	f := NewTrackedFailures()
	if f == nil {
		t.Fatal("NewTrackedFailures returned nil")
	}
	f.Record("Zsh", "compile plugins", errors.New("boom"))
	f.Record("Tmux", "install TPM", errors.New("no network"))
	f.Record("Ignored", "nil", nil)

	snap := f.Snapshot()
	if len(snap) != 2 {
		t.Fatalf("Snapshot len = %d, want 2", len(snap))
	}
	formatted := f.Format()
	for _, want := range []string{"Zsh", "compile plugins", "Tmux", "install TPM"} {
		if !strings.Contains(formatted, want) {
			t.Fatalf("Format missing %q:\n%s", want, formatted)
		}
	}

	var nilFailures *TrackedFailures
	if got := nilFailures.Snapshot(); got != nil {
		t.Fatalf("nil Snapshot = %#v, want nil", got)
	}
	if got := nilFailures.Format(); got != "" {
		t.Fatalf("nil Format = %q, want empty", got)
	}
}
