package registry

import "testing"

func TestHasApplicableStrategy(t *testing.T) {
	aptOnly := Tool{
		Name: "nala", Command: "nala",
		Strategies: []InstallStrategy{
			{Managers: []string{"apt"}, Method: MethodPackageManager, Package: "nala"},
		},
	}
	withCargoFallback := Tool{
		Name: "tree-sitter-cli", Command: "tree-sitter",
		Strategies: []InstallStrategy{
			{Managers: []string{"apt"}, Method: MethodPackageManager, Package: "tree-sitter-cli"},
			{Method: MethodCargo, Crate: "tree-sitter-cli"},
		},
	}
	noStrategies := Tool{Name: "configonly", Command: "configonly"}

	cases := []struct {
		name string
		tool Tool
		mgr  string
		want bool
	}{
		{"apt-only on pacman is excluded", aptOnly, "pacman", false},
		{"apt-only on apt applies", aptOnly, "apt", true},
		{"cargo fallback applies on pacman", withCargoFallback, "pacman", true},
		{"no strategies is always applicable", noStrategies, "pacman", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tool := c.tool
			if got := HasApplicableStrategy(&tool, c.mgr); got != c.want {
				t.Errorf("HasApplicableStrategy(%s, %q) = %v, want %v",
					c.tool.Name, c.mgr, got, c.want)
			}
		})
	}
}
