package orchestrator

import (
	"testing"

	"github.com/chaseddevelopment/dotfiles/installer/internal/registry"
)

// TestDerivedDepsCargo covers the dns1 regression: a MethodCargo
// strategy must add "cargo" to the task's deps so the scheduler
// waits for the rust tool (Command="cargo") to finish installing.
func TestDerivedDepsCargo(t *testing.T) {
	tool := &registry.Tool{
		Name: "dust", Command: "dust",
		Strategies: []registry.InstallStrategy{
			{Method: registry.MethodCargo, Crate: "du-dust"},
		},
	}
	got := appendDerivedDeps(nil, tool, "apt", map[string]bool{})
	if !containsString(got, "cargo") {
		t.Fatalf("expected cargo in derived deps, got %v", got)
	}
}

// TestDerivedDepsRequires covers MethodCustom closures: without an
// explicit Requires declaration the orchestrator has no way to know
// what the closure shells out to. Declaring Requires adds the dep.
func TestDerivedDepsRequires(t *testing.T) {
	tool := &registry.Tool{
		Name: "tpm", Command: "tpm",
		Strategies: []registry.InstallStrategy{
			{Method: registry.MethodCustom, Requires: []string{"git"}},
		},
	}
	got := appendDerivedDeps(nil, tool, "apt", map[string]bool{})
	if !containsString(got, "git") {
		t.Fatalf("expected git in derived deps, got %v", got)
	}
}

// TestDerivedDepsInstalledSkipped prevents redundant edges: when
// the required tool is already present on the host, the derivation
// drops the dep (no scheduling gain, just noise).
func TestDerivedDepsInstalledSkipped(t *testing.T) {
	tool := &registry.Tool{
		Name: "dust", Command: "dust",
		Strategies: []registry.InstallStrategy{
			{Method: registry.MethodCargo, Crate: "du-dust"},
		},
	}
	installed := map[string]bool{"cargo": true}
	got := appendDerivedDeps(nil, tool, "apt", installed)
	if containsString(got, "cargo") {
		t.Fatalf("expected cargo skipped when installed, got %v", got)
	}
}

// TestDerivedDepsNoSelfEdge — a tool whose own Command matches the
// required method-provider shouldn't depend on itself. Matters for
// `rust` (Command="cargo"): if anyone adds a MethodCargo strategy
// to rust (e.g. for update via `cargo install rustup`), it must not
// form a cycle.
func TestDerivedDepsNoSelfEdge(t *testing.T) {
	tool := &registry.Tool{
		Name: "rust", Command: "cargo",
		Strategies: []registry.InstallStrategy{
			{Method: registry.MethodCargo, Crate: "rustup"},
		},
	}
	got := appendDerivedDeps(nil, tool, "apt", map[string]bool{})
	if containsString(got, "cargo") {
		t.Fatalf("expected no self-edge, got %v", got)
	}
}

// TestDerivedDepsStrategyApplicability: a strategy that doesn't
// match the active package manager contributes nothing. On brew,
// an apt-only MethodCustom with Requires=["curl"] is irrelevant.
func TestDerivedDepsStrategyApplicability(t *testing.T) {
	tool := &registry.Tool{
		Name: "example", Command: "example",
		Strategies: []registry.InstallStrategy{
			{
				Managers: []string{"apt"},
				Method:   registry.MethodCustom,
				Requires: []string{"curl"},
			},
			{Managers: []string{"brew"}, Method: registry.MethodPackageManager, Package: "example"},
		},
	}
	gotBrew := appendDerivedDeps(nil, tool, "brew", map[string]bool{})
	if containsString(gotBrew, "curl") {
		t.Errorf("brew path: expected no curl dep, got %v", gotBrew)
	}
	gotApt := appendDerivedDeps(nil, tool, "apt", map[string]bool{})
	if !containsString(gotApt, "curl") {
		t.Errorf("apt path: expected curl dep, got %v", gotApt)
	}
}
