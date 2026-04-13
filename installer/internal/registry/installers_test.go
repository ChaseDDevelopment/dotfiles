package registry

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chaseddevelopment/dotfiles/installer/internal/executor"
	"github.com/chaseddevelopment/dotfiles/installer/internal/platform"
)

func newInstallerCtx(t *testing.T) (*InstallContext, string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	log, err := executor.NewLogFile(filepath.Join(home, "test.log"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { log.Close() })
	return &InstallContext{
		Runner:   executor.NewRunner(log, false),
		PkgMgr:   &stubPkgMgr{name: "apt"},
		Platform: &platform.Platform{OS: platform.Linux, Arch: platform.AMD64},
	}, home
}

func TestOfficialInstallersAndVersionChecks(t *testing.T) {
	ic, home := newInstallerCtx(t)
	fakebin := filepath.Join(home, "bin")
	if err := os.MkdirAll(fakebin, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fakebin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("REG_LOG", filepath.Join(home, "registry.log"))

	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(fakebin, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	write("curl", `#!/bin/sh
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
printf 'curl %s\n' "$*" >> "$REG_LOG"
`)
	write("bash", `#!/bin/sh
printf 'bash %s\n' "$*" >> "$REG_LOG"
exit 0
`)
	write("sh", `#!/bin/sh
printf 'sh %s\n' "$*" >> "$REG_LOG"
exit 0
`)
	write("git", `#!/bin/sh
printf 'git %s\n' "$*" >> "$REG_LOG"
exit 0
`)
	write("tar", `#!/bin/sh
printf 'nvim-linux64/\nnvim-linux64/bin/nvim\n'
`)
	write("sudo", `#!/bin/sh
printf 'sudo %s\n' "$*" >> "$REG_LOG"
exit 0
`)
	write("dpkg", `#!/bin/sh
printf 'amd64'
`)
	write("tree-sitter", `#!/bin/sh
printf 'tree-sitter 0.21.0'
`)
	write("tool-old", `#!/bin/sh
printf 'tool-old 0.9.0'
`)

	origLatest := latestVersionFn
	latestVersionFn = func(repo string, stripV bool) (string, error) {
		if repo == "nvm-sh/nvm" {
			return "v1.2.3", nil
		}
		return "3.3.3", nil
	}
	defer func() { latestVersionFn = origLatest }()

	if err := installNvm(context.Background(), ic); err != nil {
		t.Fatalf("installNvm: %v", err)
	}
	if err := installAtuin(context.Background(), ic); err != nil {
		t.Fatalf("installAtuin: %v", err)
	}
	if err := installTPM(context.Background(), ic); err != nil {
		t.Fatalf("installTPM: %v", err)
	}
	if err := InstallNeovimApt(context.Background(), ic); err != nil {
		t.Fatalf("InstallNeovimApt: %v", err)
	}
	if err := installGhCLI(context.Background(), ic); err != nil {
		t.Fatalf("installGhCLI: %v", err)
	}

	treeTool := &Tool{Name: "tree", Command: "tree-sitter", MinVersion: "0.20.0"}
	if !CheckVersion(treeTool) {
		t.Fatal("expected tree-sitter version check to pass")
	}
	if got := InstalledVersion(treeTool); got != "0.21.0" {
		t.Fatalf("InstalledVersion = %q", got)
	}
	oldTool := &Tool{Name: "old", Command: "tool-old", MinVersion: "1.0.0"}
	if CheckVersion(oldTool) {
		t.Fatal("expected old tool version check to fail")
	}

	data, err := os.ReadFile(filepath.Join(home, "registry.log"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	for _, want := range []string{
		"bash /",
		"sh /",
		"git clone https://github.com/tmux-plugins/tpm",
		"sudo ln -s /opt/nvim-linux64/bin/nvim /usr/local/bin/nvim",
		"sudo install -m 0644",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("registry log missing %q:\n%s", want, got)
		}
	}
}

func TestAdditionalRegistryInstallersAndHelpers(t *testing.T) {
	ic, home := newInstallerCtx(t)
	fakebin := filepath.Join(home, "bin")
	if err := os.MkdirAll(fakebin, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fakebin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("REG_LOG", filepath.Join(home, "registry-extra.log"))
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(fakebin, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"curl", "tar", "sudo", "fc-cache", "yay", "cargo", "brew", "unzip"} {
		body := `#!/bin/sh
printf '%s %s\n' "` + name + `" "$*" >> "$REG_LOG"
if [ "` + name + `" = "curl" ]; then
  dest=""
  while [ "$#" -gt 0 ]; do
    if [ "$1" = "-o" ]; then
      dest="$2"
      shift 2
      continue
    fi
    shift
  done
  : > "$dest"
fi
exit 0
`
		write(name, body)
	}

	origLatest := latestVersionFn
	latestVersionFn = func(repo string, stripV bool) (string, error) { return "3.3.3", nil }
	defer func() { latestVersionFn = origLatest }()

	if err := installNerdFontLinux(context.Background(), ic); err != nil {
		t.Fatalf("installNerdFontLinux: %v", err)
	}
	if err := installTailspin(context.Background(), ic); err != nil {
		t.Fatalf("installTailspin: %v", err)
	}
	if err := installNeovimPacman(context.Background(), ic); err != nil {
		t.Fatalf("installNeovimPacman: %v", err)
	}
	if err := installYaziApt(context.Background(), ic); err != nil {
		t.Fatalf("installYaziApt: %v", err)
	}
	if err := installTreeSitterCLI(context.Background(), ic); err != nil {
		t.Fatalf("installTreeSitterCLI: %v", err)
	}

	if Lookup("nvim") == nil {
		t.Fatal("expected Lookup to find nvim")
	}
	if !ShouldInstall(&Tool{OSFilter: []string{"linux"}}, &platform.Platform{OS: platform.Linux}) {
		t.Fatal("expected linux OSFilter to match")
	}
	if ShouldInstall(&Tool{OSFilter: []string{"darwin"}}, &platform.Platform{OS: platform.Linux}) {
		t.Fatal("unexpected OSFilter match")
	}
	tool := &Tool{Name: "brew-only", IsInstalledFunc: func() bool { return true }, Command: "brew-only", MinVersion: ""}
	if !IsInstalled(tool) || CheckInstalled(tool) != StatusInstalled {
		t.Fatal("expected IsInstalledFunc tool to be installed")
	}
	if FirstPkgMgrStrategy(&Tool{
		Strategies: []InstallStrategy{{Method: MethodCustom}, {Method: MethodPackageManager, Managers: []string{"apt"}}},
	}, "apt") != nil {
		t.Fatal("non-pkgmgr first strategy should make tool ineligible")
	}

	homeFont := filepath.Join(home, ".local", "share", "fonts", "JetBrainsMono Nerd Font Complete.ttf")
	if err := os.MkdirAll(filepath.Dir(homeFont), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(homeFont, []byte("font"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !isNerdFontInstalled() {
		t.Fatal("expected nerd font detection from font directory")
	}

	data, err := os.ReadFile(filepath.Join(home, "registry-extra.log"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	for _, want := range []string{
		"tar -xJf",
		"sudo install -m 755 ",
		"yay -S --noconfirm neovim-git",
		"sudo dpkg -i ",
		"unzip -o ",
		"fc-cache -fv",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("registry extra log missing %q:\n%s", want, got)
		}
	}
}
