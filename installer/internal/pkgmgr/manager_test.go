package pkgmgr

import (
	"errors"
	"reflect"
	"testing"
)

// TestDedupeNames covers the mapping+dedupe step each package
// manager uses before building its single shell invocation.
// Preservation of insertion order matters for mock-runner tests
// that assert the exact command.
func TestDedupeNames(t *testing.T) {
	mapper := func(g string) []string {
		switch g {
		case "nodejs":
			return []string{"nodejs", "npm"}
		case "fd":
			return []string{"fd-find"}
		case "bat":
			return []string{"bat"}
		case "build-essential-macos":
			return nil
		}
		return []string{g}
	}

	cases := []struct {
		name     string
		generics []string
		want     []string
	}{
		{
			name:     "single generic with fan-out",
			generics: []string{"nodejs"},
			want:     []string{"nodejs", "npm"},
		},
		{
			name:     "multiple generics, order preserved",
			generics: []string{"fd", "bat", "nodejs"},
			want:     []string{"fd-find", "bat", "nodejs", "npm"},
		},
		{
			name:     "duplicate generic",
			generics: []string{"fd", "fd"},
			want:     []string{"fd-find"},
		},
		{
			name:     "duplicate mapped name across generics",
			generics: []string{"nodejs", "npm"},
			want:     []string{"nodejs", "npm"},
		},
		{
			name:     "mapper returns nil (skip)",
			generics: []string{"build-essential-macos", "fd"},
			want:     []string{"fd-find"},
		},
		{
			name:     "empty input",
			generics: nil,
			want:     nil,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := dedupeNames(mapper, c.generics)
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("dedupeNames() = %v, want %v", got, c.want)
			}
		})
	}
}

// TestAttribute covers the three post-batch outcomes that the
// orchestrator's fan-out relies on: total failure propagates the
// classified error (caller can errors.Is for typed sentinels);
// partial failure wraps in BatchFailure with the failed generic
// names; shell-fail-but-everything-installed treats as success to
// avoid double-install loops.
func TestAttribute(t *testing.T) {
	ErrShell := errors.New("exit status 100")
	ErrClassified := errors.New("nala install: " + ErrShell.Error())

	t.Run("total failure returns classified as-is", func(t *testing.T) {
		isInstalled := func(string) bool { return false }
		got := attribute(ErrClassified, []string{"a", "b"}, isInstalled)
		if !errors.Is(got, ErrClassified) {
			t.Errorf("expected classified error preserved, got %v", got)
		}
		var bf *BatchFailure
		if errors.As(got, &bf) {
			t.Errorf("total failure should NOT wrap in BatchFailure, got %T", got)
		}
	})

	t.Run("partial failure wraps in BatchFailure", func(t *testing.T) {
		// "a" installed despite shell error; "b", "c" did not.
		isInstalled := func(g string) bool { return g == "a" }
		got := attribute(ErrClassified, []string{"a", "b", "c"}, isInstalled)
		var bf *BatchFailure
		if !errors.As(got, &bf) {
			t.Fatalf("expected BatchFailure, got %T: %v", got, got)
		}
		if !reflect.DeepEqual(bf.FailedNames, []string{"b", "c"}) {
			t.Errorf("FailedNames = %v, want [b c]", bf.FailedNames)
		}
		if !errors.Is(got, ErrClassified) {
			t.Errorf("BatchFailure must unwrap to classified error")
		}
	})

	t.Run("shell failed but all installed → nil", func(t *testing.T) {
		isInstalled := func(string) bool { return true }
		got := attribute(ErrClassified, []string{"a", "b"}, isInstalled)
		if got != nil {
			t.Errorf("expected nil, got %v", got)
		}
	})

	t.Run("nil classified error → nil", func(t *testing.T) {
		got := attribute(nil, []string{"a"}, func(string) bool { return false })
		if got != nil {
			t.Errorf("expected nil passthrough, got %v", got)
		}
	})
}

// TestBatchFailureErrorChain ensures errors.Is on a BatchFailure
// still reaches the underlying typed sentinel (ErrDpkgInterrupted
// etc.) — registry.ExecuteInstall depends on this for its doctor
// retry path.
func TestBatchFailureErrorChain(t *testing.T) {
	inner := ErrDpkgInterrupted
	bf := &BatchFailure{
		FailedNames: []string{"ffmpeg"},
		Wrapped:     inner,
	}
	if !errors.Is(bf, ErrDpkgInterrupted) {
		t.Errorf("errors.Is(BatchFailure, ErrDpkgInterrupted) must be true")
	}
	if errors.Is(bf, ErrAptFatal) {
		t.Errorf("errors.Is(BatchFailure, ErrAptFatal) must be false for this wrap chain")
	}
}
