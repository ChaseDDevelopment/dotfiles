package github

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/chaseddevelopment/dotfiles/installer/internal/platform"
)

// tagPattern matches release tags that start with an optional "v"
// followed by a digit — enough to reject placeholders like "latest",
// error pages, or empty segments returned by non-redirect responses.
var tagPattern = regexp.MustCompile(`^v?\d`)

var latestVersionClient = &http.Client{
	Timeout: 15 * time.Second,
	CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

// downloadFileFn is a package-level seam over downloadFile so tests
// can inject errors on the three call sites (downloadTarball,
// downloadBinary, future callers) without spinning up an HTTP
// roundtripper. Mirrors the convention used by latestVersionClient
// and downloadClient. Tests must save and defer-restore.
var downloadFileFn = downloadFile

// URLPattern identifies the GitHub release URL format.
type URLPattern int

const (
	// PatternTargetTriple: {repo}/releases/download/v{ver}/{name}_{triple}.tar.gz
	PatternTargetTriple URLPattern = iota
	// PatternVersionPrefixed: {repo}/releases/download/{ver}/{name}-{ver}-{triple}.tar.gz
	PatternVersionPrefixed
	// PatternCustomOSArch: {repo}/releases/download/v{ver}/{name}_{ver}_{OS}_{arch}.tar.gz
	PatternCustomOSArch
	// PatternRawBinary: {repo}/releases/latest/download/{name}_{os}_{arch}
	PatternRawBinary
)

// Runner is the interface required for executing shell commands
// during GitHub release installs. Satisfied by *executor.Runner.
type Runner interface {
	Run(ctx context.Context, name string, args ...string) error
}

// Config holds parameters for downloading from GitHub Releases.
//
// StripVPrefix controls how LatestVersion normalizes the resolved
// tag: when true, a leading "v" is stripped from the returned
// version string (so "v1.2.3" becomes "1.2.3"). BuildURL then
// re-prefixes the tag with "v" where the URL pattern requires it.
// The name was previously "StripV" which obscured what it actually
// did when set in per-tool registry entries.
type Config struct {
	Repo         string     // e.g., "eza-community/eza"
	Pattern      URLPattern // which URL format
	Binary       string     // binary name inside the archive
	StripVPrefix bool       // strip leading 'v' from version tag
	LibC         string     // "musl" or "gnu" (for TargetTriple pattern)
	PinVersion   string     // pin to specific version (bypasses LatestVersion)
	// ArchiveFormat selects the archive extension/extractor for tools
	// that don't ship tar.gz releases. Empty means "tar.gz" (the
	// historical default). Recognized values: "tar.gz", "zip".
	ArchiveFormat string
}

// archiveExt returns the URL extension (including the leading dot)
// for the configured archive format, defaulting to tar.gz.
func (c *Config) archiveExt() string {
	switch c.ArchiveFormat {
	case "zip":
		return ".zip"
	default:
		return ".tar.gz"
	}
}

// LatestVersion fetches the latest release tag from a GitHub repository
// by following the /releases/latest redirect.
func LatestVersion(repo string, stripV bool) (string, error) {
	url := fmt.Sprintf("https://github.com/%s/releases/latest", repo)
	resp, err := latestVersionClient.Head(url)
	if err != nil {
		return "", fmt.Errorf("HEAD %s: %w", url, err)
	}
	defer resp.Body.Close()

	// GitHub returns 302 for a real redirect to the tag page. A 200
	// means we're looking at an error page (rate limit, repo moved)
	// — the Location header may be empty or garbage.
	if resp.StatusCode < 300 || resp.StatusCode >= 400 {
		return "", fmt.Errorf(
			"unexpected status %d fetching %s (expected redirect)",
			resp.StatusCode, url,
		)
	}

	loc := resp.Header.Get("Location")
	if loc == "" {
		return "", fmt.Errorf("no redirect from %s", url)
	}

	// Location looks like:
	//   https://github.com/org/repo/releases/tag/v1.2.3
	parts := strings.Split(strings.TrimRight(loc, "/"), "/")
	tag := parts[len(parts)-1]

	if !tagPattern.MatchString(tag) {
		return "", fmt.Errorf(
			"unexpected tag %q from redirect %s", tag, loc,
		)
	}

	if stripV {
		tag = strings.TrimPrefix(tag, "v")
	}
	return tag, nil
}

// BuildURL constructs the download URL for a GitHub release asset.
func BuildURL(cfg *Config, p *platform.Platform, version string) (url string, isTarball bool) {
	switch cfg.Pattern {
	case PatternTargetTriple:
		triple := p.TargetTriple(cfg.LibC)
		tag := version
		if cfg.StripVPrefix {
			tag = "v" + version
		}
		return fmt.Sprintf(
			"https://github.com/%s/releases/download/%s/%s_%s%s",
			cfg.Repo, tag, cfg.Binary, triple, cfg.archiveExt(),
		), true

	case PatternVersionPrefixed:
		triple := p.TargetTriple(cfg.LibC)
		return fmt.Sprintf(
			"https://github.com/%s/releases/download/%s/%s-%s-%s%s",
			cfg.Repo, version, cfg.Binary, version, triple, cfg.archiveExt(),
		), true

	case PatternCustomOSArch:
		osName, arch := p.TitleStyle()
		tag := "v" + version
		return fmt.Sprintf(
			"https://github.com/%s/releases/download/%s/%s_%s_%s_%s%s",
			cfg.Repo, tag, cfg.Binary, version, osName, arch, cfg.archiveExt(),
		), true

	case PatternRawBinary:
		osName, arch := p.GoStyle()
		// Use the resolved version when available so PinVersion
		// actually pins. An empty version falls back to the
		// /releases/latest redirect for backwards compatibility.
		if version == "" {
			return fmt.Sprintf(
				"https://github.com/%s/releases/latest/download/%s_%s_%s",
				cfg.Repo, cfg.Binary, osName, arch,
			), false
		}
		tag := version
		if !strings.HasPrefix(tag, "v") {
			tag = "v" + tag
		}
		return fmt.Sprintf(
			"https://github.com/%s/releases/download/%s/%s_%s_%s",
			cfg.Repo, tag, cfg.Binary, osName, arch,
		), false
	}

	return "", false
}

// DownloadAndInstall downloads a release asset and installs the
// binary to /usr/local/bin. The runner is required — it owns
// logging, TTY passthrough, and dry-run handling. Callers that
// genuinely have no runner should construct a no-op one rather
// than passing nil.
func DownloadAndInstall(
	ctx context.Context,
	url, binaryName string,
	isTarball bool,
	runner Runner,
) error {
	if runner == nil {
		return errors.New("DownloadAndInstall: runner is nil")
	}
	tmpDir, err := os.MkdirTemp("", "dotsetup-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	if isTarball {
		if strings.HasSuffix(url, ".zip") {
			return downloadZip(ctx, url, binaryName, tmpDir, runner)
		}
		return downloadTarball(ctx, url, binaryName, tmpDir, runner)
	}
	return downloadBinary(ctx, url, binaryName, tmpDir, runner)
}

func downloadZip(
	ctx context.Context,
	url, binaryName, tmpDir string,
	runner Runner,
) error {
	zipPath := filepath.Join(tmpDir, "archive.zip")
	if err := downloadFileFn(ctx, url, zipPath); err != nil {
		return err
	}

	extractDir := filepath.Join(tmpDir, "extract")
	if err := os.MkdirAll(extractDir, 0o755); err != nil {
		return err
	}

	if err := extractZip(zipPath, extractDir); err != nil {
		return fmt.Errorf("zip extract: %w", err)
	}

	binPath, err := findBinary(extractDir, binaryName)
	if err != nil {
		return err
	}

	return installBinary(ctx, binPath, binaryName, runner)
}

// extractZip decompresses a zip archive into dstDir with the same
// zip-slip and symlink protections as extractTarGz. Files are capped
// at 2 GiB to bound memory pressure from malformed archives.
func extractZip(archivePath, dstDir string) error {
	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("zip open: %w", err)
	}
	defer zr.Close()

	dstAbs, err := filepath.Abs(dstDir)
	if err != nil {
		return err
	}
	dstAbs = filepath.Clean(dstAbs)

	for _, f := range zr.File {
		targetPath, err := safeJoin(dstAbs, f.Name)
		if err != nil {
			return fmt.Errorf("unsafe entry %q: %w", f.Name, err)
		}

		mode := f.Mode()
		if mode&os.ModeSymlink != 0 {
			// Matches extractTarGz: skip symlinks outright.
			continue
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(targetPath, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("zip read %s: %w", f.Name, err)
		}
		out, err := os.OpenFile(
			targetPath,
			os.O_WRONLY|os.O_CREATE|os.O_TRUNC,
			mode.Perm()&0o777,
		)
		if err != nil {
			rc.Close()
			return err
		}
		if _, err := io.CopyN(out, rc, 2<<30); err != nil && !errors.Is(err, io.EOF) {
			out.Close()
			rc.Close()
			return fmt.Errorf("write %s: %w", f.Name, err)
		}
		if err := out.Close(); err != nil {
			rc.Close()
			return err
		}
		rc.Close()
	}
	return nil
}

func downloadTarball(
	ctx context.Context,
	url, binaryName, tmpDir string,
	runner Runner,
) error {
	tarPath := filepath.Join(tmpDir, "archive.tar.gz")
	if err := downloadFileFn(ctx, url, tarPath); err != nil {
		return err
	}

	extractDir := filepath.Join(tmpDir, "extract")
	if err := os.MkdirAll(extractDir, 0o755); err != nil {
		return err
	}

	if err := extractTarGz(tarPath, extractDir); err != nil {
		return fmt.Errorf("tar extract: %w", err)
	}

	binPath, err := findBinary(extractDir, binaryName)
	if err != nil {
		return err
	}

	return installBinary(ctx, binPath, binaryName, runner)
}

// extractTarGz decompresses a gzipped tarball into dstDir, rejecting
// any entry whose cleaned path escapes dstDir (zip-slip) and any
// symlink whose target escapes dstDir. Using Go's stdlib instead of
// shelling to `tar -xzf` gives us per-entry validation the external
// binary can't provide portably.
func extractTarGz(archivePath, dstDir string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("gzip: %w", err)
	}
	defer gz.Close()

	dstAbs, err := filepath.Abs(dstDir)
	if err != nil {
		return err
	}
	dstAbs = filepath.Clean(dstAbs)

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("tar read: %w", err)
		}

		targetPath, err := safeJoin(dstAbs, hdr.Name)
		if err != nil {
			return fmt.Errorf("unsafe entry %q: %w", hdr.Name, err)
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, 0o755); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
				return err
			}
			out, err := os.OpenFile(
				targetPath,
				os.O_WRONLY|os.O_CREATE|os.O_TRUNC,
				os.FileMode(hdr.Mode)&0o777,
			)
			if err != nil {
				return err
			}
			// Cap bytes copied to prevent decompression-bomb abuse.
			// 2 GiB is well above any legitimate tool release asset.
			if _, err := io.CopyN(out, tr, 2<<30); err != nil && !errors.Is(err, io.EOF) {
				out.Close()
				return fmt.Errorf("write %s: %w", hdr.Name, err)
			}
			if err := out.Close(); err != nil {
				return err
			}
		case tar.TypeSymlink, tar.TypeLink:
			// Reject any link whose resolved target escapes dstDir.
			// We don't actually create the link since findBinary
			// refuses symlinks anyway; skipping is safer than
			// creating a link with a reduced attack surface.
			if _, err := safeJoin(dstAbs, hdr.Linkname); err != nil {
				return fmt.Errorf(
					"unsafe link %q -> %q: %w",
					hdr.Name, hdr.Linkname, err,
				)
			}
			// Intentional: skip creation. Tool archives that rely on
			// symlinks aren't in the installer's supported set; if
			// that changes, create within dstDir only.
		default:
			// Skip device nodes, FIFOs, etc — no tool needs them and
			// refusing to handle them removes attack surface.
		}
	}
}

// safeJoin resolves name against dstAbs and returns the absolute
// path only if it stays inside dstAbs. dstAbs must be absolute and
// cleaned (no trailing separator). Rejects absolute entry names
// outright — tar archives should only contain relative paths — and
// uses filepath.Rel to catch any ".." traversal after normalization.
func safeJoin(dstAbs, name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("empty entry name")
	}
	if filepath.IsAbs(name) {
		return "", fmt.Errorf("absolute path not allowed")
	}
	full := filepath.Join(dstAbs, name)
	rel, err := filepath.Rel(dstAbs, full)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("path escapes extraction root")
	}
	return full, nil
}

func downloadBinary(
	ctx context.Context,
	url, binaryName, tmpDir string,
	runner Runner,
) error {
	binPath := filepath.Join(tmpDir, binaryName)
	if err := downloadFileFn(ctx, url, binPath); err != nil {
		return err
	}
	return installBinary(ctx, binPath, binaryName, runner)
}

// downloadClient is used for fetching release assets. It has a
// generous timeout because some archives are large.
var downloadClient = &http.Client{Timeout: 5 * time.Minute}

func downloadFile(ctx context.Context, url, dest string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := downloadClient.Do(req)
	if err != nil {
		return fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: HTTP %d", url, resp.StatusCode)
	}

	f, err := os.Create(dest)
	if err != nil {
		return err
	}

	_, copyErr := io.Copy(f, resp.Body)
	if closeErr := f.Close(); copyErr == nil {
		copyErr = closeErr
	}
	return copyErr
}

// findBinary walks the extraction directory looking for a regular,
// executable file named `name`. Symlinks and non-regular files are
// rejected — even after zip-slip protection during extraction, a
// symlink pointing inside dstDir could still redirect the eventual
// `sudo install` to an attacker-controlled target on re-entry.
func findBinary(dir, name string) (string, error) {
	var found string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || info.Name() != name {
			return nil
		}
		// Lstat so we see the link itself, not its target.
		li, lerr := os.Lstat(path)
		if lerr != nil {
			return lerr
		}
		if li.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf(
				"binary %q is a symlink in archive — refusing to install",
				name,
			)
		}
		if !li.Mode().IsRegular() {
			return fmt.Errorf(
				"binary %q is not a regular file in archive", name,
			)
		}
		found = path
		return filepath.SkipAll
	})
	if err != nil {
		return "", err
	}
	if found == "" {
		return "", fmt.Errorf("binary %q not found in archive", name)
	}
	return found, nil
}

// Sha256File returns the hex-encoded SHA-256 digest of the file at
// path. Used by self-update verification and by the updater when
// logging remote scripts before they execute.
func Sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// FetchChecksums downloads a SHA256SUMS file from the given URL and
// parses it into a map of filename → hex digest. The format is the
// standard `sha256sum` output: one entry per line, "<hex>  <name>".
func FetchChecksums(ctx context.Context, url string) (map[string]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := downloadClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch checksums: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(
			"fetch checksums: HTTP %d", resp.StatusCode,
		)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	out := make(map[string]string)
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		// sha256sum prepends the name with "*" for binary mode.
		name := strings.TrimPrefix(fields[1], "*")
		out[name] = strings.ToLower(fields[0])
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("checksums file empty or unparseable")
	}
	return out, nil
}

// VerifyFile checks that the SHA-256 digest of the file at path
// matches the expected hex digest (case-insensitive). Returns an
// error describing the mismatch on failure.
func VerifyFile(path, expectedHex string) error {
	got, err := Sha256File(path)
	if err != nil {
		return fmt.Errorf("hash %s: %w", path, err)
	}
	if !strings.EqualFold(got, expectedHex) {
		return fmt.Errorf(
			"sha256 mismatch for %s: got %s, expected %s",
			filepath.Base(path), got, expectedHex,
		)
	}
	return nil
}

func installBinary(
	ctx context.Context,
	srcPath, name string,
	runner Runner,
) error {
	dest := filepath.Join("/usr/local/bin", name)
	return runner.Run(
		ctx, "sudo", "install", "-m", "755", srcPath, dest,
	)
}
