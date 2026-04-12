package update

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/chaseddevelopment/dotfiles/installer/internal/executor"
	"github.com/chaseddevelopment/dotfiles/installer/internal/github"
)

// cosignIdentityRegexp and cosignIssuer bind accepted signatures to
// this repo's release workflow. Any signature produced by a
// different repo or workflow (e.g. a fork attempting to serve a
// fake release) fails verification.
const (
	cosignIdentityRegexp = "^https://github.com/ChaseDDevelopment/dotfiles/.github/workflows/release-installer.yml@refs/tags/v.+$"
	cosignIssuer         = "https://token.actions.githubusercontent.com"
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

// SelfUpdate downloads, integrity-verifies, and replaces the
// current binary with the latest release from GitHub.
//
// Integrity protocol: the release publishes a SHA256SUMS file
// alongside each `dotsetup-<os>-<arch>` asset. We download both,
// look up the expected digest for the current platform, hash the
// downloaded binary, and refuse to install on mismatch. This is the
// minimum bar to prevent a compromised release asset from silently
// replacing the running executable; signature verification
// (minisign/cosign) is tracked as a follow-up.
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

	assetName := fmt.Sprintf("dotsetup-%s-%s", osName, arch)
	binURL := fmt.Sprintf(
		"https://github.com/%s/releases/download/%s/%s",
		selfRepo, latest, assetName,
	)
	sumsURL := fmt.Sprintf(
		"https://github.com/%s/releases/download/%s/SHA256SUMS",
		selfRepo, latest,
	)

	// Fetch and parse the manifest first. If signing fails, we fail
	// here before touching the filesystem with a binary we can't
	// verify.
	sums, err := github.FetchChecksums(ctx, sumsURL)
	if err != nil {
		return fmt.Errorf(
			"fetch release checksums (SHA256SUMS is required for "+
				"self-update): %w", err,
		)
	}
	expected, ok := sums[assetName]
	if !ok {
		return fmt.Errorf(
			"SHA256SUMS missing entry for %s (release %s may be "+
				"incomplete)", assetName, latest,
		)
	}

	// Optionally verify the Sigstore signature over SHA256SUMS
	// before trusting any of the digests it contains. This closes
	// the remaining threat where an attacker with release-write
	// access to this repo could swap both the binary AND the
	// matching SHA256SUMS entry — cosign's keyless signatures are
	// rooted in GitHub OIDC for a specific workflow+tag and can't
	// be silently regenerated without that OIDC context.
	if err := verifyCosignSignature(ctx, runner, selfRepo, latest); err != nil {
		return fmt.Errorf("update signature check failed: %w", err)
	}

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("find current binary: %w", err)
	}

	tmpFile := exe + ".new"
	if err := runner.Run(
		ctx, "curl", "-fsSL", binURL, "-o", tmpFile,
	); err != nil {
		cleanupTmp(runner, tmpFile)
		return fmt.Errorf("download update: %w", err)
	}

	// Integrity gate. Verify BEFORE chmod/rename so a tampered
	// binary never becomes executable.
	if err := github.VerifyFile(tmpFile, expected); err != nil {
		cleanupTmp(runner, tmpFile)
		return fmt.Errorf("update integrity check failed: %w", err)
	}
	runner.Log.Write(fmt.Sprintf(
		"Verified %s sha256=%s", assetName, expected,
	))

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

// verifyCosignSignature downloads SHA256SUMS, SHA256SUMS.sig, and
// SHA256SUMS.pem for the given tag, then shells out to the `cosign`
// CLI to verify the signature against the expected GitHub OIDC
// identity. Policy:
//
//   - If cosign is on PATH, a verification failure is fatal.
//   - If cosign is NOT on PATH and DOTSETUP_REQUIRE_COSIGN is set,
//     the update is refused.
//   - If cosign is NOT on PATH and the env var is unset, log a
//     visible warning and fall back to SHA256-only verification.
//     This keeps the installer usable on systems where cosign isn't
//     available while still letting security-conscious users
//     enforce strict mode.
func verifyCosignSignature(
	ctx context.Context,
	runner *executor.Runner,
	repo, tag string,
) error {
	cosign, err := exec.LookPath("cosign")
	if err != nil {
		if os.Getenv("DOTSETUP_REQUIRE_COSIGN") != "" {
			return fmt.Errorf(
				"cosign not installed and DOTSETUP_REQUIRE_COSIGN is set",
			)
		}
		runner.Log.Write(
			"NOTE: cosign not installed; self-update proceeding " +
				"with SHA256-only verification. Install cosign " +
				"(brew install cosign) for full signature checks.",
		)
		return nil
	}

	// Download all three artifacts into a temp dir. If any of them
	// are missing (e.g. an older release predates signing), treat
	// that as a hard failure so silently-unsigned artifacts can't
	// slip through.
	tmpDir, err := os.MkdirTemp("", "dotsetup-cosign-*")
	if err != nil {
		return fmt.Errorf("create cosign temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	files := map[string]string{
		"SHA256SUMS":     fmt.Sprintf("https://github.com/%s/releases/download/%s/SHA256SUMS", repo, tag),
		"SHA256SUMS.sig": fmt.Sprintf("https://github.com/%s/releases/download/%s/SHA256SUMS.sig", repo, tag),
		"SHA256SUMS.pem": fmt.Sprintf("https://github.com/%s/releases/download/%s/SHA256SUMS.pem", repo, tag),
	}
	paths := make(map[string]string, len(files))
	for name, url := range files {
		dst := filepath.Join(tmpDir, name)
		if err := runner.Run(ctx, "curl", "-fsSL", url, "-o", dst); err != nil {
			return fmt.Errorf("download %s: %w", name, err)
		}
		paths[name] = dst
	}

	out, err := exec.CommandContext(
		ctx, cosign, "verify-blob",
		"--certificate", paths["SHA256SUMS.pem"],
		"--signature", paths["SHA256SUMS.sig"],
		"--certificate-identity-regexp", cosignIdentityRegexp,
		"--certificate-oidc-issuer", cosignIssuer,
		paths["SHA256SUMS"],
	).CombinedOutput()
	if err != nil {
		return fmt.Errorf(
			"cosign verify-blob: %w\noutput: %s",
			err, strings.TrimSpace(string(out)),
		)
	}
	runner.Log.Write(fmt.Sprintf(
		"Verified SHA256SUMS signature for %s via cosign", tag,
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

