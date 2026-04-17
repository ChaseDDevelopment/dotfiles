package update

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/chaseddevelopment/dotfiles/installer/internal/executor"
)

func runtimeGOOS() string   { return runtime.GOOS }
func runtimeGOARCH() string { return runtime.GOARCH }

func newSelfUpdateCtx(t *testing.T) (*executor.Runner, string) {
	t.Helper()
	dir := t.TempDir()
	log, err := executor.NewLogFile(filepath.Join(dir, "test.log"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { log.Close() })
	return executor.NewRunner(log, false), dir
}

// TestSelfUpdateCheckFailsPropagates covers the CheckSelfUpdate-error
// path in SelfUpdate (latestVersionFn returns an error).
func TestSelfUpdateCheckFailsPropagates(t *testing.T) {
	runner, _ := newSelfUpdateCtx(t)
	orig := latestVersionFn
	defer func() { latestVersionFn = orig }()
	latestVersionFn = func(string, bool) (string, error) {
		return "", fmt.Errorf("rate limited")
	}
	if err := SelfUpdate(context.Background(), runner, "v1.2.3"); err == nil {
		t.Fatal("expected SelfUpdate to surface CheckSelfUpdate failure")
	}
}

// TestSelfUpdateFetchChecksumsFails covers the fetchChecksumsFn error
// branch — distinct from the "missing entry" case already covered.
func TestSelfUpdateFetchChecksumsFails(t *testing.T) {
	runner, _ := newSelfUpdateCtx(t)
	origLatest := latestVersionFn
	origChecksums := fetchChecksumsFn
	defer func() {
		latestVersionFn = origLatest
		fetchChecksumsFn = origChecksums
	}()
	latestVersionFn = func(string, bool) (string, error) { return "v1.2.4", nil }
	fetchChecksumsFn = func(context.Context, string) (map[string]string, error) {
		return nil, fmt.Errorf("404 not found")
	}
	err := SelfUpdate(context.Background(), runner, "v1.2.3")
	if err == nil || !strings.Contains(err.Error(), "fetch release checksums") {
		t.Fatalf("expected fetch-checksums error, got %v", err)
	}
}

// TestSelfUpdateCosignFails covers the verifyCosignSignatureFn error
// path (signature check is the gate before touching the filesystem).
func TestSelfUpdateCosignFails(t *testing.T) {
	runner, _ := newSelfUpdateCtx(t)
	origLatest := latestVersionFn
	origChecksums := fetchChecksumsFn
	origCosign := verifyCosignSignatureFn
	defer func() {
		latestVersionFn = origLatest
		fetchChecksumsFn = origChecksums
		verifyCosignSignatureFn = origCosign
	}()
	latestVersionFn = func(string, bool) (string, error) { return "v1.2.4", nil }
	fetchChecksumsFn = func(context.Context, string) (map[string]string, error) {
		// Use a key that matches runtime.GOOS/ARCH to reach the cosign step.
		key := fmt.Sprintf("dotsetup-%s-%s", runtimeGOOS(), runtimeGOARCH())
		return map[string]string{key: "deadbeef"}, nil
	}
	verifyCosignSignatureFn = func(context.Context, *executor.Runner, string, string) error {
		return fmt.Errorf("bad cert")
	}
	err := SelfUpdate(context.Background(), runner, "v1.2.3")
	if err == nil || !strings.Contains(err.Error(), "signature check failed") {
		t.Fatalf("expected cosign failure, got %v", err)
	}
}

// TestSelfUpdateExecutablePathFails covers the os.Executable seam.
func TestSelfUpdateExecutablePathFails(t *testing.T) {
	runner, _ := newSelfUpdateCtx(t)
	restoreSeams(t)
	executablePathFn = func() (string, error) {
		return "", fmt.Errorf("no exe")
	}
	err := SelfUpdate(context.Background(), runner, "v1.2.3")
	if err == nil || !strings.Contains(err.Error(), "find current binary") {
		t.Fatalf("expected find-binary error, got %v", err)
	}
}

// TestSelfUpdateDownloadFails covers the curl-failed branch and
// confirms the staging file is cleaned up.
func TestSelfUpdateDownloadFails(t *testing.T) {
	runner, dir := newSelfUpdateCtx(t)
	restoreSeams(t)

	exe := filepath.Join(dir, "dotsetup")
	if err := os.WriteFile(exe, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	executablePathFn = func() (string, error) { return exe, nil }

	// curl stub that always fails.
	bin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "curl"),
		[]byte("#!/bin/sh\nexit 22\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)

	err := SelfUpdate(context.Background(), runner, "v1.2.3")
	if err == nil || !strings.Contains(err.Error(), "download update") {
		t.Fatalf("expected download error, got %v", err)
	}
	if _, err := os.Stat(exe + ".new"); !os.IsNotExist(err) {
		t.Fatalf("staging file should be cleaned up, stat err=%v", err)
	}
}

// TestCleanupTmpErrorBranch covers the "remove errored with non-
// NotExist error" branch — planted via a non-file path.
func TestCleanupTmpErrorBranch(t *testing.T) {
	runner, dir := newSelfUpdateCtx(t)
	// Plant a directory where cleanupTmp tries to os.Remove a file.
	// os.Remove returns an error like "not empty" on non-empty dirs.
	nonEmpty := filepath.Join(dir, "nonempty")
	if err := os.MkdirAll(nonEmpty, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nonEmpty, "child"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	cleanupTmp(runner, nonEmpty)
	// Read the log; expect a NOTE entry for the failed remove.
	data, err := os.ReadFile(runner.Log.Path())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "NOTE: remove update staging") {
		t.Fatalf("expected cleanup-warn note, got:\n%s", data)
	}
}

// TestCopyFileErrorBranches covers ReadFile failure (missing src) and
// Stat failure via a removed-in-between race — best-effort.
func TestCopyFileErrorBranches(t *testing.T) {
	dir := t.TempDir()
	if err := copyFile(filepath.Join(dir, "missing"), filepath.Join(dir, "dst")); err == nil {
		t.Fatal("expected copyFile to fail on missing src")
	}
}

// TestVerifyCosignVerifyBlobFails covers the branch where cosign is
// on PATH but verify-blob returns non-zero.
func TestVerifyCosignVerifyBlobFails(t *testing.T) {
	runner, dir := newSelfUpdateCtx(t)
	bin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	// curl: happy path writes a placeholder file
	if err := os.WriteFile(filepath.Join(bin, "curl"), []byte(`#!/bin/sh
dest=""
prev=""
for a in "$@"; do
  if [ "$prev" = "-o" ]; then dest="$a"; fi
  prev="$a"
done
printf 'x' > "$dest"
`), 0o755); err != nil {
		t.Fatal(err)
	}
	// cosign: exit non-zero with a helpful message
	if err := os.WriteFile(filepath.Join(bin, "cosign"), []byte(
		"#!/bin/sh\necho 'bad signature' >&2\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	err := verifyCosignSignature(context.Background(), runner, "owner/repo", "v1.2.3")
	if err == nil || !strings.Contains(err.Error(), "cosign verify-blob") {
		t.Fatalf("expected verify-blob failure, got %v", err)
	}
}

// TestVerifyCosignCurlFails covers the curl-error branch inside the
// cosign artifact download loop.
func TestVerifyCosignCurlFails(t *testing.T) {
	runner, dir := newSelfUpdateCtx(t)
	bin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	if err := os.WriteFile(filepath.Join(bin, "curl"),
		[]byte("#!/bin/sh\nexit 22\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "cosign"),
		[]byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	err := verifyCosignSignature(context.Background(), runner, "owner/repo", "v1.2.3")
	if err == nil || !strings.Contains(err.Error(), "download ") {
		t.Fatalf("expected cosign curl error, got %v", err)
	}
}

// restoreSeams snapshots the mutable package vars and restores them
// after the test — used by tests that override several at once.
func restoreSeams(t *testing.T) {
	t.Helper()
	origLatest := latestVersionFn
	origChecksums := fetchChecksumsFn
	origVerify := verifyFileFn
	origExe := executablePathFn
	origCosign := verifyCosignSignatureFn
	origCopy := copyFileFn
	t.Cleanup(func() {
		latestVersionFn = origLatest
		fetchChecksumsFn = origChecksums
		verifyFileFn = origVerify
		executablePathFn = origExe
		verifyCosignSignatureFn = origCosign
		copyFileFn = origCopy
	})
	// Defaults that make SelfUpdate reach the specific branch under test.
	latestVersionFn = func(string, bool) (string, error) { return "v1.2.4", nil }
	fetchChecksumsFn = func(context.Context, string) (map[string]string, error) {
		key := fmt.Sprintf("dotsetup-%s-%s", runtimeGOOS(), runtimeGOARCH())
		return map[string]string{key: "deadbeef"}, nil
	}
	verifyFileFn = func(string, string) error { return nil }
	verifyCosignSignatureFn = func(context.Context, *executor.Runner, string, string) error {
		return nil
	}
	copyFileFn = copyFile
}
