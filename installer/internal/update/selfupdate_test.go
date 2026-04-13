package update

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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
