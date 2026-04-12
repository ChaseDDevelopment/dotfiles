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
	label   string
	action  string // "install", "configure", "cleanup"
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
	width int
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

// markActive is called when an engine task starts running.
func (m *progressModel) markActive(id, label string) {
	name := stripLabelPrefix(label)
	m.labelByID[id] = name
	if _, exists := m.toolStatuses[name]; !exists {
		m.toolNames = append(m.toolNames, name)
		m.totalTools = len(m.toolNames)
	}
	m.toolStatuses[name] = statusActive
	m.active = append(m.active, activeTask{id: id, label: label})
}

// markDone is called when an engine task finishes.
func (m *progressModel) markDone(id string, err error) {
	name := m.nameForID(id)
	action := "install"
	if strings.HasPrefix(id, "setup-") {
		action = "configure"
	} else if strings.HasPrefix(id, "cleanup-") {
		action = "cleanup"
	}

	if err != nil {
		m.toolStatuses[name] = statusFailed
		m.steps = append(m.steps, stepResult{
			label: name, action: action,
			status: "failed", success: false, err: err,
		})
	} else {
		m.toolStatuses[name] = statusDone
		status := "installed"
		if action == "configure" {
			status = "configured"
		} else if action == "cleanup" {
			status = "cleaned"
		}
		m.steps = append(m.steps, stepResult{
			label: name, action: action,
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
		label:   name,
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
		m.resizeVerboseViewport()
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
	m.verboseViewport.SetHeight(12)
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

	// Tool grid (3 columns).
	if len(m.toolNames) > 0 {
		cols := 3
		if w < 60 {
			cols = 2
		}
		colWidth := (w - 4) / cols

		for i, name := range m.toolNames {
			status := m.toolStatuses[name]
			icon := m.statusIcon(status)
			label := name
			if len(label) > colWidth-5 {
				label = label[:colWidth-6] + "…"
			}

			pad := dimStyle.Render(fmt.Sprintf("%-*s", colWidth-4, label))
			cell := fmt.Sprintf("%s %s", icon, pad)
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
	m.progress.SetWidth(w - 14)
	bar := m.progress.ViewAs(pct)
	b.WriteString(bar + counter + elapsed + panelGap("\n\n"))

	// Active tasks.
	if len(m.active) > 0 {
		for _, at := range m.active {
			name := stripLabelPrefix(at.label)
			b.WriteString(panelGap("  ") + m.spinner.View() + panelGap(" ") + selectedStyle.Render(name) + panelGap("\n"))
		}
		// Verbose: show recent output lines.
		if m.verbose && len(m.recentLines) > 0 {
			b.WriteString(panelGap("\n"))
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
