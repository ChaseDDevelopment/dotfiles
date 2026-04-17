package executor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRunnerProbeAndEnvOutput(t *testing.T) {
	r := newTestRunner(t, false)
	dir := t.TempDir()
	fakebin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(fakebin, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fakebin+string(os.PathListSeparator)+os.Getenv("PATH"))
	if err := os.WriteFile(filepath.Join(fakebin, "probe-cmd"), []byte(`#!/usr/bin/env bash
echo "value:$SPECIAL"
exit 7
`), 0o755); err != nil {
		t.Fatal(err)
	}

	out, err := r.RunWithEnvAndOutput(context.Background(), []string{"SPECIAL=works"}, "bash", "-c", "echo $SPECIAL")
	if err != nil {
		t.Fatalf("RunWithEnvAndOutput: %v", err)
	}
	if strings.TrimSpace(out) != "works" {
		t.Fatalf("env output = %q", out)
	}

	out, err = r.RunProbe(context.Background(), "probe-cmd")
	if err == nil {
		t.Fatal("RunProbe should return non-zero exit error")
	}
	if !strings.Contains(out, "value:") {
		t.Fatalf("probe output missing: %q", out)
	}
}

func TestHasSudoAndNeedsSudo(t *testing.T) {
	dir := t.TempDir()
	fakebin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(fakebin, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fakebin)

	if HasSudo() || NeedsSudo() {
		t.Fatal("expected no sudo when not on PATH")
	}

	stateFile := filepath.Join(dir, "sudo.state")
	t.Setenv("SUDO_STATE", stateFile)
	if err := os.WriteFile(filepath.Join(fakebin, "sudo"), []byte(`#!/bin/sh
if [ "$1" = "-n" ] && [ "$2" = "-v" ]; then
  if [ -f "$SUDO_STATE" ]; then
    exit 0
  fi
  exit 1
fi
exit 0
`), 0o755); err != nil {
		t.Fatal(err)
	}

	if !HasSudo() {
		t.Fatal("expected sudo to be found on PATH")
	}
	if !NeedsSudo() {
		t.Fatal("expected sudo credentials to be missing initially")
	}
	if err := os.WriteFile(stateFile, []byte("cached"), 0o644); err != nil {
		t.Fatal(err)
	}
	if NeedsSudo() {
		t.Fatal("expected cached sudo credentials")
	}
}

// TestNeedsSudoOnMixedNopasswdHost covers the kashyyyk failure mode:
// stock Ubuntu cloud-init boxes put the user in the sudo group (which
// requires a password) *and* add a NOPASSWD drop-in. Under that config
// `sudo -n true` exits 0 (NOPASSWD matches /usr/bin/true) but a later
// `sudo -v` still wants a password — so NeedsSudo must probe with -v
// to return true here, otherwise PreAuth is skipped and the TUI hits a
// blocking password prompt mid-install.
func TestNeedsSudoOnMixedNopasswdHost(t *testing.T) {
	dir := t.TempDir()
	fakebin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(fakebin, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fakebin)

	// `sudo -n true` → 0 (NOPASSWD rule wins for actual commands),
	// `sudo -n -v`  → 1 (%sudo rule forces password for refresh).
	if err := os.WriteFile(filepath.Join(fakebin, "sudo"), []byte(`#!/bin/sh
if [ "$1" = "-n" ] && [ "$2" = "-v" ]; then
  exit 1
fi
if [ "$1" = "-n" ] && [ "$2" = "true" ]; then
  exit 0
fi
exit 0
`), 0o755); err != nil {
		t.Fatal(err)
	}

	if !NeedsSudo() {
		t.Fatal("NeedsSudo must return true on mixed NOPASSWD host " +
			"(sudo -n true=0 but sudo -n -v=1) so PreAuth runs before " +
			"the TUI takes stdin")
	}
}

func TestStartKeepaliveLogsFailuresAndStops(t *testing.T) {
	r := newTestRunner(t, false)
	dir := t.TempDir()
	fakebin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(fakebin, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fakebin)
	if err := os.WriteFile(filepath.Join(fakebin, "sudo"), []byte(`#!/bin/sh
exit 1
`), 0o755); err != nil {
		t.Fatal(err)
	}

	origInterval := sudoKeepaliveInterval
	sudoKeepaliveInterval = 10 * time.Millisecond
	defer func() { sudoKeepaliveInterval = origInterval }()

	ctx, cancel := context.WithCancel(context.Background())
	stop := StartKeepalive(ctx, r.Log)
	defer stop()

	var logText string
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		rawLog, err := os.ReadFile(r.Log.Path())
		if err != nil {
			t.Fatal(err)
		}
		logText = string(rawLog)
		if strings.Contains(logText, "ERROR: sudo keepalive giving up after 3 failures") {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	cancel()

	if strings.Count(logText, "WARNING: sudo keepalive failed") < 3 {
		t.Fatalf("runner log missing warning:\n%s", logText)
	}
	if !strings.Contains(logText, "ERROR: sudo keepalive giving up after 3 failures") {
		t.Fatalf("runner log missing terminal error:\n%s", logText)
	}
}
