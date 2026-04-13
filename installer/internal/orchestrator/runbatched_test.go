package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/chaseddevelopment/dotfiles/installer/internal/executor"
	"github.com/chaseddevelopment/dotfiles/installer/internal/pkgmgr"
	"github.com/chaseddevelopment/dotfiles/installer/internal/platform"
	"github.com/chaseddevelopment/dotfiles/installer/internal/registry"
	"github.com/chaseddevelopment/dotfiles/installer/internal/state"
)

// --- fakes -----------------------------------------------------------------

// routedPkgMgr is a pkgmgr.PackageManager fake focused on exactly the
// three observables runBatchedInstall cares about:
//
//  1. whether Install was called (and with which generics),
//  2. what Install returned (nil | *BatchFailure | ErrAptFatal | raw),
//  3. what IsInstalled(generic) answers (for the total-failure probe
//     that decides despite-error success vs. fall-through).
//
// Kept separate from batchPkgMgr (orchestrator_test.go) so its semantics
// don't drift as more coverage is piled on.
type routedPkgMgr struct {
	mu sync.Mutex

	name string

	installCalls   [][]string
	installCalls32 atomic.Int32 // lock-free concurrent counter for runOnce tests

	// installReturn is the exact error Install should return. Use
	// &pkgmgr.BatchFailure{...} for partial, a wrapped ErrAptFatal for
	// fatal, a plain error for "raw total failure", or nil for clean.
	installReturn error

	// installedLookup answers IsInstalled(generic). Used by the
	// runBatchedInstall total-failure branch to probe whether the tool
	// actually landed despite the batch error.
	installedLookup map[string]bool

	isInstalledQ []string
}

var _ pkgmgr.PackageManager = (*routedPkgMgr)(nil)

func (r *routedPkgMgr) Name() string { return r.name }

func (r *routedPkgMgr) Install(
	_ context.Context, generics ...string,
) error {
	// atomic counter first so concurrent runOnce callers race on
	// this single counter without contending on the mutex.
	r.installCalls32.Add(1)
	r.mu.Lock()
	cp := make([]string, len(generics))
	copy(cp, generics)
	r.installCalls = append(r.installCalls, cp)
	ret := r.installReturn
	r.mu.Unlock()
	return ret
}

func (r *routedPkgMgr) IsInstalled(name string) bool {
	r.mu.Lock()
	r.isInstalledQ = append(r.isInstalledQ, name)
	ok := r.installedLookup[name]
	r.mu.Unlock()
	return ok
}

func (r *routedPkgMgr) UpdateAll(_ context.Context) error { return nil }

func (r *routedPkgMgr) MapName(generic string) []string {
	return []string{generic}
}

// --- helpers ---------------------------------------------------------------

// newRoutingContext builds a BuildConfig + registry.InstallContext
// wired to the given routedPkgMgr. Returns the store so tests can
// assert RecordInstall side-effects.
func newRoutingContext(
	t *testing.T, mgr *routedPkgMgr, dryRun bool,
) (*executor.Runner, *registry.InstallContext, *platform.Platform, *state.Store) {
	t.Helper()
	dir := t.TempDir()
	lf, err := executor.NewLogFile(filepath.Join(dir, "test.log"))
	if err != nil {
		t.Fatalf("NewLogFile: %v", err)
	}
	t.Cleanup(func() { lf.Close() })
	runner := executor.NewRunner(lf, dryRun)
	plat := &platform.Platform{
		OS:             platform.MacOS,
		Arch:           platform.ARM64,
		OSName:         "macOS",
		PackageManager: platform.PkgBrew,
	}
	st := state.NewStore(filepath.Join(dir, "state.json"))
	ic := &registry.InstallContext{
		Runner:   runner,
		PkgMgr:   mgr,
		Platform: plat,
	}
	return runner, ic, plat, st
}

// --- runBatchedInstall table tests -----------------------------------------

// TestRunBatchedInstall exercises the four observable outcomes of
// runBatchedInstall:
//
//	clean        → no fallback, post-install + state record fire
//	partial      → ExecuteInstallSkippingPkgMgr runs for failed-only
//	fatal apt    → no fallback, error propagated with "fatal" phrase
//	raw non-bf   → total-failure probe path; if IsInstalled lies
//	                true, success despite error (no fallback);
//	                if false, fallback runs
//
// Each case uses a tool with a non-pkgmgr fallback strategy (MethodCustom)
// whose closure flips a bool — that flag is the observable signal for
// "did ExecuteInstallSkippingPkgMgr actually route here".
func TestRunBatchedInstall(t *testing.T) {
	type caseCfg struct {
		name string

		// mgr inputs
		installReturn   error
		installedLookup map[string]bool

		// test expectations
		wantInstallCalls     int
		wantFallbackInvoked  bool
		wantStateRecorded    bool
		wantErrSubstr        string // empty = no error expected
	}

	cases := []caseCfg{
		{
			name:                "clean install records state, no fallback",
			installReturn:       nil,
			wantInstallCalls:    1,
			wantFallbackInvoked: false,
			wantStateRecorded:   true,
		},
		{
			name: "partial batch failure routes failed tool to fallback",
			installReturn: &pkgmgr.BatchFailure{
				FailedNames: []string{"fail"},
				Wrapped:     errors.New("partial batch err"),
			},
			wantInstallCalls:    1,
			wantFallbackInvoked: true,
			wantStateRecorded:   true,
		},
		{
			name:                "fatal apt propagates without fallback",
			installReturn:       fmt.Errorf("wrapped: %w", pkgmgr.ErrAptFatal),
			wantInstallCalls:    1,
			wantFallbackInvoked: false,
			wantStateRecorded:   false,
			wantErrSubstr:       "fatal condition",
		},
		{
			name:          "raw total failure falls back when probe says uninstalled",
			installReturn: errors.New("lock held"),
			// installedLookup left empty → IsInstalled("fail") == false
			wantInstallCalls:    1,
			wantFallbackInvoked: true,
			wantStateRecorded:   true,
		},
		{
			name:                "raw total failure skips fallback when probe says installed",
			installReturn:       errors.New("warnings but landed"),
			installedLookup:     map[string]bool{"fail": true},
			wantInstallCalls:    1,
			wantFallbackInvoked: false,
			wantStateRecorded:   true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mgr := &routedPkgMgr{
				name:            "brew",
				installReturn:   tc.installReturn,
				installedLookup: tc.installedLookup,
			}
			_, ic, plat, st := newRoutingContext(t, mgr, true)

			fallbackCalled := false
			tool := &registry.Tool{
				Name:    "fail",
				Command: "fail",
				Strategies: []registry.InstallStrategy{
					{Method: registry.MethodPackageManager, Package: "fail"},
					{
						Method: registry.MethodCustom,
						CustomFunc: func(_ context.Context, _ *registry.InstallContext) error {
							fallbackCalled = true
							return nil
						},
					},
				},
			}
			entry := &batchEntry{
				tool: *tool,
				strategy: &registry.InstallStrategy{
					Method:  registry.MethodPackageManager,
					Package: "fail",
				},
				genericPkg: "fail",
			}
			bs := &batchState{}

			err := runBatchedInstall(
				context.Background(), tool, entry, ic, plat, bs,
				[]string{"fail"}, st,
			)

			// Error expectation.
			if tc.wantErrSubstr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			} else {
				if err == nil {
					t.Fatalf("want error containing %q, got nil",
						tc.wantErrSubstr)
				}
				if !strings.Contains(err.Error(), tc.wantErrSubstr) {
					t.Fatalf(
						"err = %q, want substring %q",
						err.Error(), tc.wantErrSubstr,
					)
				}
			}

			// Install call count.
			if got := int(mgr.installCalls32.Load()); got != tc.wantInstallCalls {
				t.Errorf(
					"mgr.Install called %d times, want %d",
					got, tc.wantInstallCalls,
				)
			}

			// Fallback observable.
			if fallbackCalled != tc.wantFallbackInvoked {
				t.Errorf(
					"fallback called = %v, want %v "+
						"(ExecuteInstallSkippingPkgMgr routing)",
					fallbackCalled, tc.wantFallbackInvoked,
				)
			}

			// State-record observable.
			_, recorded := st.LookupTool("fail")
			if recorded != tc.wantStateRecorded {
				t.Errorf(
					"state recorded = %v, want %v",
					recorded, tc.wantStateRecorded,
				)
			}
		})
	}
}

// TestRunBatchedInstallCleanSkipsFallback asserts runBatchedInstall
// for a tool whose batch cleanly succeeded does NOT invoke
// ExecuteInstallSkippingPkgMgr. Done via a fallback strategy whose
// CustomFunc would flip a flag; the flag must stay false.
//
// This is the symmetric pair to the "partial batch routes to
// fallback" row in the table: proving runBatchedInstall's
// failedThisTool branch stays false on the happy path.
func TestRunBatchedInstallCleanSkipsFallback(t *testing.T) {
	mgr := &routedPkgMgr{name: "brew"}
	_, ic, plat, st := newRoutingContext(t, mgr, true)

	fallbackCalled := false
	tool := &registry.Tool{
		Name:    "ok",
		Command: "ok",
		Strategies: []registry.InstallStrategy{
			{Method: registry.MethodPackageManager, Package: "ok"},
			{
				Method: registry.MethodCustom,
				CustomFunc: func(_ context.Context, _ *registry.InstallContext) error {
					fallbackCalled = true
					return nil
				},
			},
		},
	}
	entry := &batchEntry{
		tool: *tool,
		strategy: &registry.InstallStrategy{
			Method:  registry.MethodPackageManager,
			Package: "ok",
		},
		genericPkg: "ok",
	}
	bs := &batchState{}

	if err := runBatchedInstall(
		context.Background(), tool, entry, ic, plat, bs,
		[]string{"ok"}, st,
	); err != nil {
		t.Fatalf("runBatchedInstall: %v", err)
	}
	if fallbackCalled {
		t.Fatal("fallback invoked on clean batch success; " +
			"failedThisTool routing regressed")
	}
	if _, ok := st.LookupTool("ok"); !ok {
		t.Fatal("state.RecordInstall did not fire on clean success")
	}
}

// --- batchState.runOnce tests ----------------------------------------------

// TestBatchStateRunOnceOnlyRunsOnce fires N concurrent callers at
// batchState.runOnce and asserts mgr.Install was invoked exactly once.
// Regressing the sync.Once wrapper (e.g. replacing with naive "if not
// ran" flag) would surface here as >1 Install calls under -race.
func TestBatchStateRunOnceOnlyRunsOnce(t *testing.T) {
	mgr := &routedPkgMgr{name: "brew", installReturn: nil}
	var bs batchState

	const N = 32
	var wg sync.WaitGroup
	wg.Add(N)
	start := make(chan struct{})
	for i := 0; i < N; i++ {
		go func() {
			defer wg.Done()
			<-start
			bs.runOnce(context.Background(), mgr, []string{"a", "b", "c"})
		}()
	}
	close(start)
	wg.Wait()

	if got := int(mgr.installCalls32.Load()); got != 1 {
		t.Fatalf(
			"mgr.Install called %d times under %d concurrent "+
				"runOnce callers, want exactly 1",
			got, N,
		)
	}
	if bs.err != nil {
		t.Fatalf("unexpected err on clean path: %v", bs.err)
	}
}

// TestBatchStateRunOncePartialFailurePopulatesMap asserts that on a
// *pkgmgr.BatchFailure, runOnce populates failedGenerics with the
// exact set of failed names — the orchestrator relies on this for
// per-tool fallthrough routing.
func TestBatchStateRunOncePartialFailurePopulatesMap(t *testing.T) {
	mgr := &routedPkgMgr{
		name: "apt",
		installReturn: &pkgmgr.BatchFailure{
			FailedNames: []string{"b", "d"},
			Wrapped:     errors.New("two failed"),
		},
	}
	var bs batchState
	bs.runOnce(context.Background(), mgr, []string{"a", "b", "c", "d"})

	if bs.err == nil {
		t.Fatal("expected bs.err to hold the BatchFailure")
	}
	var bf *pkgmgr.BatchFailure
	if !errors.As(bs.err, &bf) {
		t.Fatalf("bs.err = %v, expected *pkgmgr.BatchFailure", bs.err)
	}
	if bs.failedGenerics == nil {
		t.Fatal("failedGenerics == nil for partial failure " +
			"(reserved for total-failure sentinel); regression")
	}
	for _, want := range []string{"b", "d"} {
		if _, ok := bs.failedGenerics[want]; !ok {
			t.Errorf("failedGenerics missing %q", want)
		}
	}
	for _, notWant := range []string{"a", "c"} {
		if _, ok := bs.failedGenerics[notWant]; ok {
			t.Errorf(
				"failedGenerics unexpectedly contains %q "+
					"(should be successful set)",
				notWant,
			)
		}
	}
}

// TestBatchStateRunOnceTotalFailureNilSentinel asserts that a raw
// error (not a BatchFailure) leaves failedGenerics as nil. This is
// the documented sentinel used by runBatchedInstall's default-branch
// IsInstalled probe to distinguish total-failure-with-maybe-landed
// from a partial batch.
func TestBatchStateRunOnceTotalFailureNilSentinel(t *testing.T) {
	mgr := &routedPkgMgr{
		name:          "apt",
		installReturn: errors.New("connection refused"),
	}
	var bs batchState
	bs.runOnce(context.Background(), mgr, []string{"x", "y"})

	if bs.err == nil {
		t.Fatal("expected bs.err set for raw error")
	}
	if bs.failedGenerics != nil {
		t.Fatalf(
			"failedGenerics = %#v, want nil sentinel for "+
				"total-failure (non-BatchFailure) path",
			bs.failedGenerics,
		)
	}
}

// TestBatchStateRunOnceEmptyGenericsNoOp asserts runOnce does nothing
// when the generics list is empty — the batch would invoke an empty
// `apt install` otherwise, which apt treats as an error.
func TestBatchStateRunOnceEmptyGenericsNoOp(t *testing.T) {
	mgr := &routedPkgMgr{name: "apt"}
	var bs batchState
	bs.runOnce(context.Background(), mgr, nil)

	if got := int(mgr.installCalls32.Load()); got != 0 {
		t.Fatalf(
			"mgr.Install called %d times for empty generics, "+
				"want 0",
			got,
		)
	}
	if bs.err != nil {
		t.Fatalf("unexpected err on empty: %v", bs.err)
	}
}

