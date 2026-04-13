package github

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chaseddevelopment/dotfiles/installer/internal/platform"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type runnerRecorder struct {
	args []string
}

func (r *runnerRecorder) Run(_ context.Context, name string, args ...string) error {
	r.args = append([]string{name}, args...)
	return nil
}

func TestLatestVersion(t *testing.T) {
	orig := latestVersionClient
	latestVersionClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusFound,
				Header:     http.Header{"Location": []string{"https://github.com/org/repo/releases/tag/v1.2.3"}},
				Body:       io.NopCloser(strings.NewReader("")),
			}, nil
		}),
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
	}
	defer func() { latestVersionClient = orig }()

	got, err := LatestVersion("org/repo", true)
	if err != nil {
		t.Fatalf("LatestVersion: %v", err)
	}
	if got != "1.2.3" {
		t.Fatalf("LatestVersion = %q, want 1.2.3", got)
	}
}

func TestLatestVersionRejectsBadResponses(t *testing.T) {
	orig := latestVersionClient
	latestVersionClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{},
				Body:       io.NopCloser(strings.NewReader("")),
			}, nil
		}),
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
	}
	defer func() { latestVersionClient = orig }()

	if _, err := LatestVersion("org/repo", false); err == nil {
		t.Fatal("expected error for non-redirect response")
	}
}

func TestSafeJoin(t *testing.T) {
	dst := t.TempDir()
	dstAbs, err := filepath.Abs(dst)
	if err != nil {
		t.Fatal(err)
	}

	got, err := safeJoin(dstAbs, "bin/tool")
	if err != nil {
		t.Fatalf("safeJoin happy path: %v", err)
	}
	if !strings.HasPrefix(got, dstAbs) {
		t.Fatalf("safeJoin returned path outside root: %s", got)
	}
	if _, err := safeJoin(dstAbs, "../escape"); err == nil {
		t.Fatal("expected traversal error")
	}
}

func TestDownloadFileAndChecksums(t *testing.T) {
	orig := downloadClient
	downloadClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := "hello"
			if strings.Contains(req.URL.String(), "SHA256SUMS") {
				body = "deadbeef  *tool\ncafebabe  other\n"
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{},
			}, nil
		}),
	}
	defer func() { downloadClient = orig }()

	path := filepath.Join(t.TempDir(), "downloaded")
	if err := downloadFile(context.Background(), "https://example.invalid/tool", path); err != nil {
		t.Fatalf("downloadFile: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello" {
		t.Fatalf("downloaded data = %q", data)
	}

	sums, err := FetchChecksums(context.Background(), "https://example.invalid/SHA256SUMS")
	if err != nil {
		t.Fatalf("FetchChecksums: %v", err)
	}
	if sums["tool"] != "deadbeef" || sums["other"] != "cafebabe" {
		t.Fatalf("unexpected checksums map: %#v", sums)
	}
}

func TestDownloadAndInstallBinaryAndTarball(t *testing.T) {
	orig := downloadClient
	var tarBuf bytes.Buffer
	writeTarGz(t, filepath.Join(t.TempDir(), "ignored.tar.gz"), nil)
	gzbuf := new(bytes.Buffer)
	{
		gzPath := filepath.Join(t.TempDir(), "tool.tar.gz")
		writeTarGz(t, gzPath, []tarEntry{{name: "bin/tool", mode: 0o755, body: []byte("bin")}})
		data, err := os.ReadFile(gzPath)
		if err != nil {
			t.Fatal(err)
		}
		gzbuf.Write(data)
	}
	_ = tarBuf
	downloadClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := "raw-binary"
			if strings.Contains(req.URL.String(), ".tar.gz") {
				body = gzbuf.String()
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{},
			}, nil
		}),
	}
	defer func() { downloadClient = orig }()

	runner := &runnerRecorder{}
	if err := DownloadAndInstall(context.Background(), "https://example.invalid/tool_darwin_arm64", "tool", false, runner); err != nil {
		t.Fatalf("DownloadAndInstall raw: %v", err)
	}
	if got := strings.Join(runner.args, " "); !strings.Contains(got, "sudo install -m 755") || !strings.Contains(got, "/usr/local/bin/tool") {
		t.Fatalf("unexpected install argv: %v", runner.args)
	}

	runner = &runnerRecorder{}
	if err := DownloadAndInstall(context.Background(), "https://example.invalid/tool.tar.gz", "tool", true, runner); err != nil {
		t.Fatalf("DownloadAndInstall tarball: %v", err)
	}
	if got := strings.Join(runner.args, " "); !strings.Contains(got, "/usr/local/bin/tool") {
		t.Fatalf("unexpected tarball install argv: %v", runner.args)
	}
}

func TestBuildURLAdditionalPatterns(t *testing.T) {
	p := &platform.Platform{OS: platform.Linux, Arch: platform.AMD64}
	url, tarball := BuildURL(&Config{
		Repo: "owner/repo", Pattern: PatternVersionPrefixed, Binary: "tool", LibC: "gnu",
	}, p, "1.2.3")
	if !tarball || !strings.Contains(url, "/releases/download/1.2.3/tool-1.2.3-x86_64-unknown-linux-gnu.tar.gz") {
		t.Fatalf("unexpected version-prefixed URL: %s", url)
	}

	url, tarball = BuildURL(&Config{
		Repo: "owner/repo", Pattern: PatternTargetTriple, Binary: "tool", StripVPrefix: true, LibC: "gnu",
	}, p, "1.2.3")
	if !tarball || !strings.Contains(url, "/releases/download/v1.2.3/tool_x86_64-unknown-linux-gnu.tar.gz") {
		t.Fatalf("unexpected target-triple URL: %s", url)
	}

	url, tarball = BuildURL(&Config{
		Repo: "owner/repo", Pattern: PatternRawBinary, Binary: "tool",
	}, p, "1.2.3")
	if tarball || !strings.Contains(url, "/releases/download/v1.2.3/tool_linux_amd64") {
		t.Fatalf("unexpected raw-binary URL: %s", url)
	}

	url, tarball = BuildURL(&Config{
		Repo: "owner/repo", Pattern: PatternCustomOSArch, Binary: "tool",
	}, &platform.Platform{OS: platform.MacOS, Arch: platform.ARM64}, "1.2.3")
	if !tarball || !strings.Contains(url, "/releases/download/v1.2.3/tool_1.2.3_darwin_arm64.tar.gz") {
		t.Fatalf("unexpected custom-os-arch URL: %s", url)
	}
}
