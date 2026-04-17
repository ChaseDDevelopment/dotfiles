package config

import (
	"os"
	"path/filepath"
	"regexp"
)

// pluginDeclRe matches `set -g @plugin '<org>/<repo>'` and
// `set -g @plugin "<org>/<repo>"` lines in tmux.conf. Leading
// whitespace + inline comments are tolerated by the anchor and the
// trailing capture group. Only the <repo> portion (the basename)
// matters for matching against on-disk plugin dirs — TPM clones to
// `~/.tmux/plugins/<repo>` regardless of the org.
var pluginDeclRe = regexp.MustCompile(
	`(?m)^\s*set\s+-g\s+@plugin\s+['"]([^/'"]+)/([^'"]+)['"]`,
)

// staleTmuxPlugins returns plugin directories under pluginsDir that
// aren't referenced by any `set -g @plugin` line in tmux.conf.
// TPM itself (`tpm/`) is always preserved because it's the manager
// and isn't declared via `@plugin` — removing it would nuke the
// whole plugin system.
//
// Returns (nil, nil) when tmux.conf is missing or pluginsDir doesn't
// exist — both are legitimate states on a fresh machine, not an
// error worth surfacing.
func staleTmuxPlugins(tmuxConf, pluginsDir string) ([]string, error) {
	confData, err := os.ReadFile(tmuxConf)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	entries, err := os.ReadDir(pluginsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	declared := map[string]bool{"tpm": true}
	for _, m := range pluginDeclRe.FindAllStringSubmatch(string(confData), -1) {
		// m[2] is the repo basename, e.g. "tmux-menus" from
		// 'jaclu/tmux-menus'. That matches TPM's on-disk layout.
		declared[m[2]] = true
	}

	var stale []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if declared[e.Name()] {
			continue
		}
		stale = append(stale, filepath.Join(pluginsDir, e.Name()))
	}
	return stale, nil
}

// missingTmuxPlugins returns the basenames of plugins declared via
// `set -g @plugin` in tmux.conf that don't have a corresponding
// directory under pluginsDir. TPM is excluded — it's the manager,
// installed separately by the tools-install phase, not via TPM
// itself. Returns (nil, nil) when tmux.conf is missing (fresh box
// before symlinks land); returns the full declared list when
// pluginsDir doesn't exist (everything is "missing" in that case).
func missingTmuxPlugins(tmuxConf, pluginsDir string) ([]string, error) {
	confData, err := os.ReadFile(tmuxConf)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var declared []string
	for _, m := range pluginDeclRe.FindAllStringSubmatch(string(confData), -1) {
		// m[2] is the repo basename — TPM clones to
		// `~/.tmux/plugins/<repo>` regardless of the org.
		if m[2] == "tpm" {
			continue
		}
		declared = append(declared, m[2])
	}

	var missing []string
	for _, name := range declared {
		_, err := os.Stat(filepath.Join(pluginsDir, name))
		if err == nil {
			continue
		}
		if !os.IsNotExist(err) {
			return nil, err
		}
		missing = append(missing, name)
	}
	return missing, nil
}
