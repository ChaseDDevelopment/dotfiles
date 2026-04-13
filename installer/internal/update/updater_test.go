package update

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chaseddevelopment/dotfiles/installer/internal/executor"
	"github.com/chaseddevelopment/dotfiles/installer/internal/pkgmgr"
)

type testPkgMgr struct {
	name       string
	updated    bool
	installed  []string
	installErr error
}

func (t *testPkgMgr) Name() string              { return t.name }
func (t *testPkgMgr) IsInstalled(_ string) bool { return false }
func (t *testPkgMgr) Install(_ context.Context, names ...string) error {
	t.installed = append(t.installed, names...)
	return t.installErr
}
func (t *testPkgMgr) UpdateAll(_ context.Context) error {
	t.updated = true
	return nil
}
func (t *testPkgMgr) MapName(name string) []string { return []string{name} }

var _ pkgmgr.PackageManager = (*testPkgMgr)(nil)

func newTestRunner(t *testing.T) (*executor.Runner, string) {
	t.Helper()
	dir := t.TempDir()
	log, err := executor.NewLogFile(filepath.Join(dir, "test.log"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { log.Close() })
	return executor.NewRunner(log, false), dir
}

func writeScript(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func prependPath(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestSelfUpdateStep(t *testing.T) {
	runner, _ := newTestRunner(t)
	if got := SelfUpdateStep(runner, "dev"); got != nil {
		t.Fatal("expected nil step for dev version")
	}
	if got := SelfUpdateStep(runner, ""); got != nil {
		t.Fatal("expected nil step for empty version")
	}
	got := SelfUpdateStep(runner, "v1.2.3")
	if got == nil || got.Name != "dotsetup self-update" {
		t.Fatalf("unexpected step: %#v", got)
	}
}

func TestUpdateNvm(t *testing.T) {
	runner, dir := newTestRunner(t)
	t.Setenv("HOME", dir)
	if err := updateNvm(context.Background(), runner); err != nil {
		t.Fatalf("missing nvm dir should be noop: %v", err)
	}

	nvmDir := filepath.Join(dir, ".config", "nvm")
	if err := os.MkdirAll(nvmDir, 0o755); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(dir, "nvm.log")
	t.Setenv("NVM_LOG", logPath)
	writeScript(t, nvmDir, "nvm.sh", `
nvm() {
  printf '%s\n' "$*" >> "$NVM_LOG"
}
`)
	if err := updateNvm(context.Background(), runner); err != nil {
		t.Fatalf("updateNvm: %v", err)
	}
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	if !strings.Contains(got, "install --lts") || !strings.Contains(got, "alias default lts/*") {
		t.Fatalf("expected nvm invocations in log, got %q", got)
	}
}

func TestRunDownloadedScript(t *testing.T) {
	runner, dir := newTestRunner(t)
	fakebin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(fakebin, 0o755); err != nil {
		t.Fatal(err)
	}
	prependPath(t, fakebin)
	output := filepath.Join(dir, "script.log")
	t.Setenv("SCRIPT_LOG", output)
	writeScript(t, fakebin, "curl", `#!/usr/bin/env bash
dest=""
while [ "$#" -gt 0 ]; do
  if [ "$1" = "-o" ]; then
    dest="$2"
    shift 2
    continue
  fi
  shift
done
cat > "$dest" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' "$*" > "$SCRIPT_LOG"
EOF
`)
	if err := runDownloadedScript(context.Background(), runner, "https://example.invalid/install.sh", []string{"--channel", "LTS"}); err != nil {
		t.Fatalf("runDownloadedScript: %v", err)
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(data)) != "--channel LTS" {
		t.Fatalf("unexpected script args: %q", string(data))
	}
}

func TestUpdatePackageHelpers(t *testing.T) {
	runner, dir := newTestRunner(t)
	fakebin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(fakebin, 0o755); err != nil {
		t.Fatal(err)
	}
	prependPath(t, fakebin)
	logPath := filepath.Join(dir, "commands.log")
	t.Setenv("UPDATE_LOG", logPath)
	for _, name := range []string{"brew", "sudo", "cargo"} {
		writeScript(t, fakebin, name, fmt.Sprintf(`#!/usr/bin/env bash
printf '%%s %%s\n' %q "$*" >> "$UPDATE_LOG"
`, name))
	}

	if err := updateStarship(context.Background(), runner, &testPkgMgr{name: "brew"}); err != nil {
		t.Fatalf("updateStarship brew: %v", err)
	}
	if err := updateAtuin(context.Background(), runner, "apt"); err != nil {
		t.Fatalf("updateAtuin apt+cargo: %v", err)
	}
	if err := updateDotnet(context.Background(), runner, "brew"); err != nil {
		t.Fatalf("updateDotnet brew: %v", err)
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	for _, want := range []string{
		"brew upgrade starship",
		"cargo install atuin",
		"brew upgrade dotnet-sdk",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("command log missing %q:\n%s", want, got)
		}
	}
}

func TestUpdateHelpersAdditionalBranches(t *testing.T) {
	runner, dir := newTestRunner(t)
	fakebin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(fakebin, 0o755); err != nil {
		t.Fatal(err)
	}
	prependPath(t, fakebin)
	logPath := filepath.Join(dir, "helpers.log")
	t.Setenv("UPDATE_LOG", logPath)
	writeScript(t, fakebin, "sudo", `#!/bin/sh
printf 'sudo %s\n' "$*" >> "$UPDATE_LOG"
exit 0
`)
	writeScript(t, fakebin, "yay", `#!/bin/sh
printf 'yay %s\n' "$*" >> "$UPDATE_LOG"
exit 0
`)
	writeScript(t, fakebin, "cargo", `#!/bin/sh
printf 'cargo %s\n' "$*" >> "$UPDATE_LOG"
exit 0
`)
	writeScript(t, fakebin, "brew", `#!/bin/sh
printf 'brew %s\n' "$*" >> "$UPDATE_LOG"
exit 0
`)
	writeScript(t, fakebin, "nvim", `#!/bin/sh
printf 'nvim %s\n' "$*" >> "$UPDATE_LOG"
exit 0
`)
	writeScript(t, fakebin, "curl", `#!/bin/sh
printf 'curl %s\n' "$*" >> "$UPDATE_LOG"
dest=""
while [ "$#" -gt 0 ]; do
  if [ "$1" = "-o" ]; then
    dest="$2"
    shift 2
    continue
  fi
  shift
done
cat > "$dest" <<'EOF'
#!/bin/sh
exit 0
EOF
exit 0
`)
	writeScript(t, fakebin, "bash", `#!/bin/sh
printf 'bash %s\n' "$*" >> "$UPDATE_LOG"
exit 0
`)

	if err := updateStarship(context.Background(), runner, &testPkgMgr{name: "pacman"}); err != nil {
		t.Fatalf("updateStarship pacman: %v", err)
	}
	if err := updateAtuin(context.Background(), runner, "brew"); err != nil {
		t.Fatalf("updateAtuin brew: %v", err)
	}
	if err := updateNeovim(context.Background(), runner, &testPkgMgr{name: "pacman"}, nil); err != nil {
		t.Fatalf("updateNeovim pacman: %v", err)
	}
	if err := updateDotnet(context.Background(), runner, "apt"); err != nil {
		t.Fatalf("updateDotnet script fallback: %v", err)
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	for _, want := range []string{
		"sudo pacman -S --noconfirm starship",
		"brew upgrade atuin",
		"yay -S --noconfirm neovim-git",
		"curl -fsSL https://dot.net/v1/dotnet-install.sh",
		"bash /",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("helper log missing %q:\n%s", want, got)
		}
	}
}

func TestUpdateHelpersErrorAndFallbackBranches(t *testing.T) {
	runner, dir := newTestRunner(t)
	fakebin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(fakebin, 0o755); err != nil {
		t.Fatal(err)
	}
	prependPath(t, fakebin)
	logPath := filepath.Join(dir, "more.log")
	t.Setenv("UPDATE_LOG", logPath)

	writeScript(t, fakebin, "brew", `#!/bin/sh
printf 'brew %s\n' "$*" >> "$UPDATE_LOG"
exit 0
`)
	writeScript(t, fakebin, "nvim", `#!/bin/sh
printf 'nvim 0.12.0'
`)
	writeScript(t, fakebin, "sudo", `#!/bin/sh
printf 'sudo %s\n' "$*" >> "$UPDATE_LOG"
exit 0
`)

	if err := updateStarship(context.Background(), runner, &testPkgMgr{name: "dnf"}); err != nil {
		t.Fatalf("updateStarship dnf install path: %v", err)
	}
	if err := updateNeovim(context.Background(), runner, &testPkgMgr{name: "unknown"}, nil); err != nil {
		t.Fatalf("updateNeovim default mgr path: %v", err)
	}

	// Unsupported managers should return actionable errors.
	if err := updateStarship(context.Background(), runner, &testPkgMgr{name: "unknown"}); err == nil {
		t.Fatal("expected updateStarship unsupported-manager error")
	}
	if err := updateAtuin(context.Background(), runner, "unknown"); err == nil {
		t.Fatal("expected updateAtuin unsupported-manager error")
	}
}

func TestAllStepsAndCargoUpdates(t *testing.T) {
	runner, dir := newTestRunner(t)
	fakebin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(fakebin, 0o755); err != nil {
		t.Fatal(err)
	}
	prependPath(t, fakebin)
	logPath := filepath.Join(dir, "allsteps.log")
	t.Setenv("UPDATE_LOG", logPath)
	t.Setenv("HOME", dir)

	for _, name := range []string{"rustup", "cargo", "uv", "bun", "brew", "ya", "eza"} {
		writeScript(t, fakebin, name, fmt.Sprintf(`#!/usr/bin/env bash
printf '%%s %%s\n' %q "$*" >> "$UPDATE_LOG"
`, name))
	}

	tpmDir := filepath.Join(dir, ".tmux", "plugins", "tpm", "scripts")
	if err := os.MkdirAll(tpmDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeScript(t, tpmDir, "update_plugin.sh", `#!/usr/bin/env bash
printf 'tpm %s\n' "$*" >> "$UPDATE_LOG"
`)

	mgr := &testPkgMgr{name: "brew"}
	steps := AllSteps(runner, mgr, nil)
	if len(steps) != 12 {
		t.Fatalf("AllSteps len = %d, want 12", len(steps))
	}
	for _, step := range steps {
		if err := step.Fn(context.Background()); err != nil {
			t.Fatalf("%s: %v", step.Name, err)
		}
	}
	if !mgr.updated {
		t.Fatal("system package step did not run UpdateAll")
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	for _, want := range []string{
		"rustup update",
		"cargo install eza",
		"uv self update",
		"uv tool upgrade --all",
		"bun upgrade",
		"brew upgrade starship",
		"brew upgrade neovim",
		"ya pkg upgrade",
		"tpm all",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("AllSteps log missing %q:\n%s", want, got)
		}
	}
}
