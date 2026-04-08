package executor

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// Runner executes shell commands with output capture and logging.
type Runner struct {
	Log     *LogFile
	DryRun  bool
	Verbose bool
	// Env holds additional environment variables for subprocess execution.
	// Each entry is "KEY=VALUE".
	Env []string
	// RecentLines holds the last N lines of output when Verbose is true.
	// Read by the TUI progress view.
	RecentLines []string
	maxRecent   int
}

// NewRunner creates a Runner attached to the given log file.
func NewRunner(log *LogFile, dryRun bool) *Runner {
	return &Runner{Log: log, DryRun: dryRun, maxRecent: 8}
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

	cmd := exec.CommandContext(ctx, name, args...)
	if len(r.Env) > 0 {
		cmd.Env = append(cmd.Environ(), r.Env...)
	}

	var buf bytes.Buffer
	// Write stdout+stderr to both the buffer and the log file.
	logWriter := &logAdapter{log: r.Log}
	cmd.Stdout = io.MultiWriter(&buf, logWriter)
	cmd.Stderr = io.MultiWriter(&buf, logWriter)

	err := cmd.Run()
	output := buf.String()

	// Store recent output lines for verbose TUI display.
	if r.Verbose && output != "" {
		lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
		if len(lines) > r.maxRecent {
			lines = lines[len(lines)-r.maxRecent:]
		}
		r.RecentLines = lines
	}

	if err != nil {
		r.Log.Write(fmt.Sprintf("FAILED: %s (exit: %v)", cmdStr, err))
		return output, fmt.Errorf("%s: %w", cmdStr, err)
	}

	r.Log.Write(fmt.Sprintf("OK: %s", cmdStr))
	return output, nil
}

// RunShell executes a command string through bash.
func (r *Runner) RunShell(ctx context.Context, script string) error {
	return r.Run(ctx, "bash", "-c", script)
}

// AddEnv appends an environment variable for subsequent commands.
func (r *Runner) AddEnv(key, value string) {
	r.Env = append(r.Env, fmt.Sprintf("%s=%s", key, value))
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
