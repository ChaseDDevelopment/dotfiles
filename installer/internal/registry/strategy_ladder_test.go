package registry

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

// TestFirstPkgMgrStrategy_LadderOrdering is a table over strategy
// orderings asserting the batch-eligibility rule: a tool qualifies
// for batch install only when its first applicable strategy under
// the active manager is MethodPackageManager. If any non-pkgmgr
// strategy appears first (filtered by AppliesTo), the tool is
// ineligible and FirstPkgMgrStrategy returns nil.
//
// This rule is load-bearing: orchestrator.runBatchedInstall keys its
// bucket partitioning off this function. Returning non-nil when a
// Script or GitHubRelease strategy should have won would silently
// clobber the user's chosen install provenance with apt/brew.
func TestFirstPkgMgrStrategy_LadderOrdering(t *testing.T) {
	scriptStrat := InstallStrategy{
		Method: MethodScript,
		Script: &ScriptConfig{URL: "https://x.invalid/i.sh"},
	}
	ghStrat := InstallStrategy{
		Method: MethodGitHubRelease,
		GitHub: &GitHubConfig{Repo: "a/b"},
	}
	cargoStrat := InstallStrategy{Method: MethodCargo, Crate: "c"}
	customStrat := InstallStrategy{
		Method: MethodCustom,
		CustomFunc: func(context.Context, *InstallContext) error {
			return nil
		},
	}
	aptStrat := InstallStrategy{
		Method:   MethodPackageManager,
		Managers: []string{"apt"},
		Package:  "pkg",
	}
	brewStrat := InstallStrategy{
		Method:   MethodPackageManager,
		Managers: []string{"brew"},
		Package:  "pkg",
	}
	anyMgrStrat := InstallStrategy{
		Method:  MethodPackageManager,
		Package: "pkg",
	}

	cases := []struct {
		name       string
		mgr        string
		strategies []InstallStrategy
		want       *InstallStrategy // nil = expect nil-return
	}{
		{
			name:       "pkgmgr only — eligible",
			mgr:        "apt",
			strategies: []InstallStrategy{aptStrat},
			want:       &aptStrat,
		},
		{
			name:       "pkgmgr-any-manager — eligible",
			mgr:        "brew",
			strategies: []InstallStrategy{anyMgrStrat},
			want:       &anyMgrStrat,
		},
		{
			name:       "script first — ineligible",
			mgr:        "apt",
			strategies: []InstallStrategy{scriptStrat, aptStrat},
			want:       nil,
		},
		{
			name:       "github release first — ineligible",
			mgr:        "apt",
			strategies: []InstallStrategy{ghStrat, aptStrat},
			want:       nil,
		},
		{
			name:       "cargo first — ineligible",
			mgr:        "apt",
			strategies: []InstallStrategy{cargoStrat, aptStrat},
			want:       nil,
		},
		{
			name:       "custom first — ineligible",
			mgr:        "apt",
			strategies: []InstallStrategy{customStrat, aptStrat},
			want:       nil,
		},
		{
			name: "non-applicable pkgmgr filtered, next is pkgmgr — eligible",
			mgr:  "brew",
			// apt strategy doesn't apply under brew — AppliesTo filter
			// skips it without triggering the "non-pkgmgr first" return.
			strategies: []InstallStrategy{aptStrat, brewStrat},
			want:       &brewStrat,
		},
		{
			name: "non-applicable pkgmgr then script — ineligible",
			mgr:  "brew",
			// apt filtered (doesn't apply under brew), then script
			// appears before any applicable pkgmgr → bail out.
			strategies: []InstallStrategy{aptStrat, scriptStrat, brewStrat},
			want:       nil,
		},
		{
			name:       "no matching manager at all — ineligible (nil)",
			mgr:        "dnf",
			strategies: []InstallStrategy{aptStrat, brewStrat},
			want:       nil,
		},
		{
			name:       "empty strategies — nil",
			mgr:        "apt",
			strategies: nil,
			want:       nil,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			tool := &Tool{Name: "t", Strategies: tc.strategies}
			got := FirstPkgMgrStrategy(tool, tc.mgr)
			if tc.want == nil {
				if got != nil {
					t.Fatalf("expected nil, got %+v", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("expected non-nil strategy, got nil")
			}
			if got.Method != tc.want.Method ||
				got.Package != tc.want.Package {
				t.Fatalf("strategy mismatch:\n got  %+v\n want %+v",
					got, tc.want)
			}
		})
	}
}

// recordingCallsPkgMgr counts Install invocations so tests can assert
// pkgmgr was (or was not) called by ExecuteInstallSkippingPkgMgr.
type recordingCallsPkgMgr struct {
	name    string
	calls   int
	lastPkg []string
}

func (r *recordingCallsPkgMgr) Name() string              { return r.name }
func (r *recordingCallsPkgMgr) IsInstalled(_ string) bool { return false }
func (r *recordingCallsPkgMgr) Install(_ context.Context, names ...string) error {
	r.calls++
	r.lastPkg = append([]string(nil), names...)
	return nil
}
func (r *recordingCallsPkgMgr) UpdateAll(context.Context) error { return nil }
func (r *recordingCallsPkgMgr) MapName(n string) []string       { return []string{n} }

// TestExecuteInstallSkippingPkgMgr_NeverInvokesPkgMgr asserts that
// ExecuteInstallSkippingPkgMgr bypasses every MethodPackageManager
// strategy, even when that strategy sorts first in the ladder. This
// is the orchestrator's contract — once a batch install has already
// run for the tool, re-running per-tool pkgmgr would double-install
// successes and re-fail failures identically.
func TestExecuteInstallSkippingPkgMgr_NeverInvokesPkgMgr(t *testing.T) {
	ic := newTestCtx(t)
	pm := &recordingCallsPkgMgr{name: "apt"}
	ic.PkgMgr = pm

	customRan := false
	tool := &Tool{
		Name: "pkgmgr-first",
		Strategies: []InstallStrategy{
			{Method: MethodPackageManager, Package: "x"},
			{
				Method: MethodCustom,
				CustomFunc: func(context.Context, *InstallContext) error {
					customRan = true
					return nil
				},
			},
		},
	}
	if err := ExecuteInstallSkippingPkgMgr(
		context.Background(), tool, ic, ic.Platform,
	); err != nil {
		t.Fatalf("ExecuteInstallSkippingPkgMgr: %v", err)
	}
	if pm.calls != 0 {
		t.Fatalf("pkgmgr Install was called %d time(s) (must be 0)",
			pm.calls)
	}
	if !customRan {
		t.Fatal("fallback custom strategy did not run")
	}
}

// TestExecuteInstallSkippingPkgMgr_LadderOrder asserts the ladder of
// non-pkgmgr strategies runs in declared order: Scripts → GitHub →
// Cargo → Custom. The first one that succeeds wins; earlier failures
// fall through. This locks in the user-preferred strategy preference
// encoded in the registry (pkgmgr > script > github > cargo > custom,
// minus pkgmgr when skipping).
func TestExecuteInstallSkippingPkgMgr_LadderOrder(t *testing.T) {
	ic := newTestCtx(t)
	ic.PkgMgr = &recordingCallsPkgMgr{name: "apt"}

	// Track which strategies ran in what order. We force each
	// strategy to fail except the last so the loop walks the full
	// ladder; the Method constant for each MethodCustom entry is
	// recorded through a side-channel on the closure itself.
	var order []string
	mustFail := func(label string) func(context.Context, *InstallContext) error {
		return func(context.Context, *InstallContext) error {
			order = append(order, label)
			return fmt.Errorf("%s: deliberate fail", label)
		}
	}
	succeed := func(label string) func(context.Context, *InstallContext) error {
		return func(context.Context, *InstallContext) error {
			order = append(order, label)
			return nil
		}
	}

	// Use MethodCustom for every rung so we can both (a) force
	// failure and (b) identify which rung ran. MethodScript/
	// GitHubRelease's real implementations shell out — at the
	// strategy-loop level we only need to prove order.
	tool := &Tool{
		Name: "ladder",
		Strategies: []InstallStrategy{
			{Method: MethodPackageManager, Package: "should-not-run"},
			{Method: MethodCustom, CustomFunc: mustFail("scripts")},
			{Method: MethodCustom, CustomFunc: mustFail("github")},
			{Method: MethodCustom, CustomFunc: mustFail("cargo")},
			{Method: MethodCustom, CustomFunc: succeed("custom")},
		},
	}

	if err := ExecuteInstallSkippingPkgMgr(
		context.Background(), tool, ic, ic.Platform,
	); err != nil {
		t.Fatalf("ExecuteInstallSkippingPkgMgr: %v", err)
	}

	want := []string{"scripts", "github", "cargo", "custom"}
	if len(order) != len(want) {
		t.Fatalf("ran %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("rung %d ran %q, want %q (full order: %v)",
				i, order[i], want[i], order)
		}
	}
}

// TestExecuteInstallSkippingPkgMgr_AllFail surfaces the "all strategies
// failed" error when every non-pkgmgr rung errors. The produced error
// must wrap the last strategy failure so the caller can classify it.
func TestExecuteInstallSkippingPkgMgr_AllFail(t *testing.T) {
	ic := newTestCtx(t)
	ic.PkgMgr = &recordingCallsPkgMgr{name: "apt"}

	sentinel := errors.New("final-rung-failure")
	tool := &Tool{
		Name: "all-fail",
		Strategies: []InstallStrategy{
			{Method: MethodPackageManager, Package: "skipped"},
			{
				Method: MethodCustom,
				CustomFunc: func(context.Context, *InstallContext) error {
					return errors.New("intermediate")
				},
			},
			{
				Method: MethodCustom,
				CustomFunc: func(context.Context, *InstallContext) error {
					return sentinel
				},
			},
		},
	}
	err := ExecuteInstallSkippingPkgMgr(
		context.Background(), tool, ic, ic.Platform,
	)
	if err == nil {
		t.Fatal("expected error when every rung fails")
	}
	if !errors.Is(err, sentinel) {
		t.Fatalf("final rung error not wrapped: %v", err)
	}
}
