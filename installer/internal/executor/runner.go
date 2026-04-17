package executor

import (
	"bufio"
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
	// droppedLines counts verbose messages that couldn't be delivered
	// because the channel was full (TUI falling behind). Surfaced
	// periodically so users know some output is missing — silent
	// drops used to look identical to a quiet install.
	droppedLines int
	// progressTap receives every stdout/stderr line BEFORE the Verbose
	// filter, so consumers (e.g. the orchestrator parsing brew's
	// `==> Pouring <name>` markers during a batch install) can
	// observe progress even when verbose mode is off. Guarded by
	// tapMu because Runner goroutines mutate it mid-install;
	// separate from mu to avoid holding the main lock during a
	// per-line callback that could fan out further work.
	progressTap func(line string)
	tapMu       sync.RWMutex
	mu          sync.Mutex
	maxRecent   int
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

// SetProgressTap installs (or clears, when tap is nil) a per-line
// callback invoked for every stdout/stderr line captured by the
// executor, BEFORE the Verbose filter. Intended for short-lived
// parsers wired around a single shell invocation (e.g. the
// orchestrator tapping brew's stdout during a batch install to
// flip individual tools to done as `==> Pouring <name>` lines
// stream in). The callback runs on the scanner goroutine and MUST
// NOT block — do any fan-out via a buffered channel.
func (r *Runner) SetProgressTap(tap func(line string)) {
	r.tapMu.Lock()
	r.progressTap = tap
	r.tapMu.Unlock()
}

// emitProgress invokes the active progress tap, if any. Read side
// of tapMu is cheap so scanner goroutines that never see a tap
// installed pay only an RLock/RUnlock per line.
func (r *Runner) emitProgress(line string) {
	r.tapMu.RLock()
	tap := r.progressTap
	r.tapMu.RUnlock()
	if tap != nil {
		tap(line)
	}
}

// EmitVerbose sends a human-readable status line to the verbose
// channel. No-op when the channel is nil. Uses a non-blocking
// send so task goroutines never stall if the TUI falls behind;
// drops are counted and surfaced by RecentLinesSnapshot so users
// notice that some output went missing.
func (r *Runner) EmitVerbose(msg string) {
	ch := r.verboseCh
	if ch == nil {
		return
	}
	select {
	case ch <- msg:
	default:
		r.mu.Lock()
		r.droppedLines++
		r.mu.Unlock()
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
	return r.runCmd(ctx, "", nil, name, false, args...)
}

// RunWithEnv executes a command with additional environment
// variables applied only to this invocation. Used to pass
// per-script opt-outs (PROFILE=/dev/null, SHELL=/bin/sh, etc.)
// to upstream installers without leaking those vars into
// subsequent Run calls via Runner.Env. Each entry must be
// "KEY=VALUE".
func (r *Runner) RunWithEnv(
	ctx context.Context,
	extraEnv []string,
	name string,
	args ...string,
) error {
	_, err := r.runCmd(ctx, "", extraEnv, name, false, args...)
	return err
}

// RunWithEnvAndOutput is RunWithEnv + RunWithOutput: lets callers
// pass per-invocation env (e.g. TERM=dumb to strip the Rich progress
// box from nala's stderr) while still receiving combined output for
// error classification.
func (r *Runner) RunWithEnvAndOutput(
	ctx context.Context,
	extraEnv []string,
	name string,
	args ...string,
) (string, error) {
	return r.runCmd(ctx, "", extraEnv, name, false, args...)
}

// RunProbe executes a command without logging FAILED on non-zero exit.
// Use for commands where non-zero exit is an expected outcome.
func (r *Runner) RunProbe(
	ctx context.Context,
	name string,
	args ...string,
) (string, error) {
	return r.runCmd(ctx, "", nil, name, true, args...)
}

// RunInDir executes a command in the specified working directory.
func (r *Runner) RunInDir(
	ctx context.Context,
	dir, name string,
	args ...string,
) error {
	_, err := r.runCmd(ctx, dir, nil, name, false, args...)
	return err
}

// runCmd is the shared implementation for RunWithOutput and
// RunInDir. When quiet is true, non-zero exits are not logged as
// FAILED (useful for probe commands where failure is an expected
// outcome). extraEnv, when non-nil, is appended to the resolved
// environment for this invocation only — it does not mutate
// Runner.Env and doesn't persist across calls.
func (r *Runner) runCmd(
	ctx context.Context,
	dir string,
	extraEnv []string,
	name string,
	quiet bool,
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
	// Build the final env in layered order: process env → Runner.Env
	// (persistent) → extraEnv (per-call). Later entries win because
	// os/exec honors the last occurrence of KEY in cmd.Env.
	if len(envCopy) > 0 || len(extraEnv) > 0 {
		cmd.Env = cmd.Environ()
		if len(envCopy) > 0 {
			cmd.Env = append(cmd.Env, envCopy...)
		}
		if len(extraEnv) > 0 {
			cmd.Env = append(cmd.Env, extraEnv...)
		}
	}

	// Stream output line-by-line so the verbose TUI viewport
	// updates as the command runs rather than getting the whole
	// dump after it finishes. A single io.Pipe shared by stdout
	// and stderr keeps us on one goroutine+buffer to avoid the
	// race that two separate writers would have on the shared
	// bytes.Buffer.
	var buf bytes.Buffer
	pr, pw := io.Pipe()
	cmd.Stdout = pw
	cmd.Stderr = pw

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(pr)
		// Default scanner buffer is 64 KiB — some tools (cargo
		// build, apt progress) emit long lines and would otherwise
		// break with bufio.ErrTooLong. 1 MiB is generous but bounded.
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			line := scanner.Bytes()
			// Mirror to the aggregate buffer so the existing
			// return-value contract (CombinedOutput-style string)
			// is preserved for callers like RunProbe.
			buf.Write(line)
			buf.WriteByte('\n')
			// Raw bytes into the log so users can always recover
			// original output with ANSI intact.
			r.Log.WriteRaw(append(append([]byte{}, line...), '\n'))
			// Fan cleaned lines out to the progress tap regardless of
			// Verbose — parsers must see brew/apt progress markers
			// even on non-verbose runs so the grid stays accurate.
			cleaned := cleanLine(string(line))
			if cleaned != "" {
				r.emitProgress(cleaned)
				if r.Verbose {
					r.EmitVerbose(cleaned)
				}
			}
		}
		// scanner.Err is not fatal; a closed pipe after cmd exit
		// is the expected end-of-stream signal.
		_ = scanner.Err()
	}()

	err := cmd.Run()
	// Closing the write side unblocks the scanner loop; Wait
	// guarantees the goroutine has fully drained before we read
	// from buf.
	pw.Close()
	wg.Wait()
	output := buf.String()

	if err != nil {
		if !quiet {
			r.Log.Write(fmt.Sprintf(
				"FAILED: %s (exit: %v)", cmdStr, err,
			))
		}
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
	// Emit a single "dropped N lines" heartbeat so silent drops are
	// visible. Reset the counter so the notice appears once per
	// snapshot rather than every frame.
	if r.droppedLines > 0 {
		r.recentLines = append(r.recentLines, fmt.Sprintf(
			"(… %d verbose line(s) dropped — output faster than TUI)",
			r.droppedLines,
		))
		if len(r.recentLines) > r.maxRecent {
			r.recentLines = r.recentLines[len(r.recentLines)-r.maxRecent:]
		}
		r.droppedLines = 0
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
