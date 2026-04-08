package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Messages for the progress model.
type stepStartMsg struct{ label string }
type stepDoneMsg struct {
	label   string
	success bool
	err     error
}
type allDoneMsg struct{}

// toolStatus tracks the state of a single tool in the grid.
type toolStatus int

const (
	statusQueued toolStatus = iota
	statusActive
	statusDone
	statusFailed
)

// stepResult tracks the outcome of one install step.
type stepResult struct {
	label   string
	success bool
	err     error
}

// progressModel shows the install dashboard.
type progressModel struct {
	spinner  spinner.Model
	progress progress.Model
	steps    []stepResult
	current  string
	done     bool
	verbose  bool

	// Grid tracking.
	toolNames    []string
	toolStatuses map[string]toolStatus
	totalTools   int
	doneCount    int

	// Verbose output lines (read from Runner.RecentLines).
	recentLines []string
}

func newProgressModel() progressModel {
	s := spinner.New()
	s.Spinner = spinner.Pulse
	s.Style = progressActiveStyle

	p := progress.New(
		progress.WithGradient("#cba6f7", "#74c7ec"),
		progress.WithoutPercentage(),
	)

	return progressModel{
		spinner:      s,
		progress:     p,
		toolStatuses: make(map[string]toolStatus),
	}
}

func (m progressModel) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m progressModel) Update(msg tea.Msg) (progressModel, tea.Cmd) {
	switch msg := msg.(type) {
	case stepStartMsg:
		m.current = msg.label
		// Extract tool name from "Installing xyz" labels.
		name := msg.label
		if strings.HasPrefix(name, "Installing ") {
			name = strings.TrimPrefix(name, "Installing ")
		} else if strings.HasPrefix(name, "Setting up ") {
			name = strings.TrimPrefix(name, "Setting up ")
		} else if strings.HasPrefix(name, "Updating ") {
			name = strings.TrimPrefix(name, "Updating ")
		}
		if _, exists := m.toolStatuses[name]; !exists {
			m.toolNames = append(m.toolNames, name)
			m.totalTools = len(m.toolNames)
		}
		m.toolStatuses[name] = statusActive
		return m, nil

	case stepDoneMsg:
		m.steps = append(m.steps, stepResult{
			label:   msg.label,
			success: msg.success,
			err:     msg.err,
		})
		// Update tool status.
		name := msg.label
		if strings.HasPrefix(name, "Installing ") {
			name = strings.TrimPrefix(name, "Installing ")
		} else if strings.HasPrefix(name, "Setting up ") {
			name = strings.TrimPrefix(name, "Setting up ")
		} else if strings.HasPrefix(name, "Updating ") {
			name = strings.TrimPrefix(name, "Updating ")
		}
		if msg.success {
			m.toolStatuses[name] = statusDone
		} else {
			m.toolStatuses[name] = statusFailed
		}
		m.doneCount++
		m.current = ""
		return m, nil

	case allDoneMsg:
		m.done = true
		m.current = ""
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case progress.FrameMsg:
		mdl, cmd := m.progress.Update(msg)
		m.progress = mdl.(progress.Model)
		return m, cmd
	}
	return m, nil
}

func (m progressModel) View(width int) string {
	w := contentWidth(width)
	var b strings.Builder

	// Panel title inside content.
	b.WriteString(titleStyle.Render("  Installing"))
	b.WriteString("\n\n")

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

			cell := fmt.Sprintf("%s %-*s", icon, colWidth-4, label)
			b.WriteString(cell)

			if (i+1)%cols == 0 || i == len(m.toolNames)-1 {
				b.WriteString("\n")
			}
		}
		b.WriteString("\n")
	}

	// Progress bar.
	pct := 0.0
	if m.totalTools > 0 {
		pct = float64(m.doneCount) / float64(m.totalTools)
	}
	m.progress.Width = w - 14
	bar := m.progress.ViewAs(pct)
	pctStr := dimStyle.Render(fmt.Sprintf(" %d%%", int(pct*100)))
	b.WriteString(bar + pctStr + "\n\n")

	// Current task.
	if m.current != "" {
		b.WriteString(fmt.Sprintf("  %s %s\n", m.spinner.View(), selectedStyle.Render(m.current)))
		// Verbose: show recent output lines.
		if m.verbose && len(m.recentLines) > 0 {
			b.WriteString("\n")
			for _, line := range m.recentLines {
				truncated := line
				if len(truncated) > w-8 {
					truncated = truncated[:w-9] + "…"
				}
				b.WriteString("  " + dimStyle.Render(truncated) + "\n")
			}
		}
	} else if m.done {
		b.WriteString(successStyle.Render("  ✦ All steps completed!") + "\n")
	}

	content := b.String()

	panel := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(catMauve).
		Background(catSurface0).
		Padding(1, 2).
		Width(w).
		Render(content)

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
	default:
		return progressQueuedStyle.Render("○")
	}
}
