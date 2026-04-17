package pkgmgr

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chaseddevelopment/dotfiles/installer/internal/platform"
)

func TestGenericManagersAndNew(t *testing.T) {
	runner, dir := newPkgRunner(t)
	fakebin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(fakebin, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fakebin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("PKG_LOG", filepath.Join(dir, "pkg.log"))
	for _, name := range []string{"pacman", "dnf", "yum", "zypper"} {
		body := `#!/usr/bin/env bash
printf '%s %s\n' "` + name + `" "$*" >> "$PKG_LOG"
exit 0
`
		if err := os.WriteFile(filepath.Join(fakebin, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(fakebin, "sudo"), []byte(`#!/usr/bin/env bash
printf 'sudo %s\n' "$*" >> "$PKG_LOG"
exec "$@"
`), 0o755); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name    string
		pm      PackageManager
		install []string
		check   string
	}{
		{name: "pacman", pm: newPacman(runner), install: []string{"nodejs", "build-essential"}, check: "pacman"},
		{name: "dnf", pm: newDnf(runner), install: []string{"fd"}, check: "dnf"},
		{name: "yum", pm: newYum(runner), install: []string{"fd"}, check: "yum"},
		{name: "zypper", pm: newZypper(runner), install: []string{"nodejs"}, check: "zypper"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.pm.Install(context.Background(), tc.install...); err != nil {
				t.Fatalf("Install: %v", err)
			}
			_ = tc.pm.IsInstalled(tc.install[0])
			if err := tc.pm.UpdateAll(context.Background()); err != nil {
				t.Fatalf("UpdateAll: %v", err)
			}
		})
	}

	for _, pmType := range []platform.PkgManagerType{platform.PkgBrew, platform.PkgApt, platform.PkgPacman, platform.PkgDnf, platform.PkgYum, platform.PkgZypper} {
		pm, err := New(&platform.Platform{PackageManager: pmType}, runner)
		if err != nil {
			t.Fatalf("New(%v): %v", pmType, err)
		}
		if pm == nil {
			t.Fatalf("New(%v) returned nil manager", pmType)
		}
	}

	data, err := os.ReadFile(filepath.Join(dir, "pkg.log"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	for _, want := range []string{
		"pacman -S --needed --noconfirm nodejs npm base-devel",
		"dnf install -y fd-find",
		"yum install -y fd-find",
		"zypper --non-interactive install nodejs npm",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("package log missing %q:\n%s", want, got)
		}
	}
}
