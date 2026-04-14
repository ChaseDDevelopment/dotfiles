package config

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// nvimDriftedClones returns plugin clone directories whose checked-
// out HEAD commit doesn't match the rev recorded in
// nvim-pack-lock.json. A drifted clone must be wiped so vim.pack
// can re-clone with the currently declared `version` — `vim.pack.
// update` won't switch a detached HEAD across branches on its own.
//
// Returns (nil, nil) gracefully when the lockfile is missing, which
// is the expected state on a fresh machine or before the user has
// run `vim.pack.update` for the first time. Entries without a
// recorded rev are skipped — the declarative `vim.pack.add` step
// handles them.
//
// Entries whose dir doesn't exist yet are also skipped. The add
// step will clone them fresh.
func nvimDriftedClones(lockPath, home string) ([]string, error) {
	data, err := os.ReadFile(lockPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var lock struct {
		Plugins map[string]struct {
			Rev string `json:"rev"`
		} `json:"plugins"`
	}
	if err := json.Unmarshal(data, &lock); err != nil {
		return nil, err
	}

	optDir := filepath.Join(home, ".local", "share", "nvim",
		"site", "pack", "core", "opt")

	var drifted []string
	for name, spec := range lock.Plugins {
		if spec.Rev == "" {
			continue
		}
		dir := filepath.Join(optDir, name)
		if _, err := os.Stat(dir); err != nil {
			continue // not cloned yet — add step handles it
		}
		out, err := exec.Command(
			"git", "-C", dir, "rev-parse", "HEAD",
		).Output()
		if err != nil {
			// Corrupted clone (no .git, shallow broken, etc) —
			// safest to nuke.
			drifted = append(drifted, dir)
			continue
		}
		if strings.TrimSpace(string(out)) != spec.Rev {
			drifted = append(drifted, dir)
		}
	}
	return drifted, nil
}
