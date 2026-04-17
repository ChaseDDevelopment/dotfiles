package registry

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLookupAndVersionHelpers(t *testing.T) {
	if Lookup("nvim") == nil {
		t.Fatal("expected Lookup to find nvim")
	}
	if Lookup("definitely-missing") != nil {
		t.Fatal("unexpected tool lookup hit")
	}

	dir := t.TempDir()
	fakebin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(fakebin, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fakebin+string(os.PathListSeparator)+os.Getenv("PATH"))
	if err := os.WriteFile(filepath.Join(fakebin, "vertool"), []byte(`#!/bin/sh
printf 'vertool 1.2.3'
`), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fakebin, "badver"), []byte(`#!/bin/sh
printf 'nonsense'
`), 0o755); err != nil {
		t.Fatal(err)
	}

	tool := &Tool{Name: "vertool", Command: "vertool", MinVersion: "1.2.0"}
	raw, triplet, pre, ok := getInstalledVersion(tool)
	if !ok || raw != "1.2.3" || triplet != [3]int{1, 2, 3} || pre {
		t.Fatalf("unexpected installed version parse: raw=%q triplet=%v pre=%v ok=%v", raw, triplet, pre, ok)
	}
	if !CheckVersion(tool) {
		t.Fatal("expected CheckVersion success")
	}

	bad := &Tool{Name: "badver", Command: "badver", MinVersion: "1.0.0"}
	if CheckVersion(bad) {
		t.Fatal("expected CheckVersion failure for unparsable version output")
	}
	if got := InstalledVersion(bad); got != "" {
		t.Fatalf("InstalledVersion(bad) = %q, want empty", got)
	}

	status := CheckInstalled(&Tool{Name: "custom", IsInstalledFunc: func() bool { return false }})
	if status != StatusNotInstalled {
		t.Fatalf("CheckInstalled custom false = %v, want not installed", status)
	}

	status = CheckInstalled(&Tool{Name: "vertool", Command: "vertool", MinVersion: "2.0.0"})
	if status != StatusOutdated {
		t.Fatalf("CheckInstalled outdated = %v, want outdated", status)
	}

	if !strings.Contains(Lookup("nvim").Name, "neovim") {
		t.Fatal("expected lookup name to reference neovim")
	}
}
