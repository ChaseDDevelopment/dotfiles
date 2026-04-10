package executor

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestNewLogFile(t *testing.T) {
	t.Parallel()

	t.Run("creates log file at given path", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := filepath.Join(dir, "test.log")

		lf, err := NewLogFile(path)
		if err != nil {
			t.Fatalf("NewLogFile() error = %v", err)
		}
		defer lf.Close()

		if lf.Path() != path {
			t.Errorf("Path() = %q, want %q", lf.Path(), path)
		}

		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Fatal("log file was not created on disk")
		}
	})

	t.Run("writes initial log line", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := filepath.Join(dir, "test.log")

		lf, err := NewLogFile(path)
		if err != nil {
			t.Fatalf("NewLogFile() error = %v", err)
		}
		lf.Close()

		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile() error = %v", err)
		}
		content := string(data)
		if !strings.Contains(content, "Dotsetup install log started") {
			t.Errorf(
				"initial log line missing; got %q",
				content,
			)
		}
	})

	t.Run("returns error for invalid path", func(t *testing.T) {
		t.Parallel()
		_, err := NewLogFile("/nonexistent/dir/test.log")
		if err == nil {
			t.Fatal("expected error for invalid path, got nil")
		}
	})
}

func TestLogFileWrite(t *testing.T) {
	t.Parallel()

	t.Run("writes timestamped lines", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := filepath.Join(dir, "test.log")

		lf, err := NewLogFile(path)
		if err != nil {
			t.Fatalf("NewLogFile() error = %v", err)
		}

		lf.Write("hello world")
		lf.Write("second line")
		lf.Close()

		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile() error = %v", err)
		}
		content := string(data)

		if !strings.Contains(content, "hello world") {
			t.Error("missing 'hello world' in log")
		}
		if !strings.Contains(content, "second line") {
			t.Error("missing 'second line' in log")
		}
		// Check timestamp format [YYYY-MM-DD HH:MM:SS].
		if !strings.Contains(content, "[20") {
			t.Error("missing timestamp prefix in log")
		}
	})

	t.Run("no-op after Close", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := filepath.Join(dir, "test.log")

		lf, err := NewLogFile(path)
		if err != nil {
			t.Fatalf("NewLogFile() error = %v", err)
		}
		lf.Close()

		// Write after close should not panic.
		lf.Write("after close")

		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile() error = %v", err)
		}
		if strings.Contains(string(data), "after close") {
			t.Error("Write after Close should be a no-op")
		}
	})
}

func TestLogFileWriteRaw(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	lf, err := NewLogFile(path)
	if err != nil {
		t.Fatalf("NewLogFile() error = %v", err)
	}

	lf.WriteRaw([]byte("raw bytes\n"))
	lf.Close()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.Contains(string(data), "raw bytes\n") {
		t.Error("missing raw bytes in log")
	}
}

func TestLogFileWriteRawAfterClose(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	lf, err := NewLogFile(path)
	if err != nil {
		t.Fatalf("NewLogFile() error = %v", err)
	}
	lf.Close()

	// Should not panic.
	lf.WriteRaw([]byte("after close"))
}

func TestLogFileClose(t *testing.T) {
	t.Parallel()

	t.Run("idempotent", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := filepath.Join(dir, "test.log")

		lf, err := NewLogFile(path)
		if err != nil {
			t.Fatalf("NewLogFile() error = %v", err)
		}

		if err := lf.Close(); err != nil {
			t.Errorf("first Close() error = %v", err)
		}
		if err := lf.Close(); err != nil {
			t.Errorf(
				"second Close() should return nil, got %v",
				err,
			)
		}
	})
}

func TestLogFileConcurrentWrites(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	lf, err := NewLogFile(path)
	if err != nil {
		t.Fatalf("NewLogFile() error = %v", err)
	}

	const goroutines = 20
	const writesPerGoroutine = 50
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < writesPerGoroutine; j++ {
				lf.Write("msg from goroutine")
			}
		}(i)
	}
	wg.Wait()
	lf.Close()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	// Each Write produces one line ending with \n.
	lines := strings.Split(
		strings.TrimRight(string(data), "\n"), "\n",
	)
	// +1 for the initial "Dotsetup install log started" line.
	expected := goroutines*writesPerGoroutine + 1
	if len(lines) != expected {
		t.Errorf(
			"expected %d lines, got %d",
			expected, len(lines),
		)
	}
}
