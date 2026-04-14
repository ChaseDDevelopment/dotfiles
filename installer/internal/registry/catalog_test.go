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

	// Starship's upstream installer refuses to run under bash; we
	// must explicitly select sh. Regression guard for the kashyyyk
	// failure ("Running installation script with non-POSIX bash...").
	starship := toolByCommand(t, dev, "starship")
	var starshipScript *ScriptConfig
	for _, s := range starship.Strategies {
		if s.Method == MethodScript && s.Script != nil {
			starshipScript = s.Script
			break
		}
	}
	if starshipScript == nil {
		t.Fatalf("starship should have a MethodScript strategy: %#v", starship.Strategies)
	}
	if starshipScript.Shell != "sh" {
		t.Fatalf("starship Script.Shell = %q, want \"sh\"", starshipScript.Shell)
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
