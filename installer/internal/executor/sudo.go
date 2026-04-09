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
const sudoKeepaliveInterval = 4 * time.Minute

// NeedsSudo reports whether sudo is available and credentials
// are not yet cached (i.e. a password prompt would be required).
func NeedsSudo() bool {
	if _, err := exec.LookPath("sudo"); err != nil {
		return false
	}
	// sudo -n true succeeds silently when credentials are
	// already cached or NOPASSWD is configured.
	cmd := exec.Command("sudo", "-n", "true")
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run() != nil
}

// PreAuth prompts the user for their sudo password via
// "sudo -v", caching credentials for subsequent non-interactive
// use. Must be called before the TUI takes ownership of stdin.
func PreAuth() error {
	fmt.Fprintln(
		os.Stderr,
		"[sudo] Password required (cached for this session only):",
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
func StartKeepalive(ctx context.Context) func() {
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		ticker := time.NewTicker(sudoKeepaliveInterval)
		defer ticker.Stop()
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
					// Cache expired and can't be silently
					// refreshed — stop trying.
					return
				}
			}
		}
	}()
	return cancel
}
