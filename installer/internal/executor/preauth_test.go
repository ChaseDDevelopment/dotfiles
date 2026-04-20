package executor

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPreAuthHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_PREAUTH_HELPER") != "1" {
		return
	}
	mainStdin, _ := os.Open("/dev/null")
	defer mainStdin.Close()
	os.Stdin = mainStdin
	if err := PreAuth(); err != nil {
		os.Exit(2)
	}
	os.Exit(0)
}

func TestPreAuthAndKeepalive(t *testing.T) {
	dir := t.TempDir()
	fakebin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(fakebin, 0o755); err != nil {
		t.Fatal(err)
	}
	stateFile := filepath.Join(dir, "sudo-state")
	if err := os.WriteFile(filepath.Join(fakebin, "sudo"), []byte(`#!/bin/sh
if [ "$1" = "-v" ]; then
  exit 0
fi
if [ "$1" = "-n" ] && [ "$2" = "-v" ]; then
  count=0
  if [ -f "`+stateFile+`" ]; then
    count=$(cat "`+stateFile+`")
  fi
  count=$((count + 1))
  printf '%s' "$count" > "`+stateFile+`"
  exit 1
fi
exit 0
`), 0o755); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestPreAuthHelperProcess")
	cmd.Env = append(os.Environ(),
		"GO_WANT_PREAUTH_HELPER=1",
		"PATH="+fakebin+string(os.PathListSeparator)+os.Getenv("PATH"),
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("PreAuth helper failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "[sudo] Priming credentials") {
		t.Fatalf("expected sudo priming banner, got %s", out)
	}
}
