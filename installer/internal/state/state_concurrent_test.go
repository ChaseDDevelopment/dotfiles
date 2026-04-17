package state

import (
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
)

// TestStoreConcurrentRecordAndSave stresses the RecordInstall/Save
// pair with N goroutines. Post-conditions:
//   - No panic / data race under -race (the mu lock is required).
//   - Every tool name recorded is present in the final on-disk file.
//   - The file parses cleanly as JSON via Load (no torn writes
//     escaped the atomic-rename path).
//
// The race-detector in CI is the primary regression catch — the
// count assertion is a belt-and-braces guard in case a future
// refactor drops the lock in a non-obvious way.
func TestStoreConcurrentRecordAndSave(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	s := NewStore(path)

	const goroutines = 50
	const recordsPer = 10

	var saves atomic.Int32
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		worker := i
		go func() {
			defer wg.Done()
			for j := 0; j < recordsPer; j++ {
				name := "tool-" + strconv.Itoa(worker) +
					"-" + strconv.Itoa(j)
				s.RecordInstall(name, "1.0", "brew")
				// Every third goroutine also hammers Save to
				// exercise the locked read of s.Tools inside
				// json.MarshalIndent concurrent with other
				// writers.
				if worker%3 == 0 {
					if err := s.Save(); err != nil {
						t.Errorf("Save: %v", err)
						return
					}
					saves.Add(1)
				}
			}
		}()
	}
	wg.Wait()

	// Final save to flush the terminal state of every goroutine.
	if err := s.Save(); err != nil {
		t.Fatalf("final Save: %v", err)
	}

	// Load from disk and verify every name is present. Because
	// intermediate Save calls race with ongoing RecordInstall calls,
	// we rely on the FINAL save (after wg.Wait) having observed the
	// full map under s.mu.
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(loaded.Tools) != goroutines*recordsPer {
		t.Errorf(
			"loaded tool count = %d, want %d (%d concurrent Saves)",
			len(loaded.Tools),
			goroutines*recordsPer,
			saves.Load(),
		)
	}
	for i := 0; i < goroutines; i++ {
		for j := 0; j < recordsPer; j++ {
			name := "tool-" + strconv.Itoa(i) +
				"-" + strconv.Itoa(j)
			if _, ok := loaded.LookupTool(name); !ok {
				t.Errorf(
					"missing tool %q in final state", name,
				)
				return
			}
		}
	}
}

// TestStoreConcurrentSaveNoTempLeaks confirms that hammering Save
// from multiple goroutines does not leak ".state-*.json.tmp" files
// in the state directory. Each Save creates a temp file, then
// renames it over the target; a regression that dropped the rename
// or the defer-Remove would leave orphans behind.
func TestStoreConcurrentSaveNoTempLeaks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	s := NewStore(path)
	s.RecordInstall("seed", "1.0", "brew")

	const goroutines = 30
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			if err := s.Save(); err != nil {
				t.Errorf("Save: %v", err)
			}
		}()
	}
	wg.Wait()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() == "state.json" {
			continue
		}
		t.Errorf(
			"leftover file %q after concurrent Saves — "+
				"temp cleanup regressed",
			e.Name(),
		)
	}
}
