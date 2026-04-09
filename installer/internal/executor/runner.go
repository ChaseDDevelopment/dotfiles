package executor

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
)

// Runner executes shell commands with output capture and logging.
type Runner struct {
	Log     *LogFile
	DryRun  bool
	Verbose bool
	// Env holds additional environment variables for subprocess execution.
	// Each entry is "KEY=VALUE".
	Env []string
	// recentLines holds the last N lines of output when Verbose is true.
	// Protected by mu; use RecentLinesSnapshot() to read.
	recentLines []string
	// verboseCh receives human-readable status lines when verbose mode
	// is active. Nil until EnableVerboseChannel is called.
	verboseCh chan string
	mu        sync.Mutex
	maxRecent int
}

// NewRunner creates a Runner attached to the given log file.
func NewRunner(log *LogFile, dryRun bool) *Runner {
	return &Runner{Log: log, DryRun: dryRun, maxRecent: 8}
}

// EnableVerboseChannel creates the buffered channel used by
// EmitVerbose. Call once before starting tasks.
func (r *Runner) EnableVerboseChannel(bufSize int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.verboseCh == nil {
		r.verboseCh = make(chan string, bufSize)
	}
}

// EmitVerbose sends a human-readable status line to the verbose
// channel. No-op when the channel is nil. Uses a non-blocking
// send so task goroutines never stall if the TUI falls behind.
func (r *Runner) EmitVerbose(msg string) {
	ch := r.verboseCh
	if ch == nil {
		return
	}
	select {
	case ch <- msg:
	default:
	}
}

// Run executes a command, captures stdout+stderr to the log,
// and returns an error if the command fails.
func (r *Runner) Run(ctx context.Context, name string, args ...string) error {
	_, err := r.RunWithOutput(ctx, name, args...)
	return err
}

// RunWithOutput executes a command and returns its combined output.
func (r *Runner) RunWithOutput(
	ctx context.Context,
	name string,
	args ...string,
) (string, error) {
	cmdStr := name + " " + strings.Join(args, " ")

	if r.DryRun {
		r.Log.Write(fmt.Sprintf("[DRY RUN] %s", cmdStr))
		return "", nil
	}

	r.Log.Write(fmt.Sprintf("Running: %s", cmdStr))
	r.EmitVerbose("$ " + cmdStr)

	cmd := exec.CommandContext(ctx, name, args...)
	r.mu.Lock()
	envCopy := make([]string, len(r.Env))
	copy(envCopy, r.Env)
	r.mu.Unlock()
	if len(envCopy) > 0 {
		cmd.Env = append(cmd.Environ(), envCopy...)
	}

	var buf bytes.Buffer
	// Write stdout+stderr to both the buffer and the log file.
	logWriter := &logAdapter{log: r.Log}
	cmd.Stdout = io.MultiWriter(&buf, logWriter)
	cmd.Stderr = io.MultiWriter(&buf, logWriter)

	err := cmd.Run()
	output := buf.String()

	// Emit output lines to the verbose channel.
	if r.Verbose && output != "" {
		for _, line := range strings.Split(
			strings.TrimRight(output, "\n"), "\n",
		) {
			r.EmitVerbose(line)
		}
	}

	if err != nil {
		r.Log.Write(fmt.Sprintf("FAILED: %s (exit: %v)", cmdStr, err))
		return output, fmt.Errorf("%s: %w", cmdStr, err)
	}

	r.Log.Write(fmt.Sprintf("OK: %s", cmdStr))
	return output, nil
}

// RecentLinesSnapshot drains any pending verbose channel messages
// into recentLines and returns a copy. Safe to call from a
// different goroutine than RunWithOutput.
func (r *Runner) RecentLinesSnapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	// Drain the verbose channel into recentLines.
	if r.verboseCh != nil {
		for {
			select {
			case line := <-r.verboseCh:
				r.recentLines = append(
					r.recentLines, line,
				)
				if len(r.recentLines) > r.maxRecent {
					r.recentLines = r.recentLines[len(r.recentLines)-r.maxRecent:]
				}
			default:
				goto drained
			}
		}
	drained:
	}
	cp := make([]string, len(r.recentLines))
	copy(cp, r.recentLines)
	return cp
}

// RunInDir executes a command in the specified working directory.
func (r *Runner) RunInDir(ctx context.Context, dir, name string, args ...string) error {
	cmdStr := name + " " + strings.Join(args, " ")

	if r.DryRun {
		r.Log.Write(fmt.Sprintf("[DRY RUN] (in %s) %s", dir, cmdStr))
		return nil
	}

	r.Log.Write(fmt.Sprintf("Running (in %s): %s", dir, cmdStr))
	r.EmitVerbose("$ " + cmdStr)

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	r.mu.Lock()
	envCopy := make([]string, len(r.Env))
	copy(envCopy, r.Env)
	r.mu.Unlock()
	if len(envCopy) > 0 {
		cmd.Env = append(cmd.Environ(), envCopy...)
	}

	var buf bytes.Buffer
	logWriter := &logAdapter{log: r.Log}
	cmd.Stdout = io.MultiWriter(&buf, logWriter)
	cmd.Stderr = io.MultiWriter(&buf, logWriter)

	err := cmd.Run()
	output := buf.String()

	// Emit output lines to the verbose channel.
	if r.Verbose && output != "" {
		for _, line := range strings.Split(
			strings.TrimRight(output, "\n"), "\n",
		) {
			r.EmitVerbose(line)
		}
	}

	if err != nil {
		r.Log.Write(fmt.Sprintf(
			"FAILED: %s (exit: %v)", cmdStr, err,
		))
		return fmt.Errorf("%s: %w", cmdStr, err)
	}

	r.Log.Write(fmt.Sprintf("OK: %s", cmdStr))
	return nil
}

// RunShell executes a command string through bash.
func (r *Runner) RunShell(ctx context.Context, script string) error {
	return r.Run(ctx, "bash", "-c", script)
}

// AddEnv appends an environment variable for subsequent commands.
// Safe to call from multiple goroutines.
func (r *Runner) AddEnv(key, value string) {
	r.mu.Lock()
	r.Env = append(r.Env, fmt.Sprintf("%s=%s", key, value))
	r.mu.Unlock()
}

// LastLines returns the last n lines from a string of output.
func LastLines(output string, n int) string {
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	if len(lines) <= n {
		return strings.Join(lines, "\n")
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}

// logAdapter wraps LogFile to implement io.Writer.
type logAdapter struct {
	log *LogFile
}

func (a *logAdapter) Write(p []byte) (int, error) {
	a.log.WriteRaw(p)
	return len(p), nil
}
