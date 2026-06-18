package update

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chaseddevelopment/dotfiles/installer/internal/platform"
)

// TestSelfUpdateStepInvokesSelfUpdate verifies the returned step's
// Fn calls SelfUpdate (observed via the seam).
func TestSelfUpdateStepInvokesSelfUpdate(t *testing.T) {
	runner, _ := newSelfUpdateCtx(t)
	orig := latestVersionFn
	defer func() { latestVersionFn = orig }()
	called := false
	latestVersionFn = func(string, bool) (string, error) {
		called = true
		return "v1.2.3", nil
	}
	step := SelfUpdateStep(runner, "v1.2.3")
	if step == nil {
		t.Fatal("expected non-nil step")
	}
	if err := step.Fn(context.Background()); err != nil {
		t.Fatalf("step.Fn: %v", err)
	}
	if !called {
		t.Fatal("expected SelfUpdate to be invoked")
	}
}

// TestUpdateCargoBinariesAggregatesErrors covers the error-collection
// branch when `cargo install` fails for a cargo-OWNED tool. `dust` is
// cargo-owned on apt (its active strategy there is MethodCargo), so the
// gate lets `cargo install du-dust` run — and the failing cargo stub
// surfaces the aggregated error.
func TestUpdateCargoBinariesAggregatesErrors(t *testing.T) {
	runner, dir := newTestRunner(t)
	bin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	prependPath(t, bin)
	// cargo stub returns non-zero so every cargo install fails.
	if err := os.WriteFile(filepath.Join(bin, "cargo"),
		[]byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	// dust must be on PATH so the HasCommand(t.Command) check passes.
	if err := os.WriteFile(filepath.Join(bin, "dust"),
		[]byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	err := updateCargoBinaries(context.Background(), runner, "apt")
	if err == nil || !strings.Contains(err.Error(), "cargo update failures") {
		t.Fatalf("expected aggregated cargo error, got %v", err)
	}
}

// TestUpdateCargoBinariesSkipsPkgMgrOwned verifies the ownership gate:
// on pacman, `eza` is package-manager-owned (active strategy is
// MethodPackageManager), so the cargo-update pass must skip it entirely
// — even though eza is on PATH and has a CargoCrate. No `cargo install`
// runs, so the failing cargo stub never fires and the call returns nil.
func TestUpdateCargoBinariesSkipsPkgMgrOwned(t *testing.T) {
	runner, dir := newTestRunner(t)
	bin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	prependPath(t, bin)
	logPath := filepath.Join(dir, "log")
	t.Setenv("UPDATE_LOG", logPath)
	// cargo stub logs + fails: if it ran, we'd both see the log line
	// and get an aggregated error.
	writeScript(t, bin, "cargo", `#!/bin/sh
printf 'cargo %s\n' "$*" >> "$UPDATE_LOG"
exit 1
`)
	writeScript(t, bin, "eza", "#!/bin/sh\nexit 0\n")

	if err := updateCargoBinaries(context.Background(), runner, "pacman"); err != nil {
		t.Fatalf("expected nil (eza is pacman-owned, skipped), got %v", err)
	}
	if data, err := os.ReadFile(logPath); err == nil && strings.Contains(string(data), "cargo install") {
		t.Fatalf("cargo install must not run for pacman-owned eza:\n%s", data)
	}
}

// TestUpdateCargoBinariesSkipsWhenCargoMissing covers the early-return
// branch when cargo is not installed.
func TestUpdateCargoBinariesSkipsWhenCargoMissing(t *testing.T) {
	runner, dir := newTestRunner(t)
	t.Setenv("PATH", dir) // empty dir → no cargo
	if err := updateCargoBinaries(context.Background(), runner, "apt"); err != nil {
		t.Fatalf("expected no-op when cargo missing, got %v", err)
	}
}

// TestUpdateAtuinPacmanPath covers the pacman branch: atuin is a pacman
// package, already refreshed by the system upgrade, so updateAtuin issues
// no command of its own and returns nil.
func TestUpdateAtuinPacmanPath(t *testing.T) {
	runner, dir := newTestRunner(t)
	bin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	prependPath(t, bin)
	logPath := filepath.Join(dir, "log")
	t.Setenv("UPDATE_LOG", logPath)
	writeScript(t, bin, "atuin", "#!/bin/sh\nexit 0\n")
	// Any command stub here would record to UPDATE_LOG if invoked.
	writeScript(t, bin, "sudo", `#!/bin/sh
printf 'sudo %s\n' "$*" >> "$UPDATE_LOG"
exit 0
`)
	writeScript(t, bin, "curl", `#!/bin/sh
printf 'curl %s\n' "$*" >> "$UPDATE_LOG"
exit 0
`)
	if err := updateAtuin(context.Background(), runner, &testPkgMgr{name: "pacman"}, &platform.Platform{}); err != nil {
		t.Fatalf("updateAtuin pacman: %v", err)
	}
	if data, err := os.ReadFile(logPath); err == nil && len(strings.TrimSpace(string(data))) != 0 {
		t.Fatalf("pacman atuin path should issue no command, log:\n%s", data)
	}
}

// TestUpdateAtuinBrewPath covers the brew branch: atuin is a brew package,
// covered by the system upgrade → no command, returns nil.
func TestUpdateAtuinBrewPath(t *testing.T) {
	runner, dir := newTestRunner(t)
	bin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	prependPath(t, bin)
	logPath := filepath.Join(dir, "log")
	t.Setenv("UPDATE_LOG", logPath)
	writeScript(t, bin, "atuin", "#!/bin/sh\nexit 0\n")
	writeScript(t, bin, "brew", `#!/bin/sh
printf 'brew %s\n' "$*" >> "$UPDATE_LOG"
exit 0
`)
	if err := updateAtuin(context.Background(), runner, &testPkgMgr{name: "brew"}, &platform.Platform{}); err != nil {
		t.Fatalf("updateAtuin brew: %v", err)
	}
	if data, err := os.ReadFile(logPath); err == nil && len(strings.TrimSpace(string(data))) != 0 {
		t.Fatalf("brew atuin path should issue no command, log:\n%s", data)
	}
}

// TestUpdateAtuinMissingBinary covers the HasCommand=false early-return.
func TestUpdateAtuinMissingBinary(t *testing.T) {
	runner, dir := newTestRunner(t)
	t.Setenv("PATH", dir)
	if err := updateAtuin(context.Background(), runner, &testPkgMgr{name: "brew"}, &platform.Platform{}); err != nil {
		t.Fatalf("expected no-op when atuin missing, got %v", err)
	}
}

// TestUpdateAtuinAptInstaller covers the apt/dnf/yum branch: atuin is
// installed via its official installer, so updateAtuin re-runs that
// install strategy — `curl ... https://setup.atuin.sh -o <tmp>` then
// `sh <tmp> --non-interactive`.
func TestUpdateAtuinAptInstaller(t *testing.T) {
	runner, dir := newTestRunner(t)
	bin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	prependPath(t, bin)
	t.Setenv("HOME", dir)
	logPath := filepath.Join(dir, "log")
	t.Setenv("UPDATE_LOG", logPath)
	writeScript(t, bin, "atuin", "#!/bin/sh\nexit 0\n")
	// curl logs its args and writes an exit-0 script to the -o dest.
	writeScript(t, bin, "curl", `#!/bin/sh
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
	writeScript(t, bin, "sh", `#!/bin/sh
printf 'sh %s\n' "$*" >> "$UPDATE_LOG"
exit 0
`)
	if err := updateAtuin(context.Background(), runner, &testPkgMgr{name: "apt"}, &platform.Platform{}); err != nil {
		t.Fatalf("updateAtuin apt: %v", err)
	}
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "https://setup.atuin.sh") {
		t.Fatalf("expected official atuin installer URL, log:\n%s", data)
	}
}

// TestUpdateNeovimBrewPath covers the brew branch: the --HEAD formula is
// refreshed with `brew upgrade --fetch-HEAD neovim` (plain `brew upgrade`
// skips HEAD formulae).
func TestUpdateNeovimBrewPath(t *testing.T) {
	runner, dir := newTestRunner(t)
	bin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	prependPath(t, bin)
	logPath := filepath.Join(dir, "log")
	t.Setenv("UPDATE_LOG", logPath)
	writeScript(t, bin, "nvim", "#!/bin/sh\nexit 0\n")
	writeScript(t, bin, "brew", `#!/bin/sh
printf 'brew %s\n' "$*" >> "$UPDATE_LOG"
exit 0
`)
	if err := updateNeovim(context.Background(), runner, &testPkgMgr{name: "brew"}, &platform.Platform{}); err != nil {
		t.Fatalf("updateNeovim brew: %v", err)
	}
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "brew upgrade --fetch-HEAD neovim") {
		t.Fatalf("expected brew HEAD upgrade, log:\n%s", data)
	}
}

// TestUpdateNeovimAptPath covers the apt branch: nvim isn't an apt
// package, so updateNeovim reuses InstallNeovimApt to fetch the latest
// GitHub release tarball.
func TestUpdateNeovimAptPath(t *testing.T) {
	runner, dir := newTestRunner(t)
	bin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	prependPath(t, bin)
	t.Setenv("HOME", dir)
	logPath := filepath.Join(dir, "log")
	t.Setenv("UPDATE_LOG", logPath)
	writeScript(t, bin, "nvim", "#!/bin/sh\nexit 0\n")
	writeScript(t, bin, "curl", `#!/bin/sh
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
nvim
EOF
exit 0
`)
	writeScript(t, bin, "tar", `#!/bin/sh
printf 'nvim-linux-x86_64/\nnvim-linux-x86_64/bin/nvim\n'
`)
	writeScript(t, bin, "sudo", `#!/bin/sh
printf 'sudo %s\n' "$*" >> "$UPDATE_LOG"
exit 0
`)
	plat := &platform.Platform{OS: platform.Linux, Arch: platform.AMD64}
	if err := updateNeovim(context.Background(), runner, &testPkgMgr{name: "apt"}, plat); err != nil {
		t.Fatalf("updateNeovim apt: %v", err)
	}
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "https://github.com/neovim/neovim/releases") {
		t.Fatalf("expected GitHub release download, log:\n%s", data)
	}
}

// TestUpdateNeovimDefaultPath covers the default branch (pacman/dnf/…):
// nvim is an official package refreshed by the system upgrade, so
// updateNeovim issues no command and returns nil.
func TestUpdateNeovimDefaultPath(t *testing.T) {
	runner, dir := newTestRunner(t)
	bin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	prependPath(t, bin)
	logPath := filepath.Join(dir, "log")
	t.Setenv("UPDATE_LOG", logPath)
	writeScript(t, bin, "nvim", "#!/bin/sh\nexit 0\n")
	writeScript(t, bin, "sudo", `#!/bin/sh
printf 'sudo %s\n' "$*" >> "$UPDATE_LOG"
exit 0
`)
	if err := updateNeovim(context.Background(), runner, &testPkgMgr{name: "pacman"}, &platform.Platform{}); err != nil {
		t.Fatalf("updateNeovim pacman default: %v", err)
	}
	if data, err := os.ReadFile(logPath); err == nil && len(strings.TrimSpace(string(data))) != 0 {
		t.Fatalf("default neovim path should issue no command, log:\n%s", data)
	}
}

// TestUpdateNeovimSkipsWhenAbsent covers the HasCommand=false branch.
func TestUpdateNeovimSkipsWhenAbsent(t *testing.T) {
	runner, dir := newTestRunner(t)
	t.Setenv("PATH", dir)
	if err := updateNeovim(context.Background(), runner, &testPkgMgr{name: "brew"}, nil); err != nil {
		t.Fatalf("expected no-op when nvim missing, got %v", err)
	}
}

// TestUpdateDotnetPacmanPath covers the pacman branch of updateDotnet:
// dotnet is a pacman package, refreshed by the system upgrade, so
// updateDotnet issues no command of its own and returns nil.
func TestUpdateDotnetPacmanPath(t *testing.T) {
	runner, dir := newTestRunner(t)
	bin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	prependPath(t, bin)
	logPath := filepath.Join(dir, "log")
	t.Setenv("UPDATE_LOG", logPath)
	writeScript(t, bin, "dotnet", "#!/bin/sh\nexit 0\n")
	writeScript(t, bin, "sudo", `#!/bin/sh
printf 'sudo %s\n' "$*" >> "$UPDATE_LOG"
exit 0
`)
	writeScript(t, bin, "curl", `#!/bin/sh
printf 'curl %s\n' "$*" >> "$UPDATE_LOG"
exit 0
`)
	if err := updateDotnet(context.Background(), runner, "pacman"); err != nil {
		t.Fatalf("updateDotnet pacman: %v", err)
	}
	if data, err := os.ReadFile(logPath); err == nil && len(strings.TrimSpace(string(data))) != 0 {
		t.Fatalf("pacman dotnet path should issue no command, log:\n%s", data)
	}
}

// TestUpdateDotnetSkipsWhenAbsent covers the early return.
func TestUpdateDotnetSkipsWhenAbsent(t *testing.T) {
	runner, dir := newTestRunner(t)
	t.Setenv("PATH", dir)
	if err := updateDotnet(context.Background(), runner, "brew"); err != nil {
		t.Fatalf("expected no-op when dotnet missing, got %v", err)
	}
}

// TestRunDownloadedScriptCurlFails covers the download-error branch.
func TestRunDownloadedScriptCurlFails(t *testing.T) {
	runner, dir := newTestRunner(t)
	bin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	writeScript(t, bin, "curl", "#!/bin/sh\nexit 22\n")
	err := runDownloadedScript(context.Background(), runner,
		"https://example.invalid/x.sh", nil)
	if err == nil || !strings.Contains(err.Error(), "download") {
		t.Fatalf("expected download error, got %v", err)
	}
}

// TestAllStepsSkipsMissingCommands covers the "HasCommand → return nil"
// branches by running AllSteps with an empty PATH. Every step must
// return nil because no optional binary is present.
func TestAllStepsSkipsMissingCommands(t *testing.T) {
	runner, dir := newTestRunner(t)
	t.Setenv("PATH", dir) // no command exists
	t.Setenv("HOME", dir)
	mgr := &testPkgMgr{name: "brew", installErr: fmt.Errorf("should not run")}

	steps := AllSteps(runner, mgr, nil)
	for _, step := range steps {
		if step.Name == "System packages" {
			// UpdateAll is always called regardless of HasCommand.
			continue
		}
		if err := step.Fn(context.Background()); err != nil {
			t.Fatalf("%s with empty PATH: %v", step.Name, err)
		}
	}
}
