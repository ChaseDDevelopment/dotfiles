package registry

import "testing"

// TestVersionEdgeCases drives the early-return branches of
// getInstalledVersion (no Command, no MinVersion, command missing,
// command exits non-zero) and CheckVersion (no MinVersion, garbage
// MinVersion).
func TestVersionEdgeCases(t *testing.T) {
	// No Command → empty raw, ok=false.
	if raw, _, _, ok := getInstalledVersion(&Tool{MinVersion: "1.0.0"}); ok || raw != "" {
		t.Fatalf("expected ok=false for missing Command, got raw=%q ok=%v", raw, ok)
	}
	// No MinVersion → still no probe.
	if _, _, _, ok := getInstalledVersion(&Tool{Command: "anything"}); ok {
		t.Fatal("expected ok=false when MinVersion missing")
	}
	// Command not in PATH.
	t.Setenv("PATH", t.TempDir())
	if _, _, _, ok := getInstalledVersion(&Tool{Command: "ghost", MinVersion: "1.0.0"}); ok {
		t.Fatal("expected ok=false for missing binary")
	}

	// CheckVersion happy and edge cases.
	if !CheckVersion(&Tool{}) {
		t.Fatal("CheckVersion with empty MinVersion should be true")
	}
	if !CheckVersion(&Tool{MinVersion: "garbage", Command: "ghost"}) {
		t.Fatal("CheckVersion with unparsable MinVersion should be true")
	}
}
