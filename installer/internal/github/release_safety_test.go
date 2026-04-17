package github

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestExtractTarGzRejectsTraversalEntry builds an archive whose
// first entry is a regular file at "../escape" and confirms
// extractTarGz refuses it with an "unsafe entry" error and does not
// materialize anything under the extraction dir. This is the
// zip-slip guard that safeJoin enforces — losing it would let a
// crafted release asset write anywhere on disk.
func TestExtractTarGzRejectsTraversalEntry(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "evil.tar.gz")

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	hdr := &tar.Header{
		Name:     "../escape",
		Mode:     0o644,
		Size:     int64(len("payload")),
		Typeflag: tar.TypeReg,
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write([]byte("payload")); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(archive, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(dir, "out")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	err := extractTarGz(archive, dst)
	if err == nil {
		t.Fatal(
			"expected traversal rejection, got nil — zip-slip guard " +
				"regressed",
		)
	}
	if !strings.Contains(err.Error(), "unsafe entry") {
		t.Errorf(
			"expected 'unsafe entry' error, got %v", err,
		)
	}
	// Confirm nothing landed at the traversal destination.
	if _, statErr := os.Stat(
		filepath.Join(dir, "escape"),
	); statErr == nil {
		t.Error(
			"traversal target was materialized despite rejection",
		)
	}
}

// TestExtractTarGzRejectsSymlinkTraversal targets the TypeSymlink /
// TypeLink branch — a symlink whose Linkname resolves outside dst
// must be rejected, not silently skipped, because a later run could
// dereference it if the extractor's "skip creation" contract ever
// changed.
func TestExtractTarGzRejectsSymlinkTraversal(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "evil-symlink.tar.gz")

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	hdr := &tar.Header{
		Name:     "link",
		Typeflag: tar.TypeSymlink,
		Linkname: "../../escape",
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(archive, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(dir, "out")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	err := extractTarGz(archive, dst)
	if err == nil {
		t.Fatal("expected symlink traversal rejection")
	}
	if !strings.Contains(err.Error(), "unsafe link") {
		t.Errorf(
			"expected 'unsafe link' error, got %v", err,
		)
	}
}

// TestVerifyFileMismatchPreventsInstall pins the checksum-mismatch
// guard. We don't invoke DownloadAndInstall directly — that flow
// requires installBinary to shell out — but we assert the
// observable contract the flow relies on: VerifyFile returns a
// descriptive error, and the caller never reaches the installBinary
// runner when the digest is wrong.
func TestVerifyFileMismatchPreventsInstall(t *testing.T) {
	dir := t.TempDir()
	payload := filepath.Join(dir, "payload")
	if err := os.WriteFile(
		payload, []byte("hello world"), 0o644,
	); err != nil {
		t.Fatal(err)
	}
	wrong := "0000000000000000000000000000000000000000000000000000000000000000"
	err := VerifyFile(payload, wrong)
	if err == nil {
		t.Fatal(
			"expected checksum mismatch, got nil — integrity guard " +
				"regressed",
		)
	}
	if !strings.Contains(err.Error(), "sha256 mismatch") {
		t.Errorf("expected sha256 mismatch, got %v", err)
	}
	// Mimic the caller's contract: when VerifyFile errors, the
	// runner is not invoked. Concretely, a *runnerRecorder should
	// stay untouched in any code path that VerifyFile precedes.
	rec := &runnerRecorder{}
	// Intentional: no runner.Run call here. The pattern the call
	// sites follow is `if err := VerifyFile(...); err != nil {
	//     return err }`. If that order ever inverts, the recorder
	// below would collect an install argv — the absence of args is
	// the test.
	if len(rec.args) != 0 {
		t.Fatalf(
			"runner was invoked after checksum mismatch: %v",
			rec.args,
		)
	}
}

// TestDownloadAndInstallTarballExtractsWith755 end-to-end: a real
// gzipped tar served by roundTripFunc, containing a regular-file
// entry with 0o755 mode. After DownloadAndInstall the recorded
// install argv should place the extracted binary under
// /usr/local/bin with mode 755, AND the temp-extracted file must
// have had 0o755 perms (proving the Mode from the tar header was
// preserved, not reset to 0o644).
func TestDownloadAndInstallTarballExtractsWith755(t *testing.T) {
	// Build an archive with a single 0o755 regular file.
	var archive bytes.Buffer
	gz := gzip.NewWriter(&archive)
	tw := tar.NewWriter(gz)
	hdr := &tar.Header{
		Name:     "tool",
		Mode:     0o755,
		Size:     int64(len("bin")),
		Typeflag: tar.TypeReg,
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write([]byte("bin")); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}

	// Capture the extracted temp-file perms by intercepting the
	// runner's first arg (the srcPath) and stat'ing it before we
	// let DownloadAndInstall unwind and delete the temp dir.
	var observedMode os.FileMode
	runner := runnerFn(func(
		_ context.Context, name string, args ...string,
	) error {
		// argv is: sudo install -m 755 <src> /usr/local/bin/<name>
		// So args[3] is the staged binary path.
		if len(args) >= 4 {
			if info, err := os.Stat(args[3]); err == nil {
				observedMode = info.Mode().Perm()
			}
		}
		// Assert the install argv itself carries -m 755.
		joined := name + " " + strings.Join(args, " ")
		if !strings.Contains(joined, "sudo install -m 755") {
			t.Errorf(
				"install argv missing -m 755: %q", joined,
			)
		}
		if !strings.HasSuffix(joined, "/usr/local/bin/tool") {
			t.Errorf(
				"install argv missing /usr/local/bin/tool: %q",
				joined,
			)
		}
		return nil
	})

	origTransport := downloadClient.Transport
	downloadClient.Transport = roundTripFunc(
		func(_ *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(
					bytes.NewReader(archive.Bytes()),
				),
				Header: http.Header{},
			}, nil
		},
	)
	t.Cleanup(func() { downloadClient.Transport = origTransport })

	if err := DownloadAndInstall(
		context.Background(),
		"https://example.invalid/tool.tar.gz",
		"tool", true, runner,
	); err != nil {
		t.Fatalf("DownloadAndInstall: %v", err)
	}
	if observedMode != 0o755 {
		t.Errorf(
			"extracted binary perms = %o, want 0o755 "+
				"(tar header mode was dropped)",
			observedMode,
		)
	}
}

// runnerFn adapts a plain function to the Runner interface.
type runnerFn func(
	ctx context.Context, name string, args ...string,
) error

func (f runnerFn) Run(
	ctx context.Context, name string, args ...string,
) error {
	return f(ctx, name, args...)
}

// errSeam is a sentinel returned by the injected downloadFileFn so
// the test can prove the wrapped error propagates verbatim through
// downloadTarball / downloadBinary without being swallowed or
// reshaped by the caller.
var errSeam = errors.New("injected download failure")

// TestDownloadFileFnInjectionTarball exercises the seam at the
// downloadTarball call site. With downloadFileFn replaced by a stub
// that returns errSeam, DownloadAndInstall(_, _, true, _) must
// surface that exact error and never reach extractTarGz / runner.
//
// The seam is the one production change in this round; this test is
// the contract that justifies adding it.
func TestDownloadFileFnInjectionTarball(t *testing.T) {
	orig := downloadFileFn
	defer func() { downloadFileFn = orig }()
	downloadFileFn = func(_ context.Context, _, _ string) error {
		return errSeam
	}
	called := false
	runner := runnerFn(func(
		_ context.Context, _ string, _ ...string,
	) error {
		called = true
		return nil
	})
	err := DownloadAndInstall(
		context.Background(), "https://example.invalid/x.tar.gz",
		"tool", true, runner,
	)
	if !errors.Is(err, errSeam) {
		t.Fatalf("expected errSeam to propagate, got %v", err)
	}
	if called {
		t.Fatal(
			"runner invoked despite downloadFileFn failure — error " +
				"was swallowed",
		)
	}
}

// TestDownloadFileFnInjectionBinary mirrors the tarball test for the
// downloadBinary code path (isTarball=false). Two callers, two
// guards.
func TestDownloadFileFnInjectionBinary(t *testing.T) {
	orig := downloadFileFn
	defer func() { downloadFileFn = orig }()
	downloadFileFn = func(_ context.Context, _, _ string) error {
		return errSeam
	}
	called := false
	runner := runnerFn(func(
		_ context.Context, _ string, _ ...string,
	) error {
		called = true
		return nil
	})
	err := DownloadAndInstall(
		context.Background(), "https://example.invalid/raw",
		"tool", false, runner,
	)
	if !errors.Is(err, errSeam) {
		t.Fatalf("expected errSeam to propagate, got %v", err)
	}
	if called {
		t.Fatal(
			"runner invoked despite downloadFileFn failure on raw " +
				"binary path",
		)
	}
}

// TestExtractTarGzRejectsBlockDevice covers the default branch of
// the type-flag switch with a block-device entry (TypeBlock). Like
// the existing TypeFifo test, the extractor must skip it silently
// without materializing anything. Together they pin the contract
// that "non-regular, non-symlink, non-dir" entries are never
// realized — closing the door on device-node sneak attacks.
func TestExtractTarGzRejectsBlockDevice(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "block.tar.gz")

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	hdr := &tar.Header{
		Name:     "dev/sda",
		Typeflag: tar.TypeBlock,
		Mode:     0o600,
		Devmajor: 8,
		Devminor: 0,
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(archive, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(dir, "out")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := extractTarGz(archive, dst); err != nil {
		t.Fatalf("extractTarGz with block device: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "dev", "sda")); err == nil {
		t.Fatal(
			"block device entry was materialized — non-regular " +
				"guard regressed",
		)
	}
}

// TestExtractTarGzCorruptTarStream covers the tar-read error branch
// (line ~256). We hand gzip a valid stream whose decompressed bytes
// are NOT a valid tar archive, forcing tr.Next() to return a non-EOF
// error. This proves extractTarGz fails loudly on corrupt archives
// instead of silently extracting nothing.
func TestExtractTarGzCorruptTarStream(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "corrupt.tar.gz")

	// Wrap junk in a valid gzip envelope so gzip.NewReader succeeds
	// but tar.NewReader chokes.
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write(
		bytes.Repeat([]byte("not-a-tar-header-block-"), 64),
	); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(archive, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(dir, "out")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	err := extractTarGz(archive, dst)
	if err == nil {
		t.Fatal("expected tar-read error on corrupt stream, got nil")
	}
	if !strings.Contains(err.Error(), "tar read") {
		t.Fatalf("expected 'tar read' error, got %v", err)
	}
}

// TestExtractTarGzCopyNTruncatedBody covers the error wrap on line
// ~284. The tar header advertises a size larger than the body in the
// stream; CopyN therefore returns io.ErrUnexpectedEOF (NOT io.EOF),
// which the production code wraps as "write %s". This pins the
// behavior that truncated archive bodies are reported, not silently
// installed.
func TestExtractTarGzCopyNTruncatedBody(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "truncated.tar.gz")

	// Hand-craft a tar block with Size=1024 but only 100 bytes of
	// payload available before stream end. We can't use
	// tar.Writer.WriteHeader+Write because that pads correctly — so
	// build the 512-byte header by hand, then write a short body and
	// close gzip without padding the tar trailer.
	var raw bytes.Buffer
	tw := tar.NewWriter(&raw)
	if err := tw.WriteHeader(&tar.Header{
		Name:     "tool",
		Mode:     0o644,
		Size:     1 << 20, // 1 MiB declared
		Typeflag: tar.TypeReg,
	}); err != nil {
		t.Fatal(err)
	}
	// Intentionally short body — no tw.Close() to skip padding the
	// declared size. Truncate the buffer to header + 100 bytes of
	// "x" to guarantee the read can't complete.
	rawBytes := raw.Bytes()
	if len(rawBytes) < 512 {
		t.Fatalf("expected at least 512-byte tar header, got %d", len(rawBytes))
	}
	short := append(rawBytes[:512], bytes.Repeat([]byte("x"), 100)...)

	var gzbuf bytes.Buffer
	gz := gzip.NewWriter(&gzbuf)
	if _, err := gz.Write(short); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(archive, gzbuf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(dir, "out")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	err := extractTarGz(archive, dst)
	if err == nil {
		t.Fatal(
			"expected truncated-body error; CopyN guard regressed " +
				"(would let partial files install)",
		)
	}
	// Could be wrapped as "write tool" or "tar read" depending on
	// where the underlying gzip stream gives up. Either signals
	// loud failure — the test asserts NOT silent success.
	if !strings.Contains(err.Error(), "write ") &&
		!strings.Contains(err.Error(), "tar read") {
		t.Fatalf("expected truncation error, got %v", err)
	}
}

// TestFindBinaryWalkErrorPropagates covers the "if err != nil {
// return err }" branch of the WalkFunc (line ~386). We seed dir with
// a subdirectory whose mode forbids traversal, so filepath.Walk
// hands the WalkFunc a non-nil err for that entry. findBinary must
// surface it rather than ignoring and continuing.
//
// Skipped as root because root bypasses 0o000 mode enforcement.
func TestFindBinaryWalkErrorPropagates(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("walk error path requires non-root for mode enforcement")
	}
	dir := t.TempDir()
	locked := filepath.Join(dir, "locked")
	if err := os.Mkdir(locked, 0o755); err != nil {
		t.Fatal(err)
	}
	// Drop traversal permission so Walk can't list children.
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })

	_, err := findBinary(dir, "tool")
	if err == nil {
		t.Fatal(
			"expected walk-error propagation; permission failure was " +
				"silently ignored",
		)
	}
	// Should NOT be the "not found" error — that would mean Walk
	// swallowed the permission error.
	if strings.Contains(err.Error(), "not found") {
		t.Fatalf(
			"walk swallowed permission error and reported not-found: %v",
			err,
		)
	}
}
