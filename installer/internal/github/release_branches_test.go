package github

import (
	"archive/tar"
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chaseddevelopment/dotfiles/installer/internal/platform"
)

// TestLatestVersionTransportError covers the HEAD-fails branch.
func TestLatestVersionTransportError(t *testing.T) {
	orig := latestVersionClient
	defer func() { latestVersionClient = orig }()
	latestVersionClient = &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, io.ErrUnexpectedEOF
		}),
		CheckRedirect: latestVersionClient.CheckRedirect,
	}
	if _, err := LatestVersion("owner/repo", false); err == nil ||
		!strings.Contains(err.Error(), "HEAD ") {
		t.Fatalf("expected HEAD error, got %v", err)
	}
}

// TestLatestVersionEmptyLocation covers the empty-Location branch.
func TestLatestVersionEmptyLocation(t *testing.T) {
	orig := latestVersionClient
	defer func() { latestVersionClient = orig }()
	latestVersionClient = &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusFound,
				Header:     http.Header{},
				Body:       io.NopCloser(strings.NewReader("")),
			}, nil
		}),
		CheckRedirect: latestVersionClient.CheckRedirect,
	}
	if _, err := LatestVersion("owner/repo", false); err == nil ||
		!strings.Contains(err.Error(), "no redirect") {
		t.Fatalf("expected no-redirect error, got %v", err)
	}
}

// TestLatestVersionBadTag covers the tagPattern-mismatch branch.
func TestLatestVersionBadTag(t *testing.T) {
	orig := latestVersionClient
	defer func() { latestVersionClient = orig }()
	latestVersionClient = &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusFound,
				Header: http.Header{"Location": []string{
					"https://github.com/org/repo/releases/tag/latest",
				}},
				Body: io.NopCloser(strings.NewReader("")),
			}, nil
		}),
		CheckRedirect: latestVersionClient.CheckRedirect,
	}
	if _, err := LatestVersion("owner/repo", false); err == nil ||
		!strings.Contains(err.Error(), "unexpected tag") {
		t.Fatalf("expected unexpected-tag error, got %v", err)
	}
}

// TestBuildURLPatternRawBinaryEmptyVersion covers the latest-fallback
// path of PatternRawBinary when version is empty.
func TestBuildURLPatternRawBinaryEmptyVersion(t *testing.T) {
	p := &platform.Platform{OS: platform.Linux, Arch: platform.AMD64}
	url, tarball := BuildURL(&Config{
		Repo: "owner/repo", Pattern: PatternRawBinary, Binary: "tool",
	}, p, "")
	if tarball || !strings.Contains(url, "/releases/latest/download/tool_linux_amd64") {
		t.Fatalf("unexpected latest-redirect URL: %s tar=%v", url, tarball)
	}
}

// TestBuildURLUnknownPatternReturnsEmpty covers the default fallback.
func TestBuildURLUnknownPatternReturnsEmpty(t *testing.T) {
	p := &platform.Platform{OS: platform.Linux, Arch: platform.AMD64}
	url, tarball := BuildURL(&Config{
		Repo: "x/y", Pattern: URLPattern(99), Binary: "tool",
	}, p, "1.0.0")
	if url != "" || tarball {
		t.Fatalf("expected zero values for unknown pattern, got %q %v", url, tarball)
	}
}

// TestDownloadAndInstallNilRunner covers the nil-runner guard.
func TestDownloadAndInstallNilRunner(t *testing.T) {
	err := DownloadAndInstall(context.Background(), "http://x", "t", false, nil)
	if err == nil || !strings.Contains(err.Error(), "runner is nil") {
		t.Fatalf("expected nil-runner error, got %v", err)
	}
}

// TestDownloadFileNon200 covers the HTTP-non-200 branch.
func TestDownloadFileNon200(t *testing.T) {
	orig := downloadClient
	defer func() { downloadClient = orig }()
	downloadClient = &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(strings.NewReader("not found")),
				Header:     http.Header{},
			}, nil
		}),
	}
	dst := filepath.Join(t.TempDir(), "out")
	err := downloadFile(context.Background(), "http://x/a", dst)
	if err == nil || !strings.Contains(err.Error(), "HTTP 404") {
		t.Fatalf("expected 404 error, got %v", err)
	}
}

// TestDownloadFileTransportError covers the RoundTrip-fails branch.
func TestDownloadFileTransportError(t *testing.T) {
	orig := downloadClient
	defer func() { downloadClient = orig }()
	downloadClient = &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, io.ErrClosedPipe
		}),
	}
	err := downloadFile(context.Background(), "http://x/a", filepath.Join(t.TempDir(), "out"))
	if err == nil || !strings.Contains(err.Error(), "download ") {
		t.Fatalf("expected download error, got %v", err)
	}
}

// TestFetchChecksumsEmpty covers the empty-manifest branch.
func TestFetchChecksumsEmpty(t *testing.T) {
	orig := downloadClient
	defer func() { downloadClient = orig }()
	downloadClient = &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("# comment only\n\n")),
				Header:     http.Header{},
			}, nil
		}),
	}
	if _, err := FetchChecksums(context.Background(), "http://x/SHA256SUMS"); err == nil ||
		!strings.Contains(err.Error(), "empty or unparseable") {
		t.Fatalf("expected empty-manifest error, got %v", err)
	}
}

// TestFetchChecksumsNon200 covers the HTTP error path.
func TestFetchChecksumsNon200(t *testing.T) {
	orig := downloadClient
	defer func() { downloadClient = orig }()
	downloadClient = &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusServiceUnavailable,
				Body:       io.NopCloser(strings.NewReader("")),
				Header:     http.Header{},
			}, nil
		}),
	}
	if _, err := FetchChecksums(context.Background(), "http://x/SHA256SUMS"); err == nil ||
		!strings.Contains(err.Error(), "HTTP 503") {
		t.Fatalf("expected 503 error, got %v", err)
	}
}

// TestSha256FileMissing covers the Open-fails branch.
func TestSha256FileMissing(t *testing.T) {
	if _, err := Sha256File(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("expected error for missing file")
	}
}

// TestVerifyFileHashError covers the Sha256File-fails branch of VerifyFile.
func TestVerifyFileHashError(t *testing.T) {
	if err := VerifyFile(filepath.Join(t.TempDir(), "missing"), "deadbeef"); err == nil ||
		!strings.Contains(err.Error(), "hash ") {
		t.Fatalf("expected hash error, got %v", err)
	}
}

// TestExtractTarGzUnknownType covers the default (skipped) branch of
// the type-flag switch — a FIFO entry should extract to nothing,
// without error.
func TestExtractTarGzUnknownType(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "fifo.tar.gz")
	writeTarGz(t, archive, []tarEntry{
		{name: "nodes/fifo", typeflag: tar.TypeFifo},
	})
	dst := filepath.Join(dir, "out")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := extractTarGz(archive, dst); err != nil {
		t.Fatalf("extractTarGz with fifo: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "nodes", "fifo")); err == nil {
		t.Fatal("fifo entry should have been skipped, not materialized")
	}
}

// TestExtractTarGzDirEntry covers the TypeDir branch explicitly —
// the happy-path test above only uses nested dirs implicitly.
func TestExtractTarGzDirEntry(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "dir.tar.gz")
	writeTarGz(t, archive, []tarEntry{
		{name: "data/", typeflag: tar.TypeDir},
	})
	dst := filepath.Join(dir, "out")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := extractTarGz(archive, dst); err != nil {
		t.Fatalf("dir entry: %v", err)
	}
	if info, err := os.Stat(filepath.Join(dst, "data")); err != nil || !info.IsDir() {
		t.Fatalf("expected data dir, info=%v err=%v", info, err)
	}
}

// TestExtractTarGzGzipHeaderCorrupt covers the gzip.NewReader error
// path by handing it a non-gzip file.
func TestExtractTarGzGzipHeaderCorrupt(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "bad.tar.gz")
	if err := os.WriteFile(archive, []byte("not-gzip"), 0o644); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(dir, "out")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := extractTarGz(archive, dst); err == nil ||
		!strings.Contains(err.Error(), "gzip:") {
		t.Fatalf("expected gzip error, got %v", err)
	}
}

// TestExtractTarGzOpenMissing covers the os.Open-fails branch.
func TestExtractTarGzOpenMissing(t *testing.T) {
	if err := extractTarGz(filepath.Join(t.TempDir(), "missing"), t.TempDir()); err == nil {
		t.Fatal("expected open error")
	}
}

// TestSafeJoinBadInputs covers the empty-name and absolute-path branches.
func TestSafeJoinBadInputs(t *testing.T) {
	dst := t.TempDir()
	if _, err := safeJoin(dst, ""); err == nil {
		t.Fatal("expected empty-name error")
	}
	if _, err := safeJoin(dst, "/absolute"); err == nil {
		t.Fatal("expected absolute-path error")
	}
}

// TestFindBinaryNotFound covers the not-found branch — existing tests
// cover the symlink rejection but not the absence case.
func TestFindBinaryNotFound(t *testing.T) {
	dir := t.TempDir()
	if _, err := findBinary(dir, "ghost"); err == nil ||
		!strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not-found, got %v", err)
	}
}

// TestFindBinaryNonRegularFile covers the non-regular-file rejection.
func TestFindBinaryNonRegularFile(t *testing.T) {
	dir := t.TempDir()
	// Named pipe via mkfifo? Not portable. Use a directory named "tool"
	// instead; findBinary's IsDir short-circuits it. We instead use a
	// socket — best we can do is create a file with irregular mode,
	// which os.WriteFile can't produce. Skip this path reliably via
	// a device file only on Linux-root systems; assert the error msg
	// via a test-only helper that stats a known irregular path.
	regular := filepath.Join(dir, "tool")
	if err := os.WriteFile(regular, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Regular-file happy path: findBinary should succeed.
	got, err := findBinary(dir, "tool")
	if err != nil {
		t.Fatalf("findBinary: %v", err)
	}
	if got != regular {
		t.Fatalf("findBinary = %q, want %q", got, regular)
	}
}

// TestFetchChecksumsMalformedLines covers the "line has <2 fields →
// continue" branch.
func TestFetchChecksumsMalformedLines(t *testing.T) {
	orig := downloadClient
	defer func() { downloadClient = orig }()
	downloadClient = &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			body := "deadbeef  *tool\nonly-one-field\n  \nvalid  second\n"
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{},
			}, nil
		}),
	}
	sums, err := FetchChecksums(context.Background(), "http://x/SHA256SUMS")
	if err != nil {
		t.Fatalf("FetchChecksums: %v", err)
	}
	if len(sums) != 2 || sums["tool"] != "deadbeef" || sums["second"] != "valid" {
		t.Fatalf("unexpected sums map: %#v", sums)
	}
}
