package executor

import (
	"fmt"
	"os"
	"sync"
	"time"
)

// LogFile manages the install log file with thread-safe writes.
type LogFile struct {
	path string
	file *os.File
	mu   sync.Mutex
}

// NewLogFile opens path with O_CREATE|O_TRUNC|O_WRONLY so every
// installer run starts with a fresh log (no append across runs).
func NewLogFile(path string) (*LogFile, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("create log file %s: %w", path, err)
	}
	lf := &LogFile{path: path, file: f}
	lf.Write("Dotsetup install log started")
	return lf, nil
}

// Write appends a timestamped line to the log.
func (l *LogFile) Write(msg string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file == nil {
		return
	}
	ts := time.Now().Format("2006-01-02 15:04:05")
	if _, err := fmt.Fprintf(l.file, "[%s] %s\n", ts, msg); err != nil {
		fmt.Fprintf(os.Stderr, "log write failed: %v\n", err)
	}
}

// WriteRaw appends raw bytes to the log without timestamps.
func (l *LogFile) WriteRaw(data []byte) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file == nil {
		return
	}
	if _, err := l.file.Write(data); err != nil {
		fmt.Fprintf(os.Stderr, "log write failed: %v\n", err)
	}
}

// Path returns the log file path.
func (l *LogFile) Path() string {
	return l.path
}

// Close flushes and closes the log file.
func (l *LogFile) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file == nil {
		return nil
	}
	err := l.file.Close()
	l.file = nil
	return err
}
