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

// NewLogFile creates or truncates a log file at the given path.
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
	fmt.Fprintf(l.file, "[%s] %s\n", ts, msg)
}

// WriteRaw appends raw bytes to the log without timestamps.
func (l *LogFile) WriteRaw(data []byte) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file == nil {
		return
	}
	l.file.Write(data)
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
	return l.file.Close()
}
