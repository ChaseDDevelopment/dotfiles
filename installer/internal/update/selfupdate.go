package update

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/chaseddevelopment/dotfiles/installer/internal/executor"
	"github.com/chaseddevelopment/dotfiles/installer/internal/github"
)

const selfRepo = "ChaseDDevelopment/dotfiles"

// CheckSelfUpdate checks if a newer version of dotsetup is
// available. Returns the latest version tag or "" if already
// up to date. currentVersion should be the build-time Version
// string (e.g. "v1.2.3" or "dev").
func CheckSelfUpdate(currentVersion string) (string, error) {
	if currentVersion == "dev" || currentVersion == "" {
		return "", nil // dev builds skip update checks
	}

	latest, err := github.LatestVersion(selfRepo, false)
	if err != nil {
		return "", fmt.Errorf("check update: %w", err)
	}

	// Normalize: strip leading 'v' for comparison.
	cur := strings.TrimPrefix(currentVersion, "v")
	lat := strings.TrimPrefix(latest, "v")
	if cur == lat {
		return "", nil
	}
	return latest, nil
}

// SelfUpdate downloads and replaces the current binary with the
// latest release from GitHub.
func SelfUpdate(
	ctx context.Context,
	runner *executor.Runner,
	currentVersion string,
) error {
	latest, err := CheckSelfUpdate(currentVersion)
	if err != nil {
		return err
	}
	if latest == "" {
		runner.Log.Write("dotsetup is already up to date")
		return nil
	}

	runner.Log.Write(fmt.Sprintf(
		"Updating dotsetup: %s → %s", currentVersion, latest,
	))

	osName := runtime.GOOS
	arch := runtime.GOARCH

	url := fmt.Sprintf(
		"https://github.com/%s/releases/download/%s/dotsetup-%s-%s",
		selfRepo, latest, osName, arch,
	)

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("find current binary: %w", err)
	}

	tmpFile := exe + ".new"
	if err := runner.Run(
		ctx, "curl", "-fsSL", url, "-o", tmpFile,
	); err != nil {
		os.Remove(tmpFile)
		return fmt.Errorf("download update: %w", err)
	}

	if err := os.Chmod(tmpFile, 0o755); err != nil {
		os.Remove(tmpFile)
		return err
	}

	// Backup current binary for rollback.
	backupFile := exe + ".old"
	if err := copyFile(exe, backupFile); err != nil {
		runner.Log.Write(fmt.Sprintf(
			"WARNING: backup current binary: %v", err,
		))
		// Continue — lack of backup shouldn't block the update.
	}

	// Atomic replace.
	if err := os.Rename(tmpFile, exe); err != nil {
		// Try sudo mv as fallback for /usr/local/bin.
		if err2 := runner.Run(
			ctx, "sudo", "mv", tmpFile, exe,
		); err2 != nil {
			os.Remove(tmpFile)
			// Attempt rollback from backup.
			if _, statErr := os.Stat(backupFile); statErr == nil {
				os.Rename(backupFile, exe)
			}
			return fmt.Errorf("replace binary: %w", err)
		}
	}

	// Clean up backup on success.
	os.Remove(backupFile)

	runner.Log.Write(fmt.Sprintf(
		"Updated dotsetup to %s", latest,
	))
	return nil
}

// copyFile copies src to dst, preserving permissions.
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, info.Mode())
}
