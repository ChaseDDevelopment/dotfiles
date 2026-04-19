package registry

import (
	"testing"

	gh "github.com/chaseddevelopment/dotfiles/installer/internal/github"
)

func toolByCommand(t *testing.T, tools []Tool, command string) Tool {
	t.Helper()
	for _, tool := range tools {
		if tool.Command == command {
			return tool
		}
	}
	t.Fatalf("tool with command %q not found", command)
	return Tool{}
}

func stringSliceContains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

func TestRustToolchainCatalog(t *testing.T) {
	tools := rustToolchain()
	if len(tools) != 1 {
		t.Fatalf("rustToolchain len = %d, want 1", len(tools))
	}
	rust := tools[0]
	if rust.Command != "cargo" {
		t.Fatalf("rust command = %q, want cargo", rust.Command)
	}
	if len(rust.Strategies) != 1 || rust.Strategies[0].Method != MethodScript {
		t.Fatalf("rust strategies = %#v", rust.Strategies)
	}
	if rust.Strategies[0].Script == nil || !rust.Strategies[0].Script.NoProfileModify {
		t.Fatalf("rust installer should avoid shell profile edits: %#v", rust.Strategies[0].Script)
	}
}

func TestCliToolsCatalog(t *testing.T) {
	tools := cliTools()

	eza := toolByCommand(t, tools, "eza")
	if eza.CargoCrate != "eza" {
		t.Fatalf("eza cargo crate = %q", eza.CargoCrate)
	}
	if len(eza.Strategies) < 4 || eza.Strategies[2].Method != MethodGitHubRelease {
		t.Fatalf("eza strategies = %#v", eza.Strategies)
	}

	bat := toolByCommand(t, tools, "bat")
	if len(bat.Strategies) != 2 {
		t.Fatalf("bat strategies = %#v", bat.Strategies)
	}
	if len(bat.Strategies[1].PostInstall) != 1 || bat.Strategies[1].PostInstall[0].Target != "/usr/local/bin/bat" {
		t.Fatalf("bat apt post-install = %#v", bat.Strategies[1].PostInstall)
	}

	yq := toolByCommand(t, tools, "yq")
	if got := yq.Strategies[len(yq.Strategies)-1].GitHub; got == nil || got.Pattern != gh.PatternRawBinary {
		t.Fatalf("yq should use raw binary release download, got %#v", got)
	}

	tailspin := toolByCommand(t, tools, "tspin")
	if tailspin.CargoCrate != "tailspin" {
		t.Fatalf("tailspin cargo crate = %q", tailspin.CargoCrate)
	}
}

func TestDevAndOfficialToolCatalog(t *testing.T) {
	dev := devTools()

	neovim := toolByCommand(t, dev, "nvim")
	if !neovim.Critical || neovim.MinVersion != "0.12.0" {
		t.Fatalf("unexpected neovim metadata: %#v", neovim)
	}
	if len(neovim.Strategies) != 4 {
		t.Fatalf("unexpected neovim strategies: %#v", neovim.Strategies)
	}
	if neovim.Strategies[1].Method != MethodCustom || neovim.Strategies[2].Method != MethodCustom {
		t.Fatalf("neovim should prefer custom pacman/apt installers: %#v", neovim.Strategies)
	}

	uv := toolByCommand(t, dev, "uv")
	if uv.Strategies[0].Script == nil || !uv.Strategies[0].Script.NoProfileModify {
		t.Fatalf("uv installer should disable profile edits: %#v", uv.Strategies[0].Script)
	}

	// Oh-My-Posh: brew strategy must use the fully qualified formula
	// (triggers auto-tap) and the script fallback must pass -d to
	// install into an already-PATHed directory.
	omp := toolByCommand(t, dev, "oh-my-posh")
	var ompBrew *InstallStrategy
	var ompScriptStrategy *InstallStrategy
	var ompScript *ScriptConfig
	for i, s := range omp.Strategies {
		if s.Method == MethodPackageManager && s.Package == "jandedobbeleer/oh-my-posh/oh-my-posh" {
			ompBrew = &omp.Strategies[i]
		}
		if s.Method == MethodScript && s.Script != nil {
			ompScriptStrategy = &omp.Strategies[i]
			ompScript = s.Script
		}
	}
	if ompBrew == nil {
		t.Fatalf("oh-my-posh should have a fully-qualified brew strategy: %#v", omp.Strategies)
	}
	if ompScript == nil {
		t.Fatalf("oh-my-posh should have a MethodScript strategy: %#v", omp.Strategies)
	}
	var sawInstallDirFlag bool
	for _, a := range ompScript.Args {
		if a == "-d" {
			sawInstallDirFlag = true
			break
		}
	}
	if !sawInstallDirFlag {
		t.Fatalf("oh-my-posh script should pass -d <dir>, got %#v", ompScript.Args)
	}
	// The ohmyposh.dev install script shells out to `unzip` and fails
	// hard when it's missing. Declaring Requires lets the orchestrator
	// block this strategy until the apt batch that installs unzip
	// completes, rather than racing it (see install.log on milkyway
	// 2026-04-19 where this race left oh-my-posh uninstalled).
	if !stringSliceContains(ompScriptStrategy.Requires, "unzip") {
		t.Fatalf("oh-my-posh script strategy must declare Requires=\"unzip\": %#v", ompScriptStrategy.Requires)
	}

	// tree-sitter-cli: installTreeSitterCLI shells out to `unzip` when
	// extracting the GitHub-release zip, so the custom strategy must
	// declare it (alongside curl) to avoid the same race.
	tsc := toolByCommand(t, dev, "tree-sitter")
	var tscCustom *InstallStrategy
	for i, s := range tsc.Strategies {
		if s.Method == MethodCustom {
			tscCustom = &tsc.Strategies[i]
			break
		}
	}
	if tscCustom == nil {
		t.Fatalf("tree-sitter-cli must have a MethodCustom strategy: %#v", tsc.Strategies)
	}
	for _, dep := range []string{"curl", "unzip"} {
		if !stringSliceContains(tscCustom.Requires, dep) {
			t.Fatalf("tree-sitter-cli custom strategy must declare Requires=%q: %#v",
				dep, tscCustom.Requires)
		}
	}

	yazi := toolByCommand(t, dev, "yazi")
	if yazi.CargoCrate != "yazi-build" {
		t.Fatalf("yazi cargo crate = %q", yazi.CargoCrate)
	}
	if yazi.Strategies[len(yazi.Strategies)-1].Method != MethodCargo {
		t.Fatalf("yazi should retain cargo fallback: %#v", yazi.Strategies)
	}

	official := officialInstallerTools()

	nvm := toolByCommand(t, official, "nvm")
	if len(nvm.Strategies) != 1 || nvm.Strategies[0].Method != MethodCustom {
		t.Fatalf("nvm strategies = %#v", nvm.Strategies)
	}

	node := toolByCommand(t, official, "node")
	if node.Strategies[0].Method != MethodPackageManager || node.Strategies[0].Package != "nodejs" {
		t.Fatalf("nodejs strategy = %#v", node.Strategies[0])
	}

	atuin := toolByCommand(t, official, "atuin")
	if got := atuin.Strategies[len(atuin.Strategies)-1].Method; got != MethodCustom {
		t.Fatalf("atuin should keep custom installer fallback, got %v", got)
	}

	tpm := toolByCommand(t, official, "tpm")
	if len(tpm.Strategies) != 1 || tpm.Strategies[0].Method != MethodCustom {
		t.Fatalf("tpm strategies = %#v", tpm.Strategies)
	}
}
