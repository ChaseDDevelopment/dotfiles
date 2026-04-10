package executor

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
)

// Runner executes shell commands with output capture and logging.
type Runner struct {
	Log     *LogFile
	DryRun  bool
	Verbose bool
	// Stdin is passed to child processes so sudo can identify
	// the controlling TTY for credential cache lookups
	// (tty_tickets). Typically set to an open fd on /dev/tty.
	Stdin *os.File
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
	return &Runner{Log: log, DryRun: dryRun, maxRecent: 200}
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
	return r.runCmd(ctx, "", name, args...)
}

// RunInDir executes a command in the specified working directory.
func (r *Runner) RunInDir(
	ctx context.Context,
	dir, name string,
	args ...string,
) error {
	_, err := r.runCmd(ctx, dir, name, args...)
	return err
}

// runCmd is the shared implementation for RunWithOutput and RunInDir.
func (r *Runner) runCmd(
	ctx context.Context,
	dir, name string,
	args ...string,
) (string, error) {
	cmdStr := name + " " + strings.Join(args, " ")

	if r.DryRun {
		if dir != "" {
			r.Log.Write(fmt.Sprintf(
				"[DRY RUN] (in %s) %s", dir, cmdStr,
			))
		} else {
			r.Log.Write(fmt.Sprintf("[DRY RUN] %s", cmdStr))
		}
		return "", nil
	}

	if dir != "" {
		r.Log.Write(fmt.Sprintf(
			"Running (in %s): %s", dir, cmdStr,
		))
	} else {
		r.Log.Write(fmt.Sprintf("Running: %s", cmdStr))
	}
	r.EmitVerbose("$ " + cmdStr)

	cmd := exec.CommandContext(ctx, name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	if r.Stdin != nil {
		cmd.Stdin = r.Stdin
	}
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
			if cleaned := cleanLine(line); cleaned != "" {
				r.EmitVerbose(cleaned)
			}
		}
	}

	if err != nil {
		r.Log.Write(fmt.Sprintf(
			"FAILED: %s (exit: %v)", cmdStr, err,
		))
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

// ansiRe matches ANSI escape sequences (SGR, cursor, erase, etc.).
var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

// cleanLine simulates carriage-return overwriting and strips
// ANSI escape sequences so verbose output is readable in the TUI.
func cleanLine(s string) string {
	// Keep only the text after the last \r (carriage return).
	if i := strings.LastIndex(s, "\r"); i >= 0 {
		s = s[i+1:]
	}
	s = ansiRe.ReplaceAllString(s, "")
	return strings.TrimSpace(s)
}

// logAdapter wraps LogFile to implement io.Writer.
type logAdapter struct {
	log *LogFile
}

func (a *logAdapter) Write(p []byte) (int, error) {
	a.log.WriteRaw(p)
	return len(p), nil
}
