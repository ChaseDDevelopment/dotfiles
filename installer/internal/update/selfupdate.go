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
		cleanupTmp(runner, tmpFile)
		return fmt.Errorf("download update: %w", err)
	}

	if err := os.Chmod(tmpFile, 0o755); err != nil {
		cleanupTmp(runner, tmpFile)
		return fmt.Errorf("chmod update: %w", err)
	}

	// Backup current binary for rollback. Refuse the update if
	// this fails — without a backup a bad replace leaves the user
	// with a broken/missing binary and no way back.
	backupFile := exe + ".old"
	if err := copyFile(exe, backupFile); err != nil {
		cleanupTmp(runner, tmpFile)
		return fmt.Errorf(
			"backup current binary before update: %w", err,
		)
	}

	// Atomic replace.
	if err := os.Rename(tmpFile, exe); err != nil {
		// Try sudo mv as fallback for /usr/local/bin.
		if err2 := runner.Run(
			ctx, "sudo", "mv", tmpFile, exe,
		); err2 != nil {
			cleanupTmp(runner, tmpFile)
			// Attempt rollback from backup. If rollback also
			// fails, surface both errors so the user knows the
			// binary is in a broken state.
			if _, statErr := os.Stat(backupFile); statErr == nil {
				if rbErr := os.Rename(backupFile, exe); rbErr != nil {
					return fmt.Errorf(
						"replace binary: %w; rollback also failed: %v "+
							"(backup remains at %s)",
						err, rbErr, backupFile,
					)
				}
			}
			return fmt.Errorf("replace binary: %w", err)
		}
	}

	// Clean up backup on success. Don't fail the update over a
	// leftover .old file, but log it so it isn't invisible.
	if err := os.Remove(backupFile); err != nil && !os.IsNotExist(err) {
		runner.Log.Write(fmt.Sprintf(
			"NOTE: remove rollback backup %s: %v", backupFile, err,
		))
	}

	runner.Log.Write(fmt.Sprintf(
		"Updated dotsetup to %s", latest,
	))
	return nil
}

// cleanupTmp removes the staging file from a failed update and logs
// any removal error so it isn't hidden.
func cleanupTmp(runner *executor.Runner, tmpFile string) {
	if err := os.Remove(tmpFile); err != nil && !os.IsNotExist(err) {
		runner.Log.Write(fmt.Sprintf(
			"NOTE: remove update staging file %s: %v", tmpFile, err,
		))
	}
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
