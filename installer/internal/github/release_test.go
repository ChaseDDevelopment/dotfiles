package github

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestExtractTarGzRejectsPathTraversal exercises the zip-slip
// guard: an archive entry whose cleaned path escapes the
// extraction root must fail the whole extraction rather than land
// a file outside the sandbox.
func TestExtractTarGzRejectsPathTraversal(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "bad.tar.gz")
	writeTarGz(t, archive, []tarEntry{
		{name: "../escape.txt", mode: 0o644, body: []byte("pwned")},
	})

	dst := filepath.Join(dir, "extract")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	err := extractTarGz(archive, dst)
	if err == nil {
		t.Fatal("expected error extracting traversal entry, got nil")
	}
	if !strings.Contains(err.Error(), "path escapes extraction root") &&
		!strings.Contains(err.Error(), "unsafe entry") {
		t.Fatalf("expected path-escape error, got %v", err)
	}
	// Confirm nothing was written outside dst.
	if _, err := os.Stat(filepath.Join(dir, "escape.txt")); err == nil {
		t.Fatal("escape.txt was written outside the extraction root")
	}
}

// TestExtractTarGzRejectsSymlinkEscape ensures a symlink entry
// whose target escapes the extraction root is refused.
func TestExtractTarGzRejectsSymlinkEscape(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "bad.tar.gz")
	writeTarGz(t, archive, []tarEntry{
		{name: "link", linkname: "../../etc/passwd", typeflag: tar.TypeSymlink},
	})

	dst := filepath.Join(dir, "extract")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	err := extractTarGz(archive, dst)
	if err == nil {
		t.Fatal("expected error on escaping symlink, got nil")
	}
	if !strings.Contains(err.Error(), "unsafe link") {
		t.Fatalf("expected unsafe-link error, got %v", err)
	}
}

// TestExtractTarGzHappyPath ensures legitimate archives still
// extract correctly after the safe-path rewrite.
func TestExtractTarGzHappyPath(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "ok.tar.gz")
	writeTarGz(t, archive, []tarEntry{
		{name: "bin/", typeflag: tar.TypeDir},
		{name: "bin/tool", mode: 0o755, body: []byte("#!/bin/sh\necho hi\n")},
	})
	dst := filepath.Join(dir, "extract")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := extractTarGz(archive, dst); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dst, "bin", "tool"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(got), "#!/bin/sh") {
		t.Fatalf("unexpected extracted content: %q", got)
	}
}

// TestFindBinaryRejectsSymlink ensures the post-extraction walk
// won't hand a symlink to `sudo install` — even if extraction
// somehow produced one that stayed inside the sandbox.
func TestFindBinaryRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	realBin := filepath.Join(dir, "real")
	if err := os.WriteFile(realBin, []byte("binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	linkPath := filepath.Join(dir, "tool")
	if err := os.Symlink(realBin, linkPath); err != nil {
		t.Skipf("symlink unsupported on this fs: %v", err)
	}
	_, err := findBinary(dir, "tool")
	if err == nil {
		t.Fatal("expected findBinary to reject symlink, got nil")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("expected symlink-refusal error, got %v", err)
	}
}

// TestFetchChecksumsParse covers the manifest format: hex digest +
// filename, with the "*" binary-mode prefix stripped.
func TestSha256AndVerify(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.bin")
	if err := os.WriteFile(path, []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}
	sum, err := Sha256File(path)
	if err != nil {
		t.Fatal(err)
	}
	// Known sha256 of "hello world".
	const want = "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"
	if sum != want {
		t.Fatalf("sum=%s want=%s", sum, want)
	}
	if err := VerifyFile(path, want); err != nil {
		t.Fatal(err)
	}
	if err := VerifyFile(path, "deadbeef"); err == nil {
		t.Fatal("expected mismatch error")
	}
}

// --- helpers ---

type tarEntry struct {
	name     string
	mode     int64
	body     []byte
	typeflag byte
	linkname string
}

func writeTarGz(t *testing.T, path string, entries []tarEntry) {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, e := range entries {
		hdr := &tar.Header{
			Name:     e.name,
			Mode:     e.mode,
			Size:     int64(len(e.body)),
			Typeflag: e.typeflag,
			Linkname: e.linkname,
		}
		if hdr.Typeflag == 0 {
			hdr.Typeflag = tar.TypeReg
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if len(e.body) > 0 {
			if _, err := tw.Write(e.body); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}
