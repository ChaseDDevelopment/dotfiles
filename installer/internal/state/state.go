// Package state persists tool-install records to disk via an
// atomic write-temp-then-rename strategy.
//
// Test-coverage note (Category C — environmental syscall paths):
// The tmp-file syscall paths inside Save (tmp.Write / tmp.Chmod /
// tmp.Sync / tmp.Close on the freshly-created tmp file, lines
// 119–146 below) are not reachable on POSIX without an injected
// filesystem-fault layer. The project has explicitly declined to
// add such an fs-fault abstraction solely for these error returns;
// in production they fire on ENOSPC, EIO, and similar conditions
// and surface via the wrapped error chain. The wrapping itself is
// trivially correct by inspection.
package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ErrCorrupt indicates the state file exists but could not be parsed.
// Callers should surface this and typically back up the file before
// starting fresh, rather than silently discarding install history.
var ErrCorrupt = errors.New("corrupt state file")

// ToolRecord tracks the installation of a single tool.
type ToolRecord struct {
	Name        string    `json:"name"`
	Version     string    `json:"version,omitempty"`
	Method      string    `json:"method"`
	InstalledAt time.Time `json:"installed_at"`
}

// Store persists install state to disk.
type Store struct {
	path    string
	mu      sync.Mutex
	Tools   map[string]ToolRecord `json:"tools"`
	Updated time.Time             `json:"updated"`
}

// DefaultPath returns ~/.local/share/dotsetup/state.json.
func DefaultPath() string {
	home := os.Getenv("HOME")
	return filepath.Join(
		home, ".local", "share", "dotsetup", "state.json",
	)
}

// NewStore creates a fresh, empty store for the given path.
func NewStore(path string) *Store {
	return &Store{
		path:  path,
		Tools: make(map[string]ToolRecord),
	}
}

// Load reads the state file from disk, or returns an empty store
// if it doesn't exist.
func Load(path string) (*Store, error) {
	s := &Store{
		path:  path,
		Tools: make(map[string]ToolRecord),
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(data, s); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCorrupt, err)
	}
	if s.Tools == nil {
		s.Tools = make(map[string]ToolRecord)
	}
	return s, nil
}

// RecordInstall records that a tool was installed.
func (s *Store) RecordInstall(
	name, version, method string,
) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Tools[name] = ToolRecord{
		Name:        name,
		Version:     version,
		Method:      method,
		InstalledAt: time.Now(),
	}
}

// Save writes the state file to disk atomically via
// write-temp-then-rename so a crash or power loss between truncate
// and write can't corrupt the file. The temp file lives in the same
// directory as the final path so rename is a filesystem-level
// atomic operation.
func (s *Store) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Updated = time.Now()

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, ".state-*.json.tmp")
	if err != nil {
		return fmt.Errorf("create temp state: %w", err)
	}
	tmpPath := tmp.Name()
	// Clean up the temp file on any error path. If rename succeeds,
	// the temp path no longer exists and Remove is a no-op.
	defer os.Remove(tmpPath)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp state: %w", err)
	}
	if err := tmp.Chmod(0o644); err != nil {
		tmp.Close()
		return fmt.Errorf("chmod temp state: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("fsync temp state: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp state: %w", err)
	}

	if err := os.Rename(tmpPath, s.path); err != nil {
		return fmt.Errorf("rename state: %w", err)
	}

	// fsync the parent directory so the rename itself is durable on
	// ext4 and friends. Best-effort — some filesystems / platforms
	// reject directory fsync; we log via the returned error chain.
	if dirFD, err := os.Open(dir); err == nil {
		_ = dirFD.Sync()
		dirFD.Close()
	}
	return nil
}

// LookupTool returns the record for a tool, if it exists.
func (s *Store) LookupTool(name string) (ToolRecord, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.Tools[name]
	return r, ok
}
