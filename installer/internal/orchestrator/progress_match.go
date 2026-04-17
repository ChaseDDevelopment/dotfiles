package orchestrator

import (
	"context"
	"regexp"
	"strings"
	"sync"

	"github.com/chaseddevelopment/dotfiles/installer/internal/engine"
)

// progressMatcher extracts the package name from a per-manager
// progress line. It is called for EVERY stdout/stderr line during
// a batch install — noisy lines must return ok=false. Returned
// names are manager-native (e.g. brew formula names, apt package
// names); callers translate back to engine task IDs.
type progressMatcher interface {
	Match(line string) (name string, ok bool)
}

// progressMatchers maps pkgmgr.PackageManager.Name() → matcher.
// Empty matchers (nil value) mean "no per-item progress parsing
// available for this manager" and the caller falls back to the
// batch-granularity behavior from Part A.
var progressMatchers = map[string]progressMatcher{
	"brew": brewMatcher{},
	"apt":  aptMatcher{},
	"nala": aptMatcher{}, // nala passes through apt's "Setting up" lines
}

// brewPouringRe matches brew's per-formula completion marker.
// Example: "==> Pouring ripgrep--14.1.0.arm64_sonoma.bottle.tar.gz"
// (bottle install) and "==> Pouring ripgrep--14.1.0" (source build).
// Brew emits this line once per formula right after the bottle is
// unpacked into the cellar, which is the most reliable per-formula
// "done" signal; `==> Installing` fires earlier and can be printed
// for dependencies brew resolved transitively that aren't in our
// bucket. We pin to two leading equals-arrows to avoid matching
// quoted strings inside other brew diagnostics.
var brewPouringRe = regexp.MustCompile(
	`^==> Pouring ([A-Za-z0-9][A-Za-z0-9._+-]*)(?:--|\s|$)`,
)

type brewMatcher struct{}

func (brewMatcher) Match(line string) (string, bool) {
	m := brewPouringRe.FindStringSubmatch(line)
	if m == nil {
		return "", false
	}
	return m[1], true
}

// aptSettingUpRe matches dpkg's per-package completion marker.
// Examples:
//
//	"Setting up ripgrep (14.1.0-1) ..."
//	"Setting up ripgrep:amd64 (14.1.0-1) ..."
//
// dpkg emits this line once per package after unpack + trigger
// processing — the point at which the package is usable. We
// deliberately ignore `Unpacking`, `Preparing to unpack`, and
// `Processing triggers for`: those fire multiple times per package
// and for dependencies outside the bucket.
var aptSettingUpRe = regexp.MustCompile(
	`^Setting up ([A-Za-z0-9][A-Za-z0-9.+-]*)(?::[A-Za-z0-9]+)?\s*\(`,
)

type aptMatcher struct{}

func (aptMatcher) Match(line string) (string, bool) {
	m := aptSettingUpRe.FindStringSubmatch(strings.TrimSpace(line))
	if m == nil {
		return "", false
	}
	return m[1], true
}

// buildBatchProgressTap returns a stdout-line callback that emits a
// BatchProgressMsg each time the manager's completion marker fires
// for a package that maps to a bucket task. Returns nil when there
// is no matcher for the manager (non-brew/apt/nala) so the Runner
// skips tap installation entirely and pays no per-line overhead.
//
// nameToTaskID is the post-MapName reverse index built by the
// orchestrator; labelByTaskID carries the user-facing tool name for
// BatchProgressMsg.Label. Each task ID is emitted at most once per
// batch — duplicate matches (rare but possible, e.g. dpkg "Setting
// up" followed by a trigger rerun on the same package) are swallowed
// locally rather than relying on the TUI's markDone idempotency.
func buildBatchProgressTap(
	ctx context.Context,
	matcher progressMatcher,
	nameToTaskID map[string]string,
	labelByTaskID map[string]string,
) func(line string) {
	if matcher == nil || len(nameToTaskID) == 0 {
		return nil
	}
	emit := engine.EmitFromContext(ctx)
	var mu sync.Mutex
	seen := make(map[string]bool, len(nameToTaskID))
	return func(line string) {
		name, ok := matcher.Match(line)
		if !ok {
			return
		}
		taskID, ok := nameToTaskID[name]
		if !ok {
			return
		}
		mu.Lock()
		if seen[taskID] {
			mu.Unlock()
			return
		}
		seen[taskID] = true
		mu.Unlock()
		emit(engine.BatchProgressMsg{
			ID:    taskID,
			Label: labelByTaskID[taskID],
			Phase: "done",
		})
	}
}

