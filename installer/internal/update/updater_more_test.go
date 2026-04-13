package update

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
// branch when cargo install fails for at least one crate.
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
	// Also provide at least one tool binary so the "HasCommand" check
	// passes for that tool.
	if err := os.WriteFile(filepath.Join(bin, "eza"),
		[]byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	err := updateCargoBinaries(context.Background(), runner)
	if err == nil || !strings.Contains(err.Error(), "cargo update failures") {
		t.Fatalf("expected aggregated cargo error, got %v", err)
	}
}

// TestUpdateCargoBinariesSkipsWhenCargoMissing covers the early-return
// branch when cargo is not installed.
func TestUpdateCargoBinariesSkipsWhenCargoMissing(t *testing.T) {
	runner, dir := newTestRunner(t)
	t.Setenv("PATH", dir) // empty dir → no cargo
	if err := updateCargoBinaries(context.Background(), runner); err != nil {
		t.Fatalf("expected no-op when cargo missing, got %v", err)
	}
}

// TestUpdateAtuinPacmanPath covers the pacman branch (uncovered by
// existing tests which only hit brew and apt+cargo).
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
	writeScript(t, bin, "sudo", `#!/bin/sh
printf 'sudo %s\n' "$*" >> "$UPDATE_LOG"
exit 0
`)
	if err := updateAtuin(context.Background(), runner, "pacman"); err != nil {
		t.Fatalf("updateAtuin pacman: %v", err)
	}
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "sudo pacman -S --noconfirm atuin") {
		t.Fatalf("expected pacman command, log: %s", data)
	}
}

// TestUpdateAtuinMissingBinary covers the HasCommand=false early-return.
func TestUpdateAtuinMissingBinary(t *testing.T) {
	runner, dir := newTestRunner(t)
	t.Setenv("PATH", dir)
	if err := updateAtuin(context.Background(), runner, "brew"); err != nil {
		t.Fatalf("expected no-op when atuin missing, got %v", err)
	}
}

// TestUpdateAtuinAptNoCargo covers the apt/dnf/yum branch falling
// through when cargo is absent, producing the actionable error.
func TestUpdateAtuinAptNoCargo(t *testing.T) {
	runner, dir := newTestRunner(t)
	bin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	writeScript(t, bin, "atuin", "#!/bin/sh\nexit 0\n")
	// Deliberately don't install cargo.
	err := updateAtuin(context.Background(), runner, "apt")
	if err == nil || !strings.Contains(err.Error(), "install cargo") {
		t.Fatalf("expected cargo-missing error, got %v", err)
	}
}

// TestUpdateNeovimParuFallback covers the paru branch after yay fails.
// Also exercises the sudo-pacman terminal fallback.
func TestUpdateNeovimParuFallback(t *testing.T) {
	runner, dir := newTestRunner(t)
	bin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	prependPath(t, bin)
	logPath := filepath.Join(dir, "log")
	t.Setenv("UPDATE_LOG", logPath)
	writeScript(t, bin, "nvim", "#!/bin/sh\nexit 0\n")
	// yay fails → continue
	writeScript(t, bin, "yay", `#!/bin/sh
printf 'yay %s\n' "$*" >> "$UPDATE_LOG"
exit 1
`)
	// paru fails → continue
	writeScript(t, bin, "paru", `#!/bin/sh
printf 'paru %s\n' "$*" >> "$UPDATE_LOG"
exit 1
`)
	// sudo pacman succeeds (terminal fallback).
	writeScript(t, bin, "sudo", `#!/bin/sh
printf 'sudo %s\n' "$*" >> "$UPDATE_LOG"
exit 0
`)
	if err := updateNeovim(context.Background(), runner, &testPkgMgr{name: "pacman"}, nil); err != nil {
		t.Fatalf("updateNeovim pacman fallback: %v", err)
	}
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	for _, want := range []string{"yay -S", "paru -S", "sudo pacman -S --noconfirm neovim"} {
		if !strings.Contains(got, want) {
			t.Fatalf("log missing %q:\n%s", want, got)
		}
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

// TestUpdateDotnetPacmanPath covers the pacman branch of updateDotnet.
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
	if err := updateDotnet(context.Background(), runner, "pacman"); err != nil {
		t.Fatalf("updateDotnet pacman: %v", err)
	}
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "sudo pacman -S --noconfirm dotnet-sdk") {
		t.Fatalf("expected pacman dotnet-sdk, log:\n%s", data)
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
