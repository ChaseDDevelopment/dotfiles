package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/progress"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
)

const compactVerboseLines = 8

// toolStatus tracks the state of a single tool in the grid.
type toolStatus int

const (
	statusQueued toolStatus = iota
	statusActive
	statusDone
	statusFailed
	statusSkipped
)

// stepResult tracks the outcome of one install step.
type stepResult struct {
	id      string // engine task ID, used for special-case filtering
	label   string
	action  string // "install", "configure", "cleanup", "sweep"
	status  string // "installed", "configured", "no changes", "failed"
	success bool
	err     error
}

// activeTask tracks a running task by ID and display label.
type activeTask struct {
	id    string
	label string
}

// progressModel shows the install dashboard.
type progressModel struct {
	spinner  spinner.Model
	progress progress.Model
	steps    []stepResult
	active   []activeTask // currently running tasks
	done     bool
	verbose  bool

	// Grid tracking.
	toolNames    []string
	toolStatuses map[string]toolStatus
	totalTools   int
	doneCount    int

	// Label lookup by task ID.
	labelByID map[string]string

	// Timing for elapsed display.
	startedAt time.Time

	// Verbose output lines (read from Runner.RecentLines).
	recentLines []string

	// Expanded verbose viewport (toggled with 'v').
	expandedVerbose bool
	verboseViewport viewport.Model

	// width is captured from tea.WindowSizeMsg so viewport sizing
	// can happen in Update instead of View (which would re-paginate
	// on every frame and break user scroll state).
	width  int
	height int
}

// allFinished reports whether every queued tool has reached a
// terminal status (done, failed, or skipped).
func (m *progressModel) allFinished() bool {
	if m.totalTools == 0 {
		return false
	}
	for _, s := range m.toolStatuses {
		if s == statusQueued || s == statusActive {
			return false
		}
	}
	return true
}

func newProgressModel() progressModel {
	s := spinner.New()
	s.Spinner = spinner.Pulse
	s.Style = progressActiveStyle

	p := progress.New(
		progress.WithColors(catMauve, catSapphire),
		progress.WithoutPercentage(),
	)

	return progressModel{
		spinner:      s,
		progress:     p,
		toolStatuses: make(map[string]toolStatus),
		labelByID:    make(map[string]string),
	}
}

func (m progressModel) Init() tea.Cmd {
	return m.spinner.Tick
}

// markActive is called when an engine task starts running. It is
// idempotent: (1) re-activating an already-active tool is a no-op
// so the active list never contains duplicate entries for one ID;
// (2) a tool that already reached a terminal status (done/failed)
// is NOT downgraded back to active — this matters for the batch
// fanout path in Part B, where a brew `==> Pouring <name>` can flip
// a tool to done before its own engine task eventually fires its
// TaskStartedMsg (which would otherwise reset the grid entry).
func (m *progressModel) markActive(id, label string) {
	name := stripLabelPrefix(label)
	m.labelByID[id] = name
	if _, exists := m.toolStatuses[name]; !exists {
		m.toolNames = append(m.toolNames, name)
		m.totalTools = len(m.toolNames)
	}
	switch m.toolStatuses[name] {
	case statusDone, statusFailed, statusSkipped:
		return
	}
	m.toolStatuses[name] = statusActive
	for _, a := range m.active {
		if a.id == id {
			return
		}
	}
	m.active = append(m.active, activeTask{id: id, label: label})
}

// markDone is called when an engine task finishes. It is idempotent:
// a second call for a tool already in a terminal state is a no-op,
// which prevents the summary screen from double-counting when a
// brew `==> Pouring` progress event and the peer task's own
// TaskDoneMsg both report completion for the same tool.
func (m *progressModel) markDone(id string, err error) {
	name := m.nameForID(id)
	switch m.toolStatuses[name] {
	case statusDone, statusFailed, statusSkipped:
		m.removeActive(id)
		return
	}
	action := "install"
	switch {
	case strings.HasPrefix(id, "setup-"):
		action = "configure"
	case strings.HasPrefix(id, "cleanup-"):
		action = "cleanup"
	case strings.HasPrefix(id, "sweep-"):
		// Drift sweeps and similar housekeeping tasks are not installs.
		// Classifying them as "install" would inflate the "N installed"
		// count on the summary screen after a no-op run.
		action = "sweep"
	}

	if err != nil {
		m.toolStatuses[name] = statusFailed
		m.steps = append(m.steps, stepResult{
			id: id, label: name, action: action,
			status: "failed", success: false, err: err,
		})
	} else {
		m.toolStatuses[name] = statusDone
		status := "installed"
		switch action {
		case "configure":
			status = "configured"
		case "cleanup":
			status = "cleaned"
		case "sweep":
			status = "swept"
		}
		m.steps = append(m.steps, stepResult{
			id: id, label: name, action: action,
			status: status, success: true,
		})
	}
	m.doneCount++
	m.removeActive(id)
}

// markSkipped is called when an engine task is skipped.
func (m *progressModel) markSkipped(id, label, reason string) {
	name := stripLabelPrefix(label)
	if name == "" {
		name = id
	}
	m.labelByID[id] = name
	if _, exists := m.toolStatuses[name]; !exists {
		m.toolNames = append(m.toolNames, name)
		m.totalTools = len(m.toolNames)
	}
	m.toolStatuses[name] = statusSkipped
	m.steps = append(m.steps, stepResult{
		id:      id,
		label:   name,
		action:  "skipped",
		status:  reason,
		success: false,
		err:     fmt.Errorf("skipped: %s", reason),
	})
	m.doneCount++
}

func (m *progressModel) removeActive(id string) {
	var filtered []activeTask
	for _, a := range m.active {
		if a.id != id {
			filtered = append(filtered, a)
		}
	}
	m.active = filtered
}

// nameForID returns the display name for a task ID.
func (m *progressModel) nameForID(id string) string {
	if name, ok := m.labelByID[id]; ok {
		return name
	}
	return id
}

func stripLabelPrefix(label string) string {
	for _, prefix := range []string{"Installing ", "Setting up ", "Updating "} {
		if strings.HasPrefix(label, prefix) {
			return strings.TrimPrefix(label, prefix)
		}
	}
	return label
}

func (m progressModel) Update(msg tea.Msg) (progressModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resizeVerboseViewport()
		// Size the progress bar here once per resize instead of in
		// View() per frame. The old path mutated through a value
		// receiver anyway so the set was wasted work on every tick.
		contentW := contentWidth(m.width)
		if contentW > 14 {
			m.progress.SetWidth(contentW - 14)
		}
		return m, nil

	case tea.KeyPressMsg:
		if msg.String() == "v" && m.verbose {
			m.expandedVerbose = !m.expandedVerbose
			if m.expandedVerbose {
				m.syncVerboseViewport()
			}
			return m, nil
		}
		// Forward scroll keys to viewport when expanded.
		if m.expandedVerbose {
			var cmd tea.Cmd
			m.verboseViewport, cmd = m.verboseViewport.Update(msg)
			return m, cmd
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case progress.FrameMsg:
		var cmd tea.Cmd
		m.progress, cmd = m.progress.Update(msg)
		return m, cmd
	}
	return m, nil
}

// resizeVerboseViewport updates viewport dimensions in response to
// a WindowSizeMsg. Kept separate from content updates so resize
// doesn't clobber scroll position when the user hasn't added lines.
func (m *progressModel) resizeVerboseViewport() {
	if m.width <= 0 {
		return
	}
	w := contentWidth(m.width)
	m.verboseViewport.SetWidth(w - 4)
	// Derive height from the terminal so short windows don't overflow.
	// The panel above this viewport consumes roughly 14 lines (header,
	// grid, progress bar, active-task list); leave breathing room for
	// the footer and clamp to a sane minimum.
	h := 12
	if m.height > 0 {
		h = m.height - 14
		if h < 4 {
			h = 4
		}
		if h > 24 {
			h = 24
		}
	}
	m.verboseViewport.SetHeight(h)
}

// syncVerboseViewport refreshes the expanded verbose viewport's
// content and jumps to the bottom. Called when recentLines changes
// or when the user toggles into expanded mode. Kept out of View so
// repeated View() calls don't re-paginate and fight user scrolling.
func (m *progressModel) syncVerboseViewport() {
	if !m.expandedVerbose || m.width <= 0 {
		return
	}
	w := contentWidth(m.width)
	var content strings.Builder
	for _, line := range m.recentLines {
		truncated := line
		if len(truncated) > w-8 {
			truncated = truncated[:w-9] + "…"
		}
		content.WriteString("  " + truncated + "\n")
	}
	m.verboseViewport.SetContent(content.String())
	m.verboseViewport.GotoBottom()
}

func (m progressModel) View(width int) string {
	w := contentWidth(width)
	var b strings.Builder

	// Panel title inside content.
	b.WriteString(titleStyle.Render("  Installing"))
	b.WriteString(panelGap("\n\n"))

	// Tool grid (2 columns). Active items animate via spinner so a
	// separate "running tasks" block below the grid isn't needed.
	if len(m.toolNames) > 0 {
		cols := 2
		if w < 50 {
			cols = 1
		}
		colWidth := (w - 4) / cols
		const iconWidth = 2 // icon + following space
		labelWidth := colWidth - iconWidth - 2

		for i, name := range m.toolNames {
			status := m.toolStatuses[name]
			icon := m.statusIconFor(status)
			label := name
			if len(label) > labelWidth {
				label = label[:labelWidth-1] + "…"
			}

			labelStyle := dimStyle
			if status == statusActive {
				labelStyle = selectedStyle
			}
			pad := labelStyle.Render(fmt.Sprintf("%-*s", labelWidth, label))
			cell := fmt.Sprintf("  %s %s", icon, pad)
			b.WriteString(cell)

			if (i+1)%cols == 0 || i == len(m.toolNames)-1 {
				b.WriteString(panelGap("\n"))
			}
		}
		b.WriteString(panelGap("\n"))
	}

	// Progress bar with counter and elapsed time.
	pct := 0.0
	if m.totalTools > 0 {
		pct = float64(m.doneCount) / float64(m.totalTools)
	}
	counter := dimStyle.Render(fmt.Sprintf(
		" %d/%d", m.doneCount, m.totalTools,
	))
	elapsed := ""
	if !m.startedAt.IsZero() {
		d := time.Since(m.startedAt).Truncate(time.Second)
		elapsed = dimStyle.Render(fmt.Sprintf(" %s", d))
	}
	// Width is now set in Update on WindowSizeMsg (progress_ticks
	// fire ~20×/sec and it's wasteful to re-set every frame).
	bar := m.progress.ViewAs(pct)
	b.WriteString(bar + counter + elapsed + panelGap("\n\n"))

	// Verbose log (optional, toggled with 'v').
	if len(m.active) > 0 && m.verbose && len(m.recentLines) > 0 {
		if m.expandedVerbose {
			// Viewport content + sizing is maintained in Update
			// via syncVerboseViewport / resizeVerboseViewport.
			b.WriteString(dimStyle.Render(m.verboseViewport.View()) + panelGap("\n"))
			b.WriteString(panelGap("  ") + dimStyle.Render("v: collapse  j/k: scroll") + panelGap("\n"))
		} else {
			// Compact: show last N lines.
			lines := m.recentLines
			if len(lines) > compactVerboseLines {
				lines = lines[len(lines)-compactVerboseLines:]
			}
			for _, line := range lines {
				truncated := line
				if len(truncated) > w-8 {
					truncated = truncated[:w-9] + "…"
				}
				b.WriteString(panelGap("  ") + dimStyle.Render(truncated) + panelGap("\n"))
			}
			b.WriteString(panelGap("  ") + dimStyle.Render("v: expand log") + panelGap("\n"))
		}
	} else if m.done {
		b.WriteString(successStyle.Render("  ✦ All steps completed!") + panelGap("\n"))
	}

	content := b.String()

	panel := panelStyle.Width(w).Render(content)

	return panel
}

func (m progressModel) statusIcon(s toolStatus) string {
	switch s {
	case statusDone:
		return progressDoneStyle.Render("✓")
	case statusActive:
		return progressActiveStyle.Render("●")
	case statusFailed:
		return progressFailedStyle.Render("✗")
	case statusSkipped:
		return dimStyle.Render("⊘")
	default:
		return progressQueuedStyle.Render("○")
	}
}

// statusIconFor is like statusIcon but animates the active state
// using the shared spinner, so in-progress rows in the grid pulse
// instead of showing a static dot.
func (m progressModel) statusIconFor(s toolStatus) string {
	if s == statusActive {
		return m.spinner.View()
	}
	return m.statusIcon(s)
}
