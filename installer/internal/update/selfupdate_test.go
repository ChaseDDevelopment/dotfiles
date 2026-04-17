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

func TestUpdateVersionLess(t *testing.T) {
	cases := []struct {
		name      string
		current   string
		latest    string
		wantFound bool
	}{
		{name: "equal", current: "v1.2.3", latest: "v1.2.3", wantFound: false},
		{name: "older current", current: "v1.2.2", latest: "v1.2.3", wantFound: true},
		{name: "newer current", current: "v1.2.4", latest: "v1.2.3", wantFound: false},
		{name: "prerelease current", current: "v1.2.3-rc1", latest: "v1.2.3", wantFound: true},
		{name: "prerelease latest", current: "v1.2.3", latest: "v1.2.3-rc1", wantFound: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cur, curPre, err := parseUpdateVersion(tc.current)
			if err != nil {
				t.Fatalf("parse current: %v", err)
			}
			lat, latPre, err := parseUpdateVersion(tc.latest)
			if err != nil {
				t.Fatalf("parse latest: %v", err)
			}
			if got := updateVersionLess(cur, curPre, lat, latPre); got != tc.wantFound {
				t.Fatalf("updateVersionLess(%q, %q) = %v, want %v", tc.current, tc.latest, got, tc.wantFound)
			}
		})
	}
}

func TestParseUpdateVersion(t *testing.T) {
	got, pre, err := parseUpdateVersion("v1.2.3-rc1")
	if err != nil {
		t.Fatalf("parseUpdateVersion: %v", err)
	}
	if got != [3]int{1, 2, 3} || !pre {
		t.Fatalf("unexpected parse result: got=%v pre=%v", got, pre)
	}
}

// TestParseUpdateVersionTable exercises the major
// parseUpdateVersion branches: each Atoi error path, the optional
// patch group, the v-prefix tolerance, build metadata stripping,
// and the empty / non-semver rejections.
func TestParseUpdateVersionTable(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		want    [3]int
		wantPre bool
		wantErr bool
	}{
		{name: "plain semver", in: "1.2.3", want: [3]int{1, 2, 3}},
		{name: "v prefixed", in: "v1.2.3", want: [3]int{1, 2, 3}},
		{name: "no patch", in: "v1.2", want: [3]int{1, 2, 0}},
		{name: "with prerelease", in: "v1.2.3-rc1", want: [3]int{1, 2, 3}, wantPre: true},
		{name: "build metadata stripped", in: "v1.2.3-rc.1+build.42", want: [3]int{1, 2, 3}, wantPre: true},
		{name: "leading whitespace", in: "  v0.4.1  ", want: [3]int{0, 4, 1}},
		{name: "embedded in tag text", in: "release-1.4.0-final", want: [3]int{1, 4, 0}, wantPre: true},
		{name: "empty string", in: "", wantErr: true},
		{name: "all garbage", in: "release-latest", wantErr: true},
		{name: "letters only", in: "abc", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, pre, err := parseUpdateVersion(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("parseUpdateVersion(%q) = (%v, %v, nil); want error", tc.in, got, pre)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseUpdateVersion(%q): unexpected err %v", tc.in, err)
			}
			if got != tc.want {
				t.Fatalf("parseUpdateVersion(%q) = %v, want %v", tc.in, got, tc.want)
			}
			if pre != tc.wantPre {
				t.Fatalf("parseUpdateVersion(%q) prerelease = %v, want %v", tc.in, pre, tc.wantPre)
			}
		})
	}
}

func TestParseUpdateVersionRejectsGarbage(t *testing.T) {
	_, _, err := parseUpdateVersion("release-latest")
	if err == nil {
		t.Fatal("expected parse error")
	}
	if !strings.Contains(err.Error(), "semver-like") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCheckSelfUpdateUsesOrdering(t *testing.T) {
	orig := latestVersionFn
	latestVersionFn = func(_ string, _ bool) (string, error) { return "v1.2.3", nil }
	defer func() { latestVersionFn = orig }()

	cases := []struct {
		current string
		want    string
	}{
		{current: "v1.2.2", want: "v1.2.3"},
		{current: "v1.2.3", want: ""},
		{current: "v1.2.4", want: ""},
	}
	for _, tc := range cases {
		got, err := CheckSelfUpdate(tc.current)
		if err != nil {
			t.Fatalf("CheckSelfUpdate(%q): %v", tc.current, err)
		}
		if got != tc.want {
			t.Fatalf("CheckSelfUpdate(%q) = %q, want %q", tc.current, got, tc.want)
		}
	}
}

func TestCheckSelfUpdateErrorBranches(t *testing.T) {
	orig := latestVersionFn
	defer func() { latestVersionFn = orig }()

	latestVersionFn = func(_ string, _ bool) (string, error) {
		return "", fmt.Errorf("network down")
	}
	if _, err := CheckSelfUpdate("v1.2.3"); err == nil || !strings.Contains(err.Error(), "check update") {
		t.Fatalf("expected wrapped latest-version error, got %v", err)
	}

	latestVersionFn = func(_ string, _ bool) (string, error) {
		return "release-latest", nil
	}
	if _, err := CheckSelfUpdate("v1.2.3"); err == nil || !strings.Contains(err.Error(), "parse latest version") {
		t.Fatalf("expected parse latest error, got %v", err)
	}

	if got, err := CheckSelfUpdate("dev"); err != nil || got != "" {
		t.Fatalf("dev build should skip checks, got %q err=%v", got, err)
	}
}

func TestCleanupTmpAndCopyFile(t *testing.T) {
	dir := t.TempDir()
	log, err := executor.NewLogFile(filepath.Join(dir, "test.log"))
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	runner := executor.NewRunner(log, false)

	tmpFile := filepath.Join(dir, "tmp")
	if err := os.WriteFile(tmpFile, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	cleanupTmp(runner, tmpFile)
	if _, err := os.Stat(tmpFile); !os.IsNotExist(err) {
		t.Fatalf("tmp file still exists: %v", err)
	}

	src := filepath.Join(dir, "src")
	dst := filepath.Join(dir, "dst")
	if err := os.WriteFile(src, []byte("hello"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := copyFile(src, dst); err != nil {
		t.Fatalf("copyFile: %v", err)
	}
	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello" {
		t.Fatalf("copied data = %q", data)
	}
}

// TestCopyFileMissingSource hits the os.ReadFile error branch
// inside copyFile (selfupdate.go:355–357) — when the source path
// does not exist, the function must surface the read error and
// must not create the destination.
func TestCopyFileMissingSource(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "does-not-exist")
	dst := filepath.Join(dir, "dst")

	err := copyFile(missing, dst)
	if err == nil {
		t.Fatal("expected copyFile to fail on missing source")
	}
	if !os.IsNotExist(err) {
		t.Fatalf("expected IsNotExist error, got %v", err)
	}
	if _, err := os.Stat(dst); !os.IsNotExist(err) {
		t.Fatalf("destination should not exist after failed copy: stat=%v", err)
	}
}

// TestSelfUpdateBackupRemoveNoteBranch exercises the
// `NOTE: remove rollback backup` log path at selfupdate.go:254–258.
// The strategy: drive a successful SelfUpdate but make the parent
// directory of the backup file non-writable BEFORE the post-success
// remove fires, so os.Remove on `<exe>.old` returns EACCES. The
// update itself must still succeed (the NOTE is non-fatal); the log
// must contain the marker line.
func TestSelfUpdateBackupRemoveNoteBranch(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root bypasses dir-permission checks; rm cannot fail")
	}
	if runtime.GOOS == "windows" {
		t.Skip("windows chmod semantics differ; not applicable")
	}

	dir := t.TempDir()
	log, err := executor.NewLogFile(filepath.Join(dir, "test.log"))
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	runner := executor.NewRunner(log, false)

	// Place the executable inside a dedicated subdirectory so we can
	// chmod that subdirectory read-only after the rename succeeds —
	// the .old removal will then EACCES, but the rename target
	// (already renamed in place) is unaffected.
	exeDir := filepath.Join(dir, "exedir")
	if err := os.MkdirAll(exeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(exeDir, "dotsetup")
	if err := os.WriteFile(exe, []byte("old-binary"), 0o755); err != nil {
		t.Fatal(err)
	}

	// PATH-shim curl that writes new-binary bytes to `-o` target.
	fakebin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(fakebin, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fakebin+string(os.PathListSeparator)+os.Getenv("PATH"))
	if err := os.WriteFile(filepath.Join(fakebin, "curl"), []byte(`#!/usr/bin/env bash
dest=""
while [ "$#" -gt 0 ]; do
  if [ "$1" = "-o" ]; then
    dest="$2"
    shift 2
    continue
  fi
  shift
done
printf 'new-binary' > "$dest"
`), 0o755); err != nil {
		t.Fatal(err)
	}

	origLatest := latestVersionFn
	origChecksums := fetchChecksumsFn
	origVerify := verifyFileFn
	origExe := executablePathFn
	origCosign := verifyCosignSignatureFn
	origCopy := copyFileFn
	defer func() {
		latestVersionFn = origLatest
		fetchChecksumsFn = origChecksums
		verifyFileFn = origVerify
		executablePathFn = origExe
		verifyCosignSignatureFn = origCosign
		copyFileFn = origCopy
	}()

	latestVersionFn = func(_ string, _ bool) (string, error) { return "v1.2.4", nil }
	fetchChecksumsFn = func(_ context.Context, _ string) (map[string]string, error) {
		return map[string]string{
			fmt.Sprintf("dotsetup-%s-%s", runtime.GOOS, runtime.GOARCH): "deadbeef",
		}, nil
	}
	verifyFileFn = func(_ string, _ string) error { return nil }
	executablePathFn = func() (string, error) { return exe, nil }
	verifyCosignSignatureFn = func(context.Context, *executor.Runner, string, string) error { return nil }

	// copyFileFn produces the backup, then we make the parent dir
	// read-only so os.Remove(backupFile) fails after the rename.
	copyFileFn = func(src, dst string) error {
		if err := copyFile(src, dst); err != nil {
			return err
		}
		// Strip write perms on the parent so the post-success Remove
		// of <exe>.old returns EACCES. The rename of the staging
		// file already happened (or hasn't yet — but Rename within
		// the same dir on a w-stripped dir would also fail, so we
		// must defer the chmod until after Rename).
		// Trick: schedule the chmod via a goroutine? Instead, simpler
		// approach — chmod the dir 0o555 right here. os.Rename in
		// the same directory needs WRITE on the dir. So we cannot
		// chmod yet. We need to make .old un-removable while still
		// allowing rename + chmod chain.
		//
		// Alternative: replace the .old file with a directory
		// containing a child file, so os.Remove (not RemoveAll) on
		// the directory fails with ENOTEMPTY.
		if err := os.Remove(dst); err != nil {
			return err
		}
		if err := os.MkdirAll(dst, 0o755); err != nil {
			return err
		}
		// non-empty so plain os.Remove returns ENOTEMPTY.
		if err := os.WriteFile(filepath.Join(dst, "child"), []byte("x"), 0o644); err != nil {
			return err
		}
		return nil
	}

	if err := SelfUpdate(context.Background(), runner, "v1.2.3"); err != nil {
		t.Fatalf("SelfUpdate should succeed despite backup-remove failure: %v", err)
	}

	// New binary is in place.
	got, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new-binary" {
		t.Fatalf("binary not replaced: got %q", got)
	}

	// Log must contain the NOTE about the backup removal failure.
	logBytes, err := os.ReadFile(log.Path())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(logBytes), "NOTE: remove rollback backup") {
		t.Fatalf("expected NOTE marker in log, got:\n%s", logBytes)
	}
}

func TestVerifyCosignSignature(t *testing.T) {
	dir := t.TempDir()
	log, err := executor.NewLogFile(filepath.Join(dir, "test.log"))
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	runner := executor.NewRunner(log, false)

	fakebin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(fakebin, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fakebin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("COSIGN_LOG", filepath.Join(dir, "cosign.log"))
	writeScript := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(fakebin, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeScript("curl", `#!/usr/bin/env bash
dest=""
while [ "$#" -gt 0 ]; do
  if [ "$1" = "-o" ]; then
    dest="$2"
    shift 2
    continue
  fi
  shift
done
printf 'signed' > "$dest"
`)
	writeScript("cosign", `#!/usr/bin/env bash
printf '%s\n' "$*" > "$COSIGN_LOG"
`)

	if err := verifyCosignSignature(context.Background(), runner, "owner/repo", "v1.2.3"); err != nil {
		t.Fatalf("verifyCosignSignature: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "cosign.log"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	if !strings.Contains(got, "verify-blob") || !strings.Contains(got, "SHA256SUMS") {
		t.Fatalf("unexpected cosign argv: %q", got)
	}
}

func TestVerifyCosignSignatureWithoutCosign(t *testing.T) {
	dir := t.TempDir()
	log, err := executor.NewLogFile(filepath.Join(dir, "test.log"))
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	runner := executor.NewRunner(log, false)

	t.Setenv("PATH", dir)
	t.Setenv("DOTSETUP_REQUIRE_COSIGN", "1")
	if err := verifyCosignSignature(context.Background(), runner, "owner/repo", "v1.2.3"); err == nil || !strings.Contains(err.Error(), "DOTSETUP_REQUIRE_COSIGN") {
		t.Fatalf("expected strict cosign error, got %v", err)
	}

	t.Setenv("DOTSETUP_REQUIRE_COSIGN", "")
	if err := verifyCosignSignature(context.Background(), runner, "owner/repo", "v1.2.3"); err != nil {
		t.Fatalf("expected warning-only fallback, got %v", err)
	}

	data, err := os.ReadFile(log.Path())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "cosign not installed") {
		t.Fatalf("expected fallback warning in log, got:\n%s", data)
	}
}

func TestSelfUpdateSuccess(t *testing.T) {
	dir := t.TempDir()
	log, err := executor.NewLogFile(filepath.Join(dir, "test.log"))
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	runner := executor.NewRunner(log, false)

	fakebin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(fakebin, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fakebin+string(os.PathListSeparator)+os.Getenv("PATH"))
	writeFile := filepath.Join(dir, "downloaded")
	if err := os.WriteFile(writeFile, []byte("new-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fakebin, "curl"), []byte(fmt.Sprintf(`#!/usr/bin/env bash
dest=""
while [ "$#" -gt 0 ]; do
  if [ "$1" = "-o" ]; then
    dest="$2"
    shift 2
    continue
  fi
  shift
done
cp %q "$dest"
`, writeFile)), 0o755); err != nil {
		t.Fatal(err)
	}

	exe := filepath.Join(dir, "dotsetup")
	if err := os.WriteFile(exe, []byte("old-binary"), 0o755); err != nil {
		t.Fatal(err)
	}

	origLatest := latestVersionFn
	origChecksums := fetchChecksumsFn
	origVerify := verifyFileFn
	origExe := executablePathFn
	origCosign := verifyCosignSignatureFn
	latestVersionFn = func(_ string, _ bool) (string, error) { return "v1.2.4", nil }
	fetchChecksumsFn = func(_ context.Context, _ string) (map[string]string, error) {
		return map[string]string{"dotsetup-darwin-arm64": "deadbeef"}, nil
	}
	verifyFileFn = func(_ string, _ string) error { return nil }
	executablePathFn = func() (string, error) { return exe, nil }
	verifyCosignSignatureFn = func(context.Context, *executor.Runner, string, string) error { return nil }
	defer func() {
		latestVersionFn = origLatest
		fetchChecksumsFn = origChecksums
		verifyFileFn = origVerify
		executablePathFn = origExe
		verifyCosignSignatureFn = origCosign
	}()

	if err := SelfUpdate(context.Background(), runner, "v1.2.3"); err != nil {
		t.Fatalf("SelfUpdate: %v", err)
	}
	data, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "new-binary" {
		t.Fatalf("updated binary = %q", data)
	}
	if _, err := os.Stat(exe + ".old"); !os.IsNotExist(err) {
		t.Fatalf("expected backup cleanup, stat err=%v", err)
	}
}

func TestSelfUpdateErrorBranches(t *testing.T) {
	dir := t.TempDir()
	log, err := executor.NewLogFile(filepath.Join(dir, "test.log"))
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	runner := executor.NewRunner(log, false)
	fakebin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(fakebin, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fakebin+string(os.PathListSeparator)+os.Getenv("PATH"))
	if err := os.WriteFile(filepath.Join(fakebin, "curl"), []byte(`#!/bin/sh
dest=""
while [ "$#" -gt 0 ]; do
  if [ "$1" = "-o" ]; then
    dest="$2"
    shift 2
    continue
  fi
  shift
done
printf 'new' > "$dest"
`), 0o755); err != nil {
		t.Fatal(err)
	}

	exe := filepath.Join(dir, "dotsetup")
	if err := os.WriteFile(exe, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}

	origLatest := latestVersionFn
	origChecksums := fetchChecksumsFn
	origVerify := verifyFileFn
	origExe := executablePathFn
	origCosign := verifyCosignSignatureFn
	origCopy := copyFileFn
	latestVersionFn = func(_ string, _ bool) (string, error) { return "v1.2.4", nil }
	executablePathFn = func() (string, error) { return exe, nil }
	verifyCosignSignatureFn = func(context.Context, *executor.Runner, string, string) error { return nil }
	defer func() {
		latestVersionFn = origLatest
		fetchChecksumsFn = origChecksums
		verifyFileFn = origVerify
		executablePathFn = origExe
		verifyCosignSignatureFn = origCosign
		copyFileFn = origCopy
	}()

	fetchChecksumsFn = func(_ context.Context, _ string) (map[string]string, error) {
		return map[string]string{}, nil
	}
	if err := SelfUpdate(context.Background(), runner, "v1.2.3"); err == nil || !strings.Contains(err.Error(), "missing entry") {
		t.Fatalf("expected missing checksum entry error, got %v", err)
	}

	fetchChecksumsFn = func(_ context.Context, _ string) (map[string]string, error) {
		return map[string]string{"dotsetup-darwin-arm64": "deadbeef"}, nil
	}
	verifyFileFn = func(_ string, _ string) error { return nil }
	copyFileFn = func(_, _ string) error { return fmt.Errorf("backup failed") }
	if err := SelfUpdate(context.Background(), runner, "v1.2.3"); err == nil || !strings.Contains(err.Error(), "backup current binary") {
		t.Fatalf("expected backup failure, got %v", err)
	}
}

func TestSelfUpdateAlreadyUpToDateAndIntegrityFailure(t *testing.T) {
	dir := t.TempDir()
	log, err := executor.NewLogFile(filepath.Join(dir, "test.log"))
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	runner := executor.NewRunner(log, false)

	exe := filepath.Join(dir, "dotsetup")
	if err := os.WriteFile(exe, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}

	origLatest := latestVersionFn
	origChecksums := fetchChecksumsFn
	origVerify := verifyFileFn
	origExe := executablePathFn
	origCosign := verifyCosignSignatureFn
	defer func() {
		latestVersionFn = origLatest
		fetchChecksumsFn = origChecksums
		verifyFileFn = origVerify
		executablePathFn = origExe
		verifyCosignSignatureFn = origCosign
	}()

	latestVersionFn = func(_ string, _ bool) (string, error) { return "v1.2.3", nil }
	if err := SelfUpdate(context.Background(), runner, "v1.2.3"); err != nil {
		t.Fatalf("up-to-date self update should no-op: %v", err)
	}
	data, err := os.ReadFile(log.Path())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "already up to date") {
		t.Fatalf("expected up-to-date log, got:\n%s", data)
	}

	fakebin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(fakebin, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fakebin+string(os.PathListSeparator)+os.Getenv("PATH"))
	if err := os.WriteFile(filepath.Join(fakebin, "curl"), []byte(`#!/bin/sh
dest=""
while [ "$#" -gt 0 ]; do
  if [ "$1" = "-o" ]; then
    dest="$2"
    shift 2
    continue
  fi
  shift
done
printf 'new' > "$dest"
`), 0o755); err != nil {
		t.Fatal(err)
	}

	latestVersionFn = func(_ string, _ bool) (string, error) { return "v1.2.4", nil }
	fetchChecksumsFn = func(_ context.Context, _ string) (map[string]string, error) {
		return map[string]string{"dotsetup-darwin-arm64": "deadbeef"}, nil
	}
	verifyFileFn = func(_ string, _ string) error { return fmt.Errorf("digest mismatch") }
	executablePathFn = func() (string, error) { return exe, nil }
	verifyCosignSignatureFn = func(context.Context, *executor.Runner, string, string) error { return nil }
	if err := SelfUpdate(context.Background(), runner, "v1.2.3"); err == nil || !strings.Contains(err.Error(), "integrity check failed") {
		t.Fatalf("expected integrity failure, got %v", err)
	}
	if _, err := os.Stat(exe + ".new"); !os.IsNotExist(err) {
		t.Fatalf("staging file should be cleaned up, stat err=%v", err)
	}
}
