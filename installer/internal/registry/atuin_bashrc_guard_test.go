package registry

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGuardAtuinBashrcInit covers every branch of the idempotent
// bashrc-patch helper that F2 adds to installAtuin. Root cause
// trail in `/Users/orion/.claude/plans/pluto-installer-regressions.md`:
// the upstream atuin installer (and any prior manual install)
// appends a naked `eval "$(atuin init bash)"` line that fails on
// every non-interactive bash invocation until we guard it.
func TestGuardAtuinBashrcInit(t *testing.T) {
	const unguarded = `eval "$(atuin init bash)"`
	const guardedPATH = `export PATH="$HOME/.atuin/bin:$PATH"`
	const guardedEval = `command -v atuin >/dev/null && eval "$(atuin init bash)"`

	cases := []struct {
		name       string
		initial    string // original bashrc contents; empty = file absent
		wantPatch  bool
		wantBackup bool
		// wantBody, when set, is the exact expected bashrc body
		// after the patch. Skip when patching isn't expected.
		wantBody string
	}{
		{
			name: "unguarded line present → patched and backed up",
			initial: `# user bashrc
alias ll='ls -la'
eval "$(atuin init bash)"
# trailing content
`,
			wantPatch:  true,
			wantBackup: true,
			wantBody: `# user bashrc
alias ll='ls -la'
export PATH="$HOME/.atuin/bin:$PATH"
command -v atuin >/dev/null && eval "$(atuin init bash)"
# trailing content
`,
		},
		{
			name: "already guarded → no-op",
			initial: `export PATH="$HOME/.atuin/bin:$PATH"
command -v atuin >/dev/null && eval "$(atuin init bash)"
`,
			wantPatch:  false,
			wantBackup: false,
		},
		{
			name:       "atuin line absent → no-op",
			initial:    "# nothing to see here\nalias ll='ls -la'\n",
			wantPatch:  false,
			wantBackup: false,
		},
		{
			name:       "file does not exist → no-op",
			initial:    "", // signals "don't create the file"
			wantPatch:  false,
			wantBackup: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			bashrc := filepath.Join(dir, ".bashrc")
			if tc.initial != "" {
				if err := os.WriteFile(bashrc, []byte(tc.initial), 0o644); err != nil {
					t.Fatal(err)
				}
			}

			if err := guardAtuinBashrcInit(bashrc); err != nil {
				t.Fatalf("guardAtuinBashrcInit: %v", err)
			}

			got, err := os.ReadFile(bashrc)
			if tc.initial == "" {
				if !os.IsNotExist(err) {
					t.Fatalf("bashrc should still not exist, got err=%v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("read bashrc back: %v", err)
			}
			gotStr := string(got)

			if tc.wantPatch {
				if !strings.Contains(gotStr, guardedPATH) {
					t.Errorf("missing PATH prepend, got:\n%s", gotStr)
				}
				if !strings.Contains(gotStr, guardedEval) {
					t.Errorf("missing guarded eval, got:\n%s", gotStr)
				}
				if strings.Contains(gotStr, "\n"+unguarded+"\n") {
					t.Errorf("unguarded line still present, got:\n%s", gotStr)
				}
				if tc.wantBody != "" && gotStr != tc.wantBody {
					t.Errorf("body mismatch\nwant:\n%s\n\ngot:\n%s",
						tc.wantBody, gotStr)
				}
			} else if gotStr != tc.initial {
				t.Errorf("no-op case rewrote the file\nwant:\n%s\n\ngot:\n%s",
					tc.initial, gotStr)
			}

			backupPath := bashrc + ".dotsetup.bak"
			_, backupErr := os.Stat(backupPath)
			switch {
			case tc.wantBackup && backupErr != nil:
				t.Errorf("expected backup %s, got err=%v", backupPath, backupErr)
			case !tc.wantBackup && backupErr == nil:
				t.Errorf("unexpected backup %s created", backupPath)
			}
		})
	}
}

// TestGuardAtuinBashrcInit_BackupNotOverwritten confirms that when
// a .dotsetup.bak already exists (from a previous install), rerunning
// the guard doesn't clobber it. This matters because the user may
// have diffed + stashed the original content and we don't want a
// second run to replace their snapshot with post-patch content.
func TestGuardAtuinBashrcInit_BackupNotOverwritten(t *testing.T) {
	dir := t.TempDir()
	bashrc := filepath.Join(dir, ".bashrc")
	backup := bashrc + ".dotsetup.bak"

	// First patch creates the backup normally.
	initial := "alias ll='ls -la'\neval \"$(atuin init bash)\"\n"
	if err := os.WriteFile(bashrc, []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := guardAtuinBashrcInit(bashrc); err != nil {
		t.Fatalf("first patch: %v", err)
	}
	firstBackup, err := os.ReadFile(backup)
	if err != nil {
		t.Fatalf("read backup after first patch: %v", err)
	}
	if string(firstBackup) != initial {
		t.Fatalf("first backup doesn't match initial; got:\n%s", firstBackup)
	}

	// Manually stuff the bashrc with the unguarded line again and
	// mark the existing backup with a sentinel to prove it survives.
	sentinel := "SENTINEL — USER'S OWN BACKUP\n"
	if err := os.WriteFile(backup, []byte(sentinel), 0o600); err != nil {
		t.Fatal(err)
	}
	secondBody := "# fresh content\neval \"$(atuin init bash)\"\n"
	if err := os.WriteFile(bashrc, []byte(secondBody), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := guardAtuinBashrcInit(bashrc); err != nil {
		t.Fatalf("second patch: %v", err)
	}
	gotBackup, err := os.ReadFile(backup)
	if err != nil {
		t.Fatalf("read backup after second patch: %v", err)
	}
	if string(gotBackup) != sentinel {
		t.Errorf("backup was overwritten\nwant sentinel:\n%s\n\ngot:\n%s",
			sentinel, gotBackup)
	}
}
