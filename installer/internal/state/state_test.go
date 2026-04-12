package state

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestNewStore_Empty(t *testing.T) {
	s := NewStore("/tmp/test-state.json")
	if s == nil {
		t.Fatal("NewStore returned nil")
	}
	if s.Tools == nil {
		t.Fatal("Tools map is nil")
	}
	if len(s.Tools) != 0 {
		t.Errorf("expected 0 tools, got %d", len(s.Tools))
	}
}

func TestDefaultPath(t *testing.T) {
	t.Setenv("HOME", "/fakehome")
	got := DefaultPath()
	want := "/fakehome/.local/share/dotsetup/state.json"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRecordInstall(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "state.json"))

	s.RecordInstall("zsh", "5.9", "brew")

	rec, ok := s.LookupTool("zsh")
	if !ok {
		t.Fatal("LookupTool returned false for recorded tool")
	}
	if rec.Name != "zsh" {
		t.Errorf("Name = %q, want %q", rec.Name, "zsh")
	}
	if rec.Version != "5.9" {
		t.Errorf("Version = %q, want %q", rec.Version, "5.9")
	}
	if rec.Method != "brew" {
		t.Errorf("Method = %q, want %q", rec.Method, "brew")
	}
	if rec.InstalledAt.IsZero() {
		t.Error("InstalledAt should not be zero")
	}
}

func TestRecordInstall_Overwrite(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "state.json"))

	s.RecordInstall("zsh", "5.8", "apt")
	s.RecordInstall("zsh", "5.9", "brew")

	rec, ok := s.LookupTool("zsh")
	if !ok {
		t.Fatal("LookupTool returned false")
	}
	if rec.Version != "5.9" {
		t.Errorf("Version = %q, want %q after overwrite", rec.Version, "5.9")
	}
	if rec.Method != "brew" {
		t.Errorf("Method = %q, want %q after overwrite", rec.Method, "brew")
	}
}

func TestLookupTool_NotFound(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "state.json"))

	_, ok := s.LookupTool("nonexistent")
	if ok {
		t.Error("LookupTool should return false for missing tool")
	}
}

func TestSaveAndLoad_Roundtrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	// Create and save.
	s := NewStore(path)
	s.RecordInstall("tmux", "3.4", "brew")
	s.RecordInstall("nvim", "0.10.0", "github-release")

	if err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Verify file exists.
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("state file not created: %v", err)
	}

	// Load and verify roundtrip.
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if len(loaded.Tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(loaded.Tools))
	}

	tmux, ok := loaded.LookupTool("tmux")
	if !ok {
		t.Fatal("tmux not found after load")
	}
	if tmux.Version != "3.4" {
		t.Errorf("tmux version = %q, want %q", tmux.Version, "3.4")
	}
	if tmux.Method != "brew" {
		t.Errorf("tmux method = %q, want %q", tmux.Method, "brew")
	}

	nvim, ok := loaded.LookupTool("nvim")
	if !ok {
		t.Fatal("nvim not found after load")
	}
	if nvim.Version != "0.10.0" {
		t.Errorf(
			"nvim version = %q, want %q",
			nvim.Version, "0.10.0",
		)
	}

	if loaded.Updated.IsZero() {
		t.Error("Updated timestamp should not be zero after save")
	}
}

func TestLoad_MissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent.json")

	s, err := Load(path)
	if err != nil {
		t.Fatalf("Load should not error for missing file: %v", err)
	}
	if s == nil {
		t.Fatal("Load returned nil store")
	}
	if len(s.Tools) != 0 {
		t.Errorf("expected 0 tools for missing file, got %d", len(s.Tools))
	}
}

func TestLoad_CorruptFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	// Write invalid JSON.
	os.WriteFile(path, []byte("{invalid json!!!!}"), 0o644)

	// Corrupt files must return ErrCorrupt so the caller can
	// preserve the existing file instead of silently wiping it.
	s, err := Load(path)
	if err == nil {
		t.Fatal("Load should return error for corrupt file")
	}
	if !errors.Is(err, ErrCorrupt) {
		t.Errorf("error should wrap ErrCorrupt, got: %v", err)
	}
	if s != nil {
		t.Error("Load should return nil store on corrupt file")
	}
}

func TestLoad_EmptyJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	// Write valid JSON but with null tools.
	os.WriteFile(path, []byte(`{"tools":null}`), 0o644)

	s, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.Tools == nil {
		t.Error("Tools map should be initialized, not nil")
	}
}

func TestSave_ErrorOnBadPath(t *testing.T) {
	dir := t.TempDir()

	// Create a read-only parent to force MkdirAll to fail.
	readOnly := filepath.Join(dir, "readonly")
	os.MkdirAll(readOnly, 0o755)
	os.Chmod(readOnly, 0o444)
	t.Cleanup(func() { os.Chmod(readOnly, 0o755) })

	path := filepath.Join(
		readOnly, "subdir", "state.json",
	)
	s := NewStore(path)
	s.RecordInstall("test", "1.0", "brew")

	err := s.Save()
	if err == nil {
		t.Error("expected error when parent dir is read-only")
	}
}

func TestLoad_ReadError(t *testing.T) {
	dir := t.TempDir()

	// Create a directory where the state file should be, making
	// ReadFile fail with a different error than IsNotExist.
	path := filepath.Join(dir, "state.json")
	os.MkdirAll(path, 0o755) // path is a directory, not a file

	_, err := Load(path)
	if err == nil {
		t.Error("expected error when state path is a directory")
	}
}

func TestSave_CreatesParentDirs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(
		dir, "nested", "deep", "state.json",
	)

	s := NewStore(path)
	s.RecordInstall("git", "2.45", "brew")

	if err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Errorf("state file not created in nested dir: %v", err)
	}
}

func TestSave_ValidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	s := NewStore(path)
	s.RecordInstall("fd", "10.1", "brew")
	s.Save()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("saved JSON is invalid: %v", err)
	}

	if _, ok := raw["tools"]; !ok {
		t.Error("saved JSON missing 'tools' key")
	}
	if _, ok := raw["updated"]; !ok {
		t.Error("saved JSON missing 'updated' key")
	}
}

func TestMultipleRecordAndLookup(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "state.json"))

	tools := []struct {
		name    string
		version string
		method  string
	}{
		{"zsh", "5.9", "brew"},
		{"tmux", "3.4", "brew"},
		{"nvim", "0.10.0", "github-release"},
		{"starship", "1.20", "script"},
		{"atuin", "18.0", "brew"},
	}

	for _, tool := range tools {
		s.RecordInstall(tool.name, tool.version, tool.method)
	}

	for _, tool := range tools {
		rec, ok := s.LookupTool(tool.name)
		if !ok {
			t.Errorf("tool %q not found", tool.name)
			continue
		}
		if rec.Version != tool.version {
			t.Errorf(
				"tool %q: version = %q, want %q",
				tool.name, rec.Version, tool.version,
			)
		}
		if rec.Method != tool.method {
			t.Errorf(
				"tool %q: method = %q, want %q",
				tool.name, rec.Method, tool.method,
			)
		}
	}
}

func TestSave_UpdatesTimestamp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	s := NewStore(path)
	if err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if s.Updated.IsZero() {
		t.Error("Updated should be set after Save")
	}
}

func TestLoad_PreservesInstalledAt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	s := NewStore(path)
	s.RecordInstall("bat", "0.24", "brew")
	if err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	original, _ := s.LookupTool("bat")

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	reloaded, ok := loaded.LookupTool("bat")
	if !ok {
		t.Fatal("bat not found after reload")
	}

	if !reloaded.InstalledAt.Equal(original.InstalledAt) {
		t.Errorf(
			"InstalledAt changed: %v -> %v",
			original.InstalledAt, reloaded.InstalledAt,
		)
	}
}

// TestSave_AtomicReplacesExisting confirms the atomic-rename path
// replaces an existing state.json rather than leaving a stray temp
// file. Regression guard for the pre-hardening non-atomic Save.
func TestSave_AtomicReplacesExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	s1 := NewStore(path)
	s1.RecordInstall("first", "1.0", "brew")
	if err := s1.Save(); err != nil {
		t.Fatalf("first Save: %v", err)
	}

	s2 := NewStore(path)
	s2.RecordInstall("second", "2.0", "brew")
	if err := s2.Save(); err != nil {
		t.Fatalf("second Save: %v", err)
	}

	// Parent dir should contain state.json plus no lingering tmp.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		name := e.Name()
		if name == "state.json" {
			continue
		}
		t.Errorf("unexpected leftover %q in state dir", name)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load after atomic save: %v", err)
	}
	if _, ok := loaded.LookupTool("second"); !ok {
		t.Fatal("second record not present after atomic save")
	}
}

// TestSave_PreservesOnMidWriteFailure simulates a failure between
// the rename staging and the final placement by setting the parent
// directory read-only — the rename fails, but the original file is
// preserved rather than truncated. This was the crash-corruption
// class that the pre-hardening os.WriteFile path allowed.
func TestSave_PreservesOnMidWriteFailure(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root ignores directory permissions; test is meaningless")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	good := NewStore(path)
	good.RecordInstall("keep", "1.0", "brew")
	if err := good.Save(); err != nil {
		t.Fatalf("initial Save: %v", err)
	}
	originalBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	// Strip write permission on the directory so os.Rename fails.
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })

	bad := NewStore(path)
	bad.RecordInstall("should-not-persist", "9.9", "brew")
	if err := bad.Save(); err == nil {
		t.Fatal("expected Save to fail when directory is read-only")
	}

	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(originalBytes) {
		t.Fatalf(
			"original state.json was mutated by failed Save; "+
				"before=%q after=%q", originalBytes, after,
		)
	}
}
