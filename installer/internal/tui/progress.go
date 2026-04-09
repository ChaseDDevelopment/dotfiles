package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/progress"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

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

	// Verbose output lines (read from Runner.RecentLines).
	recentLines []string
}

func newProgressModel() progressModel {
	s := spinner.New()
	s.Spinner = spinner.Pulse
	s.Style = progressActiveStyle

	p := progress.New(
		progress.WithColors(
			lipgloss.Color("#cba6f7"),
			lipgloss.Color("#74c7ec"),
		),
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
	if err != nil {
		m.toolStatuses[name] = statusFailed
		m.steps = append(m.steps, stepResult{label: name, success: false, err: err})
	} else {
		m.toolStatuses[name] = statusDone
		m.steps = append(m.steps, stepResult{label: name, success: true})
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

	// Progress bar.
	pct := 0.0
	if m.totalTools > 0 {
		pct = float64(m.doneCount) / float64(m.totalTools)
	}
	m.progress.SetWidth(w - 14)
	bar := m.progress.ViewAs(pct)
	pctStr := dimStyle.Render(fmt.Sprintf(" %d%%", int(pct*100)))
	b.WriteString(bar + pctStr + panelGap("\n\n"))

	// Active tasks.
	if len(m.active) > 0 {
		for _, at := range m.active {
			name := stripLabelPrefix(at.label)
			b.WriteString(panelGap("  ") + m.spinner.View() + panelGap(" ") + selectedStyle.Render(name) + panelGap("\n"))
		}
		// Verbose: show recent output lines.
		if m.verbose && len(m.recentLines) > 0 {
			b.WriteString(panelGap("\n"))
			for _, line := range m.recentLines {
				truncated := line
				if len(truncated) > w-8 {
					truncated = truncated[:w-9] + "…"
				}
				b.WriteString(panelGap("  ") + dimStyle.Render(truncated) + panelGap("\n"))
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
