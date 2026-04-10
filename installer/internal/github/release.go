package github

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chaseddevelopment/dotfiles/installer/internal/platform"
)

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
type Config struct {
	Repo       string     // e.g., "eza-community/eza"
	Pattern    URLPattern // which URL format
	Binary     string     // binary name inside the archive
	StripV     bool       // strip leading 'v' from version tag
	LibC       string     // "musl" or "gnu" (for TargetTriple pattern)
	PinVersion string     // pin to specific version (bypasses LatestVersion)
}

// LatestVersion fetches the latest release tag from a GitHub repository
// by following the /releases/latest redirect.
func LatestVersion(repo string, stripV bool) (string, error) {
	url := fmt.Sprintf("https://github.com/%s/releases/latest", repo)

	client := &http.Client{
		Timeout: 15 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Head(url)
	if err != nil {
		return "", fmt.Errorf("HEAD %s: %w", url, err)
	}
	defer resp.Body.Close()

	loc := resp.Header.Get("Location")
	if loc == "" {
		return "", fmt.Errorf("no redirect from %s", url)
	}

	// Location looks like:
	//   https://github.com/org/repo/releases/tag/v1.2.3
	parts := strings.Split(loc, "/")
	tag := parts[len(parts)-1]

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
		if cfg.StripV {
			tag = "v" + version
		}
		return fmt.Sprintf(
			"https://github.com/%s/releases/download/%s/%s_%s.tar.gz",
			cfg.Repo, tag, cfg.Binary, triple,
		), true

	case PatternVersionPrefixed:
		triple := p.TargetTriple(cfg.LibC)
		return fmt.Sprintf(
			"https://github.com/%s/releases/download/%s/%s-%s-%s.tar.gz",
			cfg.Repo, version, cfg.Binary, version, triple,
		), true

	case PatternCustomOSArch:
		osName, arch := p.TitleStyle()
		tag := "v" + version
		return fmt.Sprintf(
			"https://github.com/%s/releases/download/%s/%s_%s_%s_%s.tar.gz",
			cfg.Repo, tag, cfg.Binary, version, osName, arch,
		), true

	case PatternRawBinary:
		osName, arch := p.LowerStyle()
		return fmt.Sprintf(
			"https://github.com/%s/releases/latest/download/%s_%s_%s",
			cfg.Repo, cfg.Binary, osName, arch,
		), false
	}

	return "", false
}

// DownloadAndInstall downloads a release asset and installs the
// binary to /usr/local/bin. When runner is non-nil, shell commands
// (tar, sudo install) are routed through it for consistent logging,
// TTY passthrough, and dry-run support.
func DownloadAndInstall(
	ctx context.Context,
	url, binaryName string,
	isTarball bool,
	runner Runner,
) error {
	tmpDir, err := os.MkdirTemp("", "dotsetup-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	if isTarball {
		return downloadTarball(ctx, url, binaryName, tmpDir, runner)
	}
	return downloadBinary(ctx, url, binaryName, tmpDir, runner)
}

func downloadTarball(
	ctx context.Context,
	url, binaryName, tmpDir string,
	runner Runner,
) error {
	tarPath := filepath.Join(tmpDir, "archive.tar.gz")
	if err := downloadFile(ctx, url, tarPath); err != nil {
		return err
	}

	extractDir := filepath.Join(tmpDir, "extract")
	if err := os.MkdirAll(extractDir, 0o755); err != nil {
		return err
	}

	if err := runner.Run(
		ctx, "tar", "-xzf", tarPath, "-C", extractDir,
	); err != nil {
		return fmt.Errorf("tar extract: %w", err)
	}

	binPath, err := findBinary(extractDir, binaryName)
	if err != nil {
		return err
	}

	return installBinary(ctx, binPath, binaryName, runner)
}

func downloadBinary(
	ctx context.Context,
	url, binaryName, tmpDir string,
	runner Runner,
) error {
	binPath := filepath.Join(tmpDir, binaryName)
	if err := downloadFile(ctx, url, binPath); err != nil {
		return err
	}
	return installBinary(ctx, binPath, binaryName, runner)
}

func downloadFile(ctx context.Context, url, dest string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
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

func findBinary(dir, name string) (string, error) {
	var found string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && info.Name() == name {
			found = path
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if found == "" {
		return "", fmt.Errorf("binary %q not found in archive", name)
	}
	return found, nil
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
