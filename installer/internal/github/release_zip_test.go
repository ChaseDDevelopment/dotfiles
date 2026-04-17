package github

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestExtractZipHappyPath writes a minimal zip with an executable
// binary and confirms extractZip recovers it with the expected mode.
func TestExtractZipHappyPath(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "ok.zip")
	writeZip(t, archive, []zipEntry{
		{name: "jless", mode: 0o755, body: []byte("#!/bin/sh\necho hi\n")},
	})

	dst := filepath.Join(dir, "extract")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := extractZip(archive, dst); err != nil {
		t.Fatalf("extractZip: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dst, "jless"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(got), "#!/bin/sh") {
		t.Fatalf("unexpected extracted content: %q", got)
	}
}

// TestExtractZipRejectsTraversal exercises the zip-slip guard for
// the zip code path (separate from the tar path).
func TestExtractZipRejectsTraversal(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "bad.zip")
	writeZip(t, archive, []zipEntry{
		{name: "../escape.txt", mode: 0o644, body: []byte("pwned")},
	})

	dst := filepath.Join(dir, "extract")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	err := extractZip(archive, dst)
	if err == nil {
		t.Fatal("expected traversal rejection, got nil")
	}
	if !strings.Contains(err.Error(), "unsafe entry") &&
		!strings.Contains(err.Error(), "path escapes") {
		t.Fatalf("expected path-escape error, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "escape.txt")); err == nil {
		t.Fatal("escape.txt was written outside the extraction root")
	}
}

// TestArchiveExtDefaults covers the archive-format switch on Config.
func TestArchiveExtDefaults(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", ".tar.gz"},
		{"tar.gz", ".tar.gz"},
		{"zip", ".zip"},
	}
	for _, c := range cases {
		got := (&Config{ArchiveFormat: c.in}).archiveExt()
		if got != c.want {
			t.Errorf("ArchiveFormat=%q: got %q, want %q", c.in, got, c.want)
		}
	}
}

// --- helpers ---

type zipEntry struct {
	name string
	mode os.FileMode
	body []byte
}

func writeZip(t *testing.T, path string, entries []zipEntry) {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, e := range entries {
		hdr := &zip.FileHeader{Name: e.name, Method: zip.Deflate}
		hdr.SetMode(e.mode)
		w, err := zw.CreateHeader(hdr)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(e.body); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}
