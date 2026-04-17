package executor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// newTestRunner creates a Runner backed by a real log file in a
// temporary directory. The caller does not need to close the log;
// it is cleaned up automatically via t.TempDir().
func newTestRunner(t *testing.T, dryRun bool) *Runner {
	t.Helper()
	dir := t.TempDir()
	logPath := filepath.Join(dir, "test.log")
	lf, err := NewLogFile(logPath)
	if err != nil {
		t.Fatalf("NewLogFile() error = %v", err)
	}
	t.Cleanup(func() { lf.Close() })
	return NewRunner(lf, dryRun)
}

func TestNewRunner(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	logPath := filepath.Join(dir, "test.log")
	lf, err := NewLogFile(logPath)
	if err != nil {
		t.Fatalf("NewLogFile() error = %v", err)
	}
	defer lf.Close()

	r := NewRunner(lf, false)

	if r.Log != lf {
		t.Error("Log field not set correctly")
	}
	if r.DryRun {
		t.Error("DryRun should be false")
	}
	if r.maxRecent != 200 {
		t.Errorf("maxRecent = %d, want 200", r.maxRecent)
	}
}

func TestNewRunnerDryRun(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	logPath := filepath.Join(dir, "test.log")
	lf, err := NewLogFile(logPath)
	if err != nil {
		t.Fatalf("NewLogFile() error = %v", err)
	}
	defer lf.Close()

	r := NewRunner(lf, true)
	if !r.DryRun {
		t.Error("DryRun should be true")
	}
}

func TestEnableVerboseChannel(t *testing.T) {
	t.Parallel()
	r := newTestRunner(t, false)

	r.EnableVerboseChannel(10)

	// Channel should be created.
	r.mu.Lock()
	ch := r.verboseCh
	r.mu.Unlock()
	if ch == nil {
		t.Fatal("verboseCh should not be nil after Enable")
	}

	// Calling again should be a no-op (same channel).
	r.EnableVerboseChannel(20)
	r.mu.Lock()
	ch2 := r.verboseCh
	r.mu.Unlock()
	if ch != ch2 {
		t.Error(
			"EnableVerboseChannel called twice " +
				"should not replace channel",
		)
	}
}

func TestEmitVerbose(t *testing.T) {
	t.Parallel()

	t.Run("no-op without channel", func(t *testing.T) {
		t.Parallel()
		r := newTestRunner(t, false)
		// Should not panic when channel is nil.
		r.EmitVerbose("hello")
	})

	t.Run("sends message to channel", func(t *testing.T) {
		t.Parallel()
		r := newTestRunner(t, false)
		r.EnableVerboseChannel(10)

		r.EmitVerbose("test message")

		select {
		case msg := <-r.verboseCh:
			if msg != "test message" {
				t.Errorf("got %q, want %q", msg, "test message")
			}
		default:
			t.Error("expected message in channel")
		}
	})

	t.Run("non-blocking when channel full", func(t *testing.T) {
		t.Parallel()
		r := newTestRunner(t, false)
		r.EnableVerboseChannel(1)

		// Fill the channel.
		r.EmitVerbose("first")
		// This should not block.
		r.EmitVerbose("second")
		// Drain and verify only the first message.
		msg := <-r.verboseCh
		if msg != "first" {
			t.Errorf("got %q, want %q", msg, "first")
		}
	})
}

func TestRunSuccess(t *testing.T) {
	t.Parallel()
	r := newTestRunner(t, false)
	ctx := context.Background()

	err := r.Run(ctx, "echo", "hello")
	if err != nil {
		t.Errorf("Run(echo hello) error = %v", err)
	}
}

func TestRunFailure(t *testing.T) {
	t.Parallel()
	r := newTestRunner(t, false)
	ctx := context.Background()

	err := r.Run(ctx, "false")
	if err == nil {
		t.Error("Run(false) should return error")
	}
}

func TestRunWithOutput(t *testing.T) {
	t.Parallel()
	r := newTestRunner(t, false)
	ctx := context.Background()

	output, err := r.RunWithOutput(ctx, "echo", "hello world")
	if err != nil {
		t.Fatalf("RunWithOutput() error = %v", err)
	}
	if got := strings.TrimSpace(output); got != "hello world" {
		t.Errorf("output = %q, want %q", got, "hello world")
	}
}

func TestRunWithOutputFailure(t *testing.T) {
	t.Parallel()
	r := newTestRunner(t, false)
	ctx := context.Background()

	_, err := r.RunWithOutput(ctx, "false")
	if err == nil {
		t.Error("RunWithOutput(false) should return error")
	}
}

func TestRunInDir(t *testing.T) {
	t.Parallel()
	r := newTestRunner(t, false)
	ctx := context.Background()
	dir := t.TempDir()

	err := r.RunInDir(ctx, dir, "pwd")
	if err != nil {
		t.Errorf("RunInDir() error = %v", err)
	}

	// Verify the log mentions the directory.
	r.Log.Close()
	data, err := os.ReadFile(r.Log.Path())
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.Contains(string(data), dir) {
		t.Error("log should mention the working directory")
	}
}

func TestRunDryRun(t *testing.T) {
	t.Parallel()
	r := newTestRunner(t, true)
	ctx := context.Background()

	// In dry-run mode, no command is actually executed.
	err := r.Run(ctx, "nonexistent-command-xyz")
	if err != nil {
		t.Errorf(
			"Run in dry-run mode should not error, got %v",
			err,
		)
	}

	// Verify the log contains [DRY RUN].
	r.Log.Close()
	data, err := os.ReadFile(r.Log.Path())
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.Contains(string(data), "[DRY RUN]") {
		t.Error("dry-run log should contain [DRY RUN]")
	}
}

func TestRunDryRunInDir(t *testing.T) {
	t.Parallel()
	r := newTestRunner(t, true)
	ctx := context.Background()

	err := r.RunInDir(ctx, "/tmp", "some-command")
	if err != nil {
		t.Errorf("RunInDir dry-run should not error, got %v", err)
	}

	r.Log.Close()
	data, err := os.ReadFile(r.Log.Path())
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "[DRY RUN]") {
		t.Error("log should contain [DRY RUN]")
	}
	if !strings.Contains(content, "/tmp") {
		t.Error("log should mention the directory for dry-run")
	}
}

func TestRunDryRunReturnsEmptyOutput(t *testing.T) {
	t.Parallel()
	r := newTestRunner(t, true)
	ctx := context.Background()

	output, err := r.RunWithOutput(ctx, "echo", "hello")
	if err != nil {
		t.Errorf("RunWithOutput dry-run error = %v", err)
	}
	if output != "" {
		t.Errorf("dry-run output = %q, want empty", output)
	}
}

func TestRunContextCancellation(t *testing.T) {
	t.Parallel()
	r := newTestRunner(t, false)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := r.Run(ctx, "sleep", "10")
	if err == nil {
		t.Error("Run with cancelled context should error")
	}
}

func TestRunLogsOutput(t *testing.T) {
	t.Parallel()
	r := newTestRunner(t, false)
	ctx := context.Background()

	r.Run(ctx, "echo", "logged-output")

	r.Log.Close()
	data, err := os.ReadFile(r.Log.Path())
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	content := string(data)

	if !strings.Contains(content, "Running: echo logged-output") {
		t.Error("log should contain the command being run")
	}
	if !strings.Contains(content, "OK: echo logged-output") {
		t.Error("log should contain OK on success")
	}
}

func TestRunLogsFailure(t *testing.T) {
	t.Parallel()
	r := newTestRunner(t, false)
	ctx := context.Background()

	r.Run(ctx, "false")

	r.Log.Close()
	data, err := os.ReadFile(r.Log.Path())
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.Contains(string(data), "FAILED:") {
		t.Error("log should contain FAILED on error")
	}
}

func TestRunEmitsVerboseOutput(t *testing.T) {
	t.Parallel()
	r := newTestRunner(t, false)
	r.Verbose = true
	r.EnableVerboseChannel(100)
	ctx := context.Background()

	r.RunWithOutput(ctx, "echo", "verbose-test")

	// Give a moment for the channel to be populated.
	time.Sleep(10 * time.Millisecond)
	snap := r.RecentLinesSnapshot()

	found := false
	for _, line := range snap {
		if strings.Contains(line, "verbose-test") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf(
			"expected 'verbose-test' in recent lines, got %v",
			snap,
		)
	}
}

func TestRunShell(t *testing.T) {
	t.Parallel()
	r := newTestRunner(t, false)
	ctx := context.Background()

	err := r.RunShell(ctx, "echo hello && echo world")
	if err != nil {
		t.Errorf("RunShell() error = %v", err)
	}
}

func TestAddEnv(t *testing.T) {
	t.Parallel()
	r := newTestRunner(t, false)

	r.AddEnv("MY_TEST_VAR", "my_value")

	r.mu.Lock()
	defer r.mu.Unlock()
	found := false
	for _, e := range r.Env {
		if e == "MY_TEST_VAR=my_value" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf(
			"AddEnv did not add entry; Env = %v",
			r.Env,
		)
	}
}

func TestAddEnvUsedByRun(t *testing.T) {
	t.Parallel()
	r := newTestRunner(t, false)
	ctx := context.Background()

	r.AddEnv("DOTSETUP_TEST_VAR", "42")

	output, err := r.RunWithOutput(
		ctx, "bash", "-c", "echo $DOTSETUP_TEST_VAR",
	)
	if err != nil {
		t.Fatalf("RunWithOutput() error = %v", err)
	}
	if got := strings.TrimSpace(output); got != "42" {
		t.Errorf("env var output = %q, want %q", got, "42")
	}
}

func TestRecentLinesSnapshot(t *testing.T) {
	t.Parallel()
	r := newTestRunner(t, false)
	r.EnableVerboseChannel(100)

	r.EmitVerbose("line1")
	r.EmitVerbose("line2")
	r.EmitVerbose("line3")

	snap := r.RecentLinesSnapshot()
	if len(snap) != 3 {
		t.Fatalf("expected 3 recent lines, got %d", len(snap))
	}
	if snap[0] != "line1" || snap[1] != "line2" || snap[2] != "line3" {
		t.Errorf("unexpected recent lines: %v", snap)
	}

	// Second call should return the same (already drained).
	snap2 := r.RecentLinesSnapshot()
	if len(snap2) != 3 {
		t.Errorf(
			"second snapshot should have 3 lines, got %d",
			len(snap2),
		)
	}
}

func TestRecentLinesSnapshotTruncation(t *testing.T) {
	t.Parallel()
	r := newTestRunner(t, false)
	r.maxRecent = 5
	r.EnableVerboseChannel(200)

	for i := 0; i < 10; i++ {
		r.EmitVerbose("msg")
	}

	snap := r.RecentLinesSnapshot()
	if len(snap) != 5 {
		t.Errorf("expected 5 recent lines, got %d", len(snap))
	}
}

func TestRecentLinesSnapshotWithoutChannel(t *testing.T) {
	t.Parallel()
	r := newTestRunner(t, false)

	// Should return empty slice without panic.
	snap := r.RecentLinesSnapshot()
	if len(snap) != 0 {
		t.Errorf(
			"expected 0 recent lines without channel, got %d",
			len(snap),
		)
	}
}

func TestLastLines(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		input  string
		n      int
		want   string
	}{
		{
			name:  "fewer lines than n",
			input: "a\nb\n",
			n:     5,
			want:  "a\nb",
		},
		{
			name:  "exact match",
			input: "a\nb\nc\n",
			n:     3,
			want:  "a\nb\nc",
		},
		{
			name:  "more lines than n",
			input: "a\nb\nc\nd\ne\n",
			n:     2,
			want:  "d\ne",
		},
		{
			name:  "single line",
			input: "hello",
			n:     3,
			want:  "hello",
		},
		{
			name:  "empty string",
			input: "",
			n:     3,
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := LastLines(tt.input, tt.n)
			if got != tt.want {
				t.Errorf(
					"LastLines(%q, %d) = %q, want %q",
					tt.input, tt.n, got, tt.want,
				)
			}
		})
	}
}

func TestCleanLine(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "plain text",
			input: "hello world",
			want:  "hello world",
		},
		{
			name:  "strips ANSI color",
			input: "\x1b[32mgreen\x1b[0m",
			want:  "green",
		},
		{
			name:  "carriage return overwrites",
			input: "old text\rnew text",
			want:  "new text",
		},
		{
			name:  "trims whitespace",
			input: "  spaced  ",
			want:  "spaced",
		},
		{
			name:  "empty after cleaning",
			input: "  \x1b[0m  ",
			want:  "",
		},
		{
			name:  "multiple ANSI sequences",
			input: "\x1b[1m\x1b[31mbold red\x1b[0m end",
			want:  "bold red end",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := cleanLine(tt.input)
			if got != tt.want {
				t.Errorf(
					"cleanLine(%q) = %q, want %q",
					tt.input, got, tt.want,
				)
			}
		})
	}
}

func TestLogAdapterImplementsWriter(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	logPath := filepath.Join(dir, "adapter.log")
	lf, err := NewLogFile(logPath)
	if err != nil {
		t.Fatalf("NewLogFile() error = %v", err)
	}
	defer lf.Close()

	adapter := &logAdapter{log: lf}
	n, err := adapter.Write([]byte("adapter test\n"))
	if err != nil {
		t.Errorf("logAdapter.Write() error = %v", err)
	}
	if n != len("adapter test\n") {
		t.Errorf("Write returned %d, want %d", n, len("adapter test\n"))
	}
}

func TestRunWithStdin(t *testing.T) {
	t.Parallel()
	r := newTestRunner(t, false)
	ctx := context.Background()

	// Open /dev/null as a valid Stdin file.
	devNull, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatalf("open /dev/null: %v", err)
	}
	defer devNull.Close()
	r.Stdin = devNull

	// Should still be able to run commands.
	err = r.Run(ctx, "echo", "stdin-test")
	if err != nil {
		t.Errorf("Run with Stdin set error = %v", err)
	}
}

// TestRunWithEnv_IsolatesInvocation confirms RunWithEnv passes its
// extraEnv to the child but does NOT persist into Runner.Env, so
// subsequent Run calls don't inherit the per-invocation vars.
// Regression guard against "opt-out env leaks into unrelated
// commands" class of bug.
func TestRunWithEnv_IsolatesInvocation(t *testing.T) {
	t.Parallel()
	r := newTestRunner(t, false)
	ctx := context.Background()

	tmp := t.TempDir()
	out := filepath.Join(tmp, "env.txt")

	// First run: pass extraEnv. Child should see the var.
	if err := r.RunWithEnv(
		ctx,
		[]string{"DOTSETUP_TEST=scoped"},
		"sh", "-c",
		fmt.Sprintf(`printf "%%s" "$DOTSETUP_TEST" > %q`, out),
	); err != nil {
		t.Fatalf("RunWithEnv: %v", err)
	}
	if data, _ := os.ReadFile(out); string(data) != "scoped" {
		t.Errorf("first run: got %q, want %q", data, "scoped")
	}

	// Second run via regular Run: must NOT see the var.
	if err := r.Run(
		ctx, "sh", "-c",
		fmt.Sprintf(`printf "%%s" "$DOTSETUP_TEST" > %q`, out),
	); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if data, _ := os.ReadFile(out); string(data) != "" {
		t.Errorf("follow-up Run leaked env: got %q, want empty", data)
	}

	// Runner.Env should also be untouched — AddEnv is the
	// documented way to persist vars; RunWithEnv must not.
	for _, entry := range r.Env {
		if strings.HasPrefix(entry, "DOTSETUP_TEST=") {
			t.Errorf("RunWithEnv leaked into Runner.Env: %q", entry)
		}
	}
}
