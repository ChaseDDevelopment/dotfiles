package config

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

const sampleTmuxConf = `
# comment
set -g @plugin 'tmux-plugins/tpm'
set -g @plugin 'tmux-plugins/tmux-sensible'
set -g @plugin "catppuccin/tmux"
set -g @plugin 'christoomey/vim-tmux-navigator'
`

// TestStaleTmuxPluginsRemovesUnlisted covers the core regression:
// a plugin directory whose `@plugin` line was deleted from tmux.conf
// must be flagged for removal so its in-memory bindings stop haunting
// the next tmux server start (e.g. tmux-menus clobbering `|`).
func TestStaleTmuxPluginsRemovesUnlisted(t *testing.T) {
	tmp := t.TempDir()
	conf := filepath.Join(tmp, "tmux.conf")
	if err := os.WriteFile(conf, []byte(sampleTmuxConf), 0o644); err != nil {
		t.Fatal(err)
	}
	plugins := filepath.Join(tmp, "plugins")
	for _, name := range []string{
		"tpm",                 // manager — always kept
		"tmux-sensible",       // declared — kept
		"tmux",                // declared via catppuccin/tmux — kept
		"vim-tmux-navigator",  // declared — kept
		"tmux-menus",          // REMOVED from config — should be stale
		"tmux-resurrect",      // also not declared — stale
	} {
		if err := os.MkdirAll(filepath.Join(plugins, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	stale, err := staleTmuxPlugins(conf, plugins)
	if err != nil {
		t.Fatal(err)
	}
	got := make([]string, len(stale))
	for i, p := range stale {
		got[i] = filepath.Base(p)
	}
	sort.Strings(got)
	want := []string{"tmux-menus", "tmux-resurrect"}
	if len(got) != len(want) {
		t.Fatalf("stale = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("stale = %v, want %v", got, want)
		}
	}
}

// TestStaleTmuxPluginsPreservesTpm — TPM is never declared via
// `@plugin` (it's the manager, not a managed plugin) but removing
// it would kneecap the whole setup. It must always stay.
func TestStaleTmuxPluginsPreservesTpm(t *testing.T) {
	tmp := t.TempDir()
	conf := filepath.Join(tmp, "tmux.conf")
	// Config with NO @plugin lines at all — everything on disk
	// except tpm should be flagged.
	if err := os.WriteFile(conf, []byte("# empty config\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	plugins := filepath.Join(tmp, "plugins")
	for _, name := range []string{"tpm", "orphan-plugin"} {
		if err := os.MkdirAll(filepath.Join(plugins, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	stale, err := staleTmuxPlugins(conf, plugins)
	if err != nil {
		t.Fatal(err)
	}
	if len(stale) != 1 || filepath.Base(stale[0]) != "orphan-plugin" {
		t.Fatalf("stale = %v, want only [orphan-plugin]", stale)
	}
}

// TestStaleTmuxPluginsMissingPaths — fresh-machine case: tmux.conf
// hasn't been symlinked yet, or the plugins dir doesn't exist yet.
// Neither is an error, prune is just a no-op.
func TestStaleTmuxPluginsMissingPaths(t *testing.T) {
	tmp := t.TempDir()
	stale, err := staleTmuxPlugins(
		filepath.Join(tmp, "no-conf"),
		filepath.Join(tmp, "no-dir"),
	)
	if err != nil {
		t.Fatalf("expected nil err, got %v", err)
	}
	if len(stale) != 0 {
		t.Fatalf("expected no stale entries, got %v", stale)
	}
}

// TestMissingTmuxPluginsDetectsUninstalled covers the fresh-install
// case: tmux.conf declares plugins, but the corresponding dirs don't
// exist on disk yet. The MaintainTmuxPlugins healer uses this to
// decide whether to invoke install_plugins.sh.
func TestMissingTmuxPluginsDetectsUninstalled(t *testing.T) {
	tmp := t.TempDir()
	conf := filepath.Join(tmp, "tmux.conf")
	if err := os.WriteFile(conf, []byte(sampleTmuxConf), 0o644); err != nil {
		t.Fatal(err)
	}
	plugins := filepath.Join(tmp, "plugins")
	// Only tmux-sensible is on disk; the other declared plugins
	// (tmux, vim-tmux-navigator) are missing. tpm is excluded from
	// the declared set by design — it's not a managed plugin.
	for _, name := range []string{"tpm", "tmux-sensible"} {
		if err := os.MkdirAll(filepath.Join(plugins, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	missing, err := missingTmuxPlugins(conf, plugins)
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(missing)
	want := []string{"tmux", "vim-tmux-navigator"}
	if len(missing) != len(want) {
		t.Fatalf("missing = %v, want %v", missing, want)
	}
	for i := range want {
		if missing[i] != want[i] {
			t.Fatalf("missing = %v, want %v", missing, want)
		}
	}
}

// TestMissingTmuxPluginsAllPresent — happy path, every declared
// plugin has a directory. Returns nil so MaintainTmuxPlugins
// short-circuits without invoking install_plugins.sh.
func TestMissingTmuxPluginsAllPresent(t *testing.T) {
	tmp := t.TempDir()
	conf := filepath.Join(tmp, "tmux.conf")
	if err := os.WriteFile(conf, []byte(sampleTmuxConf), 0o644); err != nil {
		t.Fatal(err)
	}
	plugins := filepath.Join(tmp, "plugins")
	for _, name := range []string{
		"tpm", "tmux-sensible", "tmux", "vim-tmux-navigator",
	} {
		if err := os.MkdirAll(filepath.Join(plugins, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	missing, err := missingTmuxPlugins(conf, plugins)
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) != 0 {
		t.Fatalf("expected no missing plugins, got %v", missing)
	}
}

// TestMissingTmuxPluginsBareDir — pluginsDir exists but is empty.
// Every declared plugin (minus tpm) should be flagged missing.
func TestMissingTmuxPluginsBareDir(t *testing.T) {
	tmp := t.TempDir()
	conf := filepath.Join(tmp, "tmux.conf")
	if err := os.WriteFile(conf, []byte(sampleTmuxConf), 0o644); err != nil {
		t.Fatal(err)
	}
	plugins := filepath.Join(tmp, "plugins")
	if err := os.MkdirAll(plugins, 0o755); err != nil {
		t.Fatal(err)
	}
	missing, err := missingTmuxPlugins(conf, plugins)
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(missing)
	want := []string{"tmux", "tmux-sensible", "vim-tmux-navigator"}
	if len(missing) != len(want) {
		t.Fatalf("missing = %v, want %v", missing, want)
	}
	for i := range want {
		if missing[i] != want[i] {
			t.Fatalf("missing = %v, want %v", missing, want)
		}
	}
}

// TestMissingTmuxPluginsNoConf — no symlinked config yet. Not an
// error; healer is a no-op until symlinks land.
func TestMissingTmuxPluginsNoConf(t *testing.T) {
	tmp := t.TempDir()
	missing, err := missingTmuxPlugins(
		filepath.Join(tmp, "no-conf"),
		filepath.Join(tmp, "no-dir"),
	)
	if err != nil {
		t.Fatalf("expected nil err, got %v", err)
	}
	if len(missing) != 0 {
		t.Fatalf("expected no missing entries, got %v", missing)
	}
}

// TestStaleTmuxPluginsIgnoresFiles — something dropped a stray file
// (screenshot, .DS_Store, etc) into ~/.tmux/plugins/. Don't try to
// remove it — we only handle plugin dirs.
func TestStaleTmuxPluginsIgnoresFiles(t *testing.T) {
	tmp := t.TempDir()
	conf := filepath.Join(tmp, "tmux.conf")
	if err := os.WriteFile(conf, []byte("# empty\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	plugins := filepath.Join(tmp, "plugins")
	if err := os.MkdirAll(plugins, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(plugins, ".DS_Store"), []byte("junk"), 0o644,
	); err != nil {
		t.Fatal(err)
	}
	stale, err := staleTmuxPlugins(conf, plugins)
	if err != nil {
		t.Fatal(err)
	}
	if len(stale) != 0 {
		t.Fatalf("expected files to be ignored, got %v", stale)
	}
}
