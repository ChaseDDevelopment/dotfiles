package executor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"
)

// sudoKeepaliveInterval is how often the keepalive goroutine
// refreshes the sudo credential cache.
var sudoKeepaliveInterval = 4 * time.Minute

// HasSudo reports whether the sudo command exists on PATH.
func HasSudo() bool {
	_, err := exec.LookPath("sudo")
	return err == nil
}

// NeedsSudo reports whether sudo is available and credentials
// are not yet cached (i.e. a password prompt would be required).
func NeedsSudo() bool {
	if _, err := exec.LookPath("sudo"); err != nil {
		return false
	}
	// Probe with `sudo -n -v` (refresh-timestamp, non-interactive)
	// rather than `sudo -n true`. On stock Ubuntu cloud-init boxes
	// (e.g. kashyyyk) the user is in the sudo group *and* has a
	// NOPASSWD drop-in; `sudo -n true` matches NOPASSWD and exits
	// 0, but a later `sudo -v` or expired-cache refresh still
	// prompts because the %sudo rule requires a password. `-v`
	// correctly reports "would I need a password for any of the
	// user's rules?", which is what PreAuth actually cares about.
	cmd := exec.Command("sudo", "-n", "-v")
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run() != nil
}

// PreAuth runs "sudo -v" to either re-stamp a valid cache or
// prompt the user for their password when the cache is stale.
// Called unconditionally at startup by main so every sudo task
// downstream sees a freshly-primed timestamp; the banner wording
// stays accurate whether or not sudo actually prompts, because
// sudo's own semantics handle both cases transparently.
//
// Must be called before the TUI takes ownership of stdin.
func PreAuth() error {
	fmt.Fprintln(
		os.Stderr,
		"[sudo] Priming credentials for this session "+
			"(password prompt only if cache is stale):",
	)
	cmd := exec.Command("sudo", "-v")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("sudo authentication failed: %w", err)
	}
	return nil
}

// StartKeepalive spawns a background goroutine that refreshes
// the sudo credential cache at regular intervals. It stops when
// ctx is cancelled. Call the returned function to stop early.
// When log is non-nil, credential expiry is logged.
//
// An initial non-interactive refresh runs before the ticker loop
// so tasks that execute inside the first sudoKeepaliveInterval
// (e.g. maintenance tasks in the opening seconds of an install)
// still benefit from a live cache. The initial refresh is
// best-effort — PreAuth should have seeded the cache a moment
// earlier; if it hasn't, the first ticker tick retries with the
// existing failure-counting / logging path.
func StartKeepalive(ctx context.Context, log *LogFile) func() {
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		ticker := time.NewTicker(sudoKeepaliveInterval)
		defer ticker.Stop()
		if ctx.Err() == nil {
			initial := exec.CommandContext(
				ctx, "sudo", "-n", "-v",
			)
			initial.Stdin = nil
			initial.Stdout = nil
			initial.Stderr = nil
			_ = initial.Run()
		}
		failures := 0
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				cmd := exec.CommandContext(
					ctx, "sudo", "-n", "-v",
				)
				cmd.Stdin = nil
				cmd.Stdout = nil
				cmd.Stderr = nil
				if cmd.Run() != nil {
					failures++
					if log != nil {
						log.Write(fmt.Sprintf(
							"WARNING: sudo keepalive failed "+
								"(attempt %d) — credentials "+
								"may have expired", failures,
						))
					}
					if failures >= 3 {
						if log != nil {
							log.Write(
								"ERROR: sudo keepalive " +
									"giving up after 3 failures",
							)
						}
						return
					}
				} else {
					failures = 0
				}
			}
		}
	}()
	return cancel
}
