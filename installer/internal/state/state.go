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

// Save writes the state file to disk.
func (s *Store) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Updated = time.Now()

	if err := os.MkdirAll(
		filepath.Dir(s.path), 0o755,
	); err != nil {
		return err
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.path, data, 0o644)
}

// LookupTool returns the record for a tool, if it exists.
func (s *Store) LookupTool(name string) (ToolRecord, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.Tools[name]
	return r, ok
}
