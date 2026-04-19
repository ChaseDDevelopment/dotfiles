package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoginShellFor verifies that loginShellFor parses the 7th
// colon-separated field of `getent passwd <user>` output. Using
// /etc/passwd (via getent) rather than $SHELL matters because
// $SHELL reflects the *current* shell, not what /bin/login will
// exec at the next session — and every "zsh broke my Proxmox
// login" forum thread roots at that distinction.
func TestLoginShellFor(t *testing.T) {
	fakebin := t.TempDir()
	write := func(script string) {
		t.Helper()
		p := filepath.Join(fakebin, "getent")
		if err := os.WriteFile(p, []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", fakebin+string(os.PathListSeparator)+os.Getenv("PATH"))

	cases := []struct {
		name, user, script, want string
	}{
		{
			name:   "zsh user",
			user:   "alice",
			script: "#!/bin/sh\nprintf 'alice:x:1000:1000:Alice,,,:/home/alice:/usr/bin/zsh\\n'\n",
			want:   "/usr/bin/zsh",
		},
		{
			name:   "bash user with trailing newline",
			user:   "bob",
			script: "#!/bin/sh\nprintf 'bob:x:1001:1001::/home/bob:/bin/bash\\n'\n",
			want:   "/bin/bash",
		},
		{
			name:   "empty user returns empty",
			user:   "",
			script: "#!/bin/sh\nexit 0\n",
			want:   "",
		},
		{
			name:   "getent failure returns empty",
			user:   "ghost",
			script: "#!/bin/sh\nexit 2\n",
			want:   "",
		},
		{
			name:   "malformed passwd line returns empty",
			user:   "weird",
			script: "#!/bin/sh\nprintf 'weird:x:1002:1002\\n'\n",
			want:   "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			write(tc.script)
			got := loginShellFor(tc.user)
			if got != tc.want {
				t.Fatalf("loginShellFor(%q) = %q, want %q",
					tc.user, got, tc.want)
			}
		})
	}
}
